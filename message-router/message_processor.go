// message_processor.go
package main

import (
	"context"
	"fmt"
	"message-router/sentiment"
	"strings"
	"time"
)

// processMessagesAsync processes Facebook webhook messages asynchronously.
//
// This is the core message processing pipeline that handles the complete lifecycle
// of incoming messages from Facebook Messenger and Instagram Direct Messages.
// The function runs asynchronously to prevent webhook timeouts while performing
// comprehensive message analysis and routing.
//
// Processing Pipeline:
//
//  1. Bot Reactivation Check: Automatically reactivates bots that have been
//     disabled for 12+ hours due to human agent inactivity
//
//  2. Message Filtering: Filters out delivery receipts, empty messages, and
//     validates message content and sender information
//
//  3. Echo Message Handling: Intelligently processes echo messages to distinguish
//     between bot responses (skip) and human agent messages (disable bot)
//
//  4. User Profile Retrieval: Fetches user profile information from Facebook/Instagram
//     APIs for personalization and conversation context
//
//  5. Thread Control Validation: Checks current thread control status to determine
//     if the bot should process the message or if a human has control
//
//  6. Sentiment Analysis: Analyzes message content using Fireworks AI to categorize
//     user intent: 'general', 'frustrated', or 'need_human'
//
//  7. Response Routing:
//     - General messages: Routes to Dify AI for automated chatbot response
//     - Frustrated users: Sends empathy message and escalates to human agents
//     - Human requests: Immediately connects users to human support
//
// Echo Message Logic:
//
// Echo messages are responses sent by bots or human agents. The function distinguishes:
//   - Bot echoes (app_id: 1195277397801905): Skip processing to avoid loops
//   - Human agent echoes (sender_id == page_id): Disable bot and update thread control
//   - Unknown patterns: Log for investigation and monitoring
//
// Error Handling:
//
// The function implements graceful error handling with fallback mechanisms:
//   - Database errors default to bot-enabled state for graceful degradation
//   - API failures are logged but don't prevent processing other messages
//   - Thread control failures default to allowing bot processing
//
// Parameters:
//   - ctx: Context for cancellation, timeouts, and database operations
//   - event: Facebook webhook event containing message entries and metadata
//   - requestID: Unique identifier for correlating logs across async processing
//
// The function processes each message entry in the webhook event and handles
// various message types while maintaining conversation state and thread control.
// All operations are logged with the requestID for debugging and monitoring.
func processMessagesAsync(ctx context.Context, event FacebookEvent, requestID string) {
	LogDebug("[%s] 🔄 Starting async message processing", requestID)

	// Step 1: Check and reactivate eligible bots before processing new messages (12-hour rule)
	checkAndReactivateBots(ctx, requestID)

	// Step 2: Process each entry in the webhook event
	for _, entry := range event.Entry {
		if len(entry.Messaging) == 0 {
			LogDebug("[%s] No messages in entry %s", requestID, entry.ID)
			continue
		}

		// Step 3: Process each message in the entry
		for msgIndex, msg := range entry.Messaging {
			// Step 4: Filter and validate the message
			if !filterAndValidateMessage(msg, requestID, msgIndex) {
				continue // Message was filtered out or invalid
			}

			// Step 5: Handle echo messages (bot responses, human agent interventions)
			echoAction, err := handleEchoMessage(ctx, msg, entry, event, requestID)
			if err != nil {
				LogError("[%s] Echo message handling failed: %v", requestID, err)
				continue
			}

			switch echoAction {
			case EchoActionSkip:
				continue // Skip this message (bot echo or unknown pattern)
			case EchoActionDisableBot:
				continue // Bot was disabled due to human agent, message processed
			case EchoActionContinue:
				// Continue with normal user message processing
			}

			// Step 6: Log that we're processing a user message
			LogInfo("[%s] 👤 User message detected - proceeding with bot processing", requestID)

			// Step 7: Gather all context needed for processing (conversation, page info, user profile)
			msgContext, err := gatherMessageContext(ctx, msg, entry, event, requestID)
			if err != nil {
				// Error already logged in gatherMessageContext
				continue
			}

			// Step 8: Check if bot should process this message
			shouldProcess, err := shouldBotProcessMessage(ctx, msg.Sender.ID)
			if err != nil {
				LogWarn("[%s] Thread control check failed, defaulting to bot: %v", requestID, err)
				shouldProcess = true // Graceful degradation
			}

			if !shouldProcess {
				LogInfo("[%s] 🔴 Bot disabled for this conversation - skipping processing", requestID)
				continue
			}

			// Step 9: Process sentiment and route accordingly
			if err := processSentimentAndRoute(ctx, msgContext, requestID); err != nil {
				LogError("[%s] Failed to process sentiment and route: %v", requestID, err)
			}
		}
	}

	LogDebug("[%s] ✅ Async message processing completed", requestID)
}

// filterAndValidateMessage filters out unwanted messages and validates content
func filterAndValidateMessage(msg MessagingEntry, requestID string, msgIndex int) bool {
	// Skip non-message events (delivery receipts, etc.)
	if msg.Delivery != nil {
		LogDebug("[%s] Skipping delivery receipt", requestID)
		return false
	}

	// Validate message content
	if msg.Message == nil || msg.Message.Text == "" {
		LogDebug("[%s] Skipping empty message from %s", requestID, msg.Sender.ID)
		return false
	}

	// Log every message we receive for debugging
	LogInfo("[%s] 📨 Raw message %d: sender=%s, recipient=%s, echo=%v, app_id=%d, text=%q",
		requestID, msgIndex, msg.Sender.ID, msg.Recipient.ID, msg.Message.IsEcho, msg.Message.AppId, msg.Message.Text)

	// Skip non-user senders (but allow regular user messages and echo messages)
	if !msg.Message.IsEcho && (strings.HasPrefix(msg.Sender.ID, "page-") || strings.HasPrefix(msg.Sender.ID, "bot-")) {
		LogDebug("[%s] Skipping non-user message from %s", requestID, msg.Sender.ID)
		return false
	}

	return true
}

// EchoAction represents what to do after processing an echo message
type EchoAction int

const (
	EchoActionContinue    EchoAction = iota // Continue with normal processing
	EchoActionSkip                          // Skip this message
	EchoActionDisableBot                    // Bot was disabled, skip processing
)

// handleEchoMessage processes echo messages intelligently
func handleEchoMessage(ctx context.Context, msg MessagingEntry, entry EntryData, event FacebookEvent, requestID string) (EchoAction, error) {
	// Only process if this is an echo message
	if !msg.Message.IsEcho {
		return EchoActionContinue, nil
	}

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
			return EchoActionSkip, nil
		} else {
			LogInfo("[%s] 👤 Instagram human agent message detected (no bot flag) - disabling bot", requestID)

			// Auto-disable bot for human agent intervention
			err := updateConversationForHumanMessage(ctx, entry.ID, msg.Recipient.ID, platform)
			if err != nil {
				LogError("[%s] ❌ Failed to disable bot for human agent: %v", requestID, err)
				return EchoActionSkip, err
			} else {
				LogInfo("[%s] ✅ Bot successfully disabled for human agent", requestID)
			}
			return EchoActionDisableBot, nil
		}
	}

	// Facebook logic (unchanged)
	if platform == "facebook" {
		// Check if this is a bot echo (our own bot responses)
		if msg.Message.AppId == 1195277397801905 {
			LogInfo("[%s] 🤖 Facebook bot echo message detected (app_id: %d) - skipping", requestID, msg.Message.AppId)
			return EchoActionSkip, nil
		}

		// Check if this is a human agent message (sender = page)
		if msg.Sender.ID == entry.ID {
			LogInfo("[%s] 👤 Facebook human agent message detected! sender=%s matches page=%s, app_id=%d",
				requestID, msg.Sender.ID, entry.ID, msg.Message.AppId)

			LogInfo("[%s] 🔴 Auto-disabling bot due to human agent intervention", requestID)
			err := updateConversationForHumanMessage(ctx, entry.ID, msg.Recipient.ID, platform)
			if err != nil {
				LogError("[%s] ❌ Failed to disable bot for human agent: %v", requestID, err)
				return EchoActionSkip, err
			} else {
				LogInfo("[%s] ✅ Bot successfully disabled for human agent", requestID)
			}
			return EchoActionDisableBot, nil
		}

		// Unknown Facebook echo message pattern
		LogWarn("[%s] ⚠️ Unknown Facebook echo pattern: sender=%s, page=%s, app_id=%d",
			requestID, msg.Sender.ID, entry.ID, msg.Message.AppId)
		return EchoActionSkip, nil
	}

	// Unknown platform
	LogWarn("[%s] ⚠️ Unknown platform echo message: platform=%s", requestID, platform)
	return EchoActionSkip, nil
}

// MessageContext contains all the context needed for processing a message
type MessageContext struct {
	Message      MessagingEntry
	Conversation *ConversationState
	PageInfo     *PageInfo
	UserName     string
	Platform     string
	RequestID    string
}

// gatherMessageContext collects all necessary context for message processing
func gatherMessageContext(ctx context.Context, msg MessagingEntry, entry EntryData, event FacebookEvent, requestID string) (*MessageContext, error) {
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
		return nil, err
	}

	pageInfo, err := getPageInfo(ctx, entry.ID, platform)
	if err != nil {
		LogError("[%s] Failed to get page info for %s: %v", requestID, entry.ID, err)
		return nil, err
	}

	// Get user profile (non-critical, don't log failures unless debug)
	userName, err := getProfileInfo(ctx, msg.Sender.ID, pageInfo.AccessToken, platform)
	if err != nil {
		LogDebug("[%s] Could not get user name for %s: %v", requestID, msg.Sender.ID, err)
		userName = "user"
	} else {
		updateConversationUsername(ctx, msg.Sender.ID, userName) // Fire and forget
	}

	return &MessageContext{
		Message:      msg,
		Conversation: conv,
		PageInfo:     pageInfo,
		UserName:     userName,
		Platform:     platform,
		RequestID:    requestID,
	}, nil
}

// processSentimentAndRoute analyzes sentiment and routes the message accordingly
func processSentimentAndRoute(ctx context.Context, msgContext *MessageContext, requestID string) error {
	// Analyze sentiment
	start := time.Now()
	analysis, err := sentimentAnalyzer.Analyze(ctx, msgContext.Message.Message.Text)
	if err != nil {
		LogError("[%s] Sentiment analysis failed for %s: %v", requestID, msgContext.Message.Sender.ID, err)
		return err
	}

	// Single consolidated log for processing status
	processingTime := time.Since(start)
	LogInfo("[%s] 🤖 Processing: %s sentiment, %d tokens, %dms",
		requestID, analysis.Status, analysis.TokensUsed, processingTime.Milliseconds())

	// Log cost details only in debug mode
	LogDebug("[%s] Estimated cost: $%.6f", requestID, float64(analysis.TokensUsed)*0.20/1_000_000)

	// Route based on sentiment analysis
	return routeBasedOnSentiment(ctx, msgContext, analysis, requestID)
}

// routeBasedOnSentiment routes messages based on sentiment analysis results
func routeBasedOnSentiment(ctx context.Context, msgContext *MessageContext, analysis *sentiment.Analysis, requestID string) error {
	switch analysis.Status {
	case "need_human":
		return handleNeedHumanRequest(ctx, msgContext, requestID)
	case "frustrated":
		return handleFrustratedUser(ctx, msgContext, requestID)
	case "general":
		return handleGeneralMessage(ctx, msgContext, requestID)
	default:
		LogWarn("[%s] Unknown sentiment status: %s", requestID, analysis.Status)
		return handleGeneralMessage(ctx, msgContext, requestID) // Default to general
	}
}

// handleNeedHumanRequest processes requests for human assistance
func handleNeedHumanRequest(ctx context.Context, msgContext *MessageContext, requestID string) error {
	LogInfo("[%s] 👤 User requested human - disabling bot", requestID)

	// Send handoff message and disable bot
	handoffMsg := "Te conectaré con un agente humano en breve. Mientras tanto, puedes seguir escribiendo y un agente te responderá."

	if err := sendPlatformResponse(ctx, msgContext.PageInfo, msgContext.Message.Sender.ID, handoffMsg); err != nil {
		LogError("[%s] Failed to send handoff message: %v", requestID, err)
	}

	// Disable bot for this conversation
	if err := updateConversationState(ctx, msgContext.Conversation, false, "User requested human assistance"); err != nil {
		LogError("[%s] Failed to disable bot: %v", requestID, err)
		return err
	}

	LogInfo("[%s] ✅ User connected to human agent", requestID)
	return nil
}

// handleFrustratedUser processes messages from frustrated users
func handleFrustratedUser(ctx context.Context, msgContext *MessageContext, requestID string) error {
	LogInfo("[%s] 😤 User frustrated - disabling bot", requestID)

	// Send empathy message and escalate
	empathyMsg := "Entiendo tu frustración. Te estoy conectando con un agente humano que podrá ayudarte mejor."

	if err := sendPlatformResponse(ctx, msgContext.PageInfo, msgContext.Message.Sender.ID, empathyMsg); err != nil {
		LogError("[%s] Failed to send empathy message: %v", requestID, err)
	}

	// Disable bot and escalate to human
	if err := updateConversationState(ctx, msgContext.Conversation, false, "User appears frustrated"); err != nil {
		LogError("[%s] Failed to disable bot: %v", requestID, err)
		return err
	}

	LogInfo("[%s] ✅ Frustrated user escalated to human agent", requestID)
	return nil
}

// handleGeneralMessage processes general messages through the AI system
func handleGeneralMessage(ctx context.Context, msgContext *MessageContext, requestID string) error {
	LogInfo("[%s] 💬 General message - forwarding to Dify AI", requestID)

	// Forward to Dify for AI response
	if err := forwardToDify(ctx, msgContext.PageInfo.PageID, msgContext.Message, msgContext.Platform); err != nil {
		LogError("[%s] Dify forwarding failed: %v", requestID, err)

		// Send fallback message to user
		fallbackMsg := "Disculpa, estoy teniendo problemas técnicos. Un agente humano te ayudará pronto."
		if sendErr := sendPlatformResponse(ctx, msgContext.PageInfo, msgContext.Message.Sender.ID, fallbackMsg); sendErr != nil {
			LogError("[%s] Failed to send fallback message: %v", requestID, sendErr)
		}

		// Disable bot due to technical error
		updateConversationState(ctx, msgContext.Conversation, false, "Error al procesar con Dify")
		return err
	}

	LogInfo("[%s] ✅ Message successfully processed by Dify AI", requestID)
	return nil
}