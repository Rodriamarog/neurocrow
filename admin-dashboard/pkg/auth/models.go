package auth

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	ClientID string `json:"client_id"`
	Role     string `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
