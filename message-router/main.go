package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"message-router/sentiment"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// =============================================================================
// LOGGING SYSTEM - Optimized for debugging experience
// =============================================================================

type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

var (
	currentLogLevel LogLevel = LogLevelInfo // Default to info level
	logLevelNames   = map[LogLevel]string{
		LogLevelError: "ERROR",
		LogLevelWarn:  "WARN",
		LogLevelInfo:  "INFO",
		LogLevelDebug: "DEBUG",
	}
)

// Log level helper functions for clean debugging experience
func LogError(format string, args ...interface{}) {
	if currentLogLevel >= LogLevelError {
		log.Printf("❌ "+format, args...)
	}
}

func LogWarn(format string, args ...interface{}) {
	if currentLogLevel >= LogLevelWarn {
		log.Printf("⚠️ "+format, args...)
	}
}

func LogInfo(format string, args ...interface{}) {
	if currentLogLevel >= LogLevelInfo {
		log.Printf("ℹ️ "+format, args...)
	}
}

func LogDebug(format string, args ...interface{}) {
	if currentLogLevel >= LogLevelDebug {
		log.Printf("🔍 "+format, args...)
	}
}

// generateRequestID creates a unique ID for correlating logs across async processing
func generateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("req_%x", b)
}

// setLogLevelFromEnv configures log level from environment variable
func setLogLevelFromEnv() {
	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	switch levelStr {
	case "ERROR":
		currentLogLevel = LogLevelError
	case "WARN", "WARNING":
		currentLogLevel = LogLevelWarn
	case "INFO":
		currentLogLevel = LogLevelInfo
	case "DEBUG":
		currentLogLevel = LogLevelDebug
	default:
		currentLogLevel = LogLevelInfo // Default
	}
	log.Printf("🔧 Log level set to: %s", logLevelNames[currentLogLevel])
}

// =============================================================================
// APPLICATION GLOBALS
// =============================================================================

var (
	db         *sql.DB // Client Manager DB
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	config            Config
	sentimentAnalyzer *sentiment.Analyzer
)

func init() {
	// Set up logging with microsecond precision
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	
	// Configure log level from environment
	setLogLevelFromEnv()
	
	LogInfo("🚀 Starting Neurocrow Message Router...")

	loadConfig()
	setupDatabase()
	setupSentimentAnalyzer()
}

func loadConfig() {
	// Load .env file if present
	if err := godotenv.Load(); err != nil {
		log.Printf("💡 Using platform environment variables (no .env file)")
	}

	config = Config{
		DatabaseURL:       getEnvOrDie("DATABASE_URL"), // Use DATABASE_URL for the single database
		FacebookAppSecret: getEnvOrDie("FACEBOOK_APP_SECRET"),
		VerifyToken:       getEnvOrDie("VERIFY_TOKEN"),
		Port:              getEnvOrDefault("PORT", "8080"),
		FireworksKey:      getEnvOrDie("FIREWORKS_API_KEY"),
		// Facebook Handover Protocol App IDs
		FacebookBotAppID:       1195277397801905, // Your bot's Facebook App ID (detected from existing code)
		FacebookPageInboxAppID: 263902037430900,  // Facebook Page Inbox App ID (constant)
		// Botpress integration (legacy - temporary during migration)
		BotpressToken: os.Getenv("BOTPRESS_TOKEN"), // Optional during migration
		// Note: Dify API keys are now stored per-page in database (multi-tenant)
	}

	// Log configuration (safely)
	log.Printf("📝 Configuration loaded:")
	log.Printf("   Database URL length: %d", len(config.DatabaseURL))
	log.Printf("   Facebook App Secret length: %d", len(config.FacebookAppSecret))
	log.Printf("   Verify Token length: %d", len(config.VerifyToken))
	log.Printf("   Fireworks API Key length: %d", len(config.FireworksKey))
	log.Printf("   Facebook Bot App ID: %d", config.FacebookBotAppID)
	log.Printf("   Facebook Page Inbox App ID: %d", config.FacebookPageInboxAppID)
	log.Printf("   Dify API keys: stored per-page in database (multi-tenant)")
	if config.BotpressToken != "" {
		log.Printf("   Botpress Token length: %d (legacy)", len(config.BotpressToken))
	} else {
		log.Printf("   Botpress Token: not set (migration mode)")
	}
	log.Printf("   Port: %s", config.Port)
}

func setupSentimentAnalyzer() {
	// Initialize sentiment analyzer
	sentimentConfig := sentiment.DefaultConfig()
	sentimentConfig.FireworksKey = config.FireworksKey
	sentimentAnalyzer = sentiment.New(sentimentConfig)

	log.Printf("✅ Sentiment analyzer initialized")
}

func getEnvOrDie(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("❌ %s environment variable is not set", key)
	}
	return value
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func setupDatabase() {
	log.Printf("📊 Setting up database connection...")

	var err error
	for i := 0; i < 3; i++ {
		log.Printf("🔄 Database connection attempt %d/3...", i+1)
		if db, err = connectDB(config.DatabaseURL, "Database"); err == nil {
			log.Printf("✅ Successfully connected to database!")
			return
		}
		log.Printf("❌ Connection attempt %d failed: %v", i+1, err)
		time.Sleep(time.Second * 2)
	}
	log.Fatal("❌ Failed to connect to database after 3 attempts")
}

func connectDB(dbURL string, dbName string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("error opening %s database: %v", dbName, err)
	}

	// Test connection
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging %s database: %v", dbName, err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Printf("⚙️ %s database connection pool configured (max: 25 connections)", dbName)
	return db, nil
}

func logMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log incoming request details
		log.Printf("🔍 Request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		log.Printf("   Headers: %v", r.Header)
		log.Printf("   Query Parameters: %v", r.URL.Query())

		// Call the next handler
		next(w, r)

		// Log request completion
		duration := time.Since(start)
		log.Printf("⏱️ Request completed in %v", duration)
	}
}

func recoverMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the stack trace
				log.Printf("❌ PANIC RECOVERED: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next(w, r)
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		log.Printf("❌ Invalid request path: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	// Allow GET, POST, and HEAD methods
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodHead {
		log.Printf("❌ Invalid method: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Only write body for GET and POST requests
	if r.Method != http.MethodHead {
		fmt.Fprintf(w, `{"status":"healthy","message":"Neurocrow Message Router is running"}`)
	}
}

func setupRouter() *http.ServeMux {
	router := http.NewServeMux()

	// Register routes with middleware
	router.HandleFunc("/", logMiddleware(healthCheckHandler))

	// Main webhook endpoint for Facebook/Instagram
	router.HandleFunc("/webhook", logMiddleware(recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// If it has Facebook signature headers, treat as Facebook webhook
		if r.Header.Get("X-Hub-Signature-256") != "" {
			log.Printf("✅ Facebook/Instagram webhook request detected")
			validateFacebookRequest(handleWebhook)(w, r)
			return
		}

		// For any other request, return OK but log it
		log.Printf("ℹ️ Unknown request type to webhook endpoint")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})))

	// New endpoint for sending messages from the dashboard
	router.HandleFunc("/send-message", logMiddleware(recoverMiddleware(handleSendMessage)))

	// Log registered routes
	log.Printf("📍 Registered routes:")
	log.Printf("   - GET/POST/HEAD / (Health Check)")
	log.Printf("   - GET/POST /webhook (Facebook/Instagram Webhook)")
	log.Printf("   - POST /send-message (Dashboard Message Sender)")
	log.Printf("🤖 AI Integration: Dify (per-page API keys)")
	log.Printf("📊 Database: Multi-tenant client support")

	return router
}

func cleanup() {
	if db != nil {
		log.Printf("🧹 Closing database connection...")
		db.Close()
	}
}

// Legacy bot reactivation system removed - now using Facebook Handover Protocol
// Thread control is managed through native Facebook handover events instead of 
// background workers and 6-hour timers

func main() {
	// Create context for graceful shutdown (used in shutdown signal handling)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Background worker removed - thread control now managed by Facebook Handover Protocol

	// Ensure cleanup on exit
	defer cleanup()

	// Set up router
	router := setupRouter()

	// Configure server
	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("🌐 Server starting on port %s", config.Port)
		log.Printf("🔗 Local URL: http://localhost:%s", config.Port)
		log.Printf("⚡ Server is ready to handle requests")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Printf("🛑 Shutdown signal received, stopping server...")

	// Cancel background workers
	cancel()

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("❌ Server shutdown error: %v", err)
	} else {
		log.Printf("✅ Server stopped gracefully")
	}
}
