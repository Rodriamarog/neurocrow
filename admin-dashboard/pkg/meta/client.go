package meta

import (
	"context"
	"net/http"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

type Profile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PictureURL string `json:"profile_picture_url"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

func (c *Client) GetProfile(ctx context.Context, threadID string) (*Profile, error) {
	// Implementation of actual Meta API call would go here
	// For now, return a mock profile
	return &Profile{
		ID:         threadID,
		Name:       "User",
		PictureURL: "/static/default-avatar.png",
	}, nil
}
