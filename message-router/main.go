// main.go
package main

import (
    "database/sql"
    "log"
    "net/http"
    "os"
    "time"

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
    log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
    log.Printf("🚀 Starting Neurocrow Message Router...")

    loadConfig()
    setupDatabase()
}

func loadConfig() {
    if err := godotenv.Load(); err != nil {
        log.Printf("💡 Using platform environment variables (no .env file)")
    }

    config = Config{
        DatabaseURL:       getEnvOrDie("DATABASE_URL"),
        FacebookAppSecret: getEnvOrDie("FACEBOOK_APP_SECRET"),
        VerifyToken:      getEnvOrDie("VERIFY_TOKEN"),
        Port:             getEnvOrDefault("PORT", "8080"),
    }
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
        return nil, err
    }

    if err = db.Ping(); err != nil {
        return nil, err
    }

    // Set connection pool settings
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(25)
    db.SetConnMaxLifetime(5 * time.Minute)
    
    log.Printf("⚙️ Database connection pool configured (max: 25 connections)")
    return db, nil
}

func recoverMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("❌ PANIC RECOVERED: %v", err)
                http.Error(w, "Internal server error", http.StatusInternalServerError)
            }
        }()
        next(w, r)
    }
}

func main() {
    router := http.NewServeMux()
    router.HandleFunc("/webhook", recoverMiddleware(validateFacebookRequest(handleWebhook)))

    log.Printf("🌐 Server starting on port %s", config.Port)
    log.Fatal(http.ListenAndServe(":"+config.Port, router))
}