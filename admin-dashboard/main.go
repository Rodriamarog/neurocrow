package main

import (
	"admin-dashboard/cache"
	"admin-dashboard/db"
	"admin-dashboard/handlers"
	"context"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	// Initialize database
	db.Init()
	defer db.DB.Close()

	// Initialize Redis
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
	limiter := handlers.NewRateLimiter()

	// Routes with more permissive rate limiting
	http.HandleFunc("/", limiter.ViewLimit.RateLimit(handlers.GetMessages))
	http.HandleFunc("/message-list", limiter.ViewLimit.RateLimit(handlers.GetMessageList))
	http.HandleFunc("/chat", limiter.ViewLimit.RateLimit(handlers.GetChat))
	http.HandleFunc("/send-message", limiter.MessageLimit.RateLimit(handlers.SendMessage))
	http.HandleFunc("/thread-preview", limiter.ViewLimit.RateLimit(handlers.GetThreadPreview))
	http.HandleFunc("/toggle-bot", limiter.ViewLimit.RateLimit(handlers.ToggleBotStatus))
	http.HandleFunc("/chat-messages", limiter.ViewLimit.RateLimit(handlers.GetChatMessages))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
