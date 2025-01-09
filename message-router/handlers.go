// handlers.go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strings"
    "time"
    "database/sql"
)

// handleWebhook processes incoming webhook requests
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

    // Health check response
    w.WriteHeader(http.StatusOK)
}

func handlePostRequest(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        log.Printf("❌ Error reading request body: %v", err)
        http.Error(w, "Error reading body", http.StatusBadRequest)
        return
    }
    r.Body = io.NopCloser(bytes.NewBuffer(body))

    // Handle Facebook messages
    if r.Header.Get("X-Hub-Signature-256") != "" {
        handleFacebookMessage(w, r, body)
        return
    }

    // Handle Botpress responses
    if isBotpressRequest(r) {
        handleBotpressResponse(w, r)
        return
    }

    // Unknown request type
    w.WriteHeader(http.StatusOK)
}

func handleFacebookMessage(w http.ResponseWriter, r *http.Request, body []byte) {
    var event FacebookEvent
    if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
        log.Printf("❌ Error parsing Facebook webhook: %v", err)
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if !isValidFacebookObject(event.Object) {
        log.Printf("❌ Unsupported webhook object: %s", event.Object)
        http.Error(w, "Unsupported webhook object", http.StatusBadRequest)
        return
    }

    // Send immediate 200 OK to Facebook
    w.WriteHeader(http.StatusOK)

    // Process messages asynchronously
    ctx := context.Background()
    go processMessagesAsync(ctx, event)
}

func processMessagesAsync(ctx context.Context, event FacebookEvent) {
    for _, entry := range event.Entry {
        for _, msg := range entry.Messaging {
            if msg.Message == nil || msg.Message.IsEcho || msg.Message.Text == "" {
                continue
            }

            // Forward message to Botpress
            if err := forwardToBotpress(ctx, entry.ID, msg); err != nil {
                log.Printf("❌ Error forwarding to Botpress: %v", err)
            }
        }
    }
}

func forwardToBotpress(ctx context.Context, pageID string, msg MessagingEntry) error {
    // Create Botpress request
    botpressReq := BotpressRequest{
        ID:             msg.Message.Mid,
        ConversationId: fmt.Sprintf("%s-%s", pageID, msg.Sender.ID),
        Channel:        "facebook",
        Type:          "text",
        Content:       msg.Message.Text,
        Payload: BotpressRequestPayload{
            Text:     msg.Message.Text,
            Type:     "text",
            PageId:   pageID,
            SenderId: msg.Sender.ID,
        },
    }

    // Get Botpress URL
    botpressURL, err := getBotpressURL(ctx, pageID)
    if err != nil {
        return fmt.Errorf("error getting Botpress URL: %v", err)
    }

    // Send to Botpress with retries
    return sendToBotpressWithRetry(ctx, botpressURL, botpressReq)
}

func handleBotpressResponse(w http.ResponseWriter, r *http.Request) {
    // Read the raw body first for logging
    body, err := io.ReadAll(r.Body)
    if err != nil {
        log.Printf("❌ Error reading request body: %v", err)
        w.WriteHeader(http.StatusOK)  // Still return 200 OK
        return
    }
    log.Printf("📥 Received Botpress body: %s", string(body))

    // For validation requests (configuration testing), just return 200 OK
    if len(body) <= 32 { // Validation messages are typically small
        log.Printf("✅ Received Botpress validation request")
        w.WriteHeader(http.StatusOK)
        return
    }

    // Try to parse as a full Botpress response
    var response BotpressResponse
    if err := json.Unmarshal(body, &response); err != nil {
        log.Printf("❌ Error parsing Botpress response: %v", err)
        w.WriteHeader(http.StatusOK)  // Still return 200 OK
        return
    }

    // Process only if we have the necessary information
    if response.ConversationId != "" && response.Payload.Text != "" {
        parts := strings.Split(response.ConversationId, "-")
        if len(parts) != 2 {
            log.Printf("❌ Invalid conversation ID format: %s", response.ConversationId)
            w.WriteHeader(http.StatusOK)
            return
        }

        pageID, senderID := parts[0], parts[1]
        
        // Send response back to Facebook
        ctx := context.Background()
        if err := sendFacebookResponse(ctx, pageID, senderID, response.Payload.Text); err != nil {
            log.Printf("❌ Error sending to Facebook: %v", err)
        }
    }

    // Always return 200 OK to Botpress
    w.WriteHeader(http.StatusOK)
}

func sendFacebookResponse(ctx context.Context, pageID, senderID, message string) error {
    pageToken, err := getPageToken(ctx, pageID)
    if err != nil {
        return fmt.Errorf("error getting page token: %v", err)
    }

    return sendFacebookMessage(ctx, pageID, pageToken, senderID, message)
}

func sendToBotpressWithRetry(ctx context.Context, url string, payload BotpressRequest) error {
    maxRetries := 3
    var lastErr error

    for attempt := 0; attempt < maxRetries; attempt++ {
        if err := sendToBotpress(ctx, url, payload); err != nil {
            lastErr = err
            log.Printf("⚠️ Botpress attempt %d failed: %v", attempt+1, err)
            time.Sleep(time.Second * time.Duration(attempt+1))
            continue
        }
        return nil
    }

    return fmt.Errorf("failed after %d attempts: %v", maxRetries, lastErr)
}

func sendToBotpress(ctx context.Context, url string, payload BotpressRequest) error {
    jsonData, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("error marshaling payload: %v", err)
    }

    log.Printf("🤖 Sending to Botpress:")
    log.Printf("   URL: %s", url)
    log.Printf("   Payload: %s", string(jsonData))

    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("error creating request: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")
    
    resp, err := httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("error sending request: %v", err)
    }
    defer resp.Body.Close()

    // Read and log response for debugging
    respBody, err := io.ReadAll(resp.Body)
    log.Printf("📥 Botpress response (status %d): %s", resp.StatusCode, string(respBody))

    // Any 2xx status code is considered success
    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        log.Printf("✅ Successfully sent message to Botpress")
        return nil
    }

    return fmt.Errorf("received status %d from Botpress", resp.StatusCode)
}

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