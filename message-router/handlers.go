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
    "os"      // Add this import
)

func handleWebhook(w http.ResponseWriter, r *http.Request) {
    if r.Method == "GET" {
        // Check if this is a Facebook webhook verification
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

        // If no Facebook params, assume it's Botpress checking the endpoint
        log.Printf("✅ Endpoint check - returning 200 OK")
        w.WriteHeader(http.StatusOK)
        return
    }

    if r.Method == "POST" {
        log.Printf("📨 Incoming webhook from %s", r.RemoteAddr)
        
        // Read and log raw webhook data
        body, err := io.ReadAll(r.Body)
        if err != nil {
            log.Printf("❌ Error reading webhook body: %v", err)
            http.Error(w, "Error reading body", http.StatusBadRequest)
            return
        }
        r.Body = io.NopCloser(bytes.NewBuffer(body))
        
        log.Printf("📄 Raw webhook data: %s", string(body))
        
        // Parse webhook event
        var event FacebookEvent
        if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
            log.Printf("❌ Error parsing webhook JSON: %v", err)
            http.Error(w, "Invalid request body", http.StatusBadRequest)
            return
        }

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

        // Facebook expects a quick 200 OK
        w.WriteHeader(http.StatusOK)
        log.Printf("✅ Sent 200 OK response to Facebook")

        // Process messages asynchronously
        go processWebhook(r.Context(), event)
    } else {
        // Handle unsupported methods
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
}

func processWebhook(ctx context.Context, event FacebookEvent) {
    for _, entry := range event.Entry {
        for _, msg := range entry.Messaging {
            // Skip if this is a delivery receipt
            if msg.Delivery != nil {
                processDeliveryReceipt(msg.Delivery)
                continue
            }

            // Skip if there's no message
            if msg.Message == nil {
                log.Printf("📝 Skipping non-message event")
                continue
            }

            // Skip echo messages
            if msg.Message.IsEcho {
                log.Printf("📝 Skipping echo message with ID: %s", msg.Message.Mid)
                continue
            }

            // Only process messages that have content
            if msg.Message.Text == "" {
                log.Printf("📝 Skipping empty message")
                continue
            }

            if err := handleMessage(ctx, entry.ID, msg); err != nil {
                log.Printf("❌ Error handling message: %v", err)
            }
        }
    }
}

func handleMessage(ctx context.Context, pageID string, msg MessagingEntry) error {
    log.Printf("🔄 Processing message:")
    log.Printf("   Page ID: %s", pageID)
    log.Printf("   Sender ID: %s", msg.Sender.ID)
    log.Printf("   Message ID: %s", msg.Message.Mid)
    log.Printf("   Content: %s", msg.Message.Text)

    // Get Botpress URL
    botpressURL, err := getBotpressURL(ctx, pageID)
    if err != nil {
        return fmt.Errorf("error getting Botpress URL: %v", err)
    }

    // Send to Botpress
    botpressResp, err := sendToBotpress(ctx, botpressURL, pageID, msg)
    if err != nil {
        return fmt.Errorf("error sending to Botpress: %v", err)
    }

    // Get page token for response
    pageToken, err := getPageToken(ctx, pageID)
    if err != nil {
        return fmt.Errorf("error getting page token: %v", err)
    }

    // Send Botpress response back to Facebook
    return sendFacebookMessage(ctx, pageID, pageToken, msg.Sender.ID, botpressResp.Payload.Text)
}

func getBotpressURL(ctx context.Context, pageID string) (string, error) {
    var botpressURL string
    err := db.QueryRowContext(ctx,
        "SELECT botpress_url FROM pages WHERE page_id = $1 AND status = 'active'",
        pageID,
    ).Scan(&botpressURL)

    if err != nil {
        if err == sql.ErrNoRows {
            return "", fmt.Errorf("no active Botpress URL found for page %s", pageID)
        }
        return "", fmt.Errorf("database error: %v", err)
    }

    return botpressURL, nil
}

func sendToBotpress(ctx context.Context, botpressURL string, pageID string, msg MessagingEntry) (*BotpressResponse, error) {
    // Create request to Botpress
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

    req, err := http.NewRequestWithContext(ctx, "POST", botpressURL, bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("error creating Botpress request: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")
    
    log.Printf("📤 Sending to Botpress:")
    log.Printf("   URL: %s", botpressURL)
    log.Printf("   Payload: %s", string(jsonData))

    // Send to Botpress
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