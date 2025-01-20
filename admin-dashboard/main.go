package main

import (
	"admin-dashboard/db"
	"admin-dashboard/handlers"
	"log"
	"net/http"
	"os"
)

func main() {
	// Initialize database
	db.Init()
	defer db.DB.Close()

	// Create rate limiters with higher limits for development
	limiter := handlers.NewRateLimiter()

	// Routes with more permissive rate limiting
	http.HandleFunc("/", limiter.ViewLimit.RateLimit(handlers.GetMessages))
	http.HandleFunc("/messages", limiter.ViewLimit.RateLimit(handlers.GetMessageList))
	http.HandleFunc("/chat", limiter.ViewLimit.RateLimit(handlers.GetChat))
	http.HandleFunc("/send-message", limiter.MessageLimit.RateLimit(handlers.SendMessage))
	http.HandleFunc("/thread-preview", limiter.ViewLimit.RateLimit(handlers.GetThreadPreview))
	http.HandleFunc("/toggle-bot", limiter.ViewLimit.RateLimit(handlers.ToggleBotStatus))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
