// Package main implements the Neurocrow Message Router service.
//
// This service processes Facebook Messenger and Instagram Direct Message webhooks,
// analyzes message sentiment, and intelligently routes conversations between users
// and AI chatbots. It serves as the core messaging infrastructure for the Neurocrow
// platform, handling multi-tenant client configurations and automated conversation management.
//
// Key Features:
//
//   - Facebook/Instagram webhook processing with signature validation
//   - Sentiment analysis using Fireworks AI to determine routing decisions
//   - Simple bot enable/disable control based on user requests and human agent activity
//   - AI chatbot integration through Dify API (multi-tenant with per-page API keys)
//   - Multi-tenant architecture supporting multiple clients and social media pages
//   - Automatic bot reactivation after 12 hours of human agent inactivity
//   - Comprehensive logging and error handling with request correlation
//
// Architecture:
//
// The service follows a webhook-driven architecture where incoming messages flow through:
//  1. Webhook validation and parsing
//  2. Message filtering and echo detection
//  3. Sentiment analysis for routing decisions
//  4. Bot enable/disable control (simple boolean flag system)
//  5. Response generation via AI or human escalation
//
// Database Schema:
//
// Multi-tenant structure: clients → social_pages → conversations → messages
//   - clients: Top-level client organizations
//   - social_pages: Facebook/Instagram pages with API credentials and Dify keys
//   - conversations: User conversation threads with bot control state
//   - messages: Individual messages with source tracking and routing metadata
//
// Bot Control:
//
// The service uses a simple bot enable/disable system:
//   - bot_enabled: Boolean flag controlling whether the bot processes messages
//   - Disabled when users request human help or agents intervene
//   - Automatically re-enabled after 12 hours of human agent inactivity
//
// Integration Points:
//
//   - Facebook Graph API: Message sending operations
//   - Fireworks AI API: Sentiment analysis for routing decisions
//   - Dify AI API: Chatbot response generation (per-tenant API keys)
//   - PostgreSQL: Multi-tenant conversation and message storage
//
// The service is designed for high availability with graceful error handling,
// automatic retries, and fallback mechanisms to ensure reliable message delivery
// and conversation continuity.
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
	"sync"
	"syscall"
	"time"

	"message-router/oauth"
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
	logLevelNames            = map[LogLevel]string{
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

	// Instagram bot flag system - tracks which messages are bot responses
	botFlags      = make(map[string]bool) // conversation_id -> is_bot_message
	botFlagsMutex = sync.RWMutex{}
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

	// Initialize OAuth database connections
	oauth.InitDB(config.DatabaseURL)
}

func loadConfig() {
	// Load .env file if present
	if err := godotenv.Load(); err != nil {
		log.Printf("💡 Using platform environment variables (no .env file)")
	}

	config = Config{
		DatabaseURL:       getEnvOrDie("DATABASE_URL"), // Use DATABASE_URL for the single database
		FacebookAppSecret: getEnvOrDie("FACEBOOK_APP_SECRET"),
		FacebookAppID:     getEnvOrDie("FACEBOOK_APP_ID"), // Added for OAuth functionality
		VerifyToken:       getEnvOrDie("VERIFY_TOKEN"),
		Port:              getEnvOrDefault("PORT", "8080"),
		FireworksKey:      getEnvOrDie("FIREWORKS_API_KEY"),
		// Instagram OAuth credentials
		InstagramAppID:        getEnvOrDie("INSTAGRAM_APP_ID"),         // Added for Instagram OAuth
		InstagramAppSecretKey: getEnvOrDie("INSTAGRAM_APP_SECRET_KEY"), // Added for Instagram OAuth
		// Facebook App IDs for echo message detection
		FacebookBotAppID:       1195277397801905, // Your bot's Facebook App ID (detected from existing code)
		FacebookPageInboxAppID: 263902037430900,  // Facebook Page Inbox App ID (unused)
		// Botpress integration (legacy - temporary during migration)
		BotpressToken: os.Getenv("BOTPRESS_TOKEN"), // Optional during migration
		// Note: Dify API keys are now stored per-page in database (multi-tenant)
	}

	// Log configuration (safely)
	log.Printf("📝 Configuration loaded:")
	log.Printf("   Database URL length: %d", len(config.DatabaseURL))
	log.Printf("   Facebook App Secret length: %d", len(config.FacebookAppSecret))
	log.Printf("   Facebook App ID length: %d", len(config.FacebookAppID))
	log.Printf("   Instagram App ID length: %d", len(config.InstagramAppID))
	log.Printf("   Instagram App Secret Key length: %d", len(config.InstagramAppSecretKey))
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

	// Instagram bot flag endpoint for Dify integration
	router.HandleFunc("/api/mark-bot-response", logMiddleware(recoverMiddleware(handleMarkBotResponse)))

	// OAuth endpoints for client onboarding
	router.HandleFunc("/facebook-token", logMiddleware(recoverMiddleware(oauth.HandleFacebookToken)))
	router.HandleFunc("/facebook-business-token", logMiddleware(recoverMiddleware(oauth.HandleFacebookBusinessToken)))
	router.HandleFunc("/instagram-token", logMiddleware(recoverMiddleware(oauth.HandleInstagramToken)))
	router.HandleFunc("/instagram-token-exchange", logMiddleware(recoverMiddleware(oauth.HandleInstagramTokenExchange)))

	// Log registered routes
	log.Printf("📍 Registered routes:")
	log.Printf("   - GET/POST/HEAD / (Health Check)")
	log.Printf("   - GET/POST /webhook (Facebook/Instagram Webhook)")
	log.Printf("   - POST /send-message (Dashboard Message Sender)")
	log.Printf("   - POST /api/mark-bot-response (Instagram Bot Flag)")
	log.Printf("   - POST /facebook-token (Facebook OAuth)")
	log.Printf("   - POST /facebook-business-token (Facebook Business OAuth)")
	log.Printf("   - POST /instagram-token (Instagram OAuth)")
	log.Printf("   - POST /instagram-token-exchange (Instagram Token Exchange)")
	log.Printf("🤖 AI Integration: Dify (per-page API keys)")
	log.Printf("📊 Database: Multi-tenant client support")
	log.Printf("📱 Instagram: Bot flag system for reliable human/bot detection")
	log.Printf("🔐 OAuth: Facebook & Instagram client onboarding")

	return router
}

func cleanup() {
	if db != nil {
		log.Printf("🧹 Closing database connection...")
		db.Close()
	}
	// Cleanup OAuth database connections
	oauth.CleanupDB()
}

// =============================================================================
// INSTAGRAM BOT FLAG SYSTEM
// =============================================================================

// setBotFlag marks a conversation as having a bot response pending
func setBotFlag(conversationID string) {
	botFlagsMutex.Lock()
	defer botFlagsMutex.Unlock()
	botFlags[conversationID] = true
	log.Printf("🤖 Bot flag SET for conversation: %s", conversationID)
}

// hasBotFlag checks if a conversation has a bot response flag
func hasBotFlag(conversationID string) bool {
	botFlagsMutex.RLock()
	defer botFlagsMutex.RUnlock()
	hasFlag := botFlags[conversationID]
	log.Printf("🔍 Bot flag CHECK for conversation %s: %v", conversationID, hasFlag)
	return hasFlag
}

// clearBotFlag removes the bot response flag for a conversation
func clearBotFlag(conversationID string) {
	botFlagsMutex.Lock()
	defer botFlagsMutex.Unlock()
	delete(botFlags, conversationID)
	log.Printf("🗑️ Bot flag CLEARED for conversation: %s", conversationID)
}

// handleMarkBotResponse handles the API endpoint for Dify to mark bot responses
func handleMarkBotResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conversationID := r.URL.Query().Get("conversation_id")
	if conversationID == "" {
		http.Error(w, "Missing conversation_id parameter", http.StatusBadRequest)
		return
	}

	setBotFlag(conversationID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"success","conversation_id":"%s","message":"Bot flag set"}`, conversationID)
}

// Bot reactivation system: 12-hour rule with message-triggered checks
// System auto-disables bot when human agents respond, auto-reactivates after 12 hours of inactivity
// No background workers needed - reactivation check runs on each message processing

func main() {
	// Create context for graceful shutdown (used in shutdown signal handling)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Bot reactivation now happens on message processing (no background worker needed)

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
		log.Printf("🤖 Bot auto-reactivation enabled (12-hour rule, message-triggered)")
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
