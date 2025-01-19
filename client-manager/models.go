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
	Platform    string     `json:"platform"`
	PageID      string     `json:"page_id"`
	Name        string     `json:"name"`
	AccessToken string     `json:"access_token"`
	Status      string     `json:"status"`
	BotpressURL string     `json:"botpress_url"`
	CreatedAt   time.Time  `json:"created_at"`
	ActivatedAt *time.Time `json:"activated_at"`
}

type FacebookPage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AccessToken string `json:"access_token"`
	Platform    string `json:"platform"`
}

type FacebookPageResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	AccessToken string   `json:"access_token"`
	Tasks       []string `json:"tasks"`
	// Add subscription status
	Subscribed bool `json:"subscribed"`
}

type FacebookUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
