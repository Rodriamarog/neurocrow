package db

import (
	"admin-dashboard/models"
	"database/sql"
	"log"
	"net/http"
	"time"
)

func FetchMessages(query string, args ...interface{}) ([]models.Message, error) {
	if len(args) > 0 && args[0] == "" {
		args = []interface{}{} // Reset args if empty string
	}
	log.Printf("📝 Executing query with args: %+v", args)
	start := time.Now()
	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		var clientID sql.NullString
		var profilePicture sql.NullString
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
			&profilePicture, // new scan target
		)
		if err != nil {
			log.Printf("Error scanning message: %v", err)
			continue
		}
		if clientID.Valid {
			msg.ClientID = &clientID.String
		}
		if profilePicture.Valid {
			msg.ProfilePictureURL = profilePicture.String
		} else {
			msg.ProfilePictureURL = ""
		}
		messages = append(messages, msg)
	}
	log.Printf("✨ Found %d messages in %v", len(messages), time.Since(start))
	return messages, nil
}

func HandleError(w http.ResponseWriter, err error, message string, statusCode int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, message, statusCode)
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
