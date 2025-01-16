package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"message-router/sentiment"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var (
	db         *sql.DB // Client Manager DB
	socialDB   *sql.DB // Social Dashboard DB
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	config            Config
	sentimentAnalyzer *sentiment.Analyzer
)

func init() {
	// Set up logging with microsecond precision
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Printf("🚀 Starting Neurocrow Message Router...")

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
		ClientManagerDB:   getEnvOrDie("CLIENT_MANAGER_DATABASE_URL"),
		SocialDashboardDB: getEnvOrDie("SOCIAL_DASHBOARD_DATABASE_URL"),
		FacebookAppSecret: getEnvOrDie("FACEBOOK_APP_SECRET"),
		VerifyToken:       getEnvOrDie("VERIFY_TOKEN"),
		Port:              getEnvOrDefault("PORT", "8080"),
		FireworksKey:      getEnvOrDie("FIREWORKS_API_KEY"),
	}

	// Log configuration (safely)
	log.Printf("📝 Configuration loaded:")
	log.Printf("   Client Manager DB URL length: %d", len(config.ClientManagerDB))
	log.Printf("   Social Dashboard DB URL length: %d", len(config.SocialDashboardDB))
	log.Printf("   Facebook App Secret length: %d", len(config.FacebookAppSecret))
	log.Printf("   Verify Token length: %d", len(config.VerifyToken))
	log.Printf("   Fireworks API Key length: %d", len(config.FireworksKey))
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
	log.Printf("📊 Setting up database connections...")

	// Setup Client Manager DB
	var err error
	for i := 0; i < 3; i++ {
		log.Printf("🔄 Client Manager DB connection attempt %d/3...", i+1)
		if db, err = connectDB(config.ClientManagerDB, "Client Manager"); err == nil {
			log.Printf("✅ Successfully connected to Client Manager DB!")
			break
		}
		log.Printf("❌ Connection attempt %d failed: %v", i+1, err)
		time.Sleep(time.Second * 2)
	}
	if err != nil {
		log.Fatal("❌ Failed to connect to Client Manager DB after 3 attempts")
	}

	// Setup Social Dashboard DB
	for i := 0; i < 3; i++ {
		log.Printf("🔄 Social Dashboard DB connection attempt %d/3...", i+1)
		if socialDB, err = connectDB(config.SocialDashboardDB, "Social Dashboard"); err == nil {
			log.Printf("✅ Successfully connected to Social Dashboard DB!")
			return
		}
		log.Printf("❌ Connection attempt %d failed: %v", i+1, err)
		time.Sleep(time.Second * 2)
	}
	log.Fatal("❌ Failed to connect to Social Dashboard DB after 3 attempts")
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

	// Main webhook endpoint for Facebook
	router.HandleFunc("/webhook", logMiddleware(recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// Check if it's a Botpress request
		if isBotpressRequest(r) {
			log.Printf("✅ Botpress request detected")
			w.WriteHeader(http.StatusOK)
			if r.Method == http.MethodPost {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"status":"ok","message":"Webhook received"}`)
			}
			return
		}

		// If it has Facebook signature headers, treat as Facebook webhook
		if r.Header.Get("X-Hub-Signature-256") != "" {
			log.Printf("✅ Facebook webhook request detected")
			validateFacebookRequest(handleWebhook)(w, r)
			return
		}

		// For any other request, return OK but log it
		log.Printf("ℹ️ Unknown request type to webhook endpoint")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})))

	// New endpoint specifically for Botpress responses
	router.HandleFunc("/botpress-response", logMiddleware(recoverMiddleware(handleBotpressResponse)))

	// Log registered routes
	log.Printf("📍 Registered routes:")
	log.Printf("   - GET/POST/HEAD / (Health Check)")
	log.Printf("   - GET/POST /webhook (Multi-purpose Webhook)")
	log.Printf("   - POST /botpress-response (Botpress Response Handler)")

	return router
}

func cleanup() {
	if db != nil {
		db.Close()
	}
	if socialDB != nil {
		socialDB.Close()
	}
}

func main() {
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

	// Start server
	log.Printf("🌐 Server starting on port %s", config.Port)
	log.Printf("🔗 Local URL: http://localhost:%s", config.Port)
	log.Printf("⚡ Server is ready to handle requests")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}
