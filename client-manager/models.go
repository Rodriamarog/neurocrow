// models.go
package main

import (
	"time"
)

type Client struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Page struct {
	ID          string     `json:"id"`
	ClientID    string     `json:"client_id"`
	Platform    string     `json:"platform"` // "facebook" or "instagram"
	PageID      string     `json:"page_id"`  // The actual Facebook/Instagram page ID
	Name        string     `json:"name"`
	AccessToken string     `json:"access_token"`
	Status      string     `json:"status"` // "pending", "active", "disabled"
	BotpressURL string     `json:"botpress_url"`
	CreatedAt   time.Time  `json:"created_at"`
	ActivatedAt *time.Time `json:"activated_at"`
}

type FacebookPage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AccessToken string `json:"access_token"`
}
