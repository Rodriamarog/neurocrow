// conversation.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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
               c.message_count
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
		var pageUUID, clientID string
		err := tx.QueryRowContext(ctx, `
			SELECT id, COALESCE(client_id, '00000000-0000-0000-0000-000000000000')
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

		log.Printf("✅ Found page UUID: %s and client_id: %s", pageUUID, clientID)

		// Insert system message
		if _, err := tx.ExecContext(ctx, `
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
		`, clientID, pageUUID, conv.Platform, conv.ThreadID, stateMsg, !botEnabled); err != nil {
			return fmt.Errorf("error logging state change: %v", err)
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
