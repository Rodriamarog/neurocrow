// db.go
package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// Make both databases accessible to other files
var DB *sql.DB       // Client-manager database (pages table)
var SocialDB *sql.DB // Main application database (social_pages table)

func initDB() {
	// Connect to client-manager database
	dbURL := os.Getenv("DATABASE_URL")
	log.Printf("Attempting to connect to client-manager database with URL: %s", dbURL)

	var err error
	DB, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Printf("Error opening client-manager database: %v", err)
		log.Fatal(err)
	}

	err = DB.Ping()
	if err != nil {
		log.Printf("Error pinging client-manager database: %v", err)
		log.Fatal(err)
	}

	log.Printf("Successfully connected to client-manager database")

	// Connect to main application database
	socialDbURL := os.Getenv("SOCIAL_DASHBOARD_DATABASE_URL")
	if socialDbURL == "" {
		log.Printf("⚠️ SOCIAL_DASHBOARD_DATABASE_URL not set - will only write to client-manager database")
		SocialDB = nil
	} else {
		log.Printf("Attempting to connect to social dashboard database")

		SocialDB, err = sql.Open("postgres", socialDbURL)
		if err != nil {
			log.Printf("Error opening social dashboard database: %v", err)
			log.Fatal(err)
		}

		err = SocialDB.Ping()
		if err != nil {
			log.Printf("Error pinging social dashboard database: %v", err)
			log.Fatal(err)
		}

		log.Printf("Successfully connected to social dashboard database")
	}

	// Create tables if they don't exist
	_, err = DB.Exec(`
        CREATE TABLE IF NOT EXISTS clients (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            name TEXT NOT NULL,
            email TEXT UNIQUE NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );

        CREATE TABLE IF NOT EXISTS pages (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            client_id UUID REFERENCES clients(id),
            platform TEXT NOT NULL,
            page_id TEXT NOT NULL,
            name TEXT NOT NULL,
            access_token TEXT NOT NULL,
            status TEXT NOT NULL DEFAULT 'pending',
            botpress_url TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            activated_at TIMESTAMP,
            UNIQUE(platform, page_id)
        );
    `)
	if err != nil {
		log.Fatal(err)
	}
}
