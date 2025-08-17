// platform_api.go
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// getPageInfo retrieves page information from database with platform-specific query
func getPageInfo(ctx context.Context, pageID string, platform string) (*PageInfo, error) {
	var info PageInfo
	info.PageID = pageID
	err := db.QueryRowContext(ctx,
		"SELECT platform, access_token FROM social_pages WHERE page_id = $1 AND platform = $2 AND status = 'active'",
		pageID, platform,
	).Scan(&info.Platform, &info.AccessToken)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active page found for ID %s with platform %s", pageID, platform)
		}
		return nil, fmt.Errorf("database error: %v", err)
	}

	return &info, nil
}


// sendPlatformResponse routes message sending to the appropriate platform API
func sendPlatformResponse(ctx context.Context, pageInfo *PageInfo, senderID, message string) error {
	switch pageInfo.Platform {
	case "facebook":
		return sendFacebookMessage(ctx, pageInfo.PageID, pageInfo.AccessToken, senderID, message)
	case "instagram":
		return sendInstagramMessage(ctx, pageInfo.AccessToken, senderID, message)
	default:
		return fmt.Errorf("unsupported platform: %s", pageInfo.Platform)
	}
}

// getProfileInfo retrieves user profile information from Facebook/Instagram Graph API
func getProfileInfo(ctx context.Context, userID string, pageToken string, platform string) (string, error) {
	log.Printf("🔍 Getting profile info for user %s (platform: %s)", userID, platform)

	// Check cache first
	if name, found := userCache.Get(userID); found {
		return name, nil
	}

	// Different endpoints and handling for Facebook and Instagram
	var userName string
	if platform == "facebook" {
		apiURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s?fields=name&access_token=%s", userID, pageToken)
		log.Printf("📡 Making Facebook API request for user %s", userID)

		var profile FacebookProfile
		if err := makeAPIRequest(ctx, apiURL, &profile); err != nil {
			return "user", err
		}
		userName = profile.Name
		log.Printf("👤 Using Facebook name: %s", userName)
	} else {
		apiURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s?fields=username&access_token=%s", userID, pageToken)
		log.Printf("📡 Making Instagram API request for user %s", userID)

		var profile InstagramProfile
		if err := makeAPIRequest(ctx, apiURL, &profile); err != nil {
			return "user", err
		}
		userName = profile.Username
		log.Printf("📸 Using Instagram username: %s", userName)
	}

	if userName == "" {
		log.Printf("⚠️ No name found in profile for user %s", userID)
		return "user", fmt.Errorf("no name found in profile")
	}

	// Cache the result
	userCache.Set(userID, userName)
	return userName, nil
}

// makeAPIRequest is a helper function to make HTTP API requests with JSON decoding
func makeAPIRequest(ctx context.Context, url string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("⏱️ API request completed in %v", time.Since(start))

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("❌ API error: Status %d, Body: %s",
			resp.StatusCode, string(respBody))
		return fmt.Errorf("error response from API: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("error decoding response: %v", err)
	}

	return nil
}

// handleSendMessage handles HTTP endpoint for sending messages directly via API
func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Error parsing send message request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get page info for access token
	pageInfo, err := getPageInfo(r.Context(), req.PageID, req.Platform)
	if err != nil {
		log.Printf("❌ Error getting page info: %v", err)
		http.Error(w, "Error getting page info", http.StatusInternalServerError)
		return
	}

	// Send message based on platform
	var sendErr error
	switch req.Platform {
	case "facebook":
		sendErr = sendFacebookMessage(r.Context(), req.PageID, pageInfo.AccessToken, req.RecipientID, req.Message)
	case "instagram":
		sendErr = sendInstagramMessage(r.Context(), pageInfo.AccessToken, req.RecipientID, req.Message)
	default:
		sendErr = fmt.Errorf("unsupported platform: %s", req.Platform)
	}

	if sendErr != nil {
		log.Printf("❌ Error sending message: %v", sendErr)
		http.Error(w, fmt.Sprintf("Error sending message: %v", sendErr), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

