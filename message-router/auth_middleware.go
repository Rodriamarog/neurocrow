package main

import (
	"database/sql"
	"net/http"
	"strings"
)

// AuthMiddleware handles authentication for content management APIs
type AuthMiddleware struct {
	db *sql.DB
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(db *sql.DB) *AuthMiddleware {
	return &AuthMiddleware{db: db}
}

// ClientAuthMiddleware validates client authentication and adds client ID to request headers
func (am *AuthMiddleware) ClientAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For now, we'll use a simple client ID approach similar to the existing OAuth flow
		// In the future, this could be enhanced with JWT tokens or session management
		
		clientID := r.Header.Get("X-Client-ID")
		if clientID == "" {
			// Try to get client ID from Authorization header (Bearer token format)
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				clientID = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}
		
		// Try to get client ID from query parameter as fallback
		if clientID == "" {
			clientID = r.URL.Query().Get("client_id")
		}
		
		if clientID == "" {
			LogError("❌ Authentication failed: No client ID provided")
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		
		// Validate that the client ID exists and has active pages
		if !am.validateClientID(clientID) {
			LogError("❌ Authentication failed: Invalid client ID %s", clientID)
			http.Error(w, "Invalid authentication", http.StatusUnauthorized)
			return
		}
		
		LogDebug("✅ Client %s authenticated successfully", clientID)
		
		// Add client ID to request headers for use by handlers
		r.Header.Set("X-Client-ID", clientID)
		
		// Continue to next handler
		next.ServeHTTP(w, r)
	}
}

// validateClientID checks if the client ID exists and has active pages
func (am *AuthMiddleware) validateClientID(clientID string) bool {
	query := `
		SELECT COUNT(*) 
		FROM social_pages 
		WHERE client_id = $1 AND status = 'active'
	`
	
	var count int
	err := am.db.QueryRow(query, clientID).Scan(&count)
	if err != nil {
		LogError("Error validating client ID %s: %v", clientID, err)
		return false
	}
	
	return count > 0
}

// CORSMiddleware handles CORS for content management APIs
func (am *AuthMiddleware) CORSMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Allow specific origins
		allowedOrigins := []string{
			"http://localhost:3000",
			"https://neurocrow.com",
			"https://www.neurocrow.com",
		}
		
		origin := r.Header.Get("Origin")
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
		
		// Set other CORS headers
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Client-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		
		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		// Continue to next handler
		next.ServeHTTP(w, r)
	}
}

// ContentAuthMiddleware combines CORS and client authentication
func (am *AuthMiddleware) ContentAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return am.CORSMiddleware(am.ClientAuthMiddleware(next))
}