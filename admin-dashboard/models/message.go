package models

import (
	"time"
)

type Message struct {
	ID                string    `json:"id"`
	ClientID          *string   `json:"client_id"`
	PageID            string    `json:"page_id"`
	Platform          string    `json:"platform"`
	FromUser          string    `json:"from_user"`
	SocialUserName    *string   `json:"social_user_name"`
	Content           string    `json:"content"`
	Timestamp         time.Time `json:"timestamp"`
	ThreadID          string    `json:"thread_id"`
	Read              bool      `json:"read"`
	Source            string    `json:"source"`
	BotEnabled        bool      `json:"bot_enabled"`
	ProfilePictureURL string    `json:"profile_picture_url"` // Ensure this is a string
}
