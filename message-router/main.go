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
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/joho/godotenv"
    _ "github.com/lib/pq"
)

var (
    db *sql.DB
    httpClient = &http.Client{
        Timeout: 10 * time.Second,
    }
)

func init() {
    log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
    log.Printf("🚀 Starting Neurocrow Message Router...")

    if err := godotenv.Load(); err != nil {
        log.Printf("💡 Using platform environment variables (no .env file)")
    }

    // Connect to database with retry logic
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        log.Fatal("❌ DATABASE_URL environment variable is not set")
    }
    
    log.Printf("📊 Database URL configured (length: %d chars)", len(dbURL))
    
    var err error
    for i := 0; i < 3; i++ {
        log.Printf("🔄 Database connection attempt %d/3...", i+1)
        db, err = sql.Open("postgres", dbURL)
        if err != nil {
            log.Printf("❌ Connection attempt %d failed: %v", i+1, err)
            time.Sleep(time.Second * 2)
            continue
        }
        
        if err = db.Ping(); err != nil {
            log.Printf("❌ Database ping failed: %v", err)
            time.Sleep(time.Second * 2)
            continue
        }
        
        log.Printf("✅ Successfully connected to database!")
        break
    }
    
    if err != nil {
        log.Fatal("❌ Failed to connect to database after 3 attempts: ", err)
    }

    // Set connection pool settings
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(25)
    db.SetConnMaxLifetime(5 * time.Minute)
    
    log.Printf("⚙️ Database connection pool configured (max: 25 connections)")

    // Verify Facebook app secret is set
    if os.Getenv("FACEBOOK_APP_SECRET") == "" {
        log.Fatal("❌ FACEBOOK_APP_SECRET environment variable is not set")
    }

    // Verify webhook token is set
    if os.Getenv("VERIFY_TOKEN") == "" {
        log.Fatal("❌ VERIFY_TOKEN environment variable is not set")
    }

    log.Printf("✅ All required environment variables are set")
}

func main() {
    router := http.NewServeMux()
    router.HandleFunc("/webhook", recoverMiddleware(validateFacebookRequest(handleWebhook)))

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
        log.Printf("💡 No PORT specified, using default: %s", port)
    }

    log.Printf("🌐 Server starting on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, router))
}

func recoverMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("❌ PANIC RECOVERED: %v", err)
                http.Error(w, "Internal server error", http.StatusInternalServerError)
            }
        }()
        next(w, r)
    }
}

func validateFacebookRequest(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        log.Printf("📥 Incoming %s request from %s", r.Method, r.RemoteAddr)

        if r.Method == "POST" {
            signature := r.Header.Get("X-Hub-Signature-256")
            if signature == "" {
                log.Printf("❌ Missing Facebook signature header")
                http.Error(w, "Missing signature", http.StatusUnauthorized)
                return
            }
            log.Printf("✅ Facebook signature header present: %s", signature)

            body, err := io.ReadAll(r.Body)
            if err != nil {
                log.Printf("❌ Error reading request body: %v", err)
                http.Error(w, "Error reading body", http.StatusBadRequest)
                return
            }
            r.Body = io.NopCloser(bytes.NewBuffer(body))

            appSecret := os.Getenv("FACEBOOK_APP_SECRET")
            expectedSig := generateFacebookSignature(body, []byte(appSecret))
            
            if !hmac.Equal([]byte(signature[7:]), []byte(expectedSig)) {
                log.Printf("❌ Invalid Facebook signature")
                http.Error(w, "Invalid signature", http.StatusUnauthorized)
                return
            }
            log.Printf("✅ Facebook signature verified successfully")
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

        log.Printf("📝 Webhook verification request received:")
        log.Printf("   Mode: %s", mode)
        log.Printf("   Token: %s", token)
        log.Printf("   Challenge: %s", challenge)

        if mode == "subscribe" && token == verifyToken {
            log.Printf("✅ Webhook verification successful!")
            w.Write([]byte(challenge))
            return
        }
        log.Printf("❌ Webhook verification failed")
        http.Error(w, "Invalid verification token", http.StatusForbidden)
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
        
        // Parse webhook event with updated structure
        var event struct {
            Object string `json:"object"`
            Entry  []struct {
                ID        string `json:"id"`
                Time      int64  `json:"time"`
                Messaging []struct {
                    Sender    struct {
                        ID string `json:"id"`
                    } `json:"sender"`
                    Recipient struct {
                        ID string `json:"id"`
                    } `json:"recipient"`
                    Message   *struct {
                        Mid     string `json:"mid"`
                        Text    string `json:"text"`
                        IsEcho  bool   `json:"is_echo"`
                    } `json:"message"`
                    Delivery *struct {
                        Mids       []string `json:"mids"`
                        Watermark  int64    `json:"watermark"`
                    } `json:"delivery"`
                } `json:"messaging"`
            } `json:"entry"`
        }

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
        if event.Object != "page" && event.Object != "instagram" {
            log.Printf("❌ Unsupported webhook object: %s", event.Object)
            http.Error(w, "Unsupported webhook object", http.StatusBadRequest)
            return
        }

        log.Printf("✅ Webhook data validated successfully")

        // Facebook expects a quick 200 OK
        w.WriteHeader(http.StatusOK)
        log.Printf("✅ Sent 200 OK response to Facebook")

        // Process messages asynchronously
        go func() {
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()

            for _, entry := range event.Entry {
                for _, msg := range entry.Messaging {
                    // Skip if this is a delivery receipt
                    if msg.Delivery != nil {
                        log.Printf("📝 Skipping delivery receipt")
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

                    pageID := msg.Recipient.ID
                    log.Printf("🔄 Processing message:")
                    log.Printf("   Page ID: %s", pageID)
                    log.Printf("   Sender ID: %s", msg.Sender.ID)
                    log.Printf("   Message ID: %s", msg.Message.Mid)
                    log.Printf("   Content: %s", msg.Message.Text)

                    // Look up Botpress webhook URL
                    var botpressURL string
                    err = db.QueryRowContext(ctx,
                        "SELECT botpress_url FROM pages WHERE page_id = $1 AND status = 'active'",
                        pageID,
                    ).Scan(&botpressURL)

                    if err != nil {
                        if err == sql.ErrNoRows {
                            log.Printf("❌ No active Botpress URL found for page %s", pageID)
                            continue
                        }
                        log.Printf("❌ Database error looking up page: %v", err)
                        continue
                    }

                    log.Printf("✅ Found Botpress URL: %s", botpressURL)

                    // Create request to Botpress with updated format
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
                        log.Printf("❌ Error creating Botpress payload: %v", err)
                        continue
                    }

                    req, err := http.NewRequestWithContext(ctx, "POST", botpressURL, bytes.NewBuffer(jsonData))
                    if err != nil {
                        log.Printf("❌ Error creating Botpress request: %v", err)
                        continue
                    }

                    req.Header.Set("Content-Type", "application/json")
                    
                    log.Printf("📤 Sending to Botpress:")
                    log.Printf("   URL: %s", botpressURL)
                    log.Printf("   Payload: %s", string(jsonData))

                    // Send to Botpress with additional logging
                    log.Printf("🔍 DEBUG: Sending request to Botpress")
                    resp, err := httpClient.Do(req)
                    if err != nil {
                        log.Printf("❌ Error sending to Botpress: %v", err)
                        continue
                    }

                    // Enhanced response logging
                    log.Printf("🔍 DEBUG: Received response from Botpress with status: %d", resp.StatusCode)
                    log.Printf("🔍 DEBUG: Response Headers: %+v", resp.Header)

                    // Read and log Botpress response
                    body, err = io.ReadAll(resp.Body)
                    resp.Body.Close()
                    if err != nil {
                        log.Printf("❌ Error reading Botpress response: %v", err)
                        continue
                    }

                    log.Printf("📩 Raw Botpress response (status %d):", resp.StatusCode)
                    log.Printf("Body length: %d bytes", len(body))
                    log.Printf("Body content: %s", string(body))

                    // Only try to parse if we have a non-empty response
                    if len(body) == 0 {
                        log.Printf("⚠️ Empty response from Botpress")
                        continue
                    }

                    // Try to parse response as JSON even if empty to see structure
                    var rawResponse interface{}
                    if err := json.Unmarshal(body, &rawResponse); err != nil {
                        log.Printf("❌ Error parsing Botpress response as JSON: %v", err)
                        // Print the raw response for debugging
                        log.Printf("🔍 DEBUG: Raw response content: %s", string(body))
                        continue
                    }

                    // Log the parsed response structure
                    prettyJSON, _ := json.MarshalIndent(rawResponse, "", "  ")
                    log.Printf("🔍 DEBUG: Parsed Botpress response structure:\n%s", string(prettyJSON))

                    // Get page token for sending response
                    var pageToken string
                    err = db.QueryRowContext(ctx,
                        "SELECT access_token FROM pages WHERE page_id = $1 AND status = 'active'",
                        pageID,
                    ).Scan(&pageToken)

                    if err != nil {
                        log.Printf("❌ Error getting page token: %v", err)
                        continue
                    }

                    // Create response for Facebook
                    fbPayload := map[string]interface{}{
                        "recipient": map[string]string{
                            "id": msg.Sender.ID,
                        },
                    }

                    // If we have a parsed response from Botpress, try to use it
                    if rawResponse != nil {
                        if response, ok := rawResponse.(map[string]interface{}); ok {
                            if text, ok := response["text"].(string); ok {
                                fbPayload["message"] = map[string]string{"text": text}
                            } else {
                                // Fallback message if we can't parse Botpress response
                                fbPayload["message"] = map[string]string{"text": "Sorry, I couldn't process that properly."}
                            }
                        }
                    } else {
                        // Default fallback message
                        fbPayload["message"] = map[string]string{"text": "Sorry, I'm having trouble understanding."}
                    }

                    jsonData, err = json.Marshal(fbPayload)
                    if err != nil {
                        log.Printf("❌ Error creating Facebook payload: %v", err)
                        continue
                    }

                    // Send to Facebook
                    fbURL := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages?access_token=%s",
                        pageID, pageToken)

                    log.Printf("📤 Sending response to Facebook:")
                    log.Printf("   URL: %s", fbURL)
                    log.Printf("   Payload: %s", string(jsonData))

                    req, err = http.NewRequestWithContext(ctx, "POST", fbURL, bytes.NewBuffer(jsonData))
                    if err != nil {
                        log.Printf("❌ Error creating Facebook request: %v", err)
                        continue
                    }

                    req.Header.Set("Content-Type", "application/json")

                    resp, err = httpClient.Do(req)
                    if err != nil {
                        log.Printf("❌ Error sending to Facebook: %v", err)
                        continue
                    }

                    fbResp, _ := io.ReadAll(resp.Body)
                    if resp.StatusCode != http.StatusOK {
                        log.Printf("❌ Facebook error (status %d): %s", resp.StatusCode, string(fbResp))
                    } else {
                        log.Printf("✅ Facebook response (status %d): %s", resp.StatusCode, string(fbResp))
                        log.Printf("✅ Message successfully sent to user")
                    }
                    resp.Body.Close()
                }
            }
        }()
    }
}