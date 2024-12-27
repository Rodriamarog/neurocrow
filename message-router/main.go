// main.go
package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var (
	db         *sql.DB
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found")
	}

	// Connect to database with retry logic
	dbURL := os.Getenv("DATABASE_URL")
	var err error
	for i := 0; i < 3; i++ {
		db, err = sql.Open("postgres", dbURL)
		if err == nil {
			if err = db.Ping(); err == nil {
				break
			}
		}
		log.Printf("Failed to connect to database, attempt %d: %v", i+1, err)
		time.Sleep(time.Second * 2)
	}
	if err != nil {
		log.Fatal("Failed to connect to database after 3 attempts")
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
}

func main() {
	router := http.NewServeMux()

	router.HandleFunc("/webhook", recoverMiddleware(validateFacebookRequest(handleWebhook)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func recoverMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Recovered from panic: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next(w, r)
	}
}

func validateFacebookRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			signature := r.Header.Get("X-Hub-Signature-256")
			if signature == "" {
				http.Error(w, "Missing signature", http.StatusUnauthorized)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Error reading body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			appSecret := os.Getenv("FACEBOOK_APP_SECRET")
			expectedSig := generateFacebookSignature(body, []byte(appSecret))

			if !hmac.Equal([]byte(signature[7:]), []byte(expectedSig)) {
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func generateFacebookSignature(body []byte, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Handle Facebook webhook verification
		verifyToken := os.Getenv("VERIFY_TOKEN")
		mode := r.URL.Query().Get("hub.mode")
		token := r.URL.Query().Get("hub.verify_token")
		challenge := r.URL.Query().Get("hub.challenge")

		if mode == "subscribe" && token == verifyToken {
			w.Write([]byte(challenge))
			return
		}
		http.Error(w, "Invalid verification token", http.StatusForbidden)
		return
	}

	if r.Method == "POST" {
		// Parse incoming webhook
		var event struct {
			Object string `json:"object"`
			Entry  []struct {
				ID      string `json:"id"`
				Time    int64  `json:"time"`
				Changes []struct {
					Value struct {
						PageID  string                 `json:"page_id"`
						Message map[string]interface{} `json:"message,omitempty"`
					} `json:"value"`
				} `json:"changes"`
			} `json:"entry"`
		}

		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate event type
		if event.Object != "page" && event.Object != "instagram" {
			http.Error(w, "Unsupported webhook object", http.StatusBadRequest)
			return
		}

		// Facebook expects a quick 200 OK
		w.WriteHeader(http.StatusOK)

		// Process messages asynchronously
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			for _, entry := range event.Entry {
				for _, change := range entry.Changes {
					pageID := change.Value.PageID

					// Look up Botpress webhook URL for this page
					var botpressURL string
					err := db.QueryRowContext(ctx,
						"SELECT botpress_url FROM pages WHERE page_id = $1 AND status = 'active'",
						pageID,
					).Scan(&botpressURL)

					if err != nil {
						if err == sql.ErrNoRows {
							log.Printf("No active Botpress URL found for page %s", pageID)
							continue
						}
						log.Printf("Database error: %v", err)
						continue
					}

					// Forward to Botpress
					payload := map[string]interface{}{
						"type":    event.Object,
						"pageId":  pageID,
						"message": change.Value.Message,
					}

					jsonData, err := json.Marshal(payload)
					if err != nil {
						log.Printf("Error marshaling payload: %v", err)
						continue
					}

					req, err := http.NewRequestWithContext(ctx, "POST", botpressURL, bytes.NewBuffer(jsonData))
					if err != nil {
						log.Printf("Error creating request: %v", err)
						continue
					}

					req.Header.Set("Content-Type", "application/json")

					resp, err := httpClient.Do(req)
					if err != nil {
						log.Printf("Error sending to Botpress: %v", err)
						continue
					}

					if resp.StatusCode != http.StatusOK {
						body, _ := io.ReadAll(resp.Body)
						log.Printf("Botpress error (status %d): %s", resp.StatusCode, string(body))
					}
					resp.Body.Close()
				}
			}
		}()
	}
}
