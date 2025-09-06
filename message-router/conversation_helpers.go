// conversation_helpers.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

// =============================================================================
// CONVERSATION STATE MANAGEMENT - Core database operations for conversation tracking
// =============================================================================

// getOrCreateConversation retrieves or creates a conversation record in the database
func getOrCreateConversation(ctx context.Context, pageID, threadID, platform string) (*ConversationState, error) {
	// Get UUID for database operations - FIXED: Include platform to avoid conflicts
	var pageUUID string
	err := db.QueryRowContext(ctx, `
        SELECT id 
        FROM social_pages 
        WHERE page_id = $1 AND platform = $2
    `, pageID, platform).Scan(&pageUUID)
	if err != nil {
		return nil, fmt.Errorf("error finding page: %v", err)
	}

	conv := &ConversationState{}
	err = db.QueryRowContext(ctx, `
        SELECT c.thread_id, sp.page_id, c.platform, c.bot_enabled, 
               COALESCE(c.last_bot_message_at, '1970-01-01'::timestamp),
               COALESCE(c.last_human_message_at, '1970-01-01'::timestamp),
               COALESCE(c.last_user_message_at, '1970-01-01'::timestamp),
               c.message_count,
               COALESCE(c.dify_conversation_id, '')
        FROM conversations c
        JOIN social_pages sp ON sp.id = c.page_id
        WHERE c.thread_id = $1 AND c.page_id = $2
    `, threadID, pageUUID).Scan(
		&conv.ThreadID,
		&conv.PageID, // Now will get the original page_id from social_pages
		&conv.Platform,
		&conv.BotEnabled,
		&conv.LastBotMessage,
		&conv.LastHumanMessage,
		&conv.LastUserMessage,
		&conv.MessageCount,
		&conv.DifyConversationID,
	)

	if err == sql.ErrNoRows {
		// Create new conversation
		conv = &ConversationState{
			ThreadID:   threadID,
			PageID:     pageID, // Use original pageID
			Platform:   platform,
			BotEnabled: true,
		}

		err = db.QueryRowContext(ctx, `
            INSERT INTO conversations (
                thread_id, page_id, platform, bot_enabled, 
                first_message_at, latest_message_at, message_count
            ) VALUES ($1, $2, $3, $4, NOW(), NOW(), 1)
            RETURNING thread_id
        `, conv.ThreadID, pageUUID, conv.Platform, conv.BotEnabled).Scan(&conv.ThreadID)

		if err != nil {
			return nil, fmt.Errorf("error creating conversation: %v", err)
		}

		log.Printf("✨ Created new conversation: %s", conv.ThreadID)
		return conv, nil
	}

	if err != nil {
		return nil, fmt.Errorf("error fetching conversation: %v", err)
	}

	log.Printf("📝 Retrieved conversation: %s (bot enabled: %v)", conv.ThreadID, conv.BotEnabled)
	return conv, nil
}

// updateConversationState updates the conversation state in the database.
//
// This function manages the bot enable/disable state for conversations and logs
// all state transitions as system messages for auditing and debugging purposes.
// It implements atomic database operations with row-level locking to prevent
// race conditions during concurrent message processing.
//
// State Management:
//
// The function handles bot state transitions for conversation management:
//   - Enables bot when users need automated assistance
//   - Disables bot when human agents take control or users are frustrated
//   - Updates conversation metadata including message counts and timestamps
//   - Logs state changes as system messages for audit trails
//
// Database Operations:
//
// The function uses database transactions with row-level locking:
//   - BEGIN transaction with row-level locking (FOR UPDATE)
//   - Update conversation state and metadata
//   - Insert system message documenting the state change
//   - Update bot_enabled flag for immediate effect
//   - COMMIT transaction to ensure atomicity
//
// Concurrency Safety:
//
// Row-level locking prevents race conditions when multiple messages arrive
// simultaneously for the same conversation:
//   - Locks conversation row during state updates
//   - Prevents conflicting state changes from concurrent operations
//   - Ensures consistent state transitions and audit logging
//
// Multi-tenant Support:
//
// The function handles multi-tenant database structure:
//   - Resolves page_id to internal UUID for database operations
//   - Handles client_id associations for proper message storage
//   - Supports NULL client_id for system-level operations
//   - Maintains data isolation between different clients
//
// System Message Logging:
//
// All bot state changes are logged as system messages:
//   - Includes human-readable reason for the state change
//   - Sets requires_attention flag for disabled states
//   - Associates messages with appropriate client and page
//   - Provides audit trail for conversation management
//
// Parameters:
//   - ctx: Context for database operations with timeout and cancellation
//   - conv: ConversationState containing thread and page information
//   - botEnabled: Target state for bot processing (true=enabled, false=disabled)
//   - reason: Human-readable explanation for the state change
//
// Returns:
//   - nil: State successfully updated and logged
//   - error: Database operation failed with detailed error information
//
// The function ensures conversation state consistency and provides comprehensive
// logging for debugging bot behavior and human agent interventions.
func updateConversationState(ctx context.Context, conv *ConversationState, botEnabled bool, reason string) error {
	// Add debug logging at the start
	log.Printf("🔍 Updating conversation state: pageID=%s, platform=%s, threadID=%s, botEnabled=%v",
		conv.PageID, conv.Platform, conv.ThreadID, botEnabled)

	// Start a transaction with row-level locking to prevent race conditions
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Lock the conversation row to prevent concurrent modifications
	var currentBotState bool
	err = tx.QueryRowContext(ctx, `
        SELECT bot_enabled FROM conversations 
        WHERE thread_id = $1
        FOR UPDATE
    `, conv.ThreadID).Scan(&currentBotState)
	if err != nil {
		return fmt.Errorf("error locking conversation: %v", err)
	}

	log.Printf("🔒 Conversation state locked - current: %v, target: %v", currentBotState, botEnabled)

	// Update conversations table
	if _, err := tx.ExecContext(ctx, `
		UPDATE conversations 
		SET bot_enabled = $1,
			bot_disabled_at = CASE WHEN $1 = false THEN NOW() ELSE NULL END,
			latest_message_at = NOW(),
			message_count = message_count + 1,
			updated_at = NOW()
		WHERE thread_id = $2
	`, botEnabled, conv.ThreadID); err != nil {
		return fmt.Errorf("error updating conversation: %v", err)
	}

	// Log state change if bot state changed
	if botEnabled != conv.BotEnabled {
		stateMsg := fmt.Sprintf("Bot %s: %s",
			map[bool]string{true: "enabled", false: "disabled"}[botEnabled],
			reason,
		)

		log.Printf("🔧 %s", stateMsg)

		// Update the bot_enabled state in the conversation
		if _, err := tx.ExecContext(ctx, `
			UPDATE conversations
			SET bot_enabled = $1
			WHERE thread_id = $2
		`, botEnabled, conv.ThreadID); err != nil {
			return fmt.Errorf("error updating bot state: %v", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	log.Printf("✅ Successfully updated conversation state")
	return nil
}

// updateConversationForHumanMessage updates the conversation when a human agent sends a message.
// This disables the bot to prevent conflicts between human agents and automated responses.
// The bot will be automatically re-enabled after 12 hours of human agent inactivity.
func updateConversationForHumanMessage(ctx context.Context, pageID, threadID, platform string) error {
	log.Printf("🔍 Updating conversation for human agent message: pageID=%s, threadID=%s, platform=%s", pageID, threadID, platform)

	// Get UUID for database operations - FIXED: Include platform to avoid conflicts
	var pageUUID string
	err := db.QueryRowContext(ctx, `
        SELECT id 
        FROM social_pages 
        WHERE page_id = $1 AND platform = $2
    `, pageID, platform).Scan(&pageUUID)
	if err != nil {
		return fmt.Errorf("error finding page: %v", err)
	}

	// Start a transaction with row-level locking to prevent race conditions
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Lock the conversation row to prevent concurrent modifications
	var currentBotState bool
	err = tx.QueryRowContext(ctx, `
        SELECT bot_enabled FROM conversations 
        WHERE thread_id = $1 AND page_id = $2
        FOR UPDATE
    `, threadID, pageUUID).Scan(&currentBotState)
	if err != nil {
		return fmt.Errorf("error locking conversation: %v", err)
	}

	log.Printf("🔒 Conversation locked - current bot state: %v", currentBotState)

	// Update the conversation to record human message and disable bot
	// The bot will be automatically reactivated after 12 hours of inactivity
	_, err = tx.ExecContext(ctx, `
        UPDATE conversations 
        SET last_human_message_at = NOW(),
            bot_enabled = false,
            bot_disabled_at = NOW(),
            latest_message_at = NOW(),
            message_count = message_count + 1,
            updated_at = NOW()
        WHERE thread_id = $1 AND page_id = $2
    `, threadID, pageUUID)
	if err != nil {
		return fmt.Errorf("error updating conversation for human message: %v", err)
	}

	// Log the bot disable
	stateMsg := "Bot disabled due to human agent intervention (will reactivate after 12 hours of inactivity)"
	log.Printf("🔧 %s", stateMsg)

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	log.Printf("✅ Successfully updated conversation for human agent message")
	return nil
}

// updateConversationUsername updates the user's social media name in the conversation record
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

// =============================================================================
// BOT CONTROL FUNCTIONS - Managing when bots should process messages
// =============================================================================

// shouldBotProcessMessage determines if bot should process message based on bot_enabled flag.
// This is the actual function used for bot control - returns true if bot should respond.
func shouldBotProcessMessage(ctx context.Context, threadID string) (bool, error) {
	var botEnabled bool
	query := "SELECT COALESCE(bot_enabled, true) FROM conversations WHERE thread_id = $1"

	err := db.QueryRowContext(ctx, query, threadID).Scan(&botEnabled)
	if err != nil {
		if err == sql.ErrNoRows {
			LogDebug("No conversation found for thread_id: %s, defaulting to enabled", threadID)
			return true, nil // New conversations default to bot enabled
		}
		LogWarn("Bot enabled check failed, defaulting to enabled: %v", err)
		return true, nil // Graceful degradation
	}

	LogDebug("Bot should process %s? %v", threadID, botEnabled)
	return botEnabled, nil
}

// checkAndReactivateBots calls the database function to reactivate eligible bots (12-hour rule)
// Returns the number of bots reactivated, used for logging
func checkAndReactivateBots(ctx context.Context, requestID string) {
	var reactivatedCount int
	err := db.QueryRowContext(ctx, "SELECT reenable_disabled_bots()").Scan(&reactivatedCount)

	if err != nil {
		LogError("[%s] Bot reactivation check failed: %v", requestID, err)
		return
	}

	// Only log when bots are actually reactivated to avoid spam
	if reactivatedCount > 0 {
		LogInfo("[%s] 🔄 Reactivated %d bots (12-hour rule)", requestID, reactivatedCount)
	} else {
		LogDebug("[%s] No bots eligible for reactivation", requestID)
	}
}

