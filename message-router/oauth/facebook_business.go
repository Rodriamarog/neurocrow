// oauth/facebook_business.go
// Facebook Business OAuth handlers - EXACT COPY from client-manager

package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func HandleFacebookBusinessToken(w http.ResponseWriter, r *http.Request) {
	LogInfo("=== Starting Facebook Business token request handling ===")

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

	LogInfo("Facebook Business user authenticated: %s (ID: %s)", fbUser.Name, fbUser.ID)

	// 2. Get both Facebook pages and Instagram Business accounts
	facebookPages, err := getConnectedPages(data.UserToken)
	if err != nil {
		LogError("Error getting Facebook pages: %v", err)
		http.Error(w, fmt.Sprintf("Could not get Facebook pages: %v", err), http.StatusInternalServerError)
		return
	}

	LogInfo("Found %d Facebook pages", len(facebookPages))

	// 3. Get Instagram Business accounts via Facebook Pages
	instagramAccounts, err := getInstagramAccountsViaFacebook(data.UserToken)
	if err != nil {
		LogError("Warning: Could not get Instagram Business accounts: %v", err)
		instagramAccounts = []InstagramAccount{} // Continue without Instagram accounts
	}

	LogInfo("Found %d Instagram Business accounts", len(instagramAccounts))

	// 4. Start transaction for single database
	tx, err := SocialDB.Begin()
	if err != nil {
		LogError("Error starting database transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 5. Create or update client
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

	// 6. Insert/update Facebook pages in social_pages table
	for _, page := range facebookPages {
		LogInfo("Processing Facebook page %s (ID: %s)", page.Name, page.ID)

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

	// 7. Insert/update Instagram accounts in social_pages table
	for _, account := range instagramAccounts {
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

	// 8. Commit transaction
	if err = tx.Commit(); err != nil {
		LogError("Error committing transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	LogInfo("Database transaction committed successfully")

	// 9. Set up webhook subscriptions for all pages/accounts (after database commit)
	allPagesAndAccounts := len(facebookPages) + len(instagramAccounts)
	LogInfo("Starting webhook subscription automation for %d pages/accounts", allPagesAndAccounts)
	webhookSuccessCount := 0

	// Set up Facebook page webhooks
	for _, page := range facebookPages {
		LogInfo("Setting up webhooks for Facebook page: %s", page.Name)

		if err := setupWebhookSubscriptions(page.ID, page.AccessToken, page.Name, page.Platform); err != nil {
			LogError("Webhook setup failed for %s: %v", page.Name, err)
		} else {
			webhookSuccessCount++
			LogInfo("Webhook setup completed for %s", page.Name)
		}
	}

	// Set up Instagram account webhooks
	for _, account := range instagramAccounts {
		LogInfo("Setting up webhooks for Instagram account: %s", account.Name)

		if err := setupWebhookSubscriptions(account.ID, account.AccessToken, account.Name, "instagram"); err != nil {
			LogError("Webhook setup failed for %s: %v", account.Name, err)
		} else {
			webhookSuccessCount++
			LogInfo("Webhook setup completed for %s", account.Name)
		}
	}

	LogInfo("Webhook automation summary: %d/%d pages/accounts configured successfully", webhookSuccessCount, allPagesAndAccounts)

	LogInfo("Successfully completed Facebook Business authentication with webhook automation")

	// 10. Return success response (no session token needed)
	response := map[string]interface{}{
		"success":            true,
		"client_id":          clientID,
		"facebook_pages":     len(facebookPages),
		"instagram_accounts": len(instagramAccounts),
		"message":            "Facebook Business authentication successful - both Facebook and Instagram accounts connected",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
