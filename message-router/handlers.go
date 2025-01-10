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

    // Handle Facebook/Instagram messages
    if r.Header.Get("X-Hub-Signature-256") != "" {
        handlePlatformMessage(w, r, body)
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

func handlePlatformMessage(w http.ResponseWriter, r *http.Request, body []byte) {
    // Log the raw webhook payload
    log.Printf("📥 Raw webhook payload: %s", string(body))

    var event FacebookEvent
    if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
        log.Printf("❌ Error parsing webhook: %v", err)
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Log parsed event details
    log.Printf("📝 Parsed webhook details:")
    log.Printf("   Object type: %s", event.Object)
    log.Printf("   Number of entries: %d", len(event.Entry))

    if !isValidFacebookObject(event.Object) {
        log.Printf("❌ Unsupported webhook object: %s", event.Object)
        http.Error(w, "Unsupported webhook object", http.StatusBadRequest)
        return
    }

    // Send immediate 200 OK
    w.WriteHeader(http.StatusOK)

    // Process messages asynchronously
    ctx := context.Background()
    go processMessagesAsync(ctx, event)
}

func processMessagesAsync(ctx context.Context, event FacebookEvent) {
    log.Printf("🔄 Processing messages asynchronously")
    for i, entry := range event.Entry {
        log.Printf("📝 Processing entry %d:", i+1)
        log.Printf("   Entry ID: %s", entry.ID)
        log.Printf("   Entry Time: %d", entry.Time)

        // Log the complete entry structure
        entryJSON, _ := json.MarshalIndent(entry, "", "  ")
        log.Printf("📝 Entry structure:\n%s", string(entryJSON))

        // Handle both Facebook and Instagram messages similarly since they use the same structure
        if len(entry.Messaging) == 0 {
            log.Printf("ℹ️ No messages in entry")
            continue
        }
        
        for j, msg := range entry.Messaging {
            log.Printf("   Message %d:", j+1)
            log.Printf("      Sender ID: %s", msg.Sender.ID)
            log.Printf("      Recipient ID: %s", msg.Recipient.ID)
            
            if msg.Message == nil {
                log.Printf("      ⚠️ No message content")
                continue
            }
            if msg.Message.IsEcho {
                log.Printf("      ⚠️ Echo message - skipping")
                continue
            }
            if msg.Message.Text == "" {
                log.Printf("      ⚠️ Empty message text")
                continue
            }

            // Forward to Botpress using the appropriate platform
            platform := event.Object // This will be either "facebook" or "instagram"
            if err := forwardToBotpress(ctx, entry.ID, msg, platform); err != nil {
                log.Printf("❌ Error forwarding to Botpress: %v", err)
            }
        }
    }
}

func forwardInstagramToBotpress(ctx context.Context, pageID string, msg InstagramMessage) error {
    // Create Botpress request for Instagram
    botpressReq := BotpressRequest{
        ID:             msg.ID,
        ConversationId: fmt.Sprintf("%s-%s", pageID, msg.From.ID),
        Channel:        "instagram",
        Type:          "text",
        Content:       msg.Text,
        Payload: BotpressRequestPayload{
            Text:     msg.Text,
            Type:     "text",
            PageId:   pageID,
            SenderId: msg.From.ID,
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

func forwardToBotpress(ctx context.Context, pageID string, msg MessagingEntry, platform string) error {
    // Create Botpress request
    botpressReq := BotpressRequest{
        ID:             msg.Message.Mid,
        ConversationId: fmt.Sprintf("%s-%s", pageID, msg.Sender.ID),
        Channel:        platform,
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
        
        // Get page info to determine platform
        ctx := context.Background()
        pageInfo, err := getPageInfo(ctx, pageID)
        if err != nil {
            log.Printf("❌ Error getting page info: %v", err)
            w.WriteHeader(http.StatusOK)
            return
        }

        // Send response based on platform
        if err := sendPlatformResponse(ctx, pageInfo, senderID, response.Payload.Text); err != nil {
            log.Printf("❌ Error sending platform response: %v", err)
        }
    }

    // Always return 200 OK to Botpress
    w.WriteHeader(http.StatusOK)
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
    // Step 1: Structure the payload according to Botpress Messaging API requirements
    botpressPayload := map[string]interface{}{
        "userId": payload.Payload.SenderId,       // Using sender ID as the user identifier
        "messageId": payload.ID,                  // Message ID for deduplication
        "conversationId": payload.ConversationId, // Compound ID (pageId-senderId)
        "type": "text",
        "text": payload.Content,                  // The actual message content
        "payload": map[string]interface{}{        // Additional context and metadata
            "source": payload.Channel,            // "facebook" or "instagram"
            "pageId": payload.Payload.PageId,
            "senderId": payload.Payload.SenderId,
            "originalPayload": payload.Payload,   // Keep original data for reference
        },
    }

    // Step 2: Convert payload to JSON and log it
    jsonData, err := json.Marshal(botpressPayload)
    if err != nil {
        return fmt.Errorf("error marshaling Botpress payload: %v", err)
    }

    // Pretty print the payload for better logging
    var prettyJSON bytes.Buffer
    if err := json.Indent(&prettyJSON, jsonData, "", "  "); err != nil {
        log.Printf("⚠️ Warning: Could not pretty print JSON: %v", err)
    }

    log.Printf("🤖 Preparing Botpress request:")
    log.Printf("   URL: %s", url)
    log.Printf("   Payload:\n%s", prettyJSON.String())

    // Step 3: Create and configure the HTTP request
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("error creating Botpress request: %v", err)
    }

    // Add required headers
    token := os.Getenv("BOTPRESS_TOKEN")
    if token == "" {
        return fmt.Errorf("BOTPRESS_TOKEN environment variable is not set")
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "bearer "+token)

    log.Printf("📤 Request headers:")
    for key, values := range req.Header {
        log.Printf("   %s: %s", key, values)
    }

    // Step 4: Send the request and handle the response
    start := time.Now()
    resp, err := httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("error sending request to Botpress: %v", err)
    }
    defer resp.Body.Close()

    // Step 5: Read and log the complete response
    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("error reading Botpress response: %v", err)
    }

    // Try to pretty print the response if it's JSON
    var prettyResp bytes.Buffer
    if json.Valid(respBody) {
        if err := json.Indent(&prettyResp, respBody, "", "  "); err != nil {
            log.Printf("⚠️ Warning: Could not pretty print response: %v", err)
        }
    }

    log.Printf("📥 Botpress response after %v:", time.Since(start))
    log.Printf("   Status: %d %s", resp.StatusCode, resp.Status)
    log.Printf("   Headers: %v", resp.Header)
    if prettyResp.Len() > 0 {
        log.Printf("   Body:\n%s", prettyResp.String())
    } else {
        log.Printf("   Body: %s", string(respBody))
    }

    // Step 6: Handle different response scenarios
    // First check for error response format
    var errorResp struct {
        Code    int    `json:"code"`
        Type    string `json:"type"`
        Message string `json:"message"`
    }

    if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Code != 0 {
        return fmt.Errorf("Botpress error: %s (Code: %d, Type: %s)", 
            errorResp.Message, errorResp.Code, errorResp.Type)
    }

    // Check if status code indicates success
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("unexpected status code from Botpress: %d", resp.StatusCode)
    }

    log.Printf("✅ Successfully sent message to Botpress")
    return nil
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

type PageInfo struct {
    Platform    string
    PageID      string
    AccessToken string
}

func getPageInfo(ctx context.Context, pageID string) (*PageInfo, error) {
    var info PageInfo
    info.PageID = pageID
    err := db.QueryRowContext(ctx,
        "SELECT platform, access_token FROM pages WHERE page_id = $1 AND status = 'active'",
        pageID,
    ).Scan(&info.Platform, &info.AccessToken)

    if err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("no active page found for ID %s", pageID)
        }
        return nil, fmt.Errorf("database error: %v", err)
    }

    return &info, nil
}

func sendPlatformResponse(ctx context.Context, pageInfo *PageInfo, senderID, message string) error {
    switch pageInfo.Platform {
    case "facebook":
        return sendFacebookMessage(ctx, pageInfo.PageID, pageInfo.AccessToken, senderID, message)
    case "instagram":
        return sendInstagramMessage(ctx, pageInfo.PageID, pageInfo.AccessToken, senderID, message)
    default:
        return fmt.Errorf("unsupported platform: %s", pageInfo.Platform)
    }
}

func isBotpressRequest(r *http.Request) bool {
    userAgent := r.Header.Get("User-Agent")
    return userAgent == "axios/1.6.8" || // Botpress uses axios
           strings.Contains(strings.ToLower(userAgent), "botpress")
}