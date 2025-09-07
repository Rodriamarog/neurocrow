// oauth/database.go
// EXACT COPY from client-manager/db.go

package oauth

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

// Single database connection for OAuth operations
var SocialDB *sql.DB // Main application database (social_pages table)

// InitDB initializes the OAuth database connection
func InitDB(databaseURL string) {
	LogInfo("📊 Initializing database connection...")

	var err error
	SocialDB, err = sql.Open("postgres", databaseURL)
	if err != nil {
		LogError("Error opening database: %v", err)
		log.Fatal(err)
	}

	// Test the connection
	if err = SocialDB.Ping(); err != nil {
		LogError("Error pinging database: %v", err)
		log.Fatal(err)
	}

	// Configure connection pool for OAuth operations
	SocialDB.SetMaxOpenConns(10)
	SocialDB.SetMaxIdleConns(10)

	LogInfo("✅ Database connection established")
}

// CleanupDB closes the OAuth database connection
func CleanupDB() {
	if SocialDB != nil {
		LogInfo("🧹 Closing database connection...")
		SocialDB.Close()
	}
}
