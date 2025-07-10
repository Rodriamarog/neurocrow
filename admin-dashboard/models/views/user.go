package views

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
