// conversation.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

// getOrCreateConversation retrieves or creates a conversation state
func getOrCreateConversation(ctx context.Context, pageID, threadID, platform string) (*ConversationState, error) {
	// First, get the UUID from the client manager database (pages table)
	var pageUUID string
	err := db.QueryRowContext(ctx, `
        SELECT id 
        FROM pages 
        WHERE page_id = $1
    `, pageID).Scan(&pageUUID)

	if err != nil {
		return nil, fmt.Errorf("error finding page in client manager: %v", err)
	}

	conv := &ConversationState{}
	err = socialDB.QueryRowContext(ctx, `
        SELECT thread_id, page_id, platform, bot_enabled, 
               COALESCE(last_bot_message_at, '1970-01-01'::timestamp),
               COALESCE(last_human_message_at, '1970-01-01'::timestamp),
               COALESCE(last_user_message_at, '1970-01-01'::timestamp),
               message_count
        FROM conversations 
        WHERE thread_id = $1 AND page_id = $2
    `, threadID, pageUUID).Scan(
		&conv.ThreadID, &conv.PageID, &conv.Platform, &conv.BotEnabled,
		&conv.LastBotMessage, &conv.LastHumanMessage, &conv.LastUserMessage,
		&conv.MessageCount,
	)

	if err == sql.ErrNoRows {
		// Create new conversation
		conv = &ConversationState{
			ThreadID:   threadID,
			PageID:     pageUUID, // Using the UUID from client manager
			Platform:   platform,
			BotEnabled: true,
		}

		err = socialDB.QueryRowContext(ctx, `
            INSERT INTO conversations (
                thread_id, page_id, platform, bot_enabled, 
                first_message_at, latest_message_at, message_count
            ) VALUES ($1, $2, $3, $4, NOW(), NOW(), 1)
            RETURNING thread_id
        `, conv.ThreadID, conv.PageID, conv.Platform, conv.BotEnabled).Scan(&conv.ThreadID)

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
	// Start a transaction
	tx, err := socialDB.BeginTx(ctx, nil)
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

		// Note: we're inserting into social dashboard's messages table
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO messages (
                client_id, 
                page_id,
                platform, 
                thread_id,
                content, 
                from_user, 
                source, 
                requires_attention,
                timestamp
            ) VALUES (
                (SELECT client_id FROM social_pages WHERE page_id = $1),
                $1, $2, $3, $4, 'system', 'system', $5,
                NOW()
            )
        `, conv.PageID, conv.Platform, conv.ThreadID, stateMsg, !botEnabled); err != nil {
			return fmt.Errorf("error logging state change: %v", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	return nil
}
