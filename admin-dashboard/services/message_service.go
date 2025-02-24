package services

import (
	"admin-dashboard/models"
	"admin-dashboard/pkg/cache"
	"context"
	"database/sql"
	"fmt"
	"html/template"
)

type MessageOptions struct {
	Limit  int
	Offset int
	Order  string
}

type MessageService struct {
	db        *sql.DB
	templates *template.Template
	cache     *cache.Cache
}

func NewMessageService(db *sql.DB, templates *template.Template, cache *cache.Cache) *MessageService {
	return &MessageService{
		db:        db,
		templates: templates,
		cache:     cache,
	}
}

func (s *MessageService) GetThreadMessages(ctx context.Context, threadID string, opts MessageOptions) ([]models.Message, error) {
	messages, err := s.cache.GetMessages(threadID)
	if err == nil {
		return messages, nil
	}

	messages, err = s.fetchMessages(ctx, threadID, opts)
	if err != nil {
		return nil, fmt.Errorf("fetch messages: %w", err)
	}

	s.cache.SetMessages(threadID, messages)
	return messages, nil
}

func (s *MessageService) fetchMessages(ctx context.Context, threadID string, opts MessageOptions) ([]models.Message, error) {
	query := `
		SELECT m.*, c.bot_enabled, c.profile_picture_url
		FROM messages m
		LEFT JOIN conversations c ON c.thread_id = m.thread_id
		WHERE m.thread_id = $1
		ORDER BY m.timestamp DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, threadID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		err := rows.Scan(&msg.ID, &msg.ClientID, &msg.PageID /* other fields */)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func (s *MessageService) GetMessages(ctx context.Context, clientID string) ([]models.Message, error) {
	query := `
		SELECT m.*, c.bot_enabled, c.profile_picture_url
		FROM messages m
		LEFT JOIN conversations c ON c.thread_id = m.thread_id
		WHERE m.client_id = $1
		ORDER BY m.timestamp DESC
		LIMIT 50
	`
	rows, err := s.db.QueryContext(ctx, query, clientID)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		err := rows.Scan(&msg.ID, &msg.ClientID, &msg.PageID /* other fields */)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func (s *MessageService) SendMessage(ctx context.Context, msg *models.Message) error {
	// Implementation
	return nil
}
