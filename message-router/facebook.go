// facebook.go
package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256" // Added missing import
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

	// Log payload details only in debug mode
	LogDebug("📤 Facebook payload: %s", string(jsonData))

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
		return fmt.Errorf("facebook error (status %d): %s", resp.StatusCode, string(fbResp))
	}

	LogDebug("✅ Facebook response (%d): %s", resp.StatusCode, string(fbResp))
	return nil
}

func sendInstagramMessage(ctx context.Context, pageID string, pageToken string, recipientID string, message string) error {
	// Instagram uses a different endpoint format
	igURL := fmt.Sprintf("https://graph.facebook.com/v19.0/me/messages?access_token=%s", pageToken)

	igPayload := map[string]interface{}{
		"recipient": map[string]string{
			"id": recipientID,
		},
		"message": map[string]string{
			"text": message,
		},
		"messaging_type": "RESPONSE",
	}

	jsonData, err := json.Marshal(igPayload)
	if err != nil {
		return fmt.Errorf("error creating Instagram payload: %v", err)
	}

	// Log payload details only in debug mode
	LogDebug("📤 Instagram payload: %s", string(jsonData))

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
		return fmt.Errorf("instagram error (status %d): %s", resp.StatusCode, string(igResp))
	}

	LogDebug("✅ Instagram response (%d): %s", resp.StatusCode, string(igResp))
	return nil
}

// isValidFacebookObject checks if the webhook object type is supported
func isValidFacebookObject(objectType string) bool {
	return objectType == "page" || objectType == "instagram"
}

// =============================================================================
// FACEBOOK HANDOVER PROTOCOL API FUNCTIONS - For thread control management
// =============================================================================

// passThreadControl passes thread control to another app (usually Facebook Page Inbox)
func passThreadControl(ctx context.Context, pageAccessToken, recipientID string, targetAppID int64, metadata string) error {
	fbURL := fmt.Sprintf("https://graph.facebook.com/v19.0/me/pass_thread_control?access_token=%s", pageAccessToken)

	payload := map[string]interface{}{
		"recipient": map[string]string{
			"id": recipientID,
		},
		"target_app_id": targetAppID,
		"metadata":      metadata,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error creating pass thread control payload: %v", err)
	}

	log.Printf("🔄 Passing thread control to app %d for user %s", targetAppID, recipientID)
	log.Printf("   Metadata: %s", metadata)
	log.Printf("   URL: %s", fbURL)
	log.Printf("   Payload: %s", string(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", fbURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating pass thread control request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending pass thread control: %v", err)
	}
	defer resp.Body.Close()

	fbResp, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("facebook pass thread control error (status %d): %s", resp.StatusCode, string(fbResp))
	}

	log.Printf("✅ Thread control passed successfully: %s", string(fbResp))
	return nil
}

// takeThreadControl takes thread control back from another app
func takeThreadControl(ctx context.Context, pageAccessToken, recipientID string, metadata string) error {
	fbURL := fmt.Sprintf("https://graph.facebook.com/v19.0/me/take_thread_control?access_token=%s", pageAccessToken)

	payload := map[string]interface{}{
		"recipient": map[string]string{
			"id": recipientID,
		},
		"metadata": metadata,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error creating take thread control payload: %v", err)
	}

	log.Printf("🔄 Taking thread control back for user %s", recipientID)
	log.Printf("   Metadata: %s", metadata)
	log.Printf("   URL: %s", fbURL)
	log.Printf("   Payload: %s", string(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", fbURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating take thread control request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending take thread control: %v", err)
	}
	defer resp.Body.Close()

	fbResp, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("facebook take thread control error (status %d): %s", resp.StatusCode, string(fbResp))
	}

	log.Printf("✅ Thread control taken successfully: %s", string(fbResp))
	return nil
}

// getThreadOwner queries who currently owns thread control
func getThreadOwner(ctx context.Context, pageAccessToken, recipientID string) (int64, error) {
	fbURL := fmt.Sprintf("https://graph.facebook.com/v19.0/me/thread_owner?recipient=%s&access_token=%s",
		recipientID, pageAccessToken)

	log.Printf("🔍 Querying thread owner for user %s", recipientID)
	log.Printf("   URL: %s", fbURL)

	req, err := http.NewRequestWithContext(ctx, "GET", fbURL, nil)
	if err != nil {
		return 0, fmt.Errorf("error creating thread owner request: %v", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error querying thread owner: %v", err)
	}
	defer resp.Body.Close()

	fbResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading thread owner response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("facebook thread owner error (status %d): %s", resp.StatusCode, string(fbResp))
	}

	log.Printf("📄 Thread owner response: %s", string(fbResp))

	var ownerResp ThreadOwnerResponse
	if err := json.Unmarshal(fbResp, &ownerResp); err != nil {
		return 0, fmt.Errorf("error parsing thread owner response: %v", err)
	}

	if len(ownerResp.Data) == 0 {
		return 0, fmt.Errorf("no thread owner data returned")
	}

	ownerAppID := ownerResp.Data[0].ThreadOwner.AppID
	log.Printf("✅ Thread owner: App ID %d", ownerAppID)
	return ownerAppID, nil
}
