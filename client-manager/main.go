// main.go
package main

import (
	"bytes"
	"database/sql"
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
		http.Error(w, fmt.Sprintf("Could not verify Facebook user: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Get connected pages
	pages, err := getConnectedPages(data.UserToken)
	if err != nil {
		log.Printf("❌ Error getting pages: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Found %d connected pages/accounts", len(pages))

	// 3. Start transactions for both databases
	tx, err := DB.Begin()
	if err != nil {
		log.Printf("❌ Error starting client-manager transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var socialTx *sql.Tx
	if SocialDB != nil {
		socialTx, err = SocialDB.Begin()
		if err != nil {
			log.Printf("❌ Error starting social dashboard transaction: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer socialTx.Rollback()
	}

	// 4. Create or update client in client-manager database
	var clientID string
	err = tx.QueryRow(`
        INSERT INTO clients (name, facebook_user_id)
        VALUES ($1, $2)
        ON CONFLICT (facebook_user_id) DO UPDATE
        SET name = EXCLUDED.name
        RETURNING id
    `, fbUser.Name, fbUser.ID).Scan(&clientID)
	if err != nil {
		log.Printf("❌ Error upserting client in client-manager database: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Upserted client in client-manager database with ID: %s", clientID)

	// 4b. Also create or update client in social dashboard database (to avoid foreign key issues)
	if socialTx != nil {
		_, err = socialTx.Exec(`
            INSERT INTO clients (id, name, facebook_user_id, created_at)
            VALUES ($1, $2, $3, NOW())
            ON CONFLICT (id) DO UPDATE
            SET name = EXCLUDED.name,
                facebook_user_id = EXCLUDED.facebook_user_id
        `, clientID, fbUser.Name, fbUser.ID)
		if err != nil {
			log.Printf("❌ Error upserting client in social dashboard database: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Upserted client in social dashboard database with ID: %s", clientID)
	}

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

	// 6. Insert/update new pages in BOTH tables
	for _, page := range pages {
		log.Printf("📝 Processing page %s (ID: %s)", page.Name, page.ID)

		// Insert/update in pages table (for client-manager)
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
			log.Printf("❌ Error processing page in pages table %s: %v", page.Name, err)
			continue
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("✅ Successfully processed page in pages table %s (rows affected: %d)", page.Name, rowsAffected)

		// Also insert/update in social_pages table (for message-router) if social database is available
		if socialTx != nil {
			socialResult, err := socialTx.Exec(`
                INSERT INTO social_pages (
                    client_id,
                    platform,
                    page_id, 
                    page_name, 
                    access_token,
                    created_at
                ) VALUES (
                    $1, $2, $3, $4, $5, NOW()
                )
                ON CONFLICT (platform, page_id) 
                DO UPDATE SET 
                    client_id = EXCLUDED.client_id,
                    page_name = EXCLUDED.page_name,
                    access_token = EXCLUDED.access_token
            `, clientID, page.Platform, page.ID, page.Name, page.AccessToken)

			if err != nil {
				log.Printf("❌ Error processing page in social_pages table %s: %v", page.Name, err)
				continue
			}

			socialRowsAffected, _ := socialResult.RowsAffected()
			log.Printf("✅ Successfully processed page in social_pages table %s (rows affected: %d)", page.Name, socialRowsAffected)
		} else {
			log.Printf("⚠️ Social database not available, skipping social_pages update for %s", page.Name)
		}
	}

	// 7. Commit both transactions
	if err = tx.Commit(); err != nil {
		log.Printf("❌ Error committing client-manager transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Client-manager transaction committed successfully")

	if socialTx != nil {
		if err = socialTx.Commit(); err != nil {
			log.Printf("❌ Error committing social dashboard transaction: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Social dashboard transaction committed successfully")
	}

	// 8. Set up webhook subscriptions for all pages (after database commits)
	log.Printf("🚀 Starting webhook subscription automation for %d pages", len(pages))
	webhookSuccessCount := 0
	for _, page := range pages {
		log.Printf("📝 Setting up webhooks for page: %s (%s)", page.Name, page.Platform)
		
		// Set up webhook subscriptions automatically
		if err := setupWebhookSubscriptions(page.ID, page.AccessToken, page.Name, page.Platform); err != nil {
			log.Printf("⚠️ Webhook setup failed for %s: %v", page.Name, err)
			// Don't fail the entire request - webhook setup is best effort
			// Client can still use the service, but might need manual webhook setup
		} else {
			webhookSuccessCount++
			log.Printf("✅ Webhook setup completed for %s", page.Name)
		}
	}

	log.Printf("🎯 Webhook automation summary: %d/%d pages configured successfully", webhookSuccessCount, len(pages))
	
	if socialTx != nil {
		log.Printf("✅ Successfully completed Facebook token request with webhook automation. Changes committed to both pages and social_pages tables.")
	} else {
		log.Printf("✅ Successfully completed Facebook token request with webhook automation. Changes committed to pages table only.")
	}
	w.WriteHeader(http.StatusOK)
}

// Enhanced getFacebookUser function
func getFacebookUser(token string) (*FacebookUser, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v23.0/me?fields=id,name&access_token=%s", token)
	log.Printf("Attempting to get Facebook user details from: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error making HTTP request to Facebook: %v", err)
		return nil, fmt.Errorf("error fetching user info from Facebook: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Printf("Error reading response body from Facebook: %v", readErr)
		return nil, fmt.Errorf("error reading Facebook response body: %w", readErr)
	}
	// Log the raw response body for debugging, regardless of status code.
	log.Printf("Facebook get user response status: %s, body: %s", resp.Status, string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		var fbError struct {
			Error struct {
				Message   string `json:"message"`
				Type      string `json:"type"`
				Code      int    `json:"code"`
				FbtraceID string `json:"fbtrace_id"`
			} `json:"error"`
		}
		// Try to unmarshal the bodyBytes we already read
		if unmarshalErr := json.Unmarshal(bodyBytes, &fbError); unmarshalErr == nil {
			log.Printf("Facebook API error (parsed from body). Message: %s, Type: %s, Code: %d, Trace: %s",
				fbError.Error.Message, fbError.Error.Type, fbError.Error.Code, fbError.Error.FbtraceID)
		} else {
			// Log if parsing the error structure itself failed
			log.Printf("Facebook API error (could not parse error JSON from body). Body was: %s", string(bodyBytes))
		}
		// Return a generic error message to the caller, specific details are logged.
		return nil, fmt.Errorf("Facebook API error (%s)", resp.Status)
	}

	var user FacebookUser
	// Try to unmarshal the bodyBytes into the User struct
	if err := json.Unmarshal(bodyBytes, &user); err != nil {
		log.Printf("Error parsing Facebook user info from successful (200 OK) response body: %v. Body was: %s", err, string(bodyBytes))
		return nil, fmt.Errorf("error parsing user info from Facebook response: %w", err)
	}

	// Basic validation that we got the essential fields
	if user.ID == "" || user.Name == "" {
		log.Printf("Facebook user details incomplete - ID: %s, Name: %s", user.ID, user.Name)
		return nil, fmt.Errorf("incomplete user data from Facebook")
	}

	// If everything is successful, log the details.
	log.Printf("Successfully fetched Facebook user: ID %s, Name %s", user.ID, user.Name)
	return &user, nil
}

func getConnectedPages(userToken string) ([]FacebookPage, error) {
	// Exchange user token for long-lived user token (60 days)
	// Note: This is NOT permanent, but the page tokens we get from it ARE permanent
	longLivedUrl := fmt.Sprintf(
		"https://graph.facebook.com/v23.0/oauth/access_token?"+
			"grant_type=fb_exchange_token&"+
			"client_id=%s&"+
			"client_secret=%s&"+
			"fb_exchange_token=%s",
		os.Getenv("FACEBOOK_APP_ID"),
		os.Getenv("FACEBOOK_APP_SECRET"),
		userToken,
	)

	log.Printf("Getting long-lived user token (60 days)")
	longLivedResp, err := http.Get(longLivedUrl)
	if err != nil {
		return nil, fmt.Errorf("error getting long-lived token: %w", err)
	}
	defer longLivedResp.Body.Close()

	// Read and log the long-lived token response
	longLivedBodyBytes, readErr := io.ReadAll(longLivedResp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("error reading long-lived token response body: %w", readErr)
	}
	log.Printf("Long-lived token response status: %s, body: %s", longLivedResp.Status, string(longLivedBodyBytes))

	var longLivedResult struct {
		AccessToken string `json:"access_token"`
		Error       struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(longLivedBodyBytes, &longLivedResult); err != nil {
		return nil, fmt.Errorf("error parsing long-lived token response: %w", err)
	}

	if longLivedResult.Error.Message != "" {
		log.Printf("❌ Facebook long-lived token error: %s (Type: %s, Code: %d)",
			longLivedResult.Error.Message, longLivedResult.Error.Type, longLivedResult.Error.Code)
		return nil, fmt.Errorf("Facebook long-lived token error: %s", longLivedResult.Error.Message)
	}

	if longLivedResult.AccessToken == "" {
		log.Printf("❌ No access token received in long-lived token response")
		return nil, fmt.Errorf("no access token received from Facebook")
	}

	log.Printf("✅ Successfully obtained long-lived user token (60 days, NOT permanent)")

	// DEBUG: Log the actual token values for manual debugging
	log.Printf("🔑 TOKEN COMPARISON FOR DEBUGGING:")
	log.Printf("   Original user token: %s", userToken)
	log.Printf("   Long-lived user token: %s", longLivedResult.AccessToken)
	log.Printf("   📝 Copy these tokens to https://developers.facebook.com/tools/debug/accesstoken/ for detailed analysis")

	// DEBUG: Check what permissions the long-lived token actually has
	debugPermUrl := fmt.Sprintf("https://graph.facebook.com/v23.0/me/permissions?access_token=%s", longLivedResult.AccessToken)
	log.Printf("🔍 Checking long-lived token permissions: %s", debugPermUrl)

	permDebugResp, err := http.Get(debugPermUrl)
	if err != nil {
		log.Printf("⚠️ Warning: Could not check long-lived token permissions: %v", err)
	} else {
		defer permDebugResp.Body.Close()
		permDebugBody, _ := io.ReadAll(permDebugResp.Body)
		log.Printf("📋 Long-lived token permissions response: %s", string(permDebugBody))

		var permissions struct {
			Data []struct {
				Permission string `json:"permission"`
				Status     string `json:"status"`
			} `json:"data"`
		}
		if json.Unmarshal(permDebugBody, &permissions) == nil {
			var grantedPermissions []string
			for _, perm := range permissions.Data {
				if perm.Status == "granted" {
					grantedPermissions = append(grantedPermissions, perm.Permission)
				}
			}

			hasReadEngagement := false
			for _, p := range grantedPermissions {
				if p == "pages_read_engagement" {
					hasReadEngagement = true
					break
				}
			}

			if !hasReadEngagement {
				log.Printf("Warning: pages_read_engagement permission not granted. This permission is required to fetch managed pages. Please ensure it's included in the frontend login scope and ask the user to re-authorize if necessary.")
			}
		}
	}

	// DEBUG: Also check permissions of the original user token for comparison
	originalPermUrl := fmt.Sprintf("https://graph.facebook.com/v23.0/me/permissions?access_token=%s", userToken)
	log.Printf("🔍 Checking original user token permissions: %s", originalPermUrl)

	originalPermResp, err := http.Get(originalPermUrl)
	if err != nil {
		log.Printf("⚠️ Warning: Could not check original token permissions: %v", err)
	} else {
		defer originalPermResp.Body.Close()
		originalPermBody, _ := io.ReadAll(originalPermResp.Body)
		log.Printf("📋 Original token permissions response: %s", string(originalPermBody))
	}

	// Use the long-lived user token to get pages (page tokens will be permanent)
	fbURL := fmt.Sprintf(
		"https://graph.facebook.com/v23.0/me/accounts?"+
			"access_token=%s&"+
			"fields=id,name,access_token,instagram_business_account{id,name,username}",
		longLivedResult.AccessToken,
	)

	log.Printf("Fetching Facebook pages and connected Instagram accounts from: %s", fbURL)
	fbResp, err := http.Get(fbURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching pages: %w", err)
	}
	defer fbResp.Body.Close()

	// Read and log the pages response
	fbBodyBytes, readErr := io.ReadAll(fbResp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("error reading pages response body: %w", readErr)
	}
	log.Printf("Facebook pages response status: %s, body: %s", fbResp.Status, string(fbBodyBytes))

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
			Message   string `json:"message"`
			Type      string `json:"type"`
			Code      int    `json:"code"`
			FbtraceID string `json:"fbtrace_id"`
		} `json:"error"`
	}

	if err := json.Unmarshal(fbBodyBytes, &fbResult); err != nil {
		return nil, fmt.Errorf("error parsing Facebook response: %w", err)
	}

	if fbResult.Error.Message != "" {
		log.Printf("❌ Facebook pages API error: %s (Type: %s, Code: %d, Trace: %s)",
			fbResult.Error.Message, fbResult.Error.Type, fbResult.Error.Code, fbResult.Error.FbtraceID)
		return nil, fmt.Errorf("Facebook pages API error: %s", fbResult.Error.Message)
	}

	// DEBUG: If no pages found, try additional debugging
	if len(fbResult.Data) == 0 {
		log.Printf("🔍 No pages found with long-lived token - performing additional debugging...")

		log.Printf("🔑 REMINDER - TOKEN VALUES FOR MANUAL DEBUGGING:")
		log.Printf("   Original user token: %s", userToken)
		log.Printf("   Long-lived user token: %s", longLivedResult.AccessToken)
		log.Printf("   📝 Test both tokens at: https://developers.facebook.com/tools/debug/accesstoken/")

		// TEST: Try the same API call with the original user token
		originalPagesURL := fmt.Sprintf(
			"https://graph.facebook.com/v23.0/me/accounts?"+
				"access_token=%s&"+
				"fields=id,name,access_token,instagram_business_account{id,name,username}",
			userToken,
		)
		log.Printf("🔍 Testing pages API with original user token: %s", originalPagesURL)

		originalPagesResp, err := http.Get(originalPagesURL)
		if err != nil {
			log.Printf("⚠️ Warning: Could not check pages with original token: %v", err)
		} else {
			defer originalPagesResp.Body.Close()
			originalPagesBody, _ := io.ReadAll(originalPagesResp.Body)
			log.Printf("📋 Original token pages response: %s", string(originalPagesBody))
		}

		// Check if user has any pages at all (without permissions filter)
		debugURL := fmt.Sprintf("https://graph.facebook.com/v23.0/me/accounts?access_token=%s", longLivedResult.AccessToken)
		log.Printf("🔍 Checking all user accounts: %s", debugURL)

		debugResp, err := http.Get(debugURL)
		if err != nil {
			log.Printf("⚠️ Warning: Could not check all accounts: %v", err)
		} else {
			defer debugResp.Body.Close()
			debugBody, _ := io.ReadAll(debugResp.Body)
			log.Printf("📋 All user accounts response: %s", string(debugBody))
		}

		// Also check user's basic info
		userInfoURL := fmt.Sprintf("https://graph.facebook.com/v23.0/me?fields=id,name,email&access_token=%s", longLivedResult.AccessToken)
		log.Printf("🔍 Checking user info: %s", userInfoURL)

		userInfoResp, err := http.Get(userInfoURL)
		if err != nil {
			log.Printf("⚠️ Warning: Could not check user info: %v", err)
		} else {
			defer userInfoResp.Body.Close()
			userInfoBody, _ := io.ReadAll(userInfoResp.Body)
			log.Printf("👤 User info response: %s", string(userInfoBody))
		}

		// TEST: Try accessing the specific page we know exists from token debugger
		// Page ID: 269054096290372 (Happiness boutique)
		specificPageURL := fmt.Sprintf("https://graph.facebook.com/v23.0/269054096290372?fields=id,name,access_token&access_token=%s", longLivedResult.AccessToken)
		log.Printf("🔍 Testing direct access to known page: %s", specificPageURL)

		specificPageResp, err := http.Get(specificPageURL)
		if err != nil {
			log.Printf("⚠️ Warning: Could not access specific page: %v", err)
		} else {
			defer specificPageResp.Body.Close()
			specificPageBody, _ := io.ReadAll(specificPageResp.Body)
			log.Printf("📄 Specific page response: %s", string(specificPageBody))
		}

		// TEST: Try the accounts endpoint with different API versions
		debugURLv23 := fmt.Sprintf("https://graph.facebook.com/v23.0/me/accounts?access_token=%s", longLivedResult.AccessToken)
		log.Printf("🔍 Testing with API v23.0: %s", debugURLv23)

		debugRespv23, err := http.Get(debugURLv23)
		if err != nil {
			log.Printf("⚠️ Warning: Could not check with v23.0: %v", err)
		} else {
			defer debugRespv23.Body.Close()
			debugBodyv23, _ := io.ReadAll(debugRespv23.Body)
			log.Printf("📋 v23.0 response: %s", string(debugBodyv23))
		}

		log.Printf("💡 Possible reasons for no pages:")
		log.Printf("   1. Permanent token doesn't inherit all permissions from user token")
		log.Printf("   2. User is not an admin of any Facebook pages")
		log.Printf("   3. User hasn't granted necessary permissions")
		log.Printf("   4. Facebook app needs additional permissions/review")
		log.Printf("   5. User doesn't have a Facebook Business account")
		log.Printf("   6. Pages are restricted or suspended")
		log.Printf("   7. API version compatibility issue")
		log.Printf("   8. Token exchange issue - permanent token not working correctly")
	}

	var allPages []FacebookPage

	// Add Facebook pages and their connected Instagram accounts
	for _, page := range fbResult.Data {
		// DEBUG: Verify that page tokens are actually permanent
		pageTokenDebugURL := fmt.Sprintf("https://graph.facebook.com/v23.0/debug_token?input_token=%s&access_token=%s|%s",
			page.AccessToken, os.Getenv("FACEBOOK_APP_ID"), os.Getenv("FACEBOOK_APP_SECRET"))

		log.Printf("🔍 Verifying page token for %s: %s", page.Name, pageTokenDebugURL)

		pageTokenResp, err := http.Get(pageTokenDebugURL)
		if err != nil {
			log.Printf("⚠️ Warning: Could not verify page token for %s: %v", page.Name, err)
		} else {
			defer pageTokenResp.Body.Close()
			pageTokenBody, _ := io.ReadAll(pageTokenResp.Body)
			log.Printf("📋 Page token info for %s: %s", page.Name, string(pageTokenBody))

			// Parse and log key info about the token
			var tokenInfo struct {
				Data struct {
					Type      string `json:"type"`
					ExpiresAt int64  `json:"expires_at"`
					IsValid   bool   `json:"is_valid"`
				} `json:"data"`
			}

			if json.Unmarshal(pageTokenBody, &tokenInfo) == nil {
				expiry := "PERMANENT (no expiration)"
				if tokenInfo.Data.ExpiresAt > 0 {
					expiry = fmt.Sprintf("EXPIRES at %d", tokenInfo.Data.ExpiresAt)
				}
				log.Printf("🔑 Token for %s: Type=%s, Valid=%v, %s",
					page.Name, tokenInfo.Data.Type, tokenInfo.Data.IsValid, expiry)
			}
		}

		// Add Facebook page with permanent page token
		allPages = append(allPages, FacebookPage{
			ID:          page.ID,
			Name:        page.Name,
			AccessToken: page.AccessToken, // This IS a permanent page token (never expires)
			Platform:    "facebook",
		})
		log.Printf("Added Facebook page: %s", page.Name)

		// If this page has a connected Instagram account, add it
		if page.Instagram.ID != "" {
			allPages = append(allPages, FacebookPage{
				ID:          page.Instagram.ID,
				Name:        page.Instagram.Name,
				AccessToken: page.AccessToken, // Use same permanent page token
				Platform:    "instagram",
			})
			log.Printf("Added connected Instagram account: %s", page.Instagram.Name)
		}
	}

	log.Printf("Found total of %d pages/accounts", len(allPages))
	return allPages, nil
}

// =============================================================================
// WEBHOOK SUBSCRIPTION AUTOMATION - For multi-tenant setup
// =============================================================================

// subscribePageToWebhooks subscribes a Facebook page to all required webhook events
func subscribePageToWebhooks(pageID, pageToken string) error {
	appID := os.Getenv("FACEBOOK_APP_ID")
	if appID == "" {
		return fmt.Errorf("FACEBOOK_APP_ID environment variable not set")
	}

	// Subscribe page to the Neurocrow app for webhook events
	subscribeURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s/subscribed_apps", pageID)
	
	// Create payload for subscribing to webhooks
	subscribePayload := map[string]interface{}{
		"subscribed_fields": []string{
			"messages",
			"messaging_postbacks", 
			"messaging_handovers",
			"messaging_policy_enforcement",
		},
	}

	jsonData, err := json.Marshal(subscribePayload)
	if err != nil {
		return fmt.Errorf("error marshaling subscribe payload: %v", err)
	}

	// Make POST request to subscribe
	req, err := http.NewRequest("POST", subscribeURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating subscribe request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.URL.RawQuery = fmt.Sprintf("access_token=%s", pageToken)

	log.Printf("🔗 Subscribing page %s to webhooks: %s", pageID, subscribeURL)
	log.Printf("📤 Subscribe payload: %s", string(jsonData))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making subscribe request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading subscribe response: %v", err)
	}

	log.Printf("📥 Webhook subscription response: %d - %s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		var fbError struct {
			Error struct {
				Message   string `json:"message"`
				Type      string `json:"type"`  
				Code      int    `json:"code"`
				FbtraceID string `json:"fbtrace_id"`
			} `json:"error"`
		}
		
		if json.Unmarshal(respBody, &fbError) == nil && fbError.Error.Message != "" {
			return fmt.Errorf("Facebook webhook subscription error: %s (Code: %d, Trace: %s)", 
				fbError.Error.Message, fbError.Error.Code, fbError.Error.FbtraceID)
		}
		return fmt.Errorf("webhook subscription failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("✅ Successfully subscribed page %s to webhooks", pageID)
	return nil
}

// configureHandoverProtocol sets up the Facebook handover protocol for the page
func configureHandoverProtocol(pageID, pageToken string) error {
	neurocrowAppID := os.Getenv("FACEBOOK_APP_ID")
	if neurocrowAppID == "" {
		return fmt.Errorf("FACEBOOK_APP_ID environment variable not set")
	}

	log.Printf("🔄 Configuring handover protocol for page %s", pageID)
	log.Printf("   Primary receiver (Neurocrow): %s", neurocrowAppID)
	log.Printf("   Secondary receiver (Page Inbox): 263902037430900")

	// Set up handover protocol using messenger_profile endpoint
	handoverURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s/messenger_profile", pageID)
	handoverPayload := map[string]interface{}{
		"primary_receiver_app_id": neurocrowAppID,
	}

	jsonData, err := json.Marshal(handoverPayload)
	if err != nil {
		return fmt.Errorf("error marshaling handover payload: %v", err)
	}

	// Make POST request to set primary receiver
	req, err := http.NewRequest("POST", handoverURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating handover request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.URL.RawQuery = fmt.Sprintf("access_token=%s", pageToken)

	log.Printf("🔗 Setting primary receiver for page %s: %s", pageID, handoverURL)
	log.Printf("📤 Handover payload: %s", string(jsonData))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making handover request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading handover response: %v", err)
	}

	log.Printf("📥 Handover protocol response: %d - %s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		var fbError struct {
			Error struct {
				Message   string `json:"message"`
				Type      string `json:"type"`
				Code      int    `json:"code"`
				FbtraceID string `json:"fbtrace_id"`
			} `json:"error"`
		}
		
		if json.Unmarshal(respBody, &fbError) == nil && fbError.Error.Message != "" {
			log.Printf("⚠️ Handover protocol setup warning: %s (Code: %d, Trace: %s)", 
				fbError.Error.Message, fbError.Error.Code, fbError.Error.FbtraceID)
			// Don't return error - handover protocol setup can fail but webhook subscription still works
		} else {
			log.Printf("⚠️ Handover protocol setup returned status %d: %s", resp.StatusCode, string(respBody))
		}
	} else {
		log.Printf("✅ Successfully configured handover protocol for page %s", pageID)
	}

	return nil
}

// verifyWebhookSetup verifies that webhook subscriptions were set up correctly
func verifyWebhookSetup(pageID, pageToken string) error {
	// Check subscribed apps for the page
	verifyURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s/subscribed_apps?access_token=%s", pageID, pageToken)
	
	log.Printf("🔍 Verifying webhook setup for page %s: %s", pageID, verifyURL)

	resp, err := http.Get(verifyURL)
	if err != nil {
		return fmt.Errorf("error verifying webhook setup: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading verification response: %v", err)
	}

	log.Printf("📋 Webhook verification response: %d - %s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook verification failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse and log the subscribed apps
	var verifyResult struct {
		Data []struct {
			ID             string   `json:"id"`
			Name           string   `json:"name"`
			SubscribedFields []string `json:"subscribed_fields"`
		} `json:"data"`
	}

	if json.Unmarshal(respBody, &verifyResult) == nil {
		log.Printf("📊 Subscribed apps for page %s:", pageID)
		for _, app := range verifyResult.Data {
			log.Printf("   App: %s (%s) - Fields: %v", app.Name, app.ID, app.SubscribedFields)
		}
		
		// Check if our app is in the list
		neurocrowAppID := os.Getenv("FACEBOOK_APP_ID")
		found := false
		for _, app := range verifyResult.Data {
			if app.ID == neurocrowAppID {
				found = true
				log.Printf("✅ Neurocrow app (%s) is subscribed with fields: %v", neurocrowAppID, app.SubscribedFields)
				break
			}
		}
		
		if !found {
			log.Printf("⚠️ Warning: Neurocrow app (%s) not found in subscribed apps list", neurocrowAppID)
		}
	}

	return nil
}

// setupWebhookSubscriptions orchestrates the complete webhook setup for a page
func setupWebhookSubscriptions(pageID, pageToken, pageName, platform string) error {
	log.Printf("🚀 Starting webhook setup for %s page: %s (%s)", platform, pageName, pageID)

	// Step 1: Subscribe page to webhooks
	if err := subscribePageToWebhooks(pageID, pageToken); err != nil {
		log.Printf("❌ Webhook subscription failed for %s: %v", pageName, err)
		return fmt.Errorf("webhook subscription failed: %v", err)
	}

	// Step 2: Configure handover protocol (only for Facebook pages, not Instagram)
	if platform == "facebook" {
		if err := configureHandoverProtocol(pageID, pageToken); err != nil {
			log.Printf("⚠️ Handover protocol setup failed for %s: %v", pageName, err)
			// Don't return error - handover protocol is optional, webhook subscription is more important
		}
	} else {
		log.Printf("ℹ️ Skipping handover protocol for Instagram account: %s", pageName)
	}

	// Step 3: Verify the setup
	if err := verifyWebhookSetup(pageID, pageToken); err != nil {
		log.Printf("⚠️ Webhook verification failed for %s: %v", pageName, err)
		// Don't return error - verification is informational
	}

	log.Printf("✅ Webhook setup completed for %s page: %s", platform, pageName)
	return nil
}
