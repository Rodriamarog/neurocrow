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
	Name       string `json:"name"`
	ProfilePic string `json:"profile_pic"`
	Error      struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// FetchProfilePicture gets the profile picture URL for a user from the Messenger Platform API
func FetchProfilePicture(userID, accessToken, platform string) (string, error) {
	// Skip test threads
	if strings.HasPrefix(userID, "thread_") {
		return "", fmt.Errorf("test thread, skipping profile picture fetch")
	}

	log.Printf("🔍 Attempting to fetch profile picture for user ID %s on %s", userID, platform)

	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s?fields=name,profile_pic&access_token=%s",
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
	// thread_id is the Meta user ID for real conversations
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
