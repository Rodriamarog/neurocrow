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

// FetchMessages is now more flexible to handle different query types
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
			&msg.SocialUserName,
		)
		if err != nil {
			log.Printf("❌ Error scanning row: %v", err)
			continue
		}

		if clientID.Valid {
			msg.ClientID = &clientID.String
		}

		if profilePicture.Valid && profilePicture.String != "" {
			msg.ProfilePictureURL = profilePicture.String
		} else {
			msg.ProfilePictureURL = "/static/default-avatar.png"
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

// FetchSingleMessage gets a single message by ID
func FetchSingleMessage(messageID string, clientID string) (models.Message, error) {
	query := `
        SELECT 
            m.id, m.client_id, m.page_id, m.platform,
            m.from_user, m.content, m.timestamp, m.thread_id,
            m.read, m.source,
            COALESCE(c.bot_enabled, TRUE) AS bot_enabled,
            COALESCE(NULLIF(TRIM(c.profile_picture_url), ''), '/static/default-avatar.png') as profile_picture_url,
            c.social_user_name
        FROM messages m
        JOIN social_pages sp ON m.page_id = sp.id
        LEFT JOIN conversations c ON c.thread_id = m.thread_id
        WHERE m.id = $1 AND sp.client_id = $2::uuid
    `

	var message models.Message
	var dbClientID, socialUserName sql.NullString

	err := DB.QueryRow(query, messageID, clientID).Scan(
		&message.ID, &dbClientID, &message.PageID, &message.Platform,
		&message.FromUser, &message.Content, &message.Timestamp, &message.ThreadID,
		&message.Read, &message.Source, &message.BotEnabled, &message.ProfilePictureURL,
		&socialUserName,
	)

	if err != nil {
		return message, err
	}

	if dbClientID.Valid {
		message.ClientID = &dbClientID.String
	}
	if socialUserName.Valid {
		message.SocialUserName = &socialUserName.String
	}

	return message, nil
}
