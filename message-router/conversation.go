// conversation.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

func getOrCreateConversation(ctx context.Context, pageID, threadID, platform string) (*ConversationState, error) {
	// Get UUID for database operations
	var pageUUID string
	err := db.QueryRowContext(ctx, `
        SELECT id 
        FROM social_pages 
        WHERE page_id = $1
    `, pageID).Scan(&pageUUID)
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

// updateConversationState updates the conversation state in the database
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

		// Add debug logging before the query
		log.Printf("🔍 Querying social_pages with page_id=%s and platform=%s", conv.PageID, conv.Platform)

		// Get page UUID and client_id using the page_id and matching the platform
		var pageUUID string
		var clientID sql.NullString // Use sql.NullString to handle NULL values
		err := tx.QueryRowContext(ctx, `
			SELECT id, client_id
			FROM social_pages 
			WHERE page_id = $1 AND platform = $2
		`, conv.PageID, conv.Platform).Scan(&pageUUID, &clientID)
		if err != nil {
			// Add debug logging for the error case
			if err == sql.ErrNoRows {
				// Also query to see what's actually in the table
				var count int
				tx.QueryRowContext(ctx, `
					SELECT COUNT(*) FROM social_pages WHERE page_id = $1
				`, conv.PageID).Scan(&count)
				log.Printf("❌ No matching page found. Found %d pages with page_id=%s", count, conv.PageID)

				// Let's see what platforms exist for this page_id
				rows, _ := tx.QueryContext(ctx, `
					SELECT platform FROM social_pages WHERE page_id = $1
				`, conv.PageID)
				defer rows.Close()

				platforms := []string{}
				for rows.Next() {
					var platform string
					rows.Scan(&platform)
					platforms = append(platforms, platform)
				}
				if len(platforms) > 0 {
					log.Printf("📝 Found platforms for this page_id: %v", platforms)
				}
			}
			return fmt.Errorf("error getting page UUID: %v", err)
		}

		log.Printf("✅ Found page UUID: %s and client_id: %v", pageUUID, clientID.String)

		// Insert system message with NULL client_id if not present
		var insertErr error
		if clientID.Valid {
			_, insertErr = tx.ExecContext(ctx, `
				INSERT INTO messages (
					id,
					client_id, 
					page_id,
					platform, 
					thread_id,
					content, 
					from_user, 
					source, 
					requires_attention,
					timestamp,
					read
				) VALUES (
					gen_random_uuid(),
					$1,     -- client_id
					$2,     -- page_id (UUID)
					$3,     -- platform
					$4,     -- thread_id 
					$5,     -- content
					'system',
					'system',
					$6,     -- requires_attention
					NOW(),
					false
				)
			`, clientID.String, pageUUID, conv.Platform, conv.ThreadID, stateMsg, !botEnabled)
		} else {
			_, insertErr = tx.ExecContext(ctx, `
				INSERT INTO messages (
					id,
					page_id,
					platform, 
					thread_id,
					content, 
					from_user, 
					source, 
					requires_attention,
					timestamp,
					read
				) VALUES (
					gen_random_uuid(),
					$1,     -- page_id (UUID)
					$2,     -- platform
					$3,     -- thread_id 
					$4,     -- content
					'system',
					'system',
					$5,     -- requires_attention
					NOW(),
					false
				)
			`, pageUUID, conv.Platform, conv.ThreadID, stateMsg, !botEnabled)
		}

		if insertErr != nil {
			return fmt.Errorf("error logging state change: %v", insertErr)
		}

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

// updateConversationForHumanMessage updates the conversation when a human agent sends a message
// This disables the bot for 6 hours to prevent double responses
// DEPRECATED: Used only as fallback when handover protocol fails. 
// New handover protocol should use Facebook's native thread control instead.
func updateConversationForHumanMessage(ctx context.Context, pageID, threadID, platform string) error {
	log.Printf("🔍 Updating conversation for human agent message: pageID=%s, threadID=%s, platform=%s", pageID, threadID, platform)

	// Get UUID for database operations
	var pageUUID string
	err := db.QueryRowContext(ctx, `
        SELECT id 
        FROM social_pages 
        WHERE page_id = $1
    `, pageID).Scan(&pageUUID)
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
	// This refreshes the 6-hour timer every time a human agent sends a message
	_, err = tx.ExecContext(ctx, `
        UPDATE conversations 
        SET last_human_message_at = NOW(),
            bot_enabled = false,
            latest_message_at = NOW(),
            message_count = message_count + 1,
            updated_at = NOW()
        WHERE thread_id = $1 AND page_id = $2
    `, threadID, pageUUID)
	if err != nil {
		return fmt.Errorf("error updating conversation for human message: %v", err)
	}

	// Get client_id for system message
	var clientID sql.NullString
	err = tx.QueryRowContext(ctx, `
        SELECT client_id
        FROM social_pages 
        WHERE page_id = $1 AND platform = $2
    `, pageID, platform).Scan(&clientID)
	if err != nil {
		return fmt.Errorf("error getting client_id: %v", err)
	}

	// Insert system message to log the bot disable
	stateMsg := "Bot disabled for 6 hours due to human agent activity (timer refreshes with each human message)"
	var insertErr error
	if clientID.Valid {
		_, insertErr = tx.ExecContext(ctx, `
            INSERT INTO messages (
                id,
                client_id, 
                page_id,
                platform, 
                thread_id,
                content, 
                from_user, 
                source, 
                requires_attention,
                timestamp,
                read
            ) VALUES (
                gen_random_uuid(),
                $1,     -- client_id
                $2,     -- page_id (UUID)
                $3,     -- platform
                $4,     -- thread_id 
                $5,     -- content
                'system',
                'system',
                true,   -- requires_attention
                NOW(),
                false
            )
        `, clientID.String, pageUUID, platform, threadID, stateMsg)
	} else {
		_, insertErr = tx.ExecContext(ctx, `
            INSERT INTO messages (
                id,
                page_id,
                platform, 
                thread_id,
                content, 
                from_user, 
                source, 
                requires_attention,
                timestamp,
                read
            ) VALUES (
                gen_random_uuid(),
                $1,     -- page_id (UUID)
                $2,     -- platform
                $3,     -- thread_id 
                $4,     -- content
                'system',
                'system',
                true,   -- requires_attention
                NOW(),
                false
            )
        `, pageUUID, platform, threadID, stateMsg)
	}

	if insertErr != nil {
		return fmt.Errorf("error logging human agent activity: %v", insertErr)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	log.Printf("✅ Successfully updated conversation for human agent message")
	return nil
}

// Legacy 6-hour timer functions removed - thread control now managed by Facebook Handover Protocol

// =============================================================================
// FACEBOOK HANDOVER PROTOCOL DATABASE FUNCTIONS - For thread control management
// =============================================================================

// updateThreadControlStatus updates the thread control status in database
func updateThreadControlStatus(ctx context.Context, threadID string, status string, reason string) error {
	query := `
        UPDATE conversations 
        SET thread_control_status = $1, 
            handover_timestamp = CURRENT_TIMESTAMP,
            handover_reason = $2,
            updated_at = CURRENT_TIMESTAMP
        WHERE thread_id = $3`

	result, err := db.ExecContext(ctx, query, status, reason, threadID)
	if err != nil {
		return fmt.Errorf("failed to update thread control status: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		LogWarn("Could not get rows affected count: %v", err)
	} else if rowsAffected == 0 {
		LogWarn("No conversation found with thread_id: %s", threadID)
		return fmt.Errorf("no conversation found with thread_id: %s", threadID)
	}

	LogDebug("✅ Thread control: %s -> %s (%s)", threadID, status, reason)
	return nil
}

// getThreadControlStatus retrieves current thread control status from database
func getThreadControlStatus(ctx context.Context, threadID string) (string, error) {
	var status string
	query := "SELECT COALESCE(thread_control_status, 'bot') FROM conversations WHERE thread_id = $1"

	err := db.QueryRowContext(ctx, query, threadID).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			LogDebug("No conversation found for thread_id: %s, defaulting to 'bot'", threadID)
			return "bot", nil // Graceful degradation
		}
		return "", fmt.Errorf("failed to get thread control status: %v", err)
	}

	LogDebug("Thread control status for %s: %s", threadID, status)
	return status, nil
}

// shouldBotProcessMessage determines if bot should process message based on bot_enabled flag
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

// getThreadControlStatusWithTimestamp retrieves thread control status with handover info
func getThreadControlStatusWithTimestamp(ctx context.Context, threadID string) (string, *time.Time, string, error) {
	var status, reason string
	var timestamp *time.Time

	query := `
        SELECT 
            COALESCE(thread_control_status, 'bot') as status,
            handover_timestamp,
            COALESCE(handover_reason, '') as reason
        FROM conversations 
        WHERE thread_id = $1`

	err := db.QueryRowContext(ctx, query, threadID).Scan(&status, &timestamp, &reason)
	if err != nil {
		if err == sql.ErrNoRows {
			LogDebug("No conversation found for thread_id: %s, defaulting to 'bot'", threadID)
			return "bot", nil, "", nil // Graceful degradation
		}
		return "", nil, "", fmt.Errorf("failed to get thread control status with timestamp: %v", err)
	}

	return status, timestamp, reason, nil
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
