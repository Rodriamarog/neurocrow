package main

import (
	"admin-dashboard/cache"
	"admin-dashboard/db"
	"admin-dashboard/handlers"
	"admin-dashboard/pkg/auth"
	"admin-dashboard/pkg/template" // new import
	"context"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	log.Printf("🚀 Starting server initialization...")

	// Initialize templates first
	template.InitTemplates()

	// Initialize database
	log.Printf("🗄️ Initializing database...")
	db.Init()
	defer db.DB.Close()

	// Initialize Redis
	log.Printf("📦 Initializing Redis...")
	cache.InitRedis()
	log.Println("🔍 Testing Redis connection...")

	// Simple Redis test
	err := cache.RedisClient.Set(context.Background(), "test_key", "test_value", time.Minute).Err()
	if err != nil {
		log.Printf("❌ Redis test failed: %v", err)
	} else {
		value, err := cache.RedisClient.Get(context.Background(), "test_key").Result()
		if err != nil {
			log.Printf("❌ Redis test failed: %v", err)
		} else {
			log.Printf("✅ Redis test successful! Retrieved value: %s", value)
		}
	}

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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("✅ Server initialization complete")
	log.Printf("🌐 Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
