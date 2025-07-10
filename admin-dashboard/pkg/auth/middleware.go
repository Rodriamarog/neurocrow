package auth

import (
	"context"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("auth_token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), "user", &User{
			ID:       claims.UserID,
			ClientID: claims.ClientID,
			Role:     claims.Role,
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
