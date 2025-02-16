package db

import (
	"admin-dashboard/models"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid" // added for UUID validation
)

// New helper function to validate profile picture URLs
func isValidProfilePicture(url string) bool {
	trimmed := strings.TrimSpace(url)
	// Accept absolute URLs and our static default
	return strings.HasPrefix(trimmed, "http://") ||
		strings.HasPrefix(trimmed, "https://") ||
		trimmed == "/static/default-avatar.png"
}

// Updated FetchMessages with extensive logging and UUID validation
func FetchMessages(clientID string, query string, args ...interface{}) ([]models.Message, error) {
	log.Printf("🔍 FetchMessages called with:")
	log.Printf("  - ClientID: %s", clientID)
	log.Printf("  - Query: %s", query)
	log.Printf("  - Additional args: %+v", args)

	// Validate UUID
	_, err := uuid.Parse(clientID)
	if err != nil {
		log.Printf("❌ Invalid UUID format for clientID: %v", err)
		return nil, fmt.Errorf("invalid UUID format: %v", err)
	}

	// Create final args array with clientID first
	queryArgs := make([]interface{}, 0, len(args)+1)
	queryArgs = append(queryArgs, clientID)
	queryArgs = append(queryArgs, args...)

	log.Printf("  - Final args for query: %+v", queryArgs)

	rows, err := DB.Query(query, queryArgs...)
	if err != nil {
		log.Printf("❌ Database query error: %v", err)
		log.Printf("  - Query that failed: %s", query)
		log.Printf("  - Args that failed: %+v", queryArgs)
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
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
			log.Printf("❌ Error scanning row: %v", err)
			continue
		}

		log.Printf("✅ Successfully scanned message:")
		log.Printf("  - ID: %s", msg.ID)
		log.Printf("  - ThreadID: %s", msg.ThreadID)
		log.Printf("  - Platform: %s", msg.Platform)
		log.Printf("  - FromUser: %s", msg.FromUser)

		// Set ClientID if valid
		if clientID.Valid {
			msg.ClientID = &clientID.String
		}

		// Simple profile picture check: if the trimmed value starts with "http://" or "https://", use it.
		// Otherwise, default to the provided URL.
		if profilePicture.Valid {
			trimmed := strings.TrimSpace(profilePicture.String)
			if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
				msg.ProfilePictureURL = trimmed
			} else {
				log.Printf("  - Invalid profile picture URL: %s", trimmed)
				msg.ProfilePictureURL = "https://www.svgrepo.com/show/452030/avatar-default.svg"
			}
		} else {
			msg.ProfilePictureURL = "https://www.svgrepo.com/show/452030/avatar-default.svg"
		}

		messages = append(messages, msg)
	}

	log.Printf("✅ FetchMessages completed. Found %d messages", len(messages))
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
	result, err := DB.Exec(`
        UPDATE conversations 
        SET bot_enabled = $2, 
            updated_at = CURRENT_TIMESTAMP
        WHERE thread_id = $1
    `, threadID, enabled)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no conversation found with thread_id: %s", threadID)
	}

	return nil
}
