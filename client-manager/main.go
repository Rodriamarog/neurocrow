// main.go
package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

var tmpl *template.Template

// Session management for demo authentication
type Session struct {
	Token     string
	ClientID  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

var (
	sessions     = make(map[string]*Session)
	sessionMutex = sync.RWMutex{}
)

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

	// Apply CORS to all routes - protected routes require authentication
	router.HandleFunc("/", corsMiddleware(handlePages))
	router.HandleFunc("/facebook-token", corsMiddleware(handleFacebookToken))
	router.HandleFunc("/instagram-token", corsMiddleware(handleInstagramToken))
	router.HandleFunc("/instagram-token-exchange", corsMiddleware(handleInstagramTokenExchange))
	router.HandleFunc("/activate-form", corsMiddleware(requireAuth(handleActivateForm)))
	router.HandleFunc("/activate-page", corsMiddleware(requireAuth(handleActivatePage)))
	router.HandleFunc("/deactivate-page", corsMiddleware(requireAuth(handleDeactivatePage)))
	router.HandleFunc("/insights", corsMiddleware(requireAuth(handleInsights)))
	router.HandleFunc("/posts", corsMiddleware(requireAuth(handlePagePosts)))
	router.HandleFunc("/pages", corsMiddleware(requireAuth(handleListPages)))
	router.HandleFunc("/logout", corsMiddleware(handleLogout))

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

// Session management functions
func generateSessionToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func createSession(clientID string) string {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	
	token := generateSessionToken()
	sessions[token] = &Session{
		Token:     token,
		ClientID:  clientID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24 hour expiry
	}
	
	log.Printf("🔐 Created session for client %s: %s", clientID, token[:16]+"...")
	return token
}

func validateSession(token string) (*Session, bool) {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()
	
	session, exists := sessions[token]
	if !exists {
		return nil, false
	}
	
	if time.Now().After(session.ExpiresAt) {
		// Session expired, clean it up
		delete(sessions, token)
		return nil, false
	}
	
	return session, true
}

func deleteSession(token string) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	
	delete(sessions, token)
	log.Printf("🔐 Deleted session: %s", token[:16]+"...")
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			// Try cookie as fallback
			if cookie, err := r.Cookie("session_token"); err == nil {
				token = cookie.Value
			}
		} else {
			// Remove "Bearer " prefix if present
			token = strings.TrimPrefix(token, "Bearer ")
		}
		
		if token == "" {
			log.Printf("🚫 No session token provided for %s", r.URL.Path)
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		
		session, valid := validateSession(token)
		if !valid {
			log.Printf("🚫 Invalid session token for %s: %s", r.URL.Path, token[:16]+"...")
			http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
			return
		}
		
		log.Printf("✅ Valid session for %s: client %s", r.URL.Path, session.ClientID)
		next(w, r)
	}
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
	
	// Create session for the authenticated user
	sessionToken := createSession(clientID)
	
	if socialTx != nil {
		log.Printf("✅ Successfully completed Facebook token request with webhook automation. Changes committed to both pages and social_pages tables.")
	} else {
		log.Printf("✅ Successfully completed Facebook token request with webhook automation. Changes committed to pages table only.")
	}
	
	// Return session token to client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"session_token": sessionToken,
		"client_id":    clientID,
		"message":      "Authentication successful",
	})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Get session token from header or cookie
	token := r.Header.Get("Authorization")
	if token == "" {
		if cookie, err := r.Cookie("session_token"); err == nil {
			token = cookie.Value
		}
	} else {
		token = strings.TrimPrefix(token, "Bearer ")
	}
	
	if token != "" {
		deleteSession(token)
		log.Printf("🔐 User logged out, session deleted")
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
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
			// Validate Instagram Business account
			if page.Instagram.Username == "" {
				log.Printf("⚠️ Instagram account %s (%s) appears to be missing username - may not be a Business account", 
					page.Instagram.Name, page.Instagram.ID)
			}
			
			allPages = append(allPages, FacebookPage{
				ID:          page.Instagram.ID,
				Name:        page.Instagram.Name,
				AccessToken: page.AccessToken, // Use same permanent page token
				Platform:    "instagram",
			})
			log.Printf("Added connected Instagram Business account: %s (@%s)", page.Instagram.Name, page.Instagram.Username)
		}
	}

	// Count Facebook vs Instagram accounts for user feedback
	fbCount := 0
	igCount := 0
	for _, page := range allPages {
		if page.Platform == "facebook" {
			fbCount++
		} else if page.Platform == "instagram" {
			igCount++
		}
	}
	
	log.Printf("Found total of %d pages/accounts: %d Facebook pages, %d Instagram Business accounts", 
		len(allPages), fbCount, igCount)
		
	// Provide helpful messaging for common scenarios
	if fbCount > 0 && igCount == 0 {
		log.Printf("ℹ️ No Instagram Business accounts found. To connect Instagram:")
		log.Printf("   1. Convert your Instagram account to a Business account")
		log.Printf("   2. Connect it to one of your Facebook Pages")
		log.Printf("   3. Ensure you have admin access to the connected Facebook Page")
	}
	
	return allPages, nil
}

// =============================================================================
// WEBHOOK SUBSCRIPTION AUTOMATION - For multi-tenant setup
// =============================================================================

// subscribePageToWebhooks subscribes a Facebook page to all required webhook events
// Platform-specific: Instagram doesn't support "messaging_handovers" field
func subscribePageToWebhooks(pageID, pageToken, platform string) error {
	appID := os.Getenv("FACEBOOK_APP_ID")
	if appID == "" {
		return fmt.Errorf("FACEBOOK_APP_ID environment variable not set")
	}

	// Subscribe page to the Neurocrow app for webhook events
	subscribeURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s/subscribed_apps", pageID)
	
	// Create platform-specific payload for subscribing to webhooks
	var subscribedFields []string
	
	if platform == "instagram" {
		// Instagram only supports basic messaging fields
		subscribedFields = []string{
			"messages",
			"messaging_postbacks",
		}
		log.Printf("📱 Using Instagram-specific webhook fields (messages, messaging_postbacks only)")
	} else {
		// Facebook pages support all fields including handovers and echoes
		subscribedFields = []string{
			"messages",
			"messaging_postbacks", 
			"messaging_handovers",
			"messaging_policy_enforcement",
			"message_echoes",
		}
		log.Printf("📘 Using Facebook-specific webhook fields (including messaging_handovers and message_echoes)")
	}
	
	subscribePayload := map[string]interface{}{
		"subscribed_fields": subscribedFields,
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

	log.Printf("🔗 Subscribing %s page %s to webhooks: %s", platform, pageID, subscribeURL)
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

	log.Printf("✅ Successfully subscribed %s page %s to webhooks", platform, pageID)
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
	// Facebook requires at least one additional field besides primary_receiver_app_id
	handoverURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s/messenger_profile", pageID)
	handoverPayload := map[string]interface{}{
		"primary_receiver_app_id": neurocrowAppID,
		"greeting": []map[string]interface{}{
			{
				"locale": "default",
				"text":   "Hola! Soy el asistente inteligente de esta página. ¿En qué puedo ayudarte hoy?",
			},
		},
		"get_started": map[string]interface{}{
			"payload": "GET_STARTED",
		},
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

	if platform == "instagram" {
		// Instagram webhooks are configured at app level in Facebook App Dashboard
		// No per-page API subscription needed - webhooks work automatically once configured in dashboard
		log.Printf("📱 Instagram webhooks configured at app level - no API subscription needed")
		log.Printf("ℹ️ Instagram account %s will receive webhooks via app-level configuration", pageName)
		log.Printf("✅ Instagram webhook setup completed for %s (app-level configuration)", pageName)
		return nil
	}

	// Facebook pages require individual API subscriptions
	log.Printf("📘 Facebook page requires individual API webhook subscription")

	// Step 1: Subscribe page to webhooks (Facebook only)
	if err := subscribePageToWebhooks(pageID, pageToken, platform); err != nil {
		log.Printf("❌ Webhook subscription failed for %s: %v", pageName, err)
		return fmt.Errorf("webhook subscription failed: %v", err)
	}

	// Step 2: Configure handover protocol (Facebook only)
	if err := configureHandoverProtocol(pageID, pageToken); err != nil {
		log.Printf("⚠️ Handover protocol setup failed for %s: %v", pageName, err)
		// Don't return error - handover protocol is optional, webhook subscription is more important
	}

	// Step 3: Verify the setup (Facebook only)
	if err := verifyWebhookSetup(pageID, pageToken); err != nil {
		log.Printf("⚠️ Webhook verification failed for %s: %v", pageName, err)
		// Don't return error - verification is informational
	}

	log.Printf("✅ Facebook webhook setup completed for %s page: %s", platform, pageName)
	return nil
}

// =============================================================================
// FACEBOOK PAGE INSIGHTS - For legitimate pages_read_engagement usage
// =============================================================================

type InsightsResponse struct {
	PageName   string                 `json:"page_name"`
	PageID     string                 `json:"page_id"`
	Platform   string                 `json:"platform"`
	Metrics    map[string]interface{} `json:"metrics"`
	TimeSeries []TimeSeriesPoint      `json:"time_series"`
	Period     string                 `json:"period"`
}

type TimeSeriesPoint struct {
	Date  string      `json:"date"`
	Value interface{} `json:"value"`
}

type FacebookInsightsData struct {
	Data []struct {
		Name   string `json:"name"`
		Period string `json:"period"`
		Values []struct {
			Value   interface{} `json:"value"`
			EndTime string      `json:"end_time"`
		} `json:"values"`
	} `json:"data"`
}

func handleInsights(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageID := r.URL.Query().Get("pageId")
	period := r.URL.Query().Get("period")
	
	// Default to last 7 days if no period specified
	if period == "" {
		period = "week"
	}

	if pageID == "" {
		http.Error(w, "pageId parameter is required", http.StatusBadRequest)
		return
	}

	log.Printf("📊 Fetching insights for page %s (period: %s)", pageID, period)

	// Get page info and access token from database
	var pageName, platform, accessToken string
	err := DB.QueryRow(`
		SELECT name, platform, access_token 
		FROM pages 
		WHERE page_id = $1 AND status IN ('active', 'pending')
	`, pageID).Scan(&pageName, &platform, &accessToken)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Page not found or not connected", http.StatusNotFound)
		} else {
			log.Printf("❌ Error querying page: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	log.Printf("📄 Found page: %s (%s) - Platform: %s", pageName, pageID, platform)

	// Get insights data from Facebook API
	insights, err := getPageInsights(pageID, accessToken, period, pageName, platform)
	if err != nil {
		log.Printf("❌ Error fetching insights: %v", err)
		http.Error(w, fmt.Sprintf("Failed to fetch insights: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(insights)
}

func getPageInsights(pageID, accessToken, period, pageName, platform string) (*InsightsResponse, error) {
	// Use simpler, more reliable metrics that are less likely to be deprecated
	// Based on 2024 Facebook API changes, many metrics have been deprecated
	metrics := []string{
		"page_fans",           // Total page likes (lifetime metric)
		"page_impressions",    // Still available but limited
	}

	// For testing, let's also try some basic metrics
	if platform == "facebook" {
		metrics = append(metrics, "page_views_total") // Basic page views
	}

	// Build metrics parameter correctly
	var quotedMetrics []string
	for _, metric := range metrics {
		quotedMetrics = append(quotedMetrics, fmt.Sprintf(`"%s"`, metric))
	}
	metricsParam := fmt.Sprintf("[%s]", strings.Join(quotedMetrics, ","))

	// Determine period parameter for Facebook API
	var fbPeriod string
	switch period {
	case "day":
		fbPeriod = "day"
	case "week":
		fbPeriod = "week" 
	case "month", "28days":
		fbPeriod = "days_28"
	default:
		fbPeriod = "week"
	}

	// Call Facebook Insights API
	insightsURL := fmt.Sprintf(
		"https://graph.facebook.com/v23.0/%s/insights?metric=%s&period=%s&access_token=%s",
		pageID, metricsParam, fbPeriod, accessToken,
	)

	log.Printf("🔍 Facebook Insights API call: %s", insightsURL)
	log.Printf("🔍 Requesting metrics: %s", metricsParam)
	log.Printf("🔍 Using period: %s", fbPeriod)

	resp, err := http.Get(insightsURL)
	if err != nil {
		return nil, fmt.Errorf("error calling Facebook API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Facebook response: %w", err)
	}

	log.Printf("📥 Facebook Insights response: %d - %s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		var fbError struct {
			Error struct {
				Message   string `json:"message"`
				Type      string `json:"type"`
				Code      int    `json:"code"`
				FbtraceID string `json:"fbtrace_id"`
			} `json:"error"`
		}
		
		if json.Unmarshal(bodyBytes, &fbError) == nil && fbError.Error.Message != "" {
			return nil, fmt.Errorf("Facebook API error: %s (Code: %d, Trace: %s)", 
				fbError.Error.Message, fbError.Error.Code, fbError.Error.FbtraceID)
		}
		return nil, fmt.Errorf("Facebook API error: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var fbData FacebookInsightsData
	if err := json.Unmarshal(bodyBytes, &fbData); err != nil {
		return nil, fmt.Errorf("error parsing Facebook insights data: %w", err)
	}

	// Process and format the data
	response := &InsightsResponse{
		PageName:   pageName,
		PageID:     pageID,
		Platform:   platform,
		Metrics:    make(map[string]interface{}),
		TimeSeries: []TimeSeriesPoint{},
		Period:     period,
	}

	// Check if we got any data
	if len(fbData.Data) == 0 {
		log.Printf("⚠️ Facebook Insights returned empty data array - trying basic page info instead")
		
		// Try to get basic page information as fallback
		pageInfoURL := fmt.Sprintf(
			"https://graph.facebook.com/v23.0/%s?fields=id,name,fan_count,followers_count&access_token=%s",
			pageID, accessToken,
		)
		
		log.Printf("🔍 Trying basic page info: %s", pageInfoURL)
		
		pageResp, err := http.Get(pageInfoURL)
		if err == nil {
			defer pageResp.Body.Close()
			pageBody, _ := io.ReadAll(pageResp.Body)
			log.Printf("📄 Basic page info response: %d - %s", pageResp.StatusCode, string(pageBody))
			
			if pageResp.StatusCode == http.StatusOK {
				var pageInfo struct {
					ID             string `json:"id"`
					Name           string `json:"name"`
					FanCount       int    `json:"fan_count"`
					FollowersCount int    `json:"followers_count"`
				}
				
				if json.Unmarshal(pageBody, &pageInfo) == nil {
					response.Metrics["page_fans"] = pageInfo.FanCount
					response.Metrics["followers_count"] = pageInfo.FollowersCount
					log.Printf("✅ Added basic page metrics: fans=%d, followers=%d", pageInfo.FanCount, pageInfo.FollowersCount)
				}
			}
		}
	} else {
		// Process each metric normally
		for _, metric := range fbData.Data {
			if len(metric.Values) > 0 {
				// Get the latest value for summary metrics
				latestValue := metric.Values[len(metric.Values)-1].Value
				response.Metrics[metric.Name] = latestValue

				// Create time series data
				for _, value := range metric.Values {
					response.TimeSeries = append(response.TimeSeries, TimeSeriesPoint{
						Date:  value.EndTime,
						Value: value.Value,
					})
				}
			}
		}
	}

	log.Printf("✅ Successfully processed insights for %s: %d metrics, %d time series points", 
		pageName, len(response.Metrics), len(response.TimeSeries))

	return response, nil
}

func handleListPages(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("📋 Listing pages request received")

	// Get pages filtered by specific client_id for Test User
	rows, err := DB.Query(`
		SELECT p.page_id, p.name, p.platform, p.client_id, c.name as client_name, p.status
		FROM pages p
		LEFT JOIN clients c ON p.client_id = c.id
		WHERE p.status IN ('active', 'pending') 
		AND p.client_id = $1
		ORDER BY p.created_at DESC
	`, "d35f63a2-b265-4bba-9cb9-84f885a8b186")
	if err != nil {
		log.Printf("❌ Error querying pages: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var pages []map[string]string
	for rows.Next() {
		var pageID, name, platform, clientID, clientName, status string
		if err := rows.Scan(&pageID, &name, &platform, &clientID, &clientName, &status); err != nil {
			log.Printf("❌ Error scanning page row: %v", err)
			continue
		}
		
		pageInfo := map[string]string{
			"page_id":     pageID,
			"name":        name,
			"platform":    platform,
			"client_id":   clientID,
			"client_name": clientName,
			"status":      status,
		}
		pages = append(pages, pageInfo)
		
		log.Printf("📄 Found page: %s (%s) - Client: %s (%s) - Status: %s", 
			name, platform, clientName, clientID, status)
	}

	log.Printf("📊 Total pages found: %d", len(pages))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pages": pages,
		"count": len(pages),
		"debug": "This endpoint returns ALL pages from ALL users - needs user filtering",
	})
}

// =============================================================================
// FACEBOOK PAGE POSTS - For legitimate pages_read_engagement usage
// =============================================================================

type PagePost struct {
	ID          string               `json:"id"`
	Message     string               `json:"message,omitempty"`
	Story       string               `json:"story,omitempty"`
	CreatedTime string               `json:"created_time"`
	From        PagePostFrom         `json:"from"`
	Likes       PagePostMetric       `json:"likes"`
	Comments    PagePostMetric       `json:"comments"`
	Shares      PagePostShares       `json:"shares,omitempty"`
	FullPicture string               `json:"full_picture,omitempty"`
	Attachments PagePostAttachments  `json:"attachments,omitempty"`
}

type PagePostFrom struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type PagePostMetric struct {
	Summary PagePostSummary `json:"summary"`
}

type PagePostSummary struct {
	TotalCount int `json:"total_count"`
}

type PagePostShares struct {
	Count int `json:"count"`
}

type PagePostAttachments struct {
	Data []PagePostAttachment `json:"data,omitempty"`
}

type PagePostAttachment struct {
	Media       PagePostMedia `json:"media,omitempty"`
	Title       string        `json:"title,omitempty"`
	Description string        `json:"description,omitempty"`
	URL         string        `json:"url,omitempty"`
}

type PagePostMedia struct {
	Image PagePostImage `json:"image,omitempty"`
}

type PagePostImage struct {
	Height int    `json:"height,omitempty"`
	Width  int    `json:"width,omitempty"`
	Src    string `json:"src,omitempty"`
}

type PagePostsResponse struct {
	PageName        string       `json:"page_name"`
	PageID          string       `json:"page_id"`
	Platform        string       `json:"platform"`
	Posts           []PagePost   `json:"posts"`
	Count           int          `json:"count"`
	ProfilePicture  string       `json:"profile_picture,omitempty"`
	FollowerCount   int64        `json:"follower_count,omitempty"`
	LikeCount       int64        `json:"like_count,omitempty"`
	About           string       `json:"about,omitempty"`
	Website         string       `json:"website,omitempty"`
}

func handlePagePosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageID := r.URL.Query().Get("pageId")
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "10"
	}

	if pageID == "" {
		http.Error(w, "pageId parameter is required", http.StatusBadRequest)
		return
	}

	log.Printf("📱 Fetching posts for page %s (limit: %s)", pageID, limit)

	// Get page info and access token from database
	var pageName, platform, accessToken string
	err := DB.QueryRow(`
		SELECT name, platform, access_token 
		FROM pages 
		WHERE page_id = $1 AND status IN ('active', 'pending')
	`, pageID).Scan(&pageName, &platform, &accessToken)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Page not found or not connected", http.StatusNotFound)
		} else {
			log.Printf("❌ Error querying page: %v", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	log.Printf("📄 Found page: %s (%s) - Platform: %s", pageName, pageID, platform)

	// Get posts data from Facebook API
	posts, err := getPagePosts(pageID, accessToken, limit, pageName, platform)
	if err != nil {
		log.Printf("❌ Error fetching posts: %v", err)
		http.Error(w, fmt.Sprintf("Failed to fetch posts: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func getPagePosts(pageID, accessToken, limit, pageName, platform string) (*PagePostsResponse, error) {
	// First get page basic info
	pageInfoURL := fmt.Sprintf(
		"https://graph.facebook.com/v23.0/%s?fields=name,picture,fan_count,followers_count,about,website&access_token=%s",
		pageID, accessToken,
	)

	log.Printf("🔍 Facebook Page Info API call: %s", pageInfoURL)

	pageResp, err := http.Get(pageInfoURL)
	if err != nil {
		log.Printf("⚠️ Error calling Facebook Page Info API (continuing with posts): %v", err)
	}
	
	var pageInfo struct {
		Name          string `json:"name"`
		Picture       struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
		FanCount       int64  `json:"fan_count"`
		FollowersCount int64  `json:"followers_count"`
		About          string `json:"about"`
		Website        string `json:"website"`
	}
	
	if pageResp != nil {
		defer pageResp.Body.Close()
		if pageResp.StatusCode == http.StatusOK {
			if pageBody, err := io.ReadAll(pageResp.Body); err == nil {
				json.Unmarshal(pageBody, &pageInfo)
				log.Printf("✅ Page info loaded: %s, followers: %d", pageInfo.Name, pageInfo.FollowersCount)
			}
		}
	}

	// Call Facebook Feed API with enhanced fields including images
	postsURL := fmt.Sprintf(
		"https://graph.facebook.com/v23.0/%s/feed?fields=id,message,story,created_time,from,likes.summary(true),comments.summary(true),shares,full_picture,attachments{media,url,title,description}&limit=%s&access_token=%s",
		pageID, limit, accessToken,
	)

	log.Printf("🔍 Facebook Posts API call: %s", postsURL)

	resp, err := http.Get(postsURL)
	if err != nil {
		return nil, fmt.Errorf("error calling Facebook API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Facebook response: %w", err)
	}

	log.Printf("📥 Facebook Posts response: %d - %s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		var fbError struct {
			Error struct {
				Message   string `json:"message"`
				Type      string `json:"type"`
				Code      int    `json:"code"`
				FbtraceID string `json:"fbtrace_id"`
			} `json:"error"`
		}
		
		if json.Unmarshal(bodyBytes, &fbError) == nil && fbError.Error.Message != "" {
			return nil, fmt.Errorf("Facebook API error: %s (Code: %d, Trace: %s)", 
				fbError.Error.Message, fbError.Error.Code, fbError.Error.FbtraceID)
		}
		return nil, fmt.Errorf("Facebook API error: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var fbData struct {
		Data []PagePost `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &fbData); err != nil {
		return nil, fmt.Errorf("error parsing Facebook posts data: %w", err)
	}

	// Build response
	response := &PagePostsResponse{
		PageName:       pageName,
		PageID:         pageID,
		Platform:       platform,
		Posts:          fbData.Data,
		Count:          len(fbData.Data),
		ProfilePicture: pageInfo.Picture.Data.URL,
		FollowerCount:  pageInfo.FollowersCount,
		LikeCount:      pageInfo.FanCount,
		About:          pageInfo.About,
		Website:        pageInfo.Website,
	}

	log.Printf("✅ Successfully processed posts for %s: %d posts found", 
		pageName, len(fbData.Data))

	return response, nil
}

// handleInstagramTokenExchange handles the Instagram OAuth code exchange for access token
func handleInstagramTokenExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("=== Starting Instagram token exchange ===")

	var data struct {
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("❌ Error decoding Instagram token exchange request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if data.Code == "" || data.RedirectURI == "" {
		log.Printf("❌ Missing required fields: code=%s, redirect_uri=%s", data.Code, data.RedirectURI)
		http.Error(w, "Missing code or redirect_uri", http.StatusBadRequest)
		return
	}

	log.Printf("📥 Instagram token exchange request: code=%s, redirect_uri=%s", data.Code, data.RedirectURI)

	// Exchange authorization code for access token using Instagram API
	instagramAppId := os.Getenv("INSTAGRAM_APP_ID")
	instagramAppSecret := os.Getenv("INSTAGRAM_APP_SECRET_KEY")
	
	if instagramAppId == "" || instagramAppSecret == "" {
		log.Printf("❌ Missing Instagram app credentials")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Build token exchange request for Instagram
	tokenURL := "https://api.instagram.com/oauth/access_token"
	
	// Prepare form data for POST request
	formData := fmt.Sprintf(
		"client_id=%s&client_secret=%s&grant_type=authorization_code&redirect_uri=%s&code=%s",
		instagramAppId, instagramAppSecret, data.RedirectURI, data.Code,
	)

	log.Printf("🔗 Making token exchange request to Instagram API")

	// Make the token exchange request (POST with form data)
	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(formData))
	if err != nil {
		log.Printf("❌ Error making token exchange request: %v", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Error reading token exchange response: %v", err)
		http.Error(w, "Failed to read token response", http.StatusInternalServerError)
		return
	}

	log.Printf("📥 Instagram token exchange response: %d - %s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		var igError struct {
			Error struct {
				Message string `json:"error_message"`
				Type    string `json:"error_type"`
				Code    int    `json:"code"`
			} `json:"error"`
			ErrorMessage string `json:"error_message"`
			ErrorType    string `json:"error_type"`
		}
		
		if json.Unmarshal(bodyBytes, &igError) == nil {
			errorMsg := igError.ErrorMessage
			if errorMsg == "" && igError.Error.Message != "" {
				errorMsg = igError.Error.Message
			}
			if errorMsg != "" {
				log.Printf("❌ Instagram token exchange error: %s (Type: %s)", errorMsg, igError.ErrorType)
				http.Error(w, fmt.Sprintf("Instagram API error: %s", errorMsg), http.StatusBadRequest)
			} else {
				log.Printf("❌ Instagram token exchange failed with status %d", resp.StatusCode)
				http.Error(w, "Token exchange failed", http.StatusBadRequest)
			}
		} else {
			log.Printf("❌ Instagram token exchange failed with status %d", resp.StatusCode)
			http.Error(w, "Token exchange failed", http.StatusBadRequest)
		}
		return
	}

	// Parse successful token response
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.Unmarshal(bodyBytes, &tokenResponse); err != nil {
		log.Printf("❌ Error parsing token response: %v", err)
		http.Error(w, "Failed to parse token response", http.StatusInternalServerError)
		return
	}

	if tokenResponse.AccessToken == "" {
		log.Printf("❌ No access token in response")
		http.Error(w, "No access token received", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Successfully exchanged Instagram authorization code for access token")

	// Return the access token to the client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token": tokenResponse.AccessToken,
		"token_type":   tokenResponse.TokenType,
		"expires_in":   tokenResponse.ExpiresIn,
		"success":      true,
	})
}

// handleInstagramToken handles Instagram access tokens and sets up Instagram Business accounts
func handleInstagramToken(w http.ResponseWriter, r *http.Request) {
	log.Printf("=== Starting Instagram token request handling ===")

	var data struct {
		UserToken string `json:"userToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("❌ Error decoding request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Get Instagram user details
	instagramUser, err := getInstagramUser(data.UserToken)
	if err != nil {
		log.Printf("❌ Error getting Instagram user details: %v", err)
		http.Error(w, fmt.Sprintf("Could not verify Instagram user: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Get Instagram Business accounts
	accounts, err := getInstagramBusinessAccounts(data.UserToken)
	if err != nil {
		log.Printf("❌ Error getting Instagram accounts: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Found %d Instagram Business accounts", len(accounts))

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
    `, instagramUser.Username, instagramUser.ID).Scan(&clientID)
	if err != nil {
		log.Printf("❌ Error upserting client in client-manager database: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Upserted client in client-manager database with ID: %s", clientID)

	// 4b. Also create or update client in social dashboard database
	if socialTx != nil {
		_, err = socialTx.Exec(`
            INSERT INTO clients (id, name, facebook_user_id, created_at)
            VALUES ($1, $2, $3, NOW())
            ON CONFLICT (id) DO UPDATE
            SET name = EXCLUDED.name,
                facebook_user_id = EXCLUDED.facebook_user_id
        `, clientID, instagramUser.Username, instagramUser.ID)
		if err != nil {
			log.Printf("❌ Error upserting client in social dashboard database: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Upserted client in social dashboard database with ID: %s", clientID)
	}

	// 5. Disable client's previous Instagram accounts
	result, err := tx.Exec(`
        UPDATE pages 
        SET status = 'disabled'
        WHERE client_id = $1 
        AND platform = 'instagram'
    `, clientID)
	if err != nil {
		log.Printf("❌ Error disabling previous Instagram accounts: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rowsAffected, _ := result.RowsAffected()
	log.Printf("✅ Disabled %d previous Instagram accounts", rowsAffected)

	// 6. Insert/update new Instagram accounts in BOTH tables
	for _, account := range accounts {
		log.Printf("📝 Processing Instagram account %s (ID: %s)", account.Name, account.ID)

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
                $1, $2, $3, $4, 'instagram',
                CASE 
                    WHEN EXISTS (
                        SELECT 1 FROM pages 
                        WHERE platform = 'instagram' 
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
        `, clientID, account.ID, account.Name, account.AccessToken)

		if err != nil {
			log.Printf("❌ Error processing Instagram account in pages table %s: %v", account.Name, err)
			continue
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("✅ Successfully processed Instagram account in pages table %s (rows affected: %d)", account.Name, rowsAffected)

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
                    $1, 'instagram', $2, $3, $4, NOW()
                )
                ON CONFLICT (platform, page_id) 
                DO UPDATE SET 
                    client_id = EXCLUDED.client_id,
                    page_name = EXCLUDED.page_name,
                    access_token = EXCLUDED.access_token
            `, clientID, account.ID, account.Name, account.AccessToken)

			if err != nil {
				log.Printf("❌ Error processing Instagram account in social_pages table %s: %v", account.Name, err)
				continue
			}

			socialRowsAffected, _ := socialResult.RowsAffected()
			log.Printf("✅ Successfully processed Instagram account in social_pages table %s (rows affected: %d)", account.Name, socialRowsAffected)
		} else {
			log.Printf("⚠️ Social database not available, skipping social_pages update for %s", account.Name)
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

	// 8. Note: Instagram webhook setup is handled at app level, not per-account
	log.Printf("ℹ️ Instagram webhooks are configured at app level in Facebook App Dashboard")

	// Create session for the authenticated user
	sessionToken := createSession(clientID)
	
	log.Printf("✅ Successfully completed Instagram token request.")
	
	// Return session token to client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"session_token": sessionToken,
		"client_id":    clientID,
		"message":      "Instagram authentication successful",
	})
}

// InstagramUser represents an Instagram user
type InstagramUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// InstagramAccount represents an Instagram Business account
type InstagramAccount struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Username    string `json:"username"`
	AccessToken string `json:"access_token"`
}

// getInstagramUser gets Instagram user information
func getInstagramUser(accessToken string) (*InstagramUser, error) {
	url := fmt.Sprintf("https://graph.instagram.com/me?fields=id,username&access_token=%s", accessToken)
	log.Printf("📡 Getting Instagram user info from: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making request to Instagram API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Instagram API response: %w", err)
	}

	log.Printf("📥 Instagram user API response: %d - %s", resp.StatusCode, string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		var igError struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		
		if json.Unmarshal(bodyBytes, &igError) == nil && igError.Error.Message != "" {
			return nil, fmt.Errorf("Instagram API error: %s", igError.Error.Message)
		}
		return nil, fmt.Errorf("Instagram API error: status %d", resp.StatusCode)
	}

	var user InstagramUser
	if err := json.Unmarshal(bodyBytes, &user); err != nil {
		return nil, fmt.Errorf("error parsing Instagram user response: %w", err)
	}

	if user.ID == "" || user.Username == "" {
		return nil, fmt.Errorf("incomplete user data from Instagram API")
	}

	log.Printf("✅ Successfully got Instagram user: %s (%s)", user.Username, user.ID)
	return &user, nil
}

// getInstagramBusinessAccounts gets Instagram Business accounts for the user
func getInstagramBusinessAccounts(accessToken string) ([]InstagramAccount, error) {
	// Note: Instagram Business API typically requires getting accounts through 
	// Facebook Pages that have connected Instagram Business accounts
	// For now, we'll create a basic account entry using the user's info
	
	user, err := getInstagramUser(accessToken)
	if err != nil {
		return nil, fmt.Errorf("error getting user info: %w", err)
	}

	// Create an account entry for the authenticated Instagram Business account
	account := InstagramAccount{
		ID:          user.ID,
		Name:        user.Username,
		Username:    user.Username,
		AccessToken: accessToken,
	}

	log.Printf("✅ Created Instagram Business account entry: %s (%s)", account.Name, account.ID)
	
	return []InstagramAccount{account}, nil
}
