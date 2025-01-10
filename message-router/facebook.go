// facebook.go
package main

import (
    "bytes"
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
)

// validateFacebookRequest is middleware to validate webhook requests
func validateFacebookRequest(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        log.Printf("📥 Incoming %s request from %s", r.Method, r.RemoteAddr)

        if r.Method == "POST" {
            signature := r.Header.Get("X-Hub-Signature-256")
            if signature == "" {
                log.Printf("❌ Missing signature header")
                http.Error(w, "Missing signature", http.StatusUnauthorized)
                return
            }
            log.Printf("✅ Signature header present: %s", signature)

            body, err := io.ReadAll(r.Body)
            if err != nil {
                log.Printf("❌ Error reading request body: %v", err)
                http.Error(w, "Error reading body", http.StatusBadRequest)
                return
            }
            r.Body = io.NopCloser(bytes.NewBuffer(body))

            appSecret := []byte(config.FacebookAppSecret)
            expectedSig := generateFacebookSignature(body, appSecret)
            
            if !hmac.Equal([]byte(signature[7:]), []byte(expectedSig)) {
                log.Printf("❌ Invalid signature")
                http.Error(w, "Invalid signature", http.StatusUnauthorized)
                return
            }
            log.Printf("✅ Signature verified successfully")
        }
        next(w, r)
    }
}

// generateFacebookSignature creates HMAC SHA256 signature for request verification
func generateFacebookSignature(body []byte, secret []byte) string {
    mac := hmac.New(sha256.New, secret)
    mac.Write(body)
    return hex.EncodeToString(mac.Sum(nil))
}

// verifyFacebookWebhook handles the initial webhook verification
func verifyFacebookWebhook(w http.ResponseWriter, r *http.Request) bool {
    verifyToken := config.VerifyToken
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
        return true
    }
    log.Printf("❌ Webhook verification failed")
    http.Error(w, "Invalid verification token", http.StatusForbidden)
    return false
}

// sendFacebookMessage sends a message to a user through Facebook Messenger
func sendFacebookMessage(ctx context.Context, pageID string, pageToken string, recipientID string, message string) error {
    fbPayload := map[string]interface{}{
        "recipient": map[string]string{
            "id": recipientID,
        },
        "message": map[string]string{
            "text": message,
        },
    }

    jsonData, err := json.Marshal(fbPayload)
    if err != nil {
        return fmt.Errorf("error creating Facebook payload: %v", err)
    }

    fbURL := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages?access_token=%s",
        pageID, pageToken)

    log.Printf("📤 Sending response to Facebook:")
    log.Printf("   URL: %s", fbURL)
    log.Printf("   Payload: %s", string(jsonData))

    req, err := http.NewRequestWithContext(ctx, "POST", fbURL, bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("error creating Facebook request: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")

    resp, err := httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("error sending to Facebook: %v", err)
    }
    defer resp.Body.Close()

    fbResp, _ := io.ReadAll(resp.Body)
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("Facebook error (status %d): %s", resp.StatusCode, string(fbResp))
    }

    log.Printf("✅ Facebook response (status %d): %s", resp.StatusCode, string(fbResp))
    return nil
}

// sendInstagramMessage sends a message to a user through Instagram
func sendInstagramMessage(ctx context.Context, pageID string, pageToken string, recipientID string, message string) error {
    igPayload := map[string]interface{}{
        "recipient_id": recipientID,
        "message": map[string]string{
            "text": message,
        },
    }

    jsonData, err := json.Marshal(igPayload)
    if err != nil {
        return fmt.Errorf("error creating Instagram payload: %v", err)
    }

    // Instagram uses a different API endpoint
    igURL := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages?access_token=%s",
        pageID, pageToken)

    log.Printf("📤 Sending response to Instagram:")
    log.Printf("   URL: %s", igURL)
    log.Printf("   Payload: %s", string(jsonData))

    req, err := http.NewRequestWithContext(ctx, "POST", igURL, bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("error creating Instagram request: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")

    resp, err := httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("error sending to Instagram: %v", err)
    }
    defer resp.Body.Close()

    igResp, _ := io.ReadAll(resp.Body)
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("Instagram error (status %d): %s", resp.StatusCode, string(igResp))
    }

    log.Printf("✅ Instagram response (status %d): %s", resp.StatusCode, string(igResp))
    return nil
}

// getPageToken retrieves the page access token from the database
func getPageToken(ctx context.Context, pageID string) (string, error) {
    var pageToken string
    err := db.QueryRowContext(ctx,
        "SELECT access_token FROM pages WHERE page_id = $1 AND status = 'active'",
        pageID,
    ).Scan(&pageToken)

    if err != nil {
        if err == sql.ErrNoRows {
            return "", fmt.Errorf("no active page found for ID %s", pageID)
        }
        return "", fmt.Errorf("database error: %v", err)
    }

    return pageToken, nil
}

// isValidFacebookObject checks if the webhook object type is supported
func isValidFacebookObject(objectType string) bool {
    return objectType == "page" || objectType == "instagram"
}

// processDeliveryReceipt handles message delivery receipts
func processDeliveryReceipt(delivery *DeliveryData) {
    if delivery == nil {
        return
    }
    
    log.Printf("📝 Processing delivery receipt:")
    log.Printf("   Watermark: %d", delivery.Watermark)
    log.Printf("   Message IDs: %v", delivery.Mids)
}