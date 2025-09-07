// oauth/instagram.go
// Instagram OAuth handlers and API functions - EXACT COPY from client-manager

package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// =============================================================================
// COPY THE FOLLOWING FUNCTIONS EXACTLY FROM CLIENT-MANAGER/MAIN.GO:
// =============================================================================
//
// 1. handleInstagramTokenExchange function (lines 1865-1993)
//    - Copy exactly as-is
//
// 2. handleInstagramToken function (lines 1994-2217)
//    - Copy exactly as-is
//
// 3. getInstagramUser function (lines 2218+, if exists)
//    - Copy exactly as-is
//
// 4. getInstagramBusinessAccounts function (lines 2264+, if exists)
//    - Copy exactly as-is
//
// =============================================================================

func HandleInstagramTokenExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	LogInfo("=== Starting Instagram token exchange ===")

	var data struct {
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		LogError("Error decoding Instagram token exchange request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if data.Code == "" || data.RedirectURI == "" {
		LogError("Missing required fields: code=%s, redirect_uri=%s", data.Code, data.RedirectURI)
		http.Error(w, "Missing code or redirect_uri", http.StatusBadRequest)
		return
	}

	LogInfo("Instagram token exchange request: code=%s, redirect_uri=%s", data.Code, data.RedirectURI)

	// Exchange authorization code for access token using Instagram API
	instagramAppId := os.Getenv("INSTAGRAM_APP_ID")
	instagramAppSecret := os.Getenv("INSTAGRAM_APP_SECRET_KEY")

	if instagramAppId == "" || instagramAppSecret == "" {
		LogError("Missing Instagram app credentials")
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

	LogInfo("Making token exchange request to Instagram API")

	// Make the token exchange request (POST with form data)
	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(formData))
	if err != nil {
		LogError("Error making token exchange request: %v", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		LogError("Error reading token exchange response: %v", err)
		http.Error(w, "Failed to read token response", http.StatusInternalServerError)
		return
	}

	LogDebug("Instagram token exchange response: %d - %s", resp.StatusCode, string(bodyBytes))

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
			if errorMsg == "" {
				errorMsg = igError.Error.Message
			}
			LogError("Instagram API error: %s", errorMsg)
			http.Error(w, fmt.Sprintf("Instagram API error: %s", errorMsg), http.StatusBadRequest)
			return
		}

		LogError("Instagram token exchange failed: %s", string(bodyBytes))
		http.Error(w, "Token exchange failed", http.StatusBadRequest)
		return
	}

	// Parse successful response
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.Unmarshal(bodyBytes, &tokenResponse); err != nil {
		LogError("Error parsing token response: %v", err)
		http.Error(w, "Failed to parse token response", http.StatusInternalServerError)
		return
	}

	if tokenResponse.AccessToken == "" {
		LogError("No access token in response")
		http.Error(w, "No access token received", http.StatusInternalServerError)
		return
	}

	LogInfo("Successfully exchanged Instagram authorization code for access token")

	// Return the access token to the client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token": tokenResponse.AccessToken,
		"token_type":   tokenResponse.TokenType,
		"expires_in":   tokenResponse.ExpiresIn,
		"success":      true,
	})
}

func getInstagramUser(accessToken string) (*InstagramUser, error) {
	url := fmt.Sprintf("https://graph.instagram.com/me?fields=id,username&access_token=%s", accessToken)
	LogDebug("Getting Instagram user info from: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making request to Instagram API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Instagram API response: %w", err)
	}

	LogDebug("Instagram user API response: %d - %s", resp.StatusCode, string(bodyBytes))

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

	LogInfo("Successfully got Instagram user: %s (%s)", user.Username, user.ID)
	return &user, nil
}

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

	LogInfo("Created Instagram Business account entry: %s (%s)", account.Name, account.ID)

	return []InstagramAccount{account}, nil
}

func getInstagramAccountsViaFacebook(facebookAccessToken string) ([]InstagramAccount, error) {
	LogInfo("Getting Instagram Business accounts through Facebook Pages...")

	// First get Facebook Pages for this user
	pages, err := getConnectedPages(facebookAccessToken)
	if err != nil {
		return nil, fmt.Errorf("error getting Facebook pages: %w", err)
	}

	var instagramAccounts []InstagramAccount

	// For each Facebook Page, check if it has a connected Instagram Business account
	for _, page := range pages {
		LogDebug("Checking page %s for connected Instagram account...", page.Name)

		// Get Instagram Business account ID for this page
		url := fmt.Sprintf("https://graph.facebook.com/v23.0/%s?fields=instagram_business_account&access_token=%s", page.ID, page.AccessToken)

		resp, err := http.Get(url)
		if err != nil {
			LogError("Error checking Instagram connection for page %s: %v", page.Name, err)
			continue
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			LogError("Error reading response for page %s: %v", page.Name, err)
			continue
		}

		var pageData struct {
			InstagramBusinessAccount struct {
				ID string `json:"id"`
			} `json:"instagram_business_account"`
		}

		if err := json.Unmarshal(bodyBytes, &pageData); err != nil {
			LogError("Error parsing response for page %s: %v", page.Name, err)
			continue
		}

		// If this page has a connected Instagram Business account
		if pageData.InstagramBusinessAccount.ID != "" {
			LogInfo("Found Instagram Business account %s connected to page %s", pageData.InstagramBusinessAccount.ID, page.Name)

			// Get Instagram account details
			igUrl := fmt.Sprintf("https://graph.facebook.com/v23.0/%s?fields=id,username,name&access_token=%s",
				pageData.InstagramBusinessAccount.ID, page.AccessToken)

			igResp, err := http.Get(igUrl)
			if err != nil {
				LogError("Error getting Instagram account details: %v", err)
				continue
			}
			defer igResp.Body.Close()

			igBodyBytes, err := io.ReadAll(igResp.Body)
			if err != nil {
				LogError("Error reading Instagram account response: %v", err)
				continue
			}

			var igAccount struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Name     string `json:"name"`
			}

			if err := json.Unmarshal(igBodyBytes, &igAccount); err != nil {
				LogError("Error parsing Instagram account response: %v", err)
				continue
			}

			// Create Instagram account entry
			account := InstagramAccount{
				ID:          igAccount.ID,
				Name:        igAccount.Name,
				Username:    igAccount.Username,
				AccessToken: page.AccessToken, // Use the page token for Instagram API calls
			}

			instagramAccounts = append(instagramAccounts, account)
			LogInfo("Added Instagram Business account: %s (@%s)", account.Name, account.Username)
		} else {
			LogInfo("Page %s has no connected Instagram Business account", page.Name)
		}
	}

	LogInfo("Found %d Instagram Business accounts total", len(instagramAccounts))
	return instagramAccounts, nil
}

func HandleInstagramToken(w http.ResponseWriter, r *http.Request) {
	LogInfo("=== Starting Instagram token request handling ===")

	var data struct {
		UserToken string `json:"userToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		LogError("Error decoding request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Get Instagram user details
	instagramUser, err := getInstagramUser(data.UserToken)
	if err != nil {
		LogError("Error getting Instagram user details: %v", err)
		http.Error(w, fmt.Sprintf("Could not verify Instagram user: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Get Instagram Business accounts
	accounts, err := getInstagramBusinessAccounts(data.UserToken)
	if err != nil {
		LogError("Error getting Instagram accounts: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	LogInfo("Found %d Instagram Business accounts", len(accounts))

	// 3. Start transaction for single database
	tx, err := SocialDB.Begin()
	if err != nil {
		LogError("Error starting database transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 4. Create or update client
	var clientID string
	err = tx.QueryRow(`
        INSERT INTO clients (name, facebook_user_id, created_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT (facebook_user_id) DO UPDATE
        SET name = EXCLUDED.name
        RETURNING id
    `, instagramUser.Username, instagramUser.ID).Scan(&clientID)
	if err != nil {
		LogError("Error upserting client: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	LogInfo("Upserted client with ID: %s", clientID)

	// 5. Insert/update Instagram accounts in social_pages table
	for _, account := range accounts {
		LogInfo("Processing Instagram account %s (ID: %s)", account.Name, account.ID)

		_, err := tx.Exec(`
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
        `, clientID, "instagram", account.ID, account.Name, account.AccessToken)

		if err != nil {
			LogError("Error processing Instagram account %s: %v", account.Name, err)
			continue
		}

		LogInfo("Successfully processed Instagram account %s", account.Name)
	}

	// 6. Commit transaction
	if err = tx.Commit(); err != nil {
		LogError("Error committing transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	LogInfo("Database transaction committed successfully")

	// 7. Set up webhook subscriptions for Instagram accounts (after database commit)
	LogInfo("Starting webhook subscription automation for %d Instagram accounts", len(accounts))
	webhookSuccessCount := 0
	for _, account := range accounts {
		LogInfo("Setting up webhooks for Instagram account: %s", account.Name)

		// Set up webhook subscriptions automatically
		if err := setupWebhookSubscriptions(account.ID, account.AccessToken, account.Name, "instagram"); err != nil {
			LogError("Webhook setup failed for %s: %v", account.Name, err)
			// Don't fail the entire request - webhook setup is best effort
		} else {
			webhookSuccessCount++
			LogInfo("Webhook setup completed for %s", account.Name)
		}
	}

	LogInfo("Webhook automation summary: %d/%d Instagram accounts configured successfully", webhookSuccessCount, len(accounts))

	LogInfo("Successfully completed Instagram token request with webhook automation")

	// Return success response (no session token needed)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"client_id": clientID,
		"message":   "Instagram authentication successful",
	})
}
