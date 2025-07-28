package meta

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type MessengerProfileResponse struct {
	Name          string `json:"name"`
	ProfilePic    string `json:"profile_pic"`         // For Facebook
	ProfilePicURL string `json:"profile_picture_url"` // For Instagram
	ID            string `json:"id"`
	Error         struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// Updated RefreshProfilePicture to be smarter about when to refresh the profile picture.
func RefreshProfilePicture(db *sql.DB, threadID string) error {
	// First check if we need to refresh by looking at the current URL.
	var currentURL string
	err := db.QueryRow(`
        SELECT profile_picture_url 
        FROM conversations 
        WHERE thread_id = $1
    `, threadID).Scan(&currentURL)

	// Refresh if no URL is found or if the URL contains an expired indicator.
	needsRefresh := err == sql.ErrNoRows ||
		strings.Contains(currentURL, "oe=") ||
		strings.Contains(currentURL, "expired")

	if !needsRefresh {
		log.Printf("✅ Profile picture for thread %s doesn't need refresh", threadID)
		return nil
	}

	// If refresh is needed, get the necessary details.
	var platform string
	var accessToken string
	err = db.QueryRow(`
        SELECT 
            m.platform,
            p.access_token
        FROM messages m
        JOIN social_pages p ON m.page_id = p.id
        WHERE m.thread_id = $1
        ORDER BY m.timestamp DESC
        LIMIT 1
    `, threadID).Scan(&platform, &accessToken)
	if err != nil {
		return fmt.Errorf("failed to get thread details: %v", err)
	}

	return UpdateProfilePictureInDB(db, threadID, accessToken, platform)
}

// Updated FetchProfilePicture to use the Messenger Platform API endpoint.
func FetchProfilePicture(userID, accessToken, platform string) (string, error) {
	if strings.HasPrefix(userID, "thread_") {
		log.Printf("🔍 Skipping test thread: %s", userID)
		return "", fmt.Errorf("test thread, skipping profile picture fetch")
	}

	log.Printf("🔍 Fetching profile picture for user %s on %s", userID, platform)
	// Use the Messenger Platform API endpoint which works for both Facebook and Instagram
	url := fmt.Sprintf("https://graph.facebook.com/v23.0/%s?fields=name,profile_pic&access_token=%s",
		userID, accessToken)

	log.Printf("📡 Calling Messenger Platform API: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch profile: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	log.Printf("📥 Messenger API Raw Response: %s", string(body))

	var mResp MessengerProfileResponse
	if err := json.Unmarshal(body, &mResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if mResp.Error.Message != "" {
		return "", fmt.Errorf("API error: %s (code %d)", mResp.Error.Message, mResp.Error.Code)
	}

	if mResp.ProfilePic == "" {
		return "", fmt.Errorf("no profile picture available")
	}
	return mResp.ProfilePic, nil
}

func UpdateProfilePictureInDB(db *sql.DB, threadID, accessToken, platform string) error {
	pictureURL, err := FetchProfilePicture(threadID, accessToken, platform)
	if err != nil {
		if strings.HasPrefix(threadID, "thread_") {
			log.Printf("ℹ️ Skipping test thread %s", threadID)
			return nil
		}
		return fmt.Errorf("failed to fetch profile picture: %v", err)
	}

	_, err = db.Exec(`
        UPDATE conversations 
        SET profile_picture_url = $1,
            updated_at = CURRENT_TIMESTAMP
        WHERE thread_id = $2
    `, pictureURL, threadID)
	if err != nil {
		return fmt.Errorf("failed to update profile picture in DB: %v", err)
	}

	log.Printf("✅ Updated profile picture for thread %s", threadID)
	return nil
}
