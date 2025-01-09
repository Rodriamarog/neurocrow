// handlers.go
package main

import (
    "bytes"
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "time"
)

// handleWebhook processes incoming webhook requests from both Facebook and Botpress
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    // Handle GET requests (Facebook verification and health checks)
    if r.Method == "GET" {
        handleGetRequest(w, r)
        return
    }

    // Handle POST requests (incoming messages)
    if r.Method == "POST" {
        handlePostRequest(w, r)
        return
    }

    // Reject all other methods
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleGetRequest handles GET requests for webhook verification
func handleGetRequest(w http.ResponseWriter, r *http.Request) {
    // Extract Facebook verification parameters
    mode := r.URL.Query().Get("hub.mode")
    token := r.URL.Query().Get("hub.verify_token")
    challenge := r.URL.Query().Get("hub.challenge")

    // If Facebook verification parameters are present, handle them
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

    // If no Facebook params, assume it's a health check
    log.Printf("✅ Endpoint check - returning 200 OK")
    w.WriteHeader(http.StatusOK)
}

// handlePostRequest handles POST requests for incoming messages
func handlePostRequest(w http.ResponseWriter, r *http.Request) {
    log.Printf("📨 Incoming webhook from %s", r.RemoteAddr)
    
    // Read request body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        log.Printf("❌ Error reading webhook body: %v", err)
        http.Error(w, "Error reading body", http.StatusBadRequest)
        return
    }
    // Restore body for further reading
    r.Body = io.NopCloser(bytes.NewBuffer(body))
    
    log.Printf("📄 Raw webhook data: %s", string(body))

    // Check if it's a Facebook request by looking for signature
    if r.Header.Get("X-Hub-Signature-256") != "" {
        handleFacebookWebhook(w, r, body)
        return
    }

    // Check if it's a Botpress request
    if isBotpressRequest(r) {
        handleBotpressWebhook(w, r)
        return
    }

    // Unknown webhook source
    log.Printf("⚠️ Unknown webhook source")
    w.WriteHeader(http.StatusOK)
}

// handleFacebookWebhook processes Facebook-specific webhook requests
func handleFacebookWebhook(w http.ResponseWriter, r *http.Request, body []byte) {
    // Parse webhook event
    var event FacebookEvent
    if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
        log.Printf("❌ Error parsing webhook JSON: %v", err)
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Log parsed data
    log.Printf("📦 Parsed webhook data:")
    log.Printf("   Platform: %s", event.Object)
    for _, entry := range event.Entry {
        log.Printf("   Entry ID: %s", entry.ID)
        log.Printf("   Timestamp: %d", entry.Time)
        log.Printf("   Messages: %d", len(entry.Messaging))
    }

    // Validate event type
    if !isValidFacebookObject(event.Object) {
        log.Printf("❌ Unsupported webhook object: %s", event.Object)
        http.Error(w, "Unsupported webhook object", http.StatusBadRequest)
        return
    }

    log.Printf("✅ Webhook data validated successfully")

    // Send immediate 200 OK to Facebook
    w.WriteHeader(http.StatusOK)
    log.Printf("✅ Sent 200 OK response to Facebook")

    // Create background context for async processing
    ctx := context.Background()

    // Process messages asynchronously
    go processWebhook(ctx, event)
}

// handleBotpressWebhook processes Botpress-specific webhook requests
func handleBotpressWebhook(w http.ResponseWriter, r *http.Request) {
    log.Printf("✅ Botpress webhook request detected")
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status":"ok","message":"Webhook received"}`)
}

// processWebhook handles the asynchronous processing of Facebook messages
func processWebhook(ctx context.Context, event FacebookEvent) {
    for _, entry := range event.Entry {
        for _, msg := range entry.Messaging {
            // Skip delivery receipts
            if msg.Delivery != nil {
                processDeliveryReceipt(msg.Delivery)
                continue
            }

            // Skip empty or echo messages
            if msg.Message == nil {
                log.Printf("📝 Skipping non-message event")
                continue
            }
            if msg.Message.IsEcho {
                log.Printf("📝 Skipping echo message with ID: %s", msg.Message.Mid)
                continue
            }
            if msg.Message.Text == "" {
                log.Printf("📝 Skipping empty message")
                continue
            }

            // Handle the message with retries
            if err := handleMessageWithRetry(ctx, entry.ID, msg); err != nil {
                log.Printf("❌ Error handling message after retries: %v", err)
            }
        }
    }
}

// handleMessageWithRetry attempts to process a message with retries
func handleMessageWithRetry(ctx context.Context, pageID string, msg MessagingEntry) error {
    maxRetries := 3
    backoff := time.Second

    for attempt := 1; attempt <= maxRetries; attempt++ {
        err := handleMessage(ctx, pageID, msg)
        if err == nil {
            return nil
        }

        log.Printf("⚠️ Attempt %d/%d failed: %v", attempt, maxRetries, err)
        if attempt < maxRetries {
            time.Sleep(backoff)
            backoff *= 2 // Exponential backoff
        }
    }

    return fmt.Errorf("failed after %d attempts", maxRetries)
}

// handleMessage processes a single message
func handleMessage(ctx context.Context, pageID string, msg MessagingEntry) error {
    log.Printf("🔄 Processing message:")
    log.Printf("   Page ID: %s", pageID)
    log.Printf("   Sender ID: %s", msg.Sender.ID)
    log.Printf("   Message ID: %s", msg.Message.Mid)
    log.Printf("   Content: %s", msg.Message.Text)

    // Get Botpress URL with timeout
    botpressURL, err := getBotpressURL(ctx, pageID)
    if err != nil {
        return fmt.Errorf("error getting Botpress URL: %v", err)
    }

    // Send to Botpress
    botpressResp, err := sendToBotpress(ctx, botpressURL, pageID, msg)
    if err != nil {
        return fmt.Errorf("error sending to Botpress: %v", err)
    }

    // Get page token with timeout
    pageToken, err := getPageToken(ctx, pageID)
    if err != nil {
        return fmt.Errorf("error getting page token: %v", err)
    }

    // Send response back to Facebook
    return sendFacebookMessage(ctx, pageID, pageToken, msg.Sender.ID, botpressResp.Payload.Text)
}

// getBotpressURL retrieves the Botpress webhook URL from the database
func getBotpressURL(ctx context.Context, pageID string) (string, error) {
    // Create a context with timeout
    queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    var botpressURL string
    err := db.QueryRowContext(queryCtx,
        "SELECT botpress_url FROM pages WHERE page_id = $1 AND status = 'active'",
        pageID,
    ).Scan(&botpressURL)

    if err != nil {
        if err == sql.ErrNoRows {
            log.Printf("❌ No active Botpress URL found for page %s", pageID)
            return "", fmt.Errorf("no active Botpress URL found for page %s", pageID)
        }
        log.Printf("❌ Database error querying Botpress URL: %v", err)
        return "", fmt.Errorf("database error: %v", err)
    }

    log.Printf("✅ Found Botpress URL for page %s", pageID)
    return botpressURL, nil
}

// sendToBotpress forwards the message to Botpress
func sendToBotpress(ctx context.Context, botpressURL string, pageID string, msg MessagingEntry) (*BotpressResponse, error) {
    // Create request payload
    botpressPayload := map[string]interface{}{
        "id": msg.Message.Mid,
        "conversationId": fmt.Sprintf("%s-%s", pageID, msg.Sender.ID),
        "channel": "facebook",
        "type": "text",
        "content": msg.Message.Text,
        "payload": map[string]interface{}{
            "text": msg.Message.Text,
            "type": "text",
            "pageId": pageID,
            "senderId": msg.Sender.ID,
        },
    }

    jsonData, err := json.Marshal(botpressPayload)
    if err != nil {
        return nil, fmt.Errorf("error creating Botpress payload: %v", err)
    }

    // Create request with timeout context
    reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(reqCtx, "POST", botpressURL, bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("error creating Botpress request: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")
    
    log.Printf("📤 Sending to Botpress:")
    log.Printf("   URL: %s", botpressURL)
    log.Printf("   Payload: %s", string(jsonData))

    // Send request
    resp, err := httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("error sending to Botpress: %v", err)
    }
    defer resp.Body.Close()

    // Read response
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("error reading Botpress response: %v", err)
    }

    log.Printf("📩 Raw Botpress response (status %d): %s", resp.StatusCode, string(body))

    // Parse response
    var botpressResp BotpressResponse
    if err := json.Unmarshal(body, &botpressResp); err != nil {
        return nil, fmt.Errorf("error parsing Botpress response: %v", err)
    }

    return &botpressResp, nil
}
