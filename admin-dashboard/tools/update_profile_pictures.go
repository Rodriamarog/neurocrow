package main

import (
	"admin-dashboard/pkg/meta"
	"database/sql"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Try to load .env from current directory and parent directory
	if err := godotenv.Load(); err != nil {
		if err := godotenv.Load("../.env"); err != nil {
			log.Fatal("Error loading .env file:", err)
		}
	}

	// Get database URL from environment
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL not set in .env file")
	}

	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	defer db.Close()

	// Test connection
	err = db.Ping()
	if err != nil {
		log.Fatal("Error pinging database:", err)
	}

	log.Println("Successfully connected to database! Starting profile picture updates...")

	// Get all conversations that need profile pictures
	rows, err := db.Query(`
       SELECT DISTINCT 
           c.thread_id,
           c.platform,
           sp.access_token
       FROM conversations c
       JOIN social_pages sp ON c.page_id = sp.id
       WHERE (c.profile_picture_url IS NULL OR c.profile_picture_url = '')
       AND NOT c.thread_id LIKE 'thread_%'  -- Skip test threads
       ORDER BY c.platform, c.thread_id
   `)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var updatedCount, skippedCount int

	for rows.Next() {
		var threadID, platform, accessToken string
		if err := rows.Scan(&threadID, &platform, &accessToken); err != nil {
			log.Printf("❌ Error scanning row: %v", err)
			continue
		}

		log.Printf("🔄 Processing thread %s (%s)", threadID, platform)

		err = meta.UpdateProfilePictureInDB(db, threadID, accessToken, platform)
		if err != nil {
			log.Printf("❌ Failed to update profile picture for thread %s: %v", threadID, err)
			skippedCount++
			continue
		}

		updatedCount++
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating rows: %v", err)
	}

	log.Printf("Profile picture update completed!")
	log.Printf("Successfully updated: %d profiles", updatedCount)
	log.Printf("Skipped/Failed: %d profiles", skippedCount)
}
