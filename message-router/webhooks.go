// webhooks.go
package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

// handleWebhook processes incoming webhook requests from Facebook and Instagram.
//
// This function serves as the main entry point for all webhook traffic from social
// media platforms. It handles both webhook verification (GET requests) and message
// processing (POST requests) while implementing proper HTTP method routing.
//
// Request Types:
//
//	GET requests: Facebook webhook verification during initial setup
//	 - Validates webhook subscription with verify token
//	 - Returns challenge parameter to complete verification
//	 - Required for Facebook to accept the webhook URL
//
//	POST requests: Actual message and event data from Facebook/Instagram
//	 - Contains user messages and delivery receipts
//	 - Processed asynchronously to prevent timeout issues
//	 - Validated using HMAC-SHA256 signature verification
//
// Security:
//
// All POST requests must include a valid X-Hub-Signature-256 header that matches
// the computed HMAC signature of the request body using the Facebook App Secret.
// This ensures requests originate from Facebook and haven't been tampered with.
//
// The function delegates signature validation to the validateFacebookRequest
// middleware, which is applied before this handler is called.
//
// Parameters:
//   - w: HTTP response writer for sending responses back to Facebook
//   - r: HTTP request containing webhook data or verification parameters
//
// Response Behavior:
//   - GET: Returns challenge parameter for successful verification, or 403 for invalid tokens
//   - POST: Always returns 200 OK immediately to prevent Facebook retries
//   - Other methods: Returns 405 Method Not Allowed
//
// The function ensures Facebook receives timely responses while processing
// message data asynchronously to handle complex operations without timeouts.
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		handleGetRequest(w, r)
		return
	}

	if r.Method == "POST" {
		handlePostRequest(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleGetRequest handles Facebook webhook verification (GET requests)
func handleGetRequest(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode != "" && token != "" && challenge != "" {
		verifyToken := os.Getenv("VERIFY_TOKEN")

		if mode == "subscribe" && token == verifyToken {
			log.Printf("✅ Facebook webhook verification successful!")
			w.Write([]byte(challenge))
			return
		}
		log.Printf("❌ Facebook webhook verification failed")
		http.Error(w, "Invalid verification token", http.StatusForbidden)
		return
	}

	log.Printf("❌ Incomplete webhook verification parameters")
	http.Error(w, "Bad request", http.StatusBadRequest)
}

// handlePostRequest handles incoming webhook data (POST requests)
func handlePostRequest(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// Always respond 200 OK immediately to Facebook to prevent retries
	w.WriteHeader(http.StatusOK)

	// Process the webhook data asynchronously
	handlePlatformMessage(w, r, body)
}

// handlePlatformMessage processes Facebook/Instagram webhook messages
func handlePlatformMessage(w http.ResponseWriter, r *http.Request, body []byte) {
	// Generate request ID for log correlation
	requestID := generateRequestID()

	// Log webhook reception (optimized)
	LogDebug("[%s] 📥 Raw webhook payload: %d bytes", requestID, len(body))

	// Parse webhook event
	var event FacebookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		LogError("[%s] Error parsing webhook JSON: %v", requestID, err)
		return
	}

	// Count total messages across all entries
	totalMessages := 0
	for _, entry := range event.Entry {
		totalMessages += len(entry.Messaging)
	}

	// Single consolidated log for webhook details
	LogInfo("[%s] 📝 Webhook: %s, %d entries, %d messages",
		requestID, event.Object, len(event.Entry), totalMessages)

	// Additional debug logging for entries
	for i, entry := range event.Entry {
		LogInfo("[%s] 📋 Entry %d: id=%s, messages=%d", requestID, i, entry.ID, len(entry.Messaging))
	}

	// Skip processing if no messages
	if totalMessages == 0 {
		LogDebug("[%s] No messages to process", requestID)
		return
	}

	// Process messages asynchronously to avoid blocking webhook response
	ctx := context.Background()
	go processMessagesAsync(ctx, event, requestID)
}

