// db/queries.go
package db

// For the GetThreadPreview function in /handlers/messages.go
const ThreadPreviewQuery = `
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
    c.profile_picture_url
FROM messages m
JOIN thread_owner t ON m.thread_id = t.thread_id
LEFT JOIN conversations c ON m.thread_id = c.thread_id
WHERE m.thread_id = $1
ORDER BY m.timestamp DESC
LIMIT 1`

// For the GetMessagesList function in /handlers/messages.go
const (
	MessageListSearchQuery = `
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
            c.profile_picture_url
        FROM latest_messages lm
        LEFT JOIN conversations c ON c.thread_id = lm.thread_id
        WHERE 
            lm.content ILIKE $1 OR 
            lm.thread_owner ILIKE $1
        ORDER BY lm.timestamp DESC`
)

// These shitters are for the /handlers/messages.go file SendMessages function
const (
	// For the message list view
	MessageListBaseQuery = `
        WITH thread_owner AS (
            SELECT DISTINCT ON (m.thread_id)
                m.thread_id, 
                m.from_user as original_sender,
                c.profile_picture_url
            FROM messages m
            LEFT JOIN conversations c ON c.thread_id = m.thread_id
            ORDER BY m.thread_id, m.timestamp ASC
        ),
        latest_messages AS (
            SELECT DISTINCT ON (m.thread_id)
                m.id, 
                m.client_id, 
                m.page_id, 
                m.platform,
                t.original_sender as thread_owner,
                t.profile_picture_url,
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
            COALESCE(c.profile_picture_url, lm.profile_picture_url) as profile_picture_url
        FROM latest_messages lm
        LEFT JOIN conversations c ON c.thread_id = lm.thread_id
        ORDER BY lm.timestamp DESC
    `

	// For getting thread details before sending a message
	GetThreadDetailsQuery = `
    WITH thread_owner AS (
        SELECT DISTINCT ON (m.thread_id)
            m.thread_id, 
            m.from_user as original_sender,
            c.profile_picture_url
        FROM messages m
        LEFT JOIN conversations c ON c.thread_id = m.thread_id
        WHERE m.thread_id = $1
        ORDER BY m.thread_id, m.timestamp ASC
    )
    SELECT 
        m.client_id,
        m.page_id,
        m.platform,
        COALESCE(c.profile_picture_url, t.profile_picture_url) as profile_picture_url
    FROM messages m
    JOIN thread_owner t ON m.thread_id = t.thread_id
    LEFT JOIN conversations c ON c.thread_id = m.thread_id
    WHERE m.thread_id = $1 
    ORDER BY m.timestamp DESC
    LIMIT 1
`

	// For inserting a new message, using NULLIF to handle empty strings as NULL
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
        RETURNING id
    `
)

// Function GetMessages in /handlers/messages.go
const MessagesBaseQuery = `
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
    c.profile_picture_url
FROM latest_messages lm
LEFT JOIN conversations c ON c.thread_id = lm.thread_id
ORDER BY lm.timestamp DESC`

// For the SendMessage function in /handlers/messages.go
const SendMessageQuery = `
INSERT INTO messages (
    client_id,
    page_id,
    platform,
    from_user,
    source,
    content,
    thread_id,
    read
) SELECT 
    client_id,
    page_id,
    platform,
    'admin',
    'human',
    $1,
    $2,
    true
FROM messages 
WHERE thread_id = $2 
LIMIT 1
RETURNING id`

// For the GetChat function in /handlers/messages.go
const GetChatQuery = `
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
    c.profile_picture_url
FROM messages m
LEFT JOIN conversations c ON m.thread_id = c.thread_id
WHERE m.thread_id = $1
    AND (m.internal IS NULL OR m.internal = false)
ORDER BY m.timestamp ASC`
