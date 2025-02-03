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
	log.Printf("🔄 Starting async message processing")
	for _, entry := range event.Entry {
		log.Printf("📝 Processing entry ID: %s", entry.ID)

		if len(entry.Messaging) == 0 {
			log.Printf("ℹ️ No messages in entry")
			continue
		}

		for _, msg := range entry.Messaging {
			// ⚠️ Delivery receipt handling
			if msg.Delivery != nil {
				log.Printf("      ⚠️ Delivery receipt - skipping")
				continue
			}

			// Message validation
			if msg.Message == nil {
				log.Printf("      ⚠️ No message content")
				continue
			}
			if msg.Message.Text == "" {
				log.Printf("      ⚠️ Empty message text")
				continue
			}
			if strings.HasPrefix(msg.Sender.ID, "page-") || strings.HasPrefix(msg.Sender.ID, "bot-") {
				log.Printf("      ⚠️ Message from non-user sender - skipping")
				continue
			}

			// Echo message handling
			if msg.Message.IsEcho {
				log.Printf("      📝 Processing echo message")
				platform := event.Object
				if platform == "page" {
					platform = "facebook"
				}

				// For Facebook, we can check the app_id
				if platform == "facebook" && msg.Message.AppId > 0 {
					log.Printf("      📝 Skipping storage of Facebook bot echo message (app_id: %d)", msg.Message.AppId)
					continue
				}

				// For Instagram, check if sender matches page and it's not a human message
				if platform == "instagram" && msg.Sender.ID == entry.ID {
					// Additional check to ensure it's really a bot message
					if _, err := getPageInfo(ctx, entry.ID); err == nil {
						log.Printf("      📝 Skipping storage of Instagram bot echo message")
						continue
					}
				}

				// If we get here, it's a human message
				log.Printf("      🔍 Detected human agent message (sender ID: %s)", msg.Sender.ID)
				if err := storeMessage(ctx, entry.ID, msg.Recipient.ID, platform, msg.Message.Text, "admin", "human", false); err != nil {
					log.Printf("❌ Error storing echo message: %v", err)
				} else {
					log.Printf("      ✅ Echo message stored successfully")
				}
				continue
			}

			// At this point, we have a valid user message
			log.Printf("      ✨ Valid message received from sender %s", msg.Sender.ID)
			log.Printf("      📨 Message content: %q", msg.Message.Text)

			// Normalize platform name
			platform := event.Object
			if platform == "page" {
				platform = "facebook"
				log.Printf("      🔄 Normalized platform from 'page' to 'facebook'")
			}

			log.Printf("      🌐 Processing message for platform: %s", platform)

			// Get or create conversation state
			log.Printf("      🔍 Getting conversation state for thread %s", msg.Sender.ID)
			conv, err := getOrCreateConversation(ctx, entry.ID, msg.Sender.ID, platform)
			if err != nil {
				log.Printf("❌ Error managing conversation state: %v", err)
				continue
			}
			log.Printf("      ✅ Conversation state retrieved, bot enabled: %v", conv.BotEnabled)

			// Get page info for access token
			log.Printf("      🔑 Fetching page info for ID: %s", entry.ID)
			pageInfo, err := getPageInfo(ctx, entry.ID)
			if err != nil {
				log.Printf("❌ Error getting page info: %v", err)
				continue
			}
			log.Printf("      ✅ Page info retrieved successfully")

			// Get user's profile info
			log.Printf("      👤 Fetching user profile info")
			userName, err := getProfileInfo(ctx, msg.Sender.ID, pageInfo.AccessToken, platform)
			if err != nil {
				log.Printf("⚠️ Could not get user name, using 'user': %v", err)
				userName = "user"
			}
			log.Printf("      📝 Using name '%s' for message storage", userName)

			// Always store the incoming message first
			log.Printf("      💾 Storing message in database")
			if err := storeMessage(ctx, entry.ID, msg.Sender.ID, platform, msg.Message.Text, userName, "user", true); err != nil {
				log.Printf("❌ Error storing message: %v", err)
			} else {
				log.Printf("      ✅ Message stored successfully")
			}

			// Only proceed with bot processing if enabled
			if conv.BotEnabled {
				log.Printf("      🤖 Bot is enabled, proceeding with message analysis")

				analysis, err := sentimentAnalyzer.Analyze(ctx, msg.Message.Text)
				if err != nil {
					log.Printf("❌ Error analyzing sentiment: %v", err)
					continue
				}

				log.Printf("      📊 Sentiment analysis complete:")
				log.Printf("         Status: %s", analysis.Status)
				log.Printf("         Tokens used: %d", analysis.TokensUsed)
				log.Printf("         Estimated cost: $%.5f", float64(analysis.TokensUsed)*0.20/1_000_000)

				// Update conversation state based on analysis
				if analysis.Status != "general" {
					log.Printf("      ⚡ Non-general status detected: %s", analysis.Status)

					// Prepare handoff message based on analysis
					handoffMsg := ""
					reason := ""

					switch analysis.Status {
					case "need_human":
						reason = "Usuario solicitó asistencia humana"
						handoffMsg = "Claro, te conectaré con un agente humano para ayudarte mejor."
						log.Printf("      👋 Human assistance requested")
					case "frustrated":
						reason = "Usuario muestra señales de frustración"
						handoffMsg = "Lamento la confusión. Te conectaré con un agente especializado inmediatamente."
						log.Printf("      😤 User frustration detected")
					}

					// Update conversation state to disable bot
					log.Printf("      🔄 Updating conversation state to disable bot")
					if err := updateConversationState(ctx, conv, false, reason); err != nil {
						log.Printf("❌ Error updating conversation state: %v", err)
					} else {
						log.Printf("      ✅ Conversation state updated successfully")
					}

					// Send handoff message to user
					log.Printf("      📤 Sending handoff message to user")
					if err := sendPlatformResponse(ctx, pageInfo, msg.Sender.ID, handoffMsg); err != nil {
						log.Printf("❌ Error sending handoff message: %v", err)
					} else {
						log.Printf("      ✅ Handoff message sent successfully")
					}

					// Store the handoff message
					log.Printf("      💾 Storing handoff message")
					if err := storeMessage(ctx, entry.ID, msg.Sender.ID, platform, handoffMsg, "system", "system", false); err != nil {
						log.Printf("❌ Error storing handoff message: %v", err)
					} else {
						log.Printf("      ✅ Handoff message stored successfully")
					}

					continue
				}

				// If sentiment is "general" and bot is enabled, forward to Botpress
				log.Printf("      🤖 Forwarding message to Botpress")
				if err := forwardToBotpress(ctx, entry.ID, msg, platform); err != nil {
					log.Printf("❌ Error forwarding to Botpress: %v", err)

					// If Botpress fails, mark for human attention
					log.Printf("      ⚠️ Botpress error, marking for human attention")
					if err := updateConversationState(ctx, conv, false, "Error al procesar con Botpress"); err != nil {
						log.Printf("❌ Error updating conversation state: %v", err)
					} else {
						log.Printf("      ✅ Conversation marked for human attention")
					}
				} else {
					log.Printf("      ✅ Message successfully forwarded to Botpress")
				}
			} else {
				log.Printf("      ℹ️ Bot is disabled, message stored for human review")
			}
		}
	}
	log.Printf("✅ Async message processing complete")
}

func storeMessage(ctx context.Context, pageID, senderID, platform, content, fromUser, source string, requiresAttention bool) error {
	// Get the UUID from social_pages instead of pages
	var pageUUID string
	err := db.QueryRowContext(ctx, `
        SELECT id 
        FROM social_pages 
        WHERE page_id = $1
    `, pageID).Scan(&pageUUID)

	if err != nil {
		return fmt.Errorf("error finding page: %v", err)
	}

	// Now use this UUID to store the message
	_, err = db.ExecContext(ctx, `
        INSERT INTO messages (
            id,           
            client_id,    
            page_id,      
            platform,     
            thread_id,    
            from_user,    
            content,      
            timestamp,    
            read,         
            source,       
            requires_attention  
        ) VALUES (
            gen_random_uuid(),  
            (SELECT client_id FROM social_pages WHERE id = $1),
            $1,                 
            $2,                 
            $3,                 
            $4,                 
            $5,                 
            NOW(),             
            false,             
            $6,                
            $7                 
        )
    `, pageUUID, platform, senderID, fromUser, content, source, requiresAttention)

	if err != nil {
		return fmt.Errorf("error storing message: %v", err)
	}

	log.Printf("✅ Message stored successfully (from: %s, source: %s)", fromUser, source)
	return nil
}

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
		} else {
			log.Printf("✅ Platform response sent successfully, storing bot response")
			if err := storeMessage(ctx, pageID, senderID, pageInfo.Platform, response.Payload.Text, "bot", "bot", false); err != nil {
				log.Printf("❌ Error storing bot response: %v", err)
			} else {
				log.Printf("✅ Stored bot response in database")
			}
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

func getPageInfo(ctx context.Context, pageID string) (*PageInfo, error) {
	var info PageInfo
	info.PageID = pageID
	err := db.QueryRowContext(ctx,
		"SELECT platform, access_token FROM social_pages WHERE page_id = $1 AND status = 'active'",
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

func getProfileInfo(ctx context.Context, userID string, pageToken string, platform string) (string, error) {
	log.Printf("🔍 Getting profile info for user %s (platform: %s)", userID, platform)

	// Check cache first
	if name, found := userCache.Get(userID); found {
		return name, nil
	}

	// Different endpoints and handling for Facebook and Instagram
	var userName string
	if platform == "facebook" {
		apiURL := fmt.Sprintf("https://graph.facebook.com/v19.0/%s?fields=name&access_token=%s", userID, pageToken)
		log.Printf("📡 Making Facebook API request for user %s", userID)

		var profile FacebookProfile
		if err := makeAPIRequest(ctx, apiURL, &profile); err != nil {
			return "user", err
		}
		userName = profile.Name
		log.Printf("👤 Using Facebook name: %s", userName)
	} else {
		apiURL := fmt.Sprintf("https://graph.facebook.com/v19.0/%s?fields=username&access_token=%s", userID, pageToken)
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
	pageInfo, err := getPageInfo(r.Context(), req.PageID)
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
