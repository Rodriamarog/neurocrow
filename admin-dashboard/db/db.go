package db

import (
    "database/sql"
    "log"
    "os"
    _ "github.com/lib/pq"
    "github.com/joho/godotenv"
)

var DB *sql.DB

func Init() {
    // Load .env file
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Error loading .env file:", err)
    }

    // Get database URL from environment
    connStr := os.Getenv("DATABASE_URL")
    if connStr == "" {
        log.Fatal("DATABASE_URL not set in .env file")
    }

    // Connect to database
    DB, err = sql.Open("postgres", connStr)
    if err != nil {
        log.Fatal("Error connecting to database:", err)
    }

    // Test connection
    err = DB.Ping()
    if err != nil {
        log.Fatal("Error pinging database:", err)
    }

    log.Println("Successfully connected to database!")
}