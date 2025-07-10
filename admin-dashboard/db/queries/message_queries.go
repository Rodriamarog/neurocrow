package queries

const (
	// Base query parts
	messageSelectBase = `
        SELECT 
            m.id, m.client_id, m.page_id, m.platform,
            m.from_user, m.content, m.timestamp, m.thread_id,
            m.read, m.source,
            COALESCE(c.bot_enabled, TRUE) AS bot_enabled,
            COALESCE(NULLIF(TRIM(c.profile_picture_url), ''), '/static/default-avatar.png') as profile_picture_url,
            c.social_user_name
    `

	messageJoinBase = `
        FROM messages m
        JOIN social_pages sp ON m.page_id = sp.id
        LEFT JOIN conversations c ON m.thread_id = c.thread_id
    `
)

// QueryBuilder for constructing SQL queries
type QueryBuilder struct {
	base    string
	joins   []string
	where   []string
	orderBy string
	limit   string
	params  []interface{}
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		base:  messageSelectBase,
		joins: []string{messageJoinBase},
	}
}
