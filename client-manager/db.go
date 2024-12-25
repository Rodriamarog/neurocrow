// db.go
package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

// Make DB accessible to other files
var DB *sql.DB

func initDB() {
	var err error
	DB, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatal(err)
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
