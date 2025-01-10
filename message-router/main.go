package main

import (
    "database/sql"
    "log"
    "net/http"
    "os"
    "time"
    "fmt"

    "github.com/joho/godotenv"
    _ "github.com/lib/pq"
)

var (
    db *sql.DB
    httpClient = &http.Client{
        Timeout: 10 * time.Second,
    }
    config Config
)

type Config struct {
    DatabaseURL      string
    FacebookAppSecret string
    VerifyToken      string
    Port             string
}

func init() {
    // Set up logging with microsecond precision
    log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
    log.Printf("🚀 Starting Neurocrow Message Router...")

    loadConfig()
    setupDatabase()
}

func loadConfig() {
    // Load .env file if present
    if err := godotenv.Load(); err != nil {
        log.Printf("💡 Using platform environment variables (no .env file)")
    }

    config = Config{
        DatabaseURL:       getEnvOrDie("DATABASE_URL"),
        FacebookAppSecret: getEnvOrDie("FACEBOOK_APP_SECRET"),
        VerifyToken:      getEnvOrDie("VERIFY_TOKEN"),
        Port:             getEnvOrDefault("PORT", "8080"),
    }

    // Log configuration (safely)
    log.Printf("📝 Configuration loaded:")
    log.Printf("   Database URL length: %d", len(config.DatabaseURL))
    log.Printf("   Facebook App Secret length: %d", len(config.FacebookAppSecret))
    log.Printf("   Verify Token length: %d", len(config.VerifyToken))
    log.Printf("   Port: %s", config.Port)
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
    log.Printf("📊 Database URL configured (length: %d chars)", len(config.DatabaseURL))
    
    var err error
    for i := 0; i < 3; i++ {
        log.Printf("🔄 Database connection attempt %d/3...", i+1)
        if db, err = connectDB(); err == nil {
            log.Printf("✅ Successfully connected to database!")
            return
        }
        log.Printf("❌ Connection attempt %d failed: %v", i+1, err)
        time.Sleep(time.Second * 2)
    }
    
    log.Fatal("❌ Failed to connect to database after 3 attempts")
}

func connectDB() (*sql.DB, error) {
    db, err := sql.Open("postgres", config.DatabaseURL)
    if err != nil {
        return nil, fmt.Errorf("error opening database: %v", err)
    }

    // Test connection
    if err = db.Ping(); err != nil {
        return nil, fmt.Errorf("error pinging database: %v", err)
    }

    // Configure connection pool
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(25)
    db.SetConnMaxLifetime(5 * time.Minute)
    
    log.Printf("⚙️ Database connection pool configured (max: 25 connections)")
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

// Update the healthCheckHandler to allow HEAD requests
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

func botpressHandler(w http.ResponseWriter, r *http.Request) {
    // Always respond with 200 OK for Botpress health checks
    w.WriteHeader(http.StatusOK)
    if r.Method == http.MethodPost {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(w, `{"status":"ok","message":"Webhook received"}`)
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

func main() {
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