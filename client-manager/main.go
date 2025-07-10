// main.go
package main

import (
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
		log.Printf("✅ Successfully completed Facebook token request, changes committed to both pages and social_pages tables.")
	} else {
		log.Printf("✅ Successfully completed Facebook token request, changes committed to pages table only.")
	}
	w.WriteHeader(http.StatusOK)
}

// Enhanced getFacebookUser function
func getFacebookUser(token string) (*FacebookUser, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/me?fields=id,name&access_token=%s", token)
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

	// Read and log the permanent token response
	permBodyBytes, readErr := io.ReadAll(permResp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("error reading permanent token response body: %w", readErr)
	}
	log.Printf("Permanent token response status: %s, body: %s", permResp.Status, string(permBodyBytes))

	var permResult struct {
		AccessToken string `json:"access_token"`
		Error       struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(permBodyBytes, &permResult); err != nil {
		return nil, fmt.Errorf("error parsing permanent token response: %w", err)
	}

	if permResult.Error.Message != "" {
		log.Printf("❌ Facebook permanent token error: %s (Type: %s, Code: %d)",
			permResult.Error.Message, permResult.Error.Type, permResult.Error.Code)
		return nil, fmt.Errorf("Facebook permanent token error: %s", permResult.Error.Message)
	}

	if permResult.AccessToken == "" {
		log.Printf("❌ No access token received in permanent token response")
		return nil, fmt.Errorf("no access token received from Facebook")
	}

	log.Printf("✅ Successfully obtained permanent token")

	// DEBUG: Log the actual token values for manual debugging
	log.Printf("🔑 TOKEN COMPARISON FOR DEBUGGING:")
	log.Printf("   Original user token: %s", userToken)
	log.Printf("   Permanent token: %s", permResult.AccessToken)
	log.Printf("   📝 Copy these tokens to https://developers.facebook.com/tools/debug/accesstoken/ for detailed analysis")

	// DEBUG: Check what permissions the permanent token actually has
	debugPermUrl := fmt.Sprintf("https://graph.facebook.com/v19.0/me/permissions?access_token=%s", permResult.AccessToken)
	log.Printf("🔍 Checking permanent token permissions: %s", debugPermUrl)

	permDebugResp, err := http.Get(debugPermUrl)
	if err != nil {
		log.Printf("⚠️ Warning: Could not check permanent token permissions: %v", err)
	} else {
		defer permDebugResp.Body.Close()
		permDebugBody, _ := io.ReadAll(permDebugResp.Body)
		log.Printf("📋 Permanent token permissions response: %s", string(permDebugBody))
	}

	// DEBUG: Also check permissions of the original user token for comparison
	originalPermUrl := fmt.Sprintf("https://graph.facebook.com/v19.0/me/permissions?access_token=%s", userToken)
	log.Printf("🔍 Checking original user token permissions: %s", originalPermUrl)

	originalPermResp, err := http.Get(originalPermUrl)
	if err != nil {
		log.Printf("⚠️ Warning: Could not check original token permissions: %v", err)
	} else {
		defer originalPermResp.Body.Close()
		originalPermBody, _ := io.ReadAll(originalPermResp.Body)
		log.Printf("📋 Original token permissions response: %s", string(originalPermBody))
	}

	// Use the permanent token to get pages
	fbURL := fmt.Sprintf(
		"https://graph.facebook.com/v19.0/me/accounts?"+
			"access_token=%s&"+
			"fields=id,name,access_token,instagram_business_account{id,name,username}",
		permResult.AccessToken,
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
		log.Printf("🔍 No pages found with permanent token - performing additional debugging...")

		log.Printf("🔑 REMINDER - TOKEN VALUES FOR MANUAL DEBUGGING:")
		log.Printf("   Original user token: %s", userToken)
		log.Printf("   Permanent token: %s", permResult.AccessToken)
		log.Printf("   📝 Test both tokens at: https://developers.facebook.com/tools/debug/accesstoken/")

		// TEST: Try the same API call with the original user token
		originalPagesURL := fmt.Sprintf(
			"https://graph.facebook.com/v19.0/me/accounts?"+
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
		debugURL := fmt.Sprintf("https://graph.facebook.com/v19.0/me/accounts?access_token=%s", permResult.AccessToken)
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
		userInfoURL := fmt.Sprintf("https://graph.facebook.com/v19.0/me?fields=id,name,email&access_token=%s", permResult.AccessToken)
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
		specificPageURL := fmt.Sprintf("https://graph.facebook.com/v19.0/269054096290372?fields=id,name,access_token&access_token=%s", permResult.AccessToken)
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
		debugURLv18 := fmt.Sprintf("https://graph.facebook.com/v18.0/me/accounts?access_token=%s", permResult.AccessToken)
		log.Printf("🔍 Testing with API v18.0: %s", debugURLv18)

		debugRespv18, err := http.Get(debugURLv18)
		if err != nil {
			log.Printf("⚠️ Warning: Could not check with v18.0: %v", err)
		} else {
			defer debugRespv18.Body.Close()
			debugBodyv18, _ := io.ReadAll(debugRespv18.Body)
			log.Printf("📋 v18.0 response: %s", string(debugBodyv18))
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
