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
