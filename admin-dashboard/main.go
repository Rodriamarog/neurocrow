// main.go
package main

import (
	"admin-dashboard/cache"
	"admin-dashboard/db"
	"admin-dashboard/handlers"
	"admin-dashboard/pkg/auth"
	"admin-dashboard/pkg/template"
	"admin-dashboard/ws"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Printf("🚀 Starting server initialization...")

	// Initialize templates first
	log.Printf("📝 Initializing templates...")
	template.InitTemplates()

	// Initialize database
	log.Printf("🗄️ Initializing database...")
	db.Init()
	defer func() {
		log.Printf("🔌 Closing database connection...")
		db.Close()
	}()

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

	// Start WebSocket hub
	log.Printf("🔌 Starting WebSocket hub...")
	go ws.GlobalHub.Run()

	// Create rate limiters
	log.Printf("⚙️ Setting up rate limiters...")
	limiter := handlers.NewRateLimiter()

	// Set up routes
	log.Printf("🛣️ Setting up routes...")

	// WebSocket route with logging
	log.Printf("🔌 Setting up WebSocket route...")
	http.HandleFunc("/ws-connect", auth.AuthMiddleware(ws.HandleWebSocket))

	// Auth routes
	log.Printf("🔐 Setting up auth routes...")
	http.HandleFunc("/login", handlers.Login)
	http.HandleFunc("/logout", handlers.Logout)

	// Protected routes with rate limiting
	log.Printf("🔒 Setting up protected routes...")
	http.HandleFunc("/", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.GetMessages)))
	http.HandleFunc("/message-list", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.GetMessageList)))
	http.HandleFunc("/chat", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.GetChat)))
	http.HandleFunc("/send-message", auth.AuthMiddleware(limiter.MessageLimit.RateLimit(handlers.SendMessage)))
	http.HandleFunc("/thread-preview", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.GetThreadPreview)))
	http.HandleFunc("/toggle-bot", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.ToggleBotStatus)))
	http.HandleFunc("/chat-messages", auth.AuthMiddleware(limiter.ViewLimit.RateLimit(handlers.GetChatMessages)))

	// Serve static files
	log.Printf("📁 Setting up static file server...")
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create server with timeouts
	srv := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("✅ Server initialization complete")
		log.Printf("🌐 Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("🛑 Server is shutting down...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("❌ Server forced to shutdown: %v", err)
	}

	log.Printf("✅ Server exited gracefully")
}
