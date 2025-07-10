package auth

type User struct {
	ID       string
	ClientID string
	Role     string
	Email    string
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
