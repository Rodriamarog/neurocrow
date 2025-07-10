package handlers

import (
	"admin-dashboard/db"
	"admin-dashboard/pkg/auth"
	"admin-dashboard/pkg/template" // new import
	"database/sql"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func Login(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔍 Login handler called with method: %s", r.Method)

	if r.Method == "GET" {
		log.Printf("📝 Attempting to render login template")
		if err := template.RenderTemplate(w, "login", nil); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Login template rendered successfully")
		return
	}

	log.Printf("🔑 Processing login POST request")
	email := r.FormValue("email")
	log.Printf("📧 Login attempt for email: %s", email)
	password := r.FormValue("password")

	var user auth.User
	var passwordHash string

	err := db.DB.QueryRow(`
        SELECT u.id, u.email, u.password_hash, u.client_id, u.role 
        FROM users u 
        WHERE u.email = $1
    `, email).Scan(&user.ID, &user.Email, &passwordHash, &user.ClientID, &user.Role)

	if err == sql.ErrNoRows {
		log.Printf("❌ Login failed: no user found with email %s", email)
		template.RenderTemplate(w, "login", map[string]string{
			"Error": "Invalid email or password",
		})
		return
	}

	if err != nil {
		log.Printf("❌ Database error during login: %v", err)
		db.HandleError(w, err, "Login error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		log.Printf("❌ Login failed: invalid password for user %s", email)
		template.RenderTemplate(w, "login", map[string]string{
			"Error": "Invalid email or password",
		})
		return
	}

	token, err := auth.GenerateToken(&user)
	if err != nil {
		log.Printf("❌ Failed to generate token for user %s: %v", email, err)
		db.HandleError(w, err, "Token generation error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,  // Enable in production
		MaxAge:   86400, // 24 hours
	})

	log.Printf("✅ User logged in successfully. User ID: %s, Email: %s, Role: %s",
		user.ID, user.Email, user.Role)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
