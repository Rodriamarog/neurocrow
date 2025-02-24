package services

import (
	"admin-dashboard/db"
	"admin-dashboard/models"
	"context"
)

type MessageService struct {
	db *db.Database
}

func NewMessageService(db *db.Database) *MessageService {
	return &MessageService{db: db}
}

func (s *MessageService) GetThreadMessages(ctx context.Context, clientID, threadID string) ([]models.Message, error) {
	return s.db.GetMessages(ctx, clientID)
}

func (s *MessageService) SendMessage(ctx context.Context, msg *models.Message) error {
	// Implementation
	return nil
}
