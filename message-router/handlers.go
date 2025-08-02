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
	"strings"
	"time"
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

	// Unknown request type - no Dify responses needed since they're handled directly
	log.Printf("ℹ️ Unknown POST request to webhook endpoint")
	w.WriteHeader(http.StatusOK)
}

func handlePlatformMessage(w http.ResponseWriter, r *http.Request, body []byte) {
	// Generate request ID for log correlation
	requestID := generateRequestID()
	
	// Log webhook reception (optimized)
	LogDebug("[%s] 📥 Raw webhook payload: %d bytes", requestID, len(body))

	var event FacebookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		LogError("[%s] Error parsing webhook payload: %v", requestID, err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Count handover events across all entries
	totalHandoverEvents := 0
	totalMessages := 0
	for _, entry := range event.Entry {
		totalHandoverEvents += len(entry.MessagingHandovers)
		totalMessages += len(entry.Messaging)
	}
	
	// Single consolidated log for webhook details
	LogInfo("[%s] 📝 Webhook: %s, %d entries, %d messages, %d handovers", 
		requestID, event.Object, len(event.Entry), totalMessages, totalHandoverEvents)
	
	// Additional debug logging for entries
	for i, entry := range event.Entry {
		LogInfo("[%s] 📋 Entry %d: id=%s, messages=%d", requestID, i, entry.ID, len(entry.Messaging))
	}

	if !isValidFacebookObject(event.Object) {
		LogError("[%s] Unsupported webhook object: %s", requestID, event.Object)
		http.Error(w, "Unsupported webhook object", http.StatusBadRequest)
		return
	}

	// Send immediate 200 OK
	w.WriteHeader(http.StatusOK)

	// Process messages asynchronously with request ID for correlation
	ctx := context.Background()
	go processMessagesAsync(ctx, event, requestID)
}

func processMessagesAsync(ctx context.Context, event FacebookEvent, requestID string) {
	LogDebug("[%s] 🔄 Starting async message processing", requestID)
	
	// Check and reactivate eligible bots before processing new messages (12-hour rule)
	checkAndReactivateBots(ctx, requestID)
	
	for _, entry := range event.Entry {
		if len(entry.Messaging) == 0 {
			LogDebug("[%s] No messages in entry %s", requestID, entry.ID)
			continue
		}

		for msgIndex, msg := range entry.Messaging {
			// Skip non-message events (delivery receipts, etc.)
			if msg.Delivery != nil {
				LogDebug("[%s] Skipping delivery receipt", requestID)
				continue
			}

			// Validate message content
			if msg.Message == nil || msg.Message.Text == "" {
				LogDebug("[%s] Skipping empty message from %s", requestID, msg.Sender.ID)
				continue
			}

			// Log every message we receive for debugging
			LogInfo("[%s] 📨 Raw message %d: sender=%s, recipient=%s, echo=%v, app_id=%d, text=%q", 
				requestID, msgIndex, msg.Sender.ID, msg.Recipient.ID, msg.Message.IsEcho, msg.Message.AppId, msg.Message.Text)

			// Handle echo messages intelligently (FIXED LOGIC)
			if msg.Message.IsEcho {
				LogInfo("[%s] 🔍 Echo message detected - analyzing app_id and sender", requestID)
				
				// Normalize platform name
				platform := event.Object
				if platform == "page" {
					platform = "facebook"
				}
				
				// Instagram-specific logic using bot flag system
				if platform == "instagram" {
					LogInfo("[%s] 📱 Instagram echo message - checking bot flag", requestID)
					
					// Construct conversation ID (pageID-senderID, but for echo messages sender=page, recipient=user)
					conversationID := fmt.Sprintf("%s-%s", entry.ID, msg.Recipient.ID)
					
					// Check if this message has a bot flag
					if hasBotFlag(conversationID) {
						LogInfo("[%s] 🤖 Instagram bot message confirmed by flag - skipping", requestID)
						clearBotFlag(conversationID) // Clear the flag after use
						continue
					} else {
						LogInfo("[%s] 👤 Instagram human agent message detected (no bot flag) - disabling bot", requestID)
						
						// Auto-disable bot for human agent intervention
						err := updateConversationForHumanMessage(ctx, entry.ID, msg.Recipient.ID, platform)
						if err != nil {
							LogError("[%s] ❌ Failed to disable bot for human agent: %v", requestID, err)
						} else {
							LogInfo("[%s] ✅ Bot successfully disabled for human agent", requestID)
						}
						continue
					}
				}
				
				// Facebook logic (unchanged)
				if platform == "facebook" {
					// Check if this is a bot echo (our own bot responses)
					if msg.Message.AppId == 1195277397801905 {
						LogInfo("[%s] 🤖 Facebook bot echo message detected (app_id: %d) - skipping", requestID, msg.Message.AppId)
						continue
					}
					
					// Check if this is a human agent message (sender = page)
					if msg.Sender.ID == entry.ID {
						LogInfo("[%s] 👤 Facebook human agent message detected! sender=%s matches page=%s, app_id=%d", 
							requestID, msg.Sender.ID, entry.ID, msg.Message.AppId)
						
						LogInfo("[%s] 🔴 Auto-disabling bot due to human agent intervention", requestID)
						err := updateConversationForHumanMessage(ctx, entry.ID, msg.Recipient.ID, platform)
						if err != nil {
							LogError("[%s] ❌ Failed to disable bot for human agent: %v", requestID, err)
						} else {
							LogInfo("[%s] ✅ Bot successfully disabled for human agent", requestID)
						}
						continue
					}
					
					// Unknown Facebook echo message pattern
					LogWarn("[%s] ⚠️ Unknown Facebook echo pattern: sender=%s, page=%s, app_id=%d", 
						requestID, msg.Sender.ID, entry.ID, msg.Message.AppId)
					continue
				}
				
				// Unknown platform
				LogWarn("[%s] ⚠️ Unknown platform echo message: platform=%s", requestID, platform)
				continue
			}

			// Skip non-user senders (but allow regular user messages)
			if strings.HasPrefix(msg.Sender.ID, "page-") || strings.HasPrefix(msg.Sender.ID, "bot-") {
				LogDebug("[%s] Skipping non-user message from %s", requestID, msg.Sender.ID)
				continue
			}

			// Non-echo message - proceed with normal user processing
			LogInfo("[%s] 👤 User message detected - proceeding with bot processing", requestID)

			// Normalize platform name
			platform := event.Object
			if platform == "page" {
				platform = "facebook"
			}

			// Single consolidated log for message reception
			LogInfo("[%s] 📥 Message: %s -> %s (%s) %q", 
				requestID, msg.Sender.ID, entry.ID, platform, msg.Message.Text)

			// Get conversation state and page info (consolidated error handling)
			conv, err := getOrCreateConversation(ctx, entry.ID, msg.Sender.ID, platform)
			if err != nil {
				LogError("[%s] Failed to get conversation state for %s: %v", requestID, msg.Sender.ID, err)
				continue
			}

			pageInfo, err := getPageInfo(ctx, entry.ID, platform)
			if err != nil {
				LogError("[%s] Failed to get page info for %s: %v", requestID, entry.ID, err)
				continue
			}

			// Get user profile (non-critical, don't log failures unless debug)
			userName, err := getProfileInfo(ctx, msg.Sender.ID, pageInfo.AccessToken, platform)
			if err != nil {
				LogDebug("[%s] Could not get user name for %s: %v", requestID, msg.Sender.ID, err)
				userName = "user"
			} else {
				updateConversationUsername(ctx, msg.Sender.ID, userName) // Fire and forget
			}

			// Check thread control status
			shouldProcess, err := shouldBotProcessMessage(ctx, msg.Sender.ID)
			if err != nil {
				LogWarn("[%s] Thread control check failed, defaulting to bot: %v", requestID, err)
				shouldProcess = true // Graceful degradation
			}

			if shouldProcess {
				// Analyze sentiment
				start := time.Now()
				analysis, err := sentimentAnalyzer.Analyze(ctx, msg.Message.Text)
				if err != nil {
					LogError("[%s] Sentiment analysis failed for %s: %v", requestID, msg.Sender.ID, err)
					continue
				}
				
				// Single consolidated log for processing status
				processingTime := time.Since(start)
				LogInfo("[%s] 🤖 Processing: %s sentiment, %d tokens, %dms", 
					requestID, analysis.Status, analysis.TokensUsed, processingTime.Milliseconds())
				
				// Log cost details only in debug mode
				LogDebug("[%s] Estimated cost: $%.6f", requestID, float64(analysis.TokensUsed)*0.20/1_000_000)

				// Handle sentiment-based actions
				if analysis.Status == "need_human" {
					LogInfo("[%s] 👤 User requested human - disabling bot", requestID)
					
					// Send handoff message and disable bot
					handoffMsg := "Te conectaré con un agente humano para ayudarte mejor."
					if err := sendPlatformResponse(ctx, pageInfo, msg.Sender.ID, handoffMsg); err != nil {
						LogError("[%s] Handoff message failed: %v", requestID, err)
					}

					// Disable bot for this conversation
					if err := updateConversationState(ctx, conv, false, "User requested human assistance"); err != nil {
						LogError("[%s] Failed to disable bot: %v", requestID, err)
					}
					continue
				}

				// For frustrated users, escalate to human agents immediately
				if analysis.Status == "frustrated" {
					LogInfo("[%s] 😤 User frustrated - disabling bot", requestID)
					
					// Send empathy message before disabling
					empathyMsg := "Entiendo tu frustración. Te conectaré con un agente para ayudarte mejor."
					if err := sendPlatformResponse(ctx, pageInfo, msg.Sender.ID, empathyMsg); err != nil {
						LogError("[%s] Empathy message failed: %v", requestID, err)
					}

					// Disable bot for this conversation
					if err := updateConversationState(ctx, conv, false, "User appears frustrated"); err != nil {
						LogError("[%s] Failed to disable bot: %v", requestID, err)
					}
					continue
				}

				// If sentiment is "general" and bot has control, forward to Dify
				if analysis.Status == "general" {
					// Re-check thread control before forwarding to Dify (prevents race conditions)
					shouldProcessDify, err := shouldBotProcessMessage(ctx, msg.Sender.ID)
					if err != nil {
						LogWarn("[%s] Thread control recheck failed, using bot: %v", requestID, err)
						shouldProcessDify = true // Graceful degradation
					}
					
					if !shouldProcessDify {
						LogDebug("[%s] Thread control lost during processing", requestID)
						continue
					}
					
					LogDebug("[%s] 🤖 Forwarding to Dify...", requestID)
					if err := forwardToDify(ctx, entry.ID, msg, platform); err != nil {
						LogError("[%s] Dify forwarding failed: %v", requestID, err)
						updateConversationState(ctx, conv, false, "Error al procesar con Dify")
					} else {
						LogDebug("[%s] ✅ Dify response sent", requestID)
					}
				}
			} else {
				LogDebug("[%s] 🔒 Human has control, message logged", requestID)
			}
		}
	}
	LogDebug("[%s] ✅ Async processing complete", requestID)
}

// storeMessage function removed - no longer storing messages

func forwardToBotpress(ctx context.Context, pageID string, msg MessagingEntry, platform string) error {
	// Create Botpress request
	botpressReq := BotpressRequest{
		ID:             msg.Message.Mid,
		ConversationId: fmt.Sprintf("%s-%s", pageID, msg.Sender.ID),
		Channel:        platform,
		Type:           "text",
		Content:        msg.Message.Text,
		Payload: BotpressRequestPayload{
			Text:     msg.Message.Text,
			Type:     "text",
			PageId:   pageID,
			SenderId: msg.Sender.ID,
		},
		Direction: "incoming", // Add this line
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
		w.WriteHeader(http.StatusOK) // Still return 200 OK
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
		w.WriteHeader(http.StatusOK) // Still return 200 OK
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

		// Get page info to determine platform (legacy Botpress - try to get any active page)
		ctx := context.Background()
		pageInfo, err := getPageInfoLegacy(ctx, pageID)
		if err != nil {
			log.Printf("❌ Error getting page info: %v", err)
			w.WriteHeader(http.StatusOK)
			return
		}

		// Send response based on platform
		if err := sendPlatformResponse(ctx, pageInfo, senderID, response.Payload.Text); err != nil {
			log.Printf("❌ Error sending platform response: %v", err)
		} else {
			log.Printf("✅ Platform response sent successfully - no storage needed")
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
		"userId":         payload.Payload.SenderId, // Using sender ID as the user identifier
		"messageId":      payload.ID,               // Message ID for deduplication
		"conversationId": payload.ConversationId,   // Compound ID (pageId-senderId)
		"type":           "text",
		"text":           payload.Content, // The actual message content
		"payload": map[string]interface{}{ // Additional context and metadata
			"source":          payload.Channel, // "facebook" or "instagram"
			"pageId":          payload.Payload.PageId,
			"senderId":        payload.Payload.SenderId,
			"originalPayload": payload.Payload, // Keep original data for reference
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
		return fmt.Errorf("botpress error: %s (code: %d, type: %s)",
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
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var botpressURL string
	err := db.QueryRowContext(queryCtx,
		"SELECT botpress_url FROM social_pages WHERE page_id = $1 AND status = 'active'",
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

// =============================================================================
// DIFY API INTEGRATION - New functions replacing Botpress
// =============================================================================

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

// =============================================================================
// FACEBOOK HANDOVER PROTOCOL EVENT PROCESSING - For thread control management
// =============================================================================

// processHandoverEvents handles Facebook handover protocol events  
func processHandoverEvents(ctx context.Context, entry EntryData, requestID string) {
	if len(entry.MessagingHandovers) == 0 {
		return
	}

	LogDebug("[%s] 🔄 Processing %d handover events", requestID, len(entry.MessagingHandovers))

	for _, handover := range entry.MessagingHandovers {
		threadID := handover.Sender.ID

		// Handle PassThreadControl events
		if handover.PassThreadControl != nil {
			newOwnerAppID := handover.PassThreadControl.NewOwnerAppID
			prevOwnerAppID := handover.PassThreadControl.PreviousOwnerAppID
			metadata := handover.PassThreadControl.Metadata

			LogInfo("[%s] ⚡ Control: app_%d -> app_%d (%s)", requestID, prevOwnerAppID, newOwnerAppID, metadata)

			if newOwnerAppID == config.FacebookBotAppID {
				// Control passed back to our bot
				err := updateThreadControlStatus(ctx, threadID, "bot", "control_returned")
				if err != nil {
					LogError("[%s] Status update failed (bot): %v", requestID, err)
				} else {
					LogInfo("[%s] ✅ Bot control restored", requestID)
				}
			} else if newOwnerAppID == config.FacebookPageInboxAppID {
				// Control passed to Facebook Page Inbox
				err := updateThreadControlStatus(ctx, threadID, "human", "passed_to_inbox")
				if err != nil {
					LogError("[%s] Status update failed (human): %v", requestID, err)
				} else {
					LogInfo("[%s] ✅ Human control active", requestID)
				}
			} else {
				// Control passed to unknown app
				LogWarn("[%s] Unknown app control: %d", requestID, newOwnerAppID)
				updateThreadControlStatus(ctx, threadID, "system", "unknown_app_control")
			}
		}

		// Handle TakeThreadControl events
		if handover.TakeThreadControl != nil {
			prevOwnerAppID := handover.TakeThreadControl.PreviousOwnerAppID
			metadata := handover.TakeThreadControl.Metadata

			LogInfo("[%s] ⚡ Control taken from app_%d (%s)", requestID, prevOwnerAppID, metadata)

			// Another app took control from us or from Facebook inbox
			err := updateThreadControlStatus(ctx, threadID, "system", "control_taken")
			if err != nil {
				LogError("[%s] Status update failed (taken): %v", requestID, err)
			} else {
				LogInfo("[%s] ✅ Control taken logged", requestID)
			}
		}

		// Handle RequestThreadControl events
		if handover.RequestThreadControl != nil {
			requestedOwnerAppID := handover.RequestThreadControl.RequestedOwnerAppID
			metadata := handover.RequestThreadControl.Metadata

			LogDebug("[%s] Control requested by app_%d (%s)", requestID, requestedOwnerAppID, metadata)
		}
	}

	LogDebug("[%s] ✅ Handover events processed", requestID)
}

func getPageInfo(ctx context.Context, pageID string, platform string) (*PageInfo, error) {
	var info PageInfo
	info.PageID = pageID
	err := db.QueryRowContext(ctx,
		"SELECT platform, access_token FROM social_pages WHERE page_id = $1 AND platform = $2 AND status = 'active'",
		pageID, platform,
	).Scan(&info.Platform, &info.AccessToken)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active page found for ID %s with platform %s", pageID, platform)
		}
		return nil, fmt.Errorf("database error: %v", err)
	}

	return &info, nil
}

// getPageInfoLegacy is a fallback function for legacy Botpress code that doesn't have platform context
// It returns the first active page found for the given pageID (could be Facebook or Instagram)
func getPageInfoLegacy(ctx context.Context, pageID string) (*PageInfo, error) {
	var info PageInfo
	info.PageID = pageID
	err := db.QueryRowContext(ctx,
		"SELECT platform, access_token FROM social_pages WHERE page_id = $1 AND status = 'active' LIMIT 1",
		pageID,
	).Scan(&info.Platform, &info.AccessToken)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active page found for ID %s", pageID)
		}
		return nil, fmt.Errorf("database error: %v", err)
	}

	log.Printf("⚠️ Legacy getPageInfo used for pageID %s, returned platform: %s", pageID, info.Platform)
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

func getProfileInfo(ctx context.Context, userID string, pageToken string, platform string) (string, error) {
	log.Printf("🔍 Getting profile info for user %s (platform: %s)", userID, platform)

	// Check cache first
	if name, found := userCache.Get(userID); found {
		return name, nil
	}

	// Different endpoints and handling for Facebook and Instagram
	var userName string
	if platform == "facebook" {
		apiURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s?fields=name&access_token=%s", userID, pageToken)
		log.Printf("📡 Making Facebook API request for user %s", userID)

		var profile FacebookProfile
		if err := makeAPIRequest(ctx, apiURL, &profile); err != nil {
			return "user", err
		}
		userName = profile.Name
		log.Printf("👤 Using Facebook name: %s", userName)
	} else {
		apiURL := fmt.Sprintf("https://graph.facebook.com/v23.0/%s?fields=username&access_token=%s", userID, pageToken)
		log.Printf("📡 Making Instagram API request for user %s", userID)

		var profile InstagramProfile
		if err := makeAPIRequest(ctx, apiURL, &profile); err != nil {
			return "user", err
		}
		userName = profile.Username
		log.Printf("📸 Using Instagram username: %s", userName)
	}

	if userName == "" {
		log.Printf("⚠️ No name found in profile for user %s", userID)
		return "user", fmt.Errorf("no name found in profile")
	}

	// Cache the result
	userCache.Set(userID, userName)
	return userName, nil
}

// Helper function to make API requests
func makeAPIRequest(ctx context.Context, url string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("⏱️ API request completed in %v", time.Since(start))

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("❌ API error: Status %d, Body: %s",
			resp.StatusCode, string(respBody))
		return fmt.Errorf("error response from API: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("error decoding response: %v", err)
	}

	return nil
}

func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Error parsing send message request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get page info for access token
	pageInfo, err := getPageInfo(r.Context(), req.PageID, req.Platform)
	if err != nil {
		log.Printf("❌ Error getting page info: %v", err)
		http.Error(w, "Error getting page info", http.StatusInternalServerError)
		return
	}

	// Send message based on platform
	var sendErr error
	switch req.Platform {
	case "facebook":
		sendErr = sendFacebookMessage(r.Context(), req.PageID, pageInfo.AccessToken, req.RecipientID, req.Message)
	case "instagram":
		sendErr = sendInstagramMessage(r.Context(), req.PageID, pageInfo.AccessToken, req.RecipientID, req.Message)
	default:
		sendErr = fmt.Errorf("unsupported platform: %s", req.Platform)
	}

	if sendErr != nil {
		log.Printf("❌ Error sending message: %v", sendErr)
		http.Error(w, fmt.Sprintf("Error sending message: %v", sendErr), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func updateConversationUsername(ctx context.Context, threadID string, userName string) error {
	var pageUUID string
	err := db.QueryRowContext(ctx, `
        SELECT page_id 
        FROM conversations 
        WHERE thread_id = $1
    `, threadID).Scan(&pageUUID)
	if err != nil {
		return fmt.Errorf("error finding conversation: %v", err)
	}

	_, err = db.ExecContext(ctx, `
        UPDATE conversations 
        SET social_user_name = $1,
            updated_at = NOW()
        WHERE thread_id = $2 AND page_id = $3
    `, userName, threadID, pageUUID)

	if err != nil {
		return fmt.Errorf("error updating conversation user name: %v", err)
	}

	log.Printf("✅ Updated conversation user name to: %s", userName)
	return nil
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
