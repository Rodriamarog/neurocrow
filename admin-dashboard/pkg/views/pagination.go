package views

import "time"

type Pagination struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalItems int64  `json:"total_items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	Pagination Pagination  `json:"pagination"`
}

type MessageListParams struct {
	ClientID  string
	ThreadID  string
	Cursor    string
	Page      int
	PageSize  int
	StartDate time.Time
	EndDate   time.Time
	Platform  string
	Status    string
}

// Add helper function to calculate offset
func (p *MessageListParams) Offset() int {
	return (p.Page - 1) * p.PageSize
}
