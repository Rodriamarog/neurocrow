package db

import (
	"admin-dashboard/models"
	"database/sql"
	"log"
	"net/http"
)

func FetchMessages(query string, args ...interface{}) ([]models.Message, error) {
	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		var clientID sql.NullString
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
			&msg.BotEnabled, // new column
		)
		if err != nil {
			log.Printf("Error scanning message: %v", err)
			continue
		}
		if clientID.Valid {
			msg.ClientID = &clientID.String
		}
		messages = append(messages, msg)
	}
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
