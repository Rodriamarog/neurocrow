package main

import (
	"admin-dashboard/db"
	"admin-dashboard/handlers"
	"admin-dashboard/pkg/auth"
	"admin-dashboard/pkg/template" // new import
	"log"
	"net/http"
	"os"
)

func main() {
	log.Printf("🚀 Starting server initialization...")

	// Initialize templates first
	template.InitTemplates()

	// Initialize database
	log.Printf("🗄️ Initializing database...")
	db.Init()
	defer db.DB.Close()

	// Create rate limiters with higher limits for development
	log.Printf("⚙️ Setting up rate limiters...")
	limiter := handlers.NewRateLimiter()

	// Set up routes
	log.Printf("🛣️ Setting up routes...")
	// Auth routes
	log.Printf("🔐 Setting up auth routes...")
	http.HandleFunc("/login", handlers.Login)
	http.HandleFunc("/logout", handlers.Logout)

	// Protected routes with consistent handler types
	log.Printf("🔒 Setting up protected routes...")
	http.HandleFunc("/", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.GetMessages)))
	http.HandleFunc("/message-list", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.GetMessageList)))
	http.HandleFunc("/chat", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.GetChat)))
	http.HandleFunc("/send-message", auth.AuthMiddleware(limiter.MessageLimit.RateLimit(handlers.SendMessage)))
	http.HandleFunc("/thread-preview", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.GetThreadPreview)))
	http.HandleFunc("/toggle-bot", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.ToggleBotStatus)))
	http.HandleFunc("/chat-messages", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.GetChatMessages)))
	http.HandleFunc("/refresh-profile-pictures", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.RefreshProfilePictures)))

	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("✅ Server initialization complete")
	log.Printf("🌐 Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
