package models

import (
    "time"
)

type Client struct {
    ID        string
    Name      string
    Email     string
    CreatedAt time.Time
}

type SocialPage struct {
    ID          string
    ClientID    string
    Platform    string    // 'facebook' or 'instagram'
    PageID      string
    PageName    string
    AccessToken string
    CreatedAt   time.Time
}