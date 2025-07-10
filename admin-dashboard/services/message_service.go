package services

import (
	"admin-dashboard/config"
	"admin-dashboard/models"
	"admin-dashboard/pkg/cache"
	"admin-dashboard/pkg/views"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"html/template"
	"time"
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
	Config    *config.MessagesConfig
}

func NewMessageService(db *sql.DB, templates *template.Template, cache *cache.Cache, cfg *config.MessagesConfig) *MessageService {
	return &MessageService{
		db:        db,
		templates: templates,
		cache:     cache,
		Config:    cfg,
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

func (s *MessageService) GetMessages(ctx context.Context, params views.MessageListParams) (*views.PaginatedResponse, error) {
	// Try cache first
	cacheKey := s.buildCacheKey(params)
	if cached, ok := s.cache.Get(cacheKey); ok {
		return cached.(*views.PaginatedResponse), nil
	}

	// Build query with proper pagination
	query, args := s.buildMessageQuery(params)

	// Get total count for pagination
	countQuery, countArgs := s.buildCountQuery(params)
	var total int64
	err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count messages: %w", err)
	}

	// Execute main query
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetch messages: %w", err)
	}
	defer rows.Close()

	messages, err := s.scanMessages(rows)
	if err != nil {
		return nil, err
	}

	// Generate next cursor
	nextCursor := ""
	if len(messages) == params.PageSize {
		nextCursor = s.generateCursor(messages[len(messages)-1])
	}

	response := &views.PaginatedResponse{
		Items: views.ToMessageViews(messages),
		Pagination: views.Pagination{
			Page:       params.Page,
			PageSize:   params.PageSize,
			TotalItems: total,
			NextCursor: nextCursor,
		},
	}

	// Cache the response
	s.cache.SetWithTTL(cacheKey, response, s.Config.CacheTimeout)

	return response, nil
}

func (s *MessageService) buildMessageQuery(params views.MessageListParams) (string, []interface{}) {
	query := `
		WITH RankedMessages AS (
			SELECT 
				m.*,
				c.profile_picture_url,
				ROW_NUMBER() OVER (
					PARTITION BY m.thread_id 
					ORDER BY m.timestamp DESC
				) as rn
			FROM messages m
			LEFT JOIN conversations c ON c.thread_id = m.thread_id
			WHERE m.client_id = $1
	`
	args := []interface{}{params.ClientID}
	argCount := 1

	// Add filters
	if params.ThreadID != "" {
		argCount++
		query += fmt.Sprintf(" AND m.thread_id = $%d", argCount)
		args = append(args, params.ThreadID)
	}

	// Add date range, platform, status filters...

	// Add pagination using Offset()
	query += `
		)
		SELECT * FROM RankedMessages 
		WHERE rn = 1
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d
	`
	args = append(args, params.PageSize, params.Offset())

	return query, args
}

func (s *MessageService) SendMessage(ctx context.Context, msg *models.Message) error {
	// Implementation
	return nil
}

func (s *MessageService) buildCacheKey(params views.MessageListParams) string {
	return fmt.Sprintf("messages:%s:%s:%d:%s:%s:%s:%s",
		params.ClientID,
		params.ThreadID,
		params.PageSize,
		params.StartDate.Format(time.RFC3339),
		params.EndDate.Format(time.RFC3339),
		params.Platform,
		params.Status,
	)
}

func (s *MessageService) buildCountQuery(params views.MessageListParams) (string, []interface{}) {
	query := `
		SELECT COUNT(*) 
		FROM messages m
		WHERE m.client_id = $1
	`
	args := []interface{}{params.ClientID}
	argCount := 1

	if params.ThreadID != "" {
		argCount++
		query += fmt.Sprintf(" AND m.thread_id = $%d", argCount)
		args = append(args, params.ThreadID)
	}

	if !params.StartDate.IsZero() {
		argCount++
		query += fmt.Sprintf(" AND m.timestamp >= $%d", argCount)
		args = append(args, params.StartDate)
	}

	if !params.EndDate.IsZero() {
		argCount++
		query += fmt.Sprintf(" AND m.timestamp <= $%d", argCount)
		args = append(args, params.EndDate)
	}

	if params.Platform != "" {
		argCount++
		query += fmt.Sprintf(" AND m.platform = $%d", argCount)
		args = append(args, params.Platform)
	}

	return query, args
}

func (s *MessageService) scanMessages(rows *sql.Rows) ([]models.Message, error) {
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
			return nil, err
		}

		if clientID.Valid {
			msg.ClientID = &clientID.String
		}

		if profilePicture.Valid {
			msg.ProfilePictureURL = profilePicture.String
		}

		messages = append(messages, msg)
	}
	return messages, nil
}

func (s *MessageService) generateCursor(msg models.Message) string {
	return base64.StdEncoding.EncodeToString([]byte(msg.Timestamp.Format(time.RFC3339)))
}
