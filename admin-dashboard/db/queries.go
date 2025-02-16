package db

// Message related queries
const (
	// Updated GetMessagesQuery with proper UUID casting for client_id
	GetMessagesQuery = `
    WITH thread_owner AS (
        SELECT DISTINCT ON (m.thread_id)
            m.thread_id, 
            m.from_user as original_sender
        FROM messages m
        JOIN social_pages sp ON m.page_id = sp.id
        WHERE sp.client_id = $1::uuid  -- Add proper UUID casting
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
            WHEN c.profile_picture_url IS NULL THEN '/static/default-avatar.png'
            WHEN c.profile_picture_url = '' THEN '/static/default-avatar.png'
            ELSE c.profile_picture_url
        END as profile_picture_url
    FROM latest_messages lm
    LEFT JOIN conversations c ON c.thread_id = lm.thread_id
    ORDER BY lm.timestamp DESC
   `

	// Updated GetMessageListSearchQuery: now joins with social_pages and filters by client_id.
	// Note: client_id is parameter $1 and the search term is $2.
	GetMessageListSearchQuery = `
        WITH thread_owner AS (
            SELECT DISTINCT ON (m.thread_id)
                m.thread_id, 
                m.from_user as original_sender
            FROM messages m
            JOIN social_pages sp ON m.page_id = sp.id
            WHERE sp.client_id = $1::uuid
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
            COALESCE(NULLIF(TRIM(c.profile_picture_url), ''), '/static/default-avatar.png') as profile_picture_url
        FROM latest_messages lm
        LEFT JOIN conversations c ON c.thread_id = lm.thread_id
        WHERE 
            CASE 
                WHEN $2 != '' THEN 
                    lm.content ILIKE '%' || $2 || '%' OR 
                    lm.thread_owner ILIKE '%' || $2 || '%'
                ELSE TRUE
            END
        ORDER BY lm.timestamp DESC;
    `

	// InsertMessageQuery remains unchanged.
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

	// Updated GetLastMessageQuery: now joins with social_pages and filters by client_id.
	// Here, parameter $1 is client_id and $2 is thread_id.
	GetLastMessageQuery = `
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
            COALESCE(c.bot_enabled, TRUE) AS bot_enabled,
            COALESCE(NULLIF(TRIM(c.profile_picture_url), ''), '/static/default-avatar.png') as profile_picture_url
        FROM messages m
        JOIN social_pages sp ON m.page_id = sp.id
        LEFT JOIN conversations c ON c.thread_id = m.thread_id
        WHERE sp.client_id = $1
          AND m.thread_id = $2
        ORDER BY m.timestamp DESC LIMIT 1`
)

// Chat related queries
const (
	// Updated GetChatQuery: now joins with social_pages and filters by client_id.
	// Expected parameters: $1 = client_id, $2 = thread_id.
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
            COALESCE(NULLIF(TRIM(c.profile_picture_url), ''), '/static/default-avatar.png') as profile_picture_url
        FROM messages m
        JOIN social_pages sp ON m.page_id = sp.id
        LEFT JOIN conversations c ON m.thread_id = c.thread_id
        WHERE sp.client_id = $1
          AND m.thread_id = $2
          AND (m.internal IS NULL OR m.internal = false)
        ORDER BY m.timestamp ASC`

	// Query to get thread details
	GetThreadDetailsQuery = `
        SELECT 
            m.client_id,
            m.page_id,
            m.platform,
            COALESCE(NULLIF(TRIM(c.profile_picture_url), ''), '/static/default-avatar.png') as profile_picture_url,
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

	// Updated GetThreadPreviewQuery: now joins with social_pages and filters by client_id.
	// Expected parameters: $1 = client_id, $2 = thread_id.
	GetThreadPreviewQuery = `
        WITH thread_owner AS (
            SELECT DISTINCT ON (m.thread_id)
                m.thread_id, 
                m.from_user as original_sender
            FROM messages m
            JOIN social_pages sp ON m.page_id = sp.id
            WHERE sp.client_id = $1
              AND m.thread_id = $2
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
            COALESCE(NULLIF(TRIM(c.profile_picture_url), ''), '/static/default-avatar.png') as profile_picture_url
        FROM messages m
        JOIN thread_owner t ON m.thread_id = t.thread_id
        LEFT JOIN conversations c ON m.thread_id = c.thread_id
        WHERE m.thread_id = $2
        ORDER BY m.timestamp DESC
        LIMIT 1`
)
