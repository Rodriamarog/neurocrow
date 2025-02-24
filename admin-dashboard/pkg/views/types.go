package views

import "time"

type MessageListView struct {
	Messages []MessageView
	User     UserView
	Filters  FilterOptions
}

type MessageView struct {
	ID        string
	Content   string
	Timestamp time.Time
	Sender    UserView
	Thread    ThreadView
}

type UserView struct {
	ID       string
	ClientID string
	Role     string
	Name     string
}

type ThreadView struct {
	ID            string
	LastMessage   string
	LastTimestamp string
	ProfilePic    string
	UserName      string
}

type FilterOptions struct {
	StartDate string
	EndDate   string
	Platform  string
	Status    string
}
