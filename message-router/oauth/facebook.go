// oauth/facebook.go
// Facebook OAuth handlers and API functions - EXACT COPY from client-manager

package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// =============================================================================
// COPY THE FOLLOWING FUNCTIONS EXACTLY FROM CLIENT-MANAGER/MAIN.GO:
// =============================================================================
//
// 1. handleFacebookToken function (lines 349-576)
//    - REMOVE session token creation at the end (lines 559-560)
//    - Change return response to not include session_token (line 572)
//
// 2. getFacebookUser function (lines 607-663)
//    - Copy exactly as-is
//
// 3. getConnectedPages function (lines 665-1006)
//    - Copy exactly as-is
//
// =============================================================================

func getFacebookUser(token string) (*FacebookUser, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v23.0/me?fields=id,name&access_token=%s", token)
	LogDebug("Attempting to get Facebook user details from: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		LogError("Error making HTTP request to Facebook: %v", err)
		return nil, fmt.Errorf("error fetching user info from Facebook: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		LogError("Error reading response body from Facebook: %v", readErr)
		return nil, fmt.Errorf("error reading Facebook response body: %w", readErr)
	}
	// Log the raw response body for debugging, regardless of status code.
	LogDebug("Facebook get user response status: %s, body: %s", resp.Status, string(bodyBytes))

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
			LogError("Facebook API error (parsed from body). Message: %s, Type: %s, Code: %d, Trace: %s",
				fbError.Error.Message, fbError.Error.Type, fbError.Error.Code, fbError.Error.FbtraceID)
		} else {
			// Log if parsing the error structure itself failed
			LogError("Facebook API error (could not parse error JSON from body). Body was: %s", string(bodyBytes))
		}
		// Return a generic error message to the caller, specific details are logged.
		return nil, fmt.Errorf("Facebook API error (%s)", resp.Status)
	}

	var user FacebookUser
	// Try to unmarshal the bodyBytes into the User struct
	if err := json.Unmarshal(bodyBytes, &user); err != nil {
		LogError("Error parsing Facebook user info from successful (200 OK) response body: %v. Body was: %s", err, string(bodyBytes))
		return nil, fmt.Errorf("error parsing user info from Facebook response: %w", err)
	}

	// Basic validation that we got the essential fields
	if user.ID == "" || user.Name == "" {
		LogError("Facebook user details incomplete - ID: %s, Name: %s", user.ID, user.Name)
		return nil, fmt.Errorf("incomplete user data from Facebook")
	}

	// If everything is successful, log the details.
	LogInfo("Successfully fetched Facebook user: ID %s, Name %s", user.ID, user.Name)
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

	LogInfo("Getting long-lived user token (60 days)")
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
	LogDebug("Long-lived token response status: %s, body: %s", longLivedResp.Status, string(longLivedBodyBytes))

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
		LogError("Facebook long-lived token error: %s (Type: %s, Code: %d)",
			longLivedResult.Error.Message, longLivedResult.Error.Type, longLivedResult.Error.Code)
		return nil, fmt.Errorf("Facebook long-lived token error: %s", longLivedResult.Error.Message)
	}

	if longLivedResult.AccessToken == "" {
		LogError("No access token received in long-lived token response")
		return nil, fmt.Errorf("no access token received from Facebook")
	}

	LogInfo("Successfully obtained long-lived user token (60 days, NOT permanent)")

	// Use the long-lived user token to get pages (page tokens will be permanent)
	fbURL := fmt.Sprintf(
		"https://graph.facebook.com/v23.0/me/accounts?"+
			"access_token=%s&"+
			"fields=id,name,access_token,instagram_business_account{id,name,username}",
		longLivedResult.AccessToken,
	)

	LogDebug("Fetching Facebook pages and connected Instagram accounts from: %s", fbURL)
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
	LogDebug("Facebook pages response status: %s, body: %s", fbResp.Status, string(fbBodyBytes))

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
		LogError("Facebook pages API error: %s (Type: %s, Code: %d, Trace: %s)",
			fbResult.Error.Message, fbResult.Error.Type, fbResult.Error.Code, fbResult.Error.FbtraceID)
		return nil, fmt.Errorf("Facebook pages API error: %s", fbResult.Error.Message)
	}

	var allPages []FacebookPage

	// Add Facebook pages and their connected Instagram accounts
	for _, page := range fbResult.Data {
		// Add Facebook page with permanent page token
		allPages = append(allPages, FacebookPage{
			ID:          page.ID,
			Name:        page.Name,
			AccessToken: page.AccessToken, // This IS a permanent page token (never expires)
			Platform:    "facebook",
		})
		LogInfo("Added Facebook page: %s", page.Name)

		// If this page has a connected Instagram account, add it
		if page.Instagram.ID != "" {
			// Validate Instagram Business account
			if page.Instagram.Username == "" {
				LogInfo("Instagram account %s (%s) appears to be missing username - may not be a Business account",
					page.Instagram.Name, page.Instagram.ID)
			}

			allPages = append(allPages, FacebookPage{
				ID:          page.Instagram.ID,
				Name:        page.Instagram.Name,
				AccessToken: page.AccessToken, // Use same permanent page token
				Platform:    "instagram",
			})
			LogInfo("Added connected Instagram Business account: %s (@%s)", page.Instagram.Name, page.Instagram.Username)
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

	LogInfo("Found total of %d pages/accounts: %d Facebook pages, %d Instagram Business accounts",
		len(allPages), fbCount, igCount)

	// Provide helpful messaging for common scenarios
	if fbCount > 0 && igCount == 0 {
		LogInfo("No Instagram Business accounts found. To connect Instagram:")
		LogInfo("   1. Convert your Instagram account to a Business account")
		LogInfo("   2. Connect it to one of your Facebook Pages")
		LogInfo("   3. Ensure you have admin access to the connected Facebook Page")
	}

	return allPages, nil
}

func HandleFacebookToken(w http.ResponseWriter, r *http.Request) {
	LogInfo("=== Starting Facebook token request handling ===")

	var data struct {
		UserToken string `json:"userToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		LogError("Error decoding request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Get user details from Facebook
	fbUser, err := getFacebookUser(data.UserToken)
	if err != nil {
		LogError("Error getting Facebook user details: %v", err)
		http.Error(w, fmt.Sprintf("Could not verify Facebook user: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Get connected pages
	pages, err := getConnectedPages(data.UserToken)
	if err != nil {
		LogError("Error getting pages: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	LogInfo("Found %d connected pages/accounts", len(pages))

	// 3. Start transaction for single database (simplified from dual database)
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
    `, fbUser.Name, fbUser.ID).Scan(&clientID)
	if err != nil {
		LogError("Error upserting client: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	LogInfo("Upserted client with ID: %s", clientID)

	// 5. Insert/update pages in social_pages table
	for _, page := range pages {
		LogInfo("Processing page %s (ID: %s)", page.Name, page.ID)

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
        `, clientID, page.Platform, page.ID, page.Name, page.AccessToken)

		if err != nil {
			LogError("Error processing page %s: %v", page.Name, err)
			continue
		}

		LogInfo("Successfully processed page %s", page.Name)
	}

	// 6. Commit transaction
	if err = tx.Commit(); err != nil {
		LogError("Error committing transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	LogInfo("Database transaction committed successfully")

	// 7. Set up webhook subscriptions for all pages (after database commit)
	LogInfo("Starting webhook subscription automation for %d pages", len(pages))
	webhookSuccessCount := 0
	for _, page := range pages {
		LogInfo("Setting up webhooks for page: %s (%s)", page.Name, page.Platform)

		// Set up webhook subscriptions automatically (simplified - no handover protocol)
		if err := setupWebhookSubscriptions(page.ID, page.AccessToken, page.Name, page.Platform); err != nil {
			LogError("Webhook setup failed for %s: %v", page.Name, err)
			// Don't fail the entire request - webhook setup is best effort
		} else {
			webhookSuccessCount++
			LogInfo("Webhook setup completed for %s", page.Name)
		}
	}

	LogInfo("Webhook automation summary: %d/%d pages configured successfully", webhookSuccessCount, len(pages))

	LogInfo("Successfully completed Facebook token request with webhook automation")

	// Return success response (no session token needed)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"client_id": clientID,
		"message":   "Authentication successful",
	})
}
