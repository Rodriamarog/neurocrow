// dify_integration.go
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
	"strings"
	"time"
)

// getDifyApiKey retrieves the Dify API key for a specific page from database
// Each client/page has their own Dify app with unique API key (multi-tenant)
func getDifyApiKey(ctx context.Context, pageID string, platform string) (string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var difyAPIKey string
	err := db.QueryRowContext(queryCtx,
		"SELECT dify_api_key FROM social_pages WHERE page_id = $1 AND platform = $2 AND status = 'active'",
		pageID, platform,
	).Scan(&difyAPIKey)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("❌ No active Dify API key found for page %s", pageID)
			return "", fmt.Errorf("no active Dify API key found for page %s", pageID)
		}
		log.Printf("❌ Database error querying Dify API key: %v", err)
		return "", fmt.Errorf("database error: %v", err)
	}

	if difyAPIKey == "" {
		log.Printf("❌ Empty Dify API key for page %s", pageID)
		return "", fmt.Errorf("empty Dify API key for page %s", pageID)
	}

	log.Printf("✅ Found Dify API key for page %s (key: app-...%s)", pageID, difyAPIKey[len(difyAPIKey)-8:])
	return difyAPIKey, nil
}

// forwardToDify sends a message to Dify API (replaces forwardToBotpress)
func forwardToDify(ctx context.Context, pageID string, msg MessagingEntry, platform string) error {
	// Get existing conversation state to retrieve any existing Dify conversation ID
	conv, err := getOrCreateConversation(ctx, pageID, msg.Sender.ID, platform)
	if err != nil {
		return fmt.Errorf("error getting conversation state: %v", err)
	}

	// Create Dify request with existing conversation ID if available
	difyReq := DifyRequest{
		Inputs:         map[string]interface{}{}, // Empty for simple chat
		Query:          msg.Message.Text,
		ResponseMode:   "blocking",                                  // Get immediate response
		User:           fmt.Sprintf("%s-%s", pageID, msg.Sender.ID), // Unique user ID
		ConversationId: conv.DifyConversationID,                     // Use existing conversation ID or empty for new
		Files:          []interface{}{},                             // No files for now
	}

	// Log conversation continuation
	if conv.DifyConversationID != "" {
		log.Printf("🔄 Continuing existing Dify conversation: %s", conv.DifyConversationID)
	} else {
		log.Printf("🆕 Starting new Dify conversation for thread: %s", msg.Sender.ID)
	}

	// Get Dify API key
	apiKey, err := getDifyApiKey(ctx, pageID, platform)
	if err != nil {
		return fmt.Errorf("error getting Dify API key: %v", err)
	}

	// Send to Dify with retries
	response, err := sendToDifyWithRetry(ctx, apiKey, difyReq)
	if err != nil {
		return err
	}

	// Handle the response immediately (unlike Botpress webhooks)
	return handleDifyResponseDirect(ctx, pageID, msg.Sender.ID, platform, response)
}

// sendToDifyWithRetry sends request to Dify with retry logic (replaces sendToBotpressWithRetry)
func sendToDifyWithRetry(ctx context.Context, apiKey string, payload DifyRequest) (*DifyResponse, error) {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if response, err := sendToDify(ctx, apiKey, payload); err != nil {
			lastErr = err
			log.Printf("⚠️ Dify attempt %d failed: %v", attempt+1, err)
			time.Sleep(time.Second * time.Duration(attempt+1))
			continue
		} else {
			return response, nil
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %v", maxRetries, lastErr)
}

// sendToDify sends the actual request to Dify API (replaces sendToBotpress)
func sendToDify(ctx context.Context, apiKey string, payload DifyRequest) (*DifyResponse, error) {
	// Dify API endpoint
	apiURL := "https://api.dify.ai/v1/chat-messages"

	// Convert payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshaling Dify payload: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating Dify request: %v", err)
	}

	// Add required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Log payload details only in debug mode
	LogDebug("🤖 Dify request payload: %s", string(jsonData))

	// Send the request
	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request to Dify: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Dify response: %v", err)
	}

	// Log response timing and basic status
	responseTime := time.Since(start)
	LogDebug("📥 Dify response: %d in %dms", resp.StatusCode, responseTime.Milliseconds())
	
	// Log full response body only in debug mode
	LogDebug("Dify response body: %s", string(respBody))

	// Handle different response scenarios
	if resp.StatusCode != http.StatusOK {
		// Try to parse as error response
		var errorResp DifyErrorResponse
		if err := json.Unmarshal(respBody, &errorResp); err == nil {
			return nil, fmt.Errorf("dify error: %s (code: %s)", errorResp.Message, errorResp.Code)
		}
		return nil, fmt.Errorf("unexpected status code from Dify: %d - %s", resp.StatusCode, string(respBody))
	}

	// Parse successful response
	var difyResp DifyResponse
	if err := json.Unmarshal(respBody, &difyResp); err != nil {
		return nil, fmt.Errorf("error parsing Dify response: %v", err)
	}

	LogDebug("✅ Dify response parsed")
	return &difyResp, nil
}

// handleDifyResponseDirect processes Dify response immediately (replaces webhook-based handleBotpressResponse)
func handleDifyResponseDirect(ctx context.Context, pageID, senderID, platform string, response *DifyResponse) error {
	log.Printf("📥 Processing Dify response for conversation")

	// Validate response
	if response.Answer == "" {
		return fmt.Errorf("empty answer from Dify")
	}

	// Store/update the Dify conversation ID for future context
	if response.ConversationId != "" {
		if err := updateDifyConversationID(ctx, senderID, response.ConversationId); err != nil {
			log.Printf("⚠️ Could not store Dify conversation ID: %v", err)
		} else {
			log.Printf("💾 Stored Dify conversation ID: %s for thread: %s", response.ConversationId, senderID)
		}
	}

	// Get page info to determine platform details
	pageInfo, err := getPageInfo(ctx, pageID, platform)
	if err != nil {
		return fmt.Errorf("error getting page info: %v", err)
	}

	// Send response to the user via appropriate platform
	if err := sendPlatformResponse(ctx, pageInfo, senderID, response.Answer); err != nil {
		return fmt.Errorf("error sending platform response: %v", err)
	}

	log.Printf("✅ Platform response sent successfully - no storage needed")
	return nil
}

// isDifyRequest checks if an incoming request is from Dify (replaces isBotpressRequest)
// Note: This might not be needed since Dify responses are handled directly, not via webhook
func isDifyRequest(r *http.Request) bool {
	userAgent := r.Header.Get("User-Agent")
	// Dify doesn't send webhooks back, so this is mainly for future compatibility
	return strings.Contains(strings.ToLower(userAgent), "dify")
}

// updateDifyConversationID stores the Dify conversation ID for maintaining context
func updateDifyConversationID(ctx context.Context, threadID string, difyConversationID string) error {
	_, err := db.ExecContext(ctx, `
        UPDATE conversations 
        SET dify_conversation_id = $1,
            updated_at = NOW()
        WHERE thread_id = $2
    `, difyConversationID, threadID)

	if err != nil {
		return fmt.Errorf("error updating Dify conversation ID: %v", err)
	}

	return nil
}