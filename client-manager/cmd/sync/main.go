// cmd/sync/main.go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var DB *sql.DB

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal("Error loading .env file")
	}

	var err error
	DB, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer DB.Close()

	log.Println("Starting page sync...")
	if err := syncPage(); err != nil {
		log.Printf("Error syncing page: %v", err)
		os.Exit(1)
	}
	log.Println("Page sync completed")
}

type FacebookPage struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func getPageInfo(pageToken string) (*FacebookPage, error) {
	// Get page info using the token
	url := fmt.Sprintf("https://graph.facebook.com/v23.0/me?access_token=%s", pageToken)
	log.Printf("Fetching page info from Facebook...")

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	log.Printf("Response status: %s", resp.Status)
	log.Printf("Response body: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Facebook API returned status: %s", resp.Status)
	}

	var page FacebookPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("error parsing page info: %w", err)
	}

	return &page, nil
}

func syncPage() error {
	pageToken := os.Getenv("PAGE_TOKEN")
	if pageToken == "" {
		return fmt.Errorf("PAGE_TOKEN not set")
	}

	// Get page info
	page, err := getPageInfo(pageToken)
	if err != nil {
		return err
	}

	log.Printf("Got page info: ID=%s, Name=%s", page.ID, page.Name)

	// Insert or update page in database
	_, err = DB.Exec(`
        INSERT INTO pages (page_id, name, access_token, status, platform)
        VALUES ($1, $2, $3, 'pending', 'facebook')
        ON CONFLICT (platform, page_id) 
        DO UPDATE SET 
            name = EXCLUDED.name,
            access_token = EXCLUDED.access_token
            WHERE pages.status != 'disabled'
    `, page.ID, page.Name, pageToken)

	if err != nil {
		return fmt.Errorf("error storing page: %w", err)
	}

	log.Printf("Successfully synced page: %s", page.Name)
	return nil
}
