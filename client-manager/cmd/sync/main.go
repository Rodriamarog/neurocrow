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
	// Load env
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Init DB
	var err error
	DB, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer DB.Close()

	log.Println("Starting page sync...")
	if err := syncPages(); err != nil {
		log.Printf("Error syncing pages: %v", err)
		os.Exit(1)
	}
	log.Println("Page sync completed")
}

type FacebookPage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AccessToken string `json:"access_token"`
}

func fetchConnectedPages() ([]FacebookPage, error) {
	appToken := os.Getenv("FACEBOOK_APP_TOKEN")
	if appToken == "" {
		return nil, fmt.Errorf("FACEBOOK_APP_TOKEN not set")
	}

	// First get access token
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/app/subscribed_apps?access_token=%s", appToken)
	log.Printf("Fetching connected pages from: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching pages: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	log.Printf("Response status: %s", resp.Status)
	log.Printf("Response body: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Facebook API returned status: %s, body: %s", resp.Status, string(body))
	}

	var result struct {
		Data  []FacebookPage `json:"data"`
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    int    `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if result.Error.Message != "" {
		return nil, fmt.Errorf("Facebook API error: %s (code: %d, type: %s)",
			result.Error.Message, result.Error.Code, result.Error.Type)
	}

	return result.Data, nil
}

func syncPages() error {
	// Get pages from Facebook
	pages, err := fetchConnectedPages()
	if err != nil {
		return err
	}

	log.Printf("Found %d pages", len(pages))

	// Add new pages to database
	for _, page := range pages {
		_, err := DB.Exec(`
           INSERT INTO pages (page_id, name, access_token, status, platform)
           VALUES ($1, $2, $3, 'pending', 'facebook')
           ON CONFLICT (platform, page_id) 
           DO UPDATE SET 
               name = EXCLUDED.name,
               access_token = EXCLUDED.access_token
               WHERE pages.status != 'disabled'
       `, page.ID, page.Name, page.AccessToken)

		if err != nil {
			log.Printf("Error storing page %s: %v", page.ID, err)
			continue
		}
		log.Printf("Synced page: %s", page.Name)
	}

	return nil
}
