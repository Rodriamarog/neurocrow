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

// FacebookProfileResponse represents the structure of Facebook's Graph API response
type FacebookProfileResponse struct {
	Picture struct {
		Data struct {
			URL          string `json:"url"`
			IsSilhouette bool   `json:"is_silhouette"` // Add this to detect default avatars
		} `json:"data"`
	} `json:"picture"`
}

func FetchProfilePicture(userID, accessToken, platform string) (string, error) {
	// Skip test users
	if strings.HasPrefix(userID, "thread_") {
		return "", fmt.Errorf("test user, skipping profile picture fetch")
	}

	log.Printf("🔍 Attempting to fetch profile picture for %s on %s", userID, platform)

	if platform == "facebook" {
		url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s?fields=picture.type(large)&access_token=%s",
			userID, accessToken)

		log.Printf("📡 Calling Facebook API: %s", url)
		resp, err := http.Get(url)
		if err != nil {
			return "", fmt.Errorf("failed to fetch Facebook profile: %v", err)
		}
		defer resp.Body.Close()

		// Read and log raw response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response body: %v", err)
		}
		log.Printf("📥 Facebook API Response: %s", string(body))

		var profile FacebookProfileResponse
		if err := json.Unmarshal(body, &profile); err != nil {
			return "", fmt.Errorf("failed to decode Facebook response: %v", err)
		}

		// Log the parsed data
		log.Printf("📊 Parsed profile picture URL: %s", profile.Picture.Data.URL)
		log.Printf("📊 Is silhouette: %v", profile.Picture.Data.IsSilhouette)

		if profile.Picture.Data.IsSilhouette || profile.Picture.Data.URL == "" {
			return "", fmt.Errorf("no custom profile picture available")
		}

		return profile.Picture.Data.URL, nil

	} else if platform == "instagram" {
		url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s?fields=profile_picture_url&access_token=%s",
			userID, accessToken)

		log.Printf("📡 Calling Instagram API: %s", url)
		resp, err := http.Get(url)
		if err != nil {
			return "", fmt.Errorf("failed to fetch Instagram profile: %v", err)
		}
		defer resp.Body.Close()

		// Read and log raw response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response body: %v", err)
		}
		log.Printf("📥 Instagram API Response: %s", string(body))

		var result struct {
			ProfilePictureURL string `json:"profile_picture_url"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("failed to decode Instagram response: %v", err)
		}

		// Log the parsed URL
		log.Printf("📊 Parsed profile picture URL: %s", result.ProfilePictureURL)

		if result.ProfilePictureURL == "" {
			return "", fmt.Errorf("no profile picture available")
		}

		return result.ProfilePictureURL, nil
	}

	return "", fmt.Errorf("unsupported platform: %s", platform)
}

func UpdateProfilePictureInDB(db *sql.DB, threadID, userID, accessToken, platform string) error {
	pictureURL, err := FetchProfilePicture(userID, accessToken, platform)
	if err != nil {
		// If we couldn't get a real profile picture, just log and return without updating
		log.Printf("Notice: Couldn't fetch profile picture for %s: %v", userID, err)
		return nil
	}

	_, err = db.Exec(`
        UPDATE conversations 
        SET profile_picture_url = $1 
        WHERE thread_id = $2
    `, pictureURL, threadID)

	if err != nil {
		return fmt.Errorf("failed to update profile picture in DB: %v", err)
	}

	return nil
}
