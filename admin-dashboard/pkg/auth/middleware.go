package auth

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🔒 Auth middleware processing request to: %s", r.URL.Path)
		cookie, err := r.Cookie("auth_token")
		if err != nil {
			log.Printf("❌ No auth cookie found: %v", err)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			log.Printf("❌ Invalid token: %v", err)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		log.Printf("✅ Auth successful for user: %s, client: %s", claims.UserID, claims.ClientID)
		// Add user info to context
		ctx := context.WithValue(r.Context(), "user", &User{
			ID:       claims.UserID,
			ClientID: claims.ClientID,
			Role:     claims.Role,
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
