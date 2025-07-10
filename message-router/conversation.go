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

	// Start a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Update conversations table
	if _, err := tx.ExecContext(ctx, `
		UPDATE conversations 
		SET bot_enabled = $1,
			latest_message_at = NOW(),
			message_count = message_count + 1
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

	// Start a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Update the conversation to record human message and disable bot
	// This refreshes the 6-hour timer every time a human agent sends a message
	_, err = tx.ExecContext(ctx, `
        UPDATE conversations 
        SET last_human_message_at = NOW(),
            bot_enabled = false,
            latest_message_at = NOW(),
            message_count = message_count + 1
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

// isRecentHumanActivity checks if there has been human agent activity within the last 6 hours
func isRecentHumanActivity(conv *ConversationState) bool {
	if conv.LastHumanMessage.IsZero() {
		return false
	}

	// Check if the last human message was within the last 6 hours
	sixHoursAgo := time.Now().Add(-6 * time.Hour)
	recentActivity := conv.LastHumanMessage.After(sixHoursAgo)

	if recentActivity {
		log.Printf("🕐 Recent human activity detected: last human message at %v (within 6 hours)", conv.LastHumanMessage)
	}

	return recentActivity
}
