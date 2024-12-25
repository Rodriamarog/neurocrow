// models.go
package main

import (
	"time"
)

type Client struct {
	ID        string
	Name      string
	Email     string
	CreatedAt time.Time
}

type Page struct {
	ID          string
	ClientID    string
	Platform    string
	PageID      string
	Name        string
	AccessToken string
	Status      string
	BotpressURL string
	CreatedAt   time.Time
	ActivatedAt *time.Time
}

// Add this type definition that was in cmd/sync/main.go
type FacebookPage struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
