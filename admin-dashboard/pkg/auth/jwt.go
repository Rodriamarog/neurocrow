package auth

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID               string `json:"user_id"`
	ClientID             string `json:"client_id"`
	Role                 string `json:"role"`
	jwt.RegisteredClaims        // Use RegisteredClaims instead of StandardClaims
}

func GenerateToken(user *User) (string, error) {
	claims := &Claims{
		UserID:   user.ID,
		ClientID: user.ClientID,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}
