// main.go
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
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
	// Get pending pages - only non-disabled ones
	pendingRows, err := DB.Query(`
        SELECT 
            p.id, p.name, p.platform, p.page_id, 
            COALESCE(c.name, 'No Client') as client_name
        FROM pages p
        LEFT JOIN clients c ON p.client_id = c.id
        WHERE p.status = 'pending'
        AND p.status != 'disabled'  -- Added this condition
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

	// Get active pages - only non-disabled ones
	activeRows, err := DB.Query(`
        SELECT 
            p.id, p.name, p.platform, p.page_id, 
            COALESCE(c.name, 'No Client') as client_name,
            p.botpress_url
        FROM pages p
        LEFT JOIN clients c ON p.client_id = c.id
        WHERE p.status = 'active'
        AND p.status != 'disabled'  -- Added this condition
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

func handleFacebookToken(w http.ResponseWriter, r *http.Request) {
	log.Printf("=== Starting Facebook token request handling ===")

	var data struct {
		UserToken string `json:"userToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("❌ Error decoding request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Get user details from Facebook
	fbUser, err := getFacebookUser(data.UserToken)
	if err != nil {
		log.Printf("❌ Error getting Facebook user details: %v", err)
		http.Error(w, "Could not verify Facebook user", http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Got Facebook user: %s (%s)", fbUser.Name, fbUser.Email)

	// 2. Get connected pages
	pages, err := getConnectedPages(data.UserToken)
	if err != nil {
		log.Printf("❌ Error getting pages: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Found %d connected pages/accounts", len(pages))

	// 3. Start transaction
	tx, err := DB.Begin()
	if err != nil {
		log.Printf("❌ Error starting transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 4. Create or update client
	var clientID string
	err = tx.QueryRow(`
        INSERT INTO clients (name, email)
        VALUES ($1, $2)
        ON CONFLICT (email) DO UPDATE
        SET name = EXCLUDED.name
        RETURNING id
    `, fbUser.Name, fbUser.Email).Scan(&clientID)
	if err != nil {
		log.Printf("❌ Error upserting client: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Upserted client with ID: %s", clientID)

	// 5. Disable client's previous pages
	result, err := tx.Exec(`
        UPDATE pages 
        SET status = 'disabled'
        WHERE client_id = $1 
        AND platform IN ('facebook', 'instagram')
    `, clientID)
	if err != nil {
		log.Printf("❌ Error disabling previous pages: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rowsAffected, _ := result.RowsAffected()
	log.Printf("✅ Disabled %d previous pages", rowsAffected)

	// 6. Insert/update new pages
	for _, page := range pages {
		log.Printf("📝 Processing page %s (ID: %s)", page.Name, page.ID)

		result, err := tx.Exec(`
            INSERT INTO pages (
                client_id,
                page_id, 
                name, 
                access_token, 
                platform,
                status
            ) VALUES (
                $1, $2, $3, $4, $5,
                CASE 
                    WHEN EXISTS (
                        SELECT 1 FROM pages 
                        WHERE platform = $5 
                        AND page_id = $2 
                        AND status = 'active'
                    ) THEN 'active'
                    ELSE 'pending'
                END
            )
            ON CONFLICT (platform, page_id) 
            DO UPDATE SET 
                client_id = EXCLUDED.client_id,
                name = EXCLUDED.name,
                access_token = EXCLUDED.access_token,
                status = CASE 
                    WHEN pages.status = 'active' THEN 'active'
                    ELSE 'pending'
                END
        `, clientID, page.ID, page.Name, page.AccessToken, page.Platform)

		if err != nil {
			log.Printf("❌ Error processing page %s: %v", page.Name, err)
			continue
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("✅ Successfully processed page %s (rows affected: %d)", page.Name, rowsAffected)
	}

	// 7. Commit transaction
	if err = tx.Commit(); err != nil {
		log.Printf("❌ Error committing transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Successfully completed Facebook token request")
	w.WriteHeader(http.StatusOK)
}

// Add this new function
func getFacebookUser(token string) (*FacebookUser, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/me?fields=id,name,email&access_token=%s", token)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching user info: %w", err)
	}
	defer resp.Body.Close()

	var user FacebookUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("error parsing user info: %w", err)
	}

	if user.Email == "" {
		return nil, fmt.Errorf("no email provided by Facebook")
	}

	return &user, nil
}

func getConnectedPages(userToken string) ([]FacebookPage, error) {
	// Exchange user token for permanent token first
	permUrl := fmt.Sprintf(
		"https://graph.facebook.com/v19.0/oauth/access_token?"+
			"grant_type=fb_exchange_token&"+
			"client_id=%s&"+
			"client_secret=%s&"+
			"fb_exchange_token=%s",
		os.Getenv("FACEBOOK_APP_ID"),
		os.Getenv("FACEBOOK_APP_SECRET"),
		userToken,
	)

	log.Printf("Getting permanent user token")
	permResp, err := http.Get(permUrl)
	if err != nil {
		return nil, fmt.Errorf("error getting permanent token: %w", err)
	}
	defer permResp.Body.Close()

	var permResult struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(permResp.Body).Decode(&permResult); err != nil {
		return nil, fmt.Errorf("error parsing permanent token response: %w", err)
	}

	// Use the permanent token to get pages
	fbURL := fmt.Sprintf(
		"https://graph.facebook.com/v19.0/me/accounts?"+
			"access_token=%s&"+
			"fields=id,name,access_token,instagram_business_account{id,name,username}",
		permResult.AccessToken,
	)

	log.Printf("Fetching Facebook pages and connected Instagram accounts")
	fbResp, err := http.Get(fbURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching pages: %w", err)
	}
	defer fbResp.Body.Close()

	var fbResult struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			AccessToken string `json:"access_token"`
			Instagram   struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Username string `json:"username"`
			} `json:"instagram_business_account"`
		} `json:"data"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(fbResp.Body).Decode(&fbResult); err != nil {
		return nil, fmt.Errorf("error parsing Facebook response: %w", err)
	}

	var allPages []FacebookPage

	// Add Facebook pages and their connected Instagram accounts
	for _, page := range fbResult.Data {
		// Add Facebook page with permanent token
		allPages = append(allPages, FacebookPage{
			ID:          page.ID,
			Name:        page.Name,
			AccessToken: page.AccessToken, // This is now a permanent token
			Platform:    "facebook",
		})
		log.Printf("Added Facebook page: %s", page.Name)

		// If this page has a connected Instagram account, add it
		if page.Instagram.ID != "" {
			allPages = append(allPages, FacebookPage{
				ID:          page.Instagram.ID,
				Name:        page.Instagram.Name,
				AccessToken: page.AccessToken, // Use same permanent token
				Platform:    "instagram",
			})
			log.Printf("Added connected Instagram account: %s", page.Instagram.Name)
		}
	}

	log.Printf("Found total of %d pages/accounts", len(allPages))
	return allPages, nil
}
