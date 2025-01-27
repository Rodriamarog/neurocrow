package db

import (
	"admin-dashboard/cache"
	"admin-dashboard/models"
	"database/sql"
	"log"
	"net/http"
)

func FetchMessages(query string, args ...interface{}) ([]models.Message, error) {
	if len(args) > 0 && args[0] == "" {
		args = []interface{}{}
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	var dbProfilePics = make(map[string]string) // Track profile URLs from DB

	for rows.Next() {
		var msg models.Message
		var clientID, profilePicture sql.NullString

		err := rows.Scan(
			&msg.ID,
			&clientID,
			&msg.PageID,
			&msg.Platform,
			&msg.FromUser,
			&msg.Content,
			&msg.Timestamp,
			&msg.ThreadID,
			&msg.Read,
			&msg.Source,
			&msg.BotEnabled,
			&profilePicture,
		)
		if err != nil {
			log.Printf("Error scanning message: %v", err)
			continue
		}

		if profilePicture.Valid {
			dbProfilePics[msg.FromUser] = profilePicture.String
			msg.ProfilePictureURL = profilePicture.String
		} else {
			msg.ProfilePictureURL = "/static/default-avatar.png"
		}

		messages = append(messages, msg)
	}

	// Collect user IDs for bulk cache lookup
	userIDs := make([]string, 0, len(dbProfilePics))
	for userID := range dbProfilePics {
		userIDs = append(userIDs, userID)
	}

	// Fetch cached profile pictures in bulk
	cachedProfiles, err := cache.BulkGetProfilePictures(userIDs)
	if err != nil {
		log.Printf("⚠️ Error bulk fetching profile pictures: %v", err)
		cachedProfiles = make(map[string]string) // Initialize empty map on error
	}

	// Update messages with cached values where available
	for i := range messages {
		if url, exists := cachedProfiles[messages[i].FromUser]; exists {
			messages[i].ProfilePictureURL = url
		}
	}

	// Async cache population for missing entries
	go func() {
		for userID, url := range dbProfilePics {
			if _, exists := cachedProfiles[userID]; !exists && url != "" {
				if err := cache.CacheProfilePicture(userID, url); err != nil {
					log.Printf("⚠️ Failed to cache %s: %v", userID, err)
				}
			}
		}
	}()

	return messages, nil
}

func HandleError(w http.ResponseWriter, err error, message string, statusCode int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, message, statusCode)
}

// Add a new function to help with debugging profile pictures
func LogThreadDetails(threadID string) {
	// Query to check both tables
	query := `
        SELECT 
            t.thread_id,
            t.profile_picture_url as thread_profile_pic,
            c.profile_picture_url as conv_profile_pic
        FROM messages t
        LEFT JOIN conversations c ON c.thread_id = t.thread_id
        WHERE t.thread_id = $1
        LIMIT 1
    `
	var threadID2 string
	var threadPic, convPic sql.NullString
	err := DB.QueryRow(query, threadID).Scan(&threadID2, &threadPic, &convPic)
	if err != nil {
		log.Printf("❌ Error checking profile pictures: %v", err)
		return
	}

	log.Printf("🔍 Thread %s profile picture details:", threadID)
	log.Printf("  - Thread table profile_picture_url: %v (Valid: %v)",
		threadPic.String, threadPic.Valid)
	log.Printf("  - Conversations table profile_picture_url: %v (Valid: %v)",
		convPic.String, convPic.Valid)
}

// UpdateBotStatus updates the bot_enabled status for a specific thread
func UpdateBotStatus(threadID string, enabled bool) error {
	log.Printf("🤖 Attempting to update bot status for thread %s to %v", threadID, enabled)

	result, err := DB.Exec(`
        UPDATE conversations 
        SET bot_enabled = $2, 
            updated_at = CURRENT_TIMESTAMP
        WHERE thread_id = $1
    `, threadID, enabled)

	if err != nil {
		log.Printf("❌ Failed to update bot status: %v", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("✅ Bot status updated. Rows affected: %d", rowsAffected)

	return nil
}
