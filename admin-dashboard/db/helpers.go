package db

import (
	"admin-dashboard/models"
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
		err := rows.Scan(
			&msg.ID, &msg.ClientID, &msg.PageID, &msg.Platform,
			&msg.FromUser, &msg.Content, &msg.Timestamp,
			&msg.ThreadID, &msg.Read,
		)
		if err != nil {
			log.Printf("Error scanning message: %v", err)
			continue
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func HandleError(w http.ResponseWriter, err error, message string, statusCode int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, message, statusCode)
}
