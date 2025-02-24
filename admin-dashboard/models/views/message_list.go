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
