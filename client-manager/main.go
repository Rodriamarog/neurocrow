// main.go
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.ParseGlob("templates/*.html"))

	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found - using environment variables")
	}

	// Print all environment variables (careful with sensitive info!)
	log.Printf("Environment variables loaded. DATABASE_URL exists: %v", os.Getenv("DATABASE_URL") != "")

	initDB()
}

func main() {
	// Wrap ALL routes with CORS middleware
	router := http.NewServeMux()

	// Apply CORS to all routes
	router.HandleFunc("/", corsMiddleware(handlePages))
	router.HandleFunc("/facebook-token", corsMiddleware(handleFacebookToken))
	router.HandleFunc("/activate-form", corsMiddleware(handleActivateForm))
	router.HandleFunc("/activate-page", corsMiddleware(handleActivatePage))
	router.HandleFunc("/deactivate-page", corsMiddleware(handleDeactivatePage))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

type PageData struct {
	ID          string
	Name        string
	Platform    string
	PageID      string
	ClientName  string
	BotpressURL string
	Status      string
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Allow both localhost and your production domain
		origin := r.Header.Get("Origin")
		allowedOrigins := []string{
			"http://localhost:3000",
			"https://neurocrow.com",
			"https://www.neurocrow.com",
		}

		// Check if the request origin is allowed
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}

		// Required CORS headers
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func handlePages(w http.ResponseWriter, r *http.Request) {
	// Get pending pages - modify the query to handle NULL client_id
	pendingRows, err := DB.Query(`
        SELECT 
            p.id, p.name, p.platform, p.page_id, 
            COALESCE(c.name, 'No Client') as client_name
        FROM pages p
        LEFT JOIN clients c ON p.client_id = c.id
        WHERE p.status = 'pending'
        ORDER BY p.created_at DESC
    `)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer pendingRows.Close()

	var pendingPages []PageData
	for pendingRows.Next() {
		var page PageData
		err := pendingRows.Scan(
			&page.ID, &page.Name, &page.Platform,
			&page.PageID, &page.ClientName,
		)
		if err != nil {
			log.Printf("Error scanning page: %v", err)
			continue
		}
		pendingPages = append(pendingPages, page)
	}

	// Get active pages
	activeRows, err := DB.Query(`
        SELECT 
            p.id, p.name, p.platform, p.page_id, 
            c.name as client_name, p.botpress_url
        FROM pages p
        JOIN clients c ON p.client_id = c.id
        WHERE p.status = 'active'
        ORDER BY p.created_at DESC
    `)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer activeRows.Close()

	var activePages []PageData
	for activeRows.Next() {
		var page PageData
		err := activeRows.Scan(
			&page.ID, &page.Name, &page.Platform,
			&page.PageID, &page.ClientName, &page.BotpressURL,
		)
		if err != nil {
			log.Printf("Error scanning page: %v", err)
			continue
		}
		activePages = append(activePages, page)
	}

	data := map[string]interface{}{
		"PendingPages": pendingPages,
		"ActivePages":  activePages,
	}

	tmpl.ExecuteTemplate(w, "layout.html", data)
}

func handleActivateForm(w http.ResponseWriter, r *http.Request) {
	pageID := r.URL.Query().Get("pageId")

	// Get page info
	var page PageData
	err := DB.QueryRow(`
        SELECT p.id, p.name, p.platform, c.name as client_name
        FROM pages p
        JOIN clients c ON p.client_id = c.id
        WHERE p.id = $1
    `, pageID).Scan(&page.ID, &page.Name, &page.Platform, &page.ClientName)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl.ExecuteTemplate(w, "activate-form", page)
}

func handleActivatePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageID := r.FormValue("pageId")
	botpressURL := r.FormValue("botpressUrl")

	// Validate botpress URL
	if botpressURL == "" {
		http.Error(w, "Botpress URL is required", http.StatusBadRequest)
		return
	}

	// Update page status
	_, err := DB.Exec(`
        UPDATE pages 
        SET status = 'active',
            botpress_url = $1,
            activated_at = NOW()
        WHERE id = $2
    `, botpressURL, pageID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleDeactivatePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageID := r.FormValue("pageId")

	// Update page status
	_, err := DB.Exec(`
        UPDATE pages 
        SET status = 'disabled',
            botpress_url = NULL,
            activated_at = NULL
        WHERE id = $1
    `, pageID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to pages view
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func fetchConnectedPages() ([]FacebookPage, error) {
	appToken := os.Getenv("FACEBOOK_APP_TOKEN")
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/app/subscribed_apps?access_token=%s", appToken)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []FacebookPage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func handleFacebookToken(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request to /facebook-token")

	var data struct {
		UserToken string `json:"userToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get currently selected pages from Facebook
	pages, err := getConnectedPages(data.UserToken)
	if err != nil {
		log.Printf("Error getting connected pages: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Begin transaction
	tx, err := DB.Begin()
	if err != nil {
		log.Printf("Error beginning transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// First, mark all pages as disabled
	_, err = tx.Exec(`
        UPDATE pages 
        SET status = 'disabled', 
            botpress_url = NULL,
            activated_at = NULL
        WHERE platform = 'facebook'
    `)
	if err != nil {
		log.Printf("Error disabling old pages: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Then insert/update currently selected pages
	for _, page := range pages {
		_, err := tx.Exec(`
            INSERT INTO pages (page_id, name, access_token, status, platform)
            VALUES ($1, $2, $3, 'pending', 'facebook')
            ON CONFLICT (platform, page_id) 
            DO UPDATE SET 
                name = EXCLUDED.name,
                access_token = EXCLUDED.access_token,
                status = 'pending'
        `, page.ID, page.Name, page.AccessToken)

		if err != nil {
			log.Printf("Error storing page %s: %v", page.Name, err)
			continue
		}
		log.Printf("Successfully stored page: %s", page.Name)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	log.Printf("Successfully handled token request")
}

func getConnectedPages(userToken string) ([]FacebookPage, error) {
	url := fmt.Sprintf(
		"https://graph.facebook.com/v19.0/me/accounts?"+
			"access_token=%s&"+
			"fields=id,name,access_token",
		userToken,
	)

	log.Printf("Fetching pages from Facebook API")
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching pages: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body for logging
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	log.Printf("Facebook API response status: %s", resp.Status)
	log.Printf("Facebook API response: %s", string(body))

	var result struct {
		Data  []FacebookPage `json:"data"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	if result.Error.Message != "" {
		return nil, fmt.Errorf("Facebook API error: %s", result.Error.Message)
	}

	log.Printf("Successfully parsed %d pages from Facebook response", len(result.Data))
	return result.Data, nil
}
