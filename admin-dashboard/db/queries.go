package db

// Message related queries
const (
	// Query to fetch messages for the main message list
	GetMessagesQuery = `
        WITH thread_owner AS (
            SELECT DISTINCT ON (m.thread_id)
                m.thread_id, 
                m.from_user as original_sender
            FROM messages m
            WHERE m.platform IN ('facebook', 'instagram')
            ORDER BY m.thread_id, m.timestamp ASC
        ),
        latest_messages AS (
            SELECT DISTINCT ON (m.thread_id)
                m.id, 
                COALESCE(m.client_id, '00000000-0000-0000-0000-000000000000') as client_id,
                m.page_id, 
                m.platform,
                t.original_sender as thread_owner,
                m.content, 
                m.timestamp, 
                m.thread_id, 
                m.read,
                m.source
            FROM messages m
            JOIN thread_owner t ON m.thread_id = t.thread_id
            ORDER BY m.thread_id, m.timestamp DESC
        )
        SELECT 
            lm.id, 
            lm.client_id,
            lm.page_id, 
            lm.platform,
            lm.thread_owner as from_user,  
            lm.content, 
            lm.timestamp, 
            lm.thread_id, 
            lm.read,
            lm.source,
            COALESCE(c.bot_enabled, TRUE) AS bot_enabled,
            CASE 
                WHEN lm.source IN ('bot', 'admin', 'system') THEN '/static/default-avatar.png'
                WHEN c.profile_picture_url IS NOT NULL THEN c.profile_picture_url
                ELSE '/static/default-avatar.png'
            END as profile_picture_url
        FROM latest_messages lm
        LEFT JOIN conversations c ON c.thread_id = lm.thread_id
        ORDER BY lm.timestamp DESC`

	// Query to search messages
	GetMessageListSearchQuery = `
        WITH thread_owner AS (
            SELECT DISTINCT ON (m.thread_id)
                m.thread_id, 
                m.from_user as original_sender
            FROM messages m
            ORDER BY m.thread_id, m.timestamp ASC
        ),
        latest_messages AS (
            SELECT DISTINCT ON (m.thread_id)
                m.id, 
                m.client_id, 
                m.page_id, 
                m.platform,
                t.original_sender as thread_owner,
                m.content, 
                m.timestamp, 
                m.thread_id, 
                m.read,
                m.source
            FROM messages m
            JOIN thread_owner t ON m.thread_id = t.thread_id
            ORDER BY m.thread_id, m.timestamp DESC
        )
        SELECT 
            lm.id, 
            lm.client_id, 
            lm.page_id, 
            lm.platform,
            lm.thread_owner as from_user,  
            lm.content, 
            lm.timestamp, 
            lm.thread_id, 
            lm.read,
            lm.source,
            COALESCE(c.bot_enabled, TRUE) AS bot_enabled,
            CASE 
                WHEN lm.source IN ('bot', 'admin', 'system') THEN '/static/default-avatar.png'
                WHEN c.profile_picture_url IS NOT NULL THEN c.profile_picture_url
                ELSE '/static/default-avatar.png'
            END as profile_picture_url
        FROM latest_messages lm
        LEFT JOIN conversations c ON c.thread_id = lm.thread_id
        WHERE 
            lm.content ILIKE $1 OR 
            lm.thread_owner ILIKE $1
        ORDER BY lm.timestamp DESC`

	// Query to insert a new message
	InsertMessageQuery = `
        INSERT INTO messages (
            client_id,
            page_id,
            platform,
            from_user,
            source,
            content,
            thread_id,
            read
        ) VALUES (
            NULLIF($1, '')::uuid,
            $2,
            $3,
            'admin',
            'human',
            $4,
            $5,
            true
        )
        RETURNING id`
)

// Chat related queries
const (
	// Query to get chat messages
	GetChatQuery = `
        SELECT 
            m.id, 
            m.client_id, 
            m.page_id, 
            m.platform, 
            m.from_user,
            m.content, 
            m.timestamp, 
            m.thread_id, 
            m.read, 
            m.source,
            COALESCE(c.bot_enabled, true) as bot_enabled,
            CASE 
                WHEN m.source IN ('bot', 'admin', 'system') THEN '/static/default-avatar.png'
                WHEN c.profile_picture_url IS NOT NULL THEN c.profile_picture_url
                ELSE '/static/default-avatar.png'
            END as profile_picture_url
        FROM messages m
        LEFT JOIN conversations c ON m.thread_id = c.thread_id
        WHERE m.thread_id = $1
            AND (m.internal IS NULL OR m.internal = false)
        ORDER BY m.timestamp ASC`

	// Query to get thread details
	GetThreadDetailsQuery = `
        SELECT 
            m.client_id,
            m.page_id,
            m.platform,
            CASE 
                WHEN m.source IN ('bot', 'admin', 'system') THEN '/static/default-avatar.png'
                WHEN c.profile_picture_url IS NOT NULL THEN c.profile_picture_url
                ELSE '/static/default-avatar.png'
            END as profile_picture_url,
            COALESCE(c.bot_enabled, TRUE) as bot_enabled
        FROM messages m
        LEFT JOIN conversations c ON c.thread_id = m.thread_id
        WHERE m.thread_id = $1 
        ORDER BY m.timestamp DESC
        LIMIT 1`
)

// Conversation related queries
const (
	// Query to update bot status
	UpdateBotStatusQuery = `
        UPDATE conversations 
        SET bot_enabled = $2, 
            updated_at = CURRENT_TIMESTAMP
        WHERE thread_id = $1`

	// Query to update profile picture
	UpdateProfilePictureQuery = `
        UPDATE conversations 
        SET profile_picture_url = $1,
            updated_at = CURRENT_TIMESTAMP
        WHERE thread_id = $2`

	// Query to get thread preview
	GetThreadPreviewQuery = `
        WITH thread_owner AS (
            SELECT DISTINCT ON (m.thread_id)
                m.thread_id, 
                m.from_user as original_sender
            FROM messages m
            WHERE m.thread_id = $1
            ORDER BY m.thread_id, m.timestamp ASC
        )
        SELECT
            m.id, 
            m.client_id, 
            m.page_id, 
            m.platform,
            t.original_sender as from_user,
            m.content, 
            m.timestamp, 
            m.thread_id, 
            m.read,
            m.source,
            COALESCE(c.bot_enabled, TRUE) AS bot_enabled,
            CASE 
                WHEN m.source IN ('bot', 'admin', 'system') THEN '/static/default-avatar.png'
                WHEN c.profile_picture_url IS NOT NULL THEN c.profile_picture_url
                ELSE '/static/default-avatar.png'
            END as profile_picture_url
        FROM messages m
        JOIN thread_owner t ON m.thread_id = t.thread_id
        LEFT JOIN conversations c ON m.thread_id = c.thread_id
        WHERE m.thread_id = $1
        ORDER BY m.timestamp DESC
        LIMIT 1`
)
