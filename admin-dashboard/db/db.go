package db

import (
	"database/sql"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}

	// Try DATABASE_URL first
	connStr := os.Getenv("DATABASE_URL")
	if connStr != "" {
		DB, err = connect(connStr)
		if err == nil {
			log.Println("Successfully connected using DATABASE_URL!")
			return
		}
		log.Printf("Failed to connect with DATABASE_URL: %v", err)
	}

	// Try DATABASE_IPV4 as fallback
	ipv4Str := os.Getenv("DATABASE_IPV4")
	if ipv4Str == "" {
		log.Fatal("Both DATABASE_URL and DATABASE_IPV4 connection attempts failed")
	}

	DB, err = connect(ipv4Str)
	if err != nil {
		log.Fatal("Failed to connect with DATABASE_IPV4:", err)
	}
	log.Println("Successfully connected using DATABASE_IPV4!")
}

func connect(connStr string) (*sql.DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func Close() {
	if DB != nil {
		DB.Close()
	}
}
