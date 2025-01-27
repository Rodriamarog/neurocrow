package handlers

import (
	"admin-dashboard/cache"
	"admin-dashboard/db"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.ParseFiles(
		"templates/layout.html",
		"templates/messages.html",
		"templates/components/message-list.html",
		"templates/components/chat-view.html",
	))
}

func GetMessages(w http.ResponseWriter, r *http.Request) {
	query := `
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
        ORDER BY lm.timestamp DESC;
    `
	messages, err := db.FetchMessages(query)
	if err != nil {
		db.HandleError(w, err, "Error fetching messages", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		tmpl.ExecuteTemplate(w, "message-list", map[string]interface{}{
			"Messages": messages,
		})
		return
	}

	data := map[string]interface{}{
		"Messages": messages,
	}

	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		if !strings.Contains(err.Error(), "write: broken pipe") {
			log.Printf("Error executing template: %v", err)
		}
	}
}

func GetChat(w http.ResponseWriter, r *http.Request) {
	threadID := r.URL.Query().Get("thread_id")
	log.Printf("GetChat called with thread_id: %s", threadID)

	query := `
        SELECT 
            m.id, m.client_id, m.page_id, m.platform, m.from_user,
            m.content, m.timestamp, m.thread_id, m.read, m.source,
            COALESCE(c.bot_enabled, true) as bot_enabled,
            c.profile_picture_url
        FROM messages m
        LEFT JOIN conversations c ON m.thread_id = c.thread_id
        WHERE m.thread_id = $1
          AND (m.internal IS NULL OR m.internal = false)
        ORDER BY m.timestamp ASC
    `
	messages, err := db.FetchMessages(query, threadID)
	if err != nil {
		db.HandleError(w, err, "Error fetching chat", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d messages for thread %s", len(messages), threadID)

	data := map[string]interface{}{
		"Messages": messages,
	}

	tmpl := template.Must(template.ParseFiles("templates/components/chat-view.html"))
	if err := tmpl.ExecuteTemplate(w, "chat-view", data); err != nil {
		db.HandleError(w, err, "Error rendering chat", http.StatusInternalServerError)
		return
	}
	log.Printf("Successfully rendered chat view for thread %s", threadID)
}

func GetMessageList(w http.ResponseWriter, r *http.Request) {
	// Add explicit debugging for request
	log.Printf("🔍 Request received at: %s", r.URL.Path)

	searchQuery := r.URL.Query().Get("search")
	log.Printf("🔍 Raw URL: %s", r.URL.String())
	log.Printf("🔍 Search query received: %q", searchQuery)

	var query string
	var args []interface{}

	if searchQuery != "" {
		query = `
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
            ORDER BY lm.timestamp DESC
        `
		args = []interface{}{fmt.Sprintf("%%%s%%", searchQuery)}
	} else {
		query = `
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
            ORDER BY lm.timestamp DESC
        `
	}

	messages, err := db.FetchMessages(query, args...)
	if err != nil {
		log.Printf("❌ Error executing query: %v", err)
		db.HandleError(w, err, "Error fetching messages", http.StatusInternalServerError)
		return
	}

	log.Printf("✨ Found %d messages matching search: %q", len(messages), searchQuery)

	tmpl := template.Must(template.ParseFiles("templates/components/message-list.html"))
	if err := tmpl.ExecuteTemplate(w, "message-list", map[string]interface{}{
		"Messages": messages,
	}); err != nil {
		db.HandleError(w, err, "Error rendering message list", http.StatusInternalServerError)
	}
}

func SendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		log.Printf("❌ Error parsing form: %v", err)
		db.HandleError(w, err, "Error parsing form", http.StatusBadRequest)
		return
	}

	threadID := r.FormValue("thread_id")
	content := strings.TrimSpace(r.FormValue("message"))

	if content == "" {
		log.Printf("❌ Attempted to send empty message to thread: %s", threadID)
		http.Error(w, "Empty message", http.StatusBadRequest)
		return
	}

	log.Printf("📤 Starting message send process for thread: %s", threadID)

	// Check cache first for thread details
	var threadPreview cache.ThreadPreview
	threadPreview, err = cache.GetThreadPreview(threadID, func(id string) (cache.ThreadPreview, error) {
		log.Printf("🔍 Cache miss for thread %s, fetching from database", id)
		return cache.ThreadPreview{}, fmt.Errorf("no cache") // Force DB fetch on first try
	})
	if err != nil {
		log.Printf("📁 Cache miss or error, fetching fresh data from DB")
	} else {
		log.Printf("📁 Found thread preview in cache: %+v", threadPreview)
	}

	// Get thread details from database
	var clientID, pageID, platform, profilePicURL sql.NullString
	var botEnabled bool
	err = db.DB.QueryRow(`
        SELECT 
            m.client_id,
            m.page_id,
            m.platform,
            c.profile_picture_url,
            COALESCE(c.bot_enabled, TRUE) as bot_enabled
        FROM messages m
        LEFT JOIN conversations c ON c.thread_id = m.thread_id
        WHERE m.thread_id = $1 
        ORDER BY m.timestamp DESC
        LIMIT 1
    `, threadID).Scan(&clientID, &pageID, &platform, &profilePicURL, &botEnabled)
	if err != nil {
		log.Printf("❌ Error fetching thread details: %v", err)
		db.HandleError(w, err, "Error sending message", http.StatusInternalServerError)
		return
	}

	log.Printf("🔍 Thread details found - ClientID: %v, PageID: %v, Platform: %v, BotEnabled: %v",
		clientID.String, pageID.String, platform.String, botEnabled)
	log.Printf("🖼️ Profile Picture URL from DB: %v (Valid: %v)",
		profilePicURL.String, profilePicURL.Valid)

	// Insert the new message
	result, err := db.DB.Exec(db.InsertMessageQuery,
		clientID.String,
		pageID.String,
		platform.String,
		content,
		threadID,
	)

	if err != nil {
		log.Printf("❌ Error storing admin message: %v", err)
		db.HandleError(w, err, "Error sending message", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("⚠️ Could not determine if message was stored: %v", err)
	} else {
		log.Printf("✅ Message stored successfully. Rows affected: %d", rowsAffected)
	}

	// Invalidate cache
	if err := cache.InvalidateThreadCache(threadID); err != nil {
		log.Printf("⚠️ Failed to invalidate thread cache: %v", err)
	} else {
		log.Printf("🔄 Thread cache invalidated successfully for thread: %s", threadID)
	}

	// First, add the new message to the messages container
	messageTmpl := template.Must(template.New("message-response").Parse(`
        <div class="flex items-start max-w-[85%] justify-end ml-auto space-x-2">
            <div class="bg-indigo-600 text-white rounded-lg px-4 py-2 order-1">
                <p class="text-sm">{{.Content}}</p>
            </div>
            <div class="h-8 w-8 rounded-full bg-gray-200 flex-shrink-0 overflow-hidden order-2">
                <img src="{{if ne .ProfilePicURL ""}}{{.ProfilePicURL}}{{else}}/static/default-avatar.png{{end}}"
                     alt=""
                     class="h-full w-full object-cover">
            </div>
        </div>
    `))

	// Execute the message template
	err = messageTmpl.Execute(w, struct {
		ThreadID      string
		Content       string
		ProfilePicURL string
	}{
		ThreadID:      threadID,
		Content:       content,
		ProfilePicURL: profilePicURL.String,
	})
	if err != nil {
		log.Printf("❌ Error rendering message: %v", err)
		db.HandleError(w, err, "Error rendering message", http.StatusInternalServerError)
		return
	}

	// Then, update the thread preview with out-of-band swap
	previewTmpl := template.Must(template.New("preview-response").Parse(`
        <div id="thread-preview-{{.ThreadID}}"
             class="p-4 hover:bg-gray-50 active:bg-gray-100 cursor-pointer"
             hx-get="/chat?thread_id={{.ThreadID}}"
             hx-target="#chat-view"
             hx-trigger="click"
             _="on htmx:afterOnLoad remove .hidden from #chat-view then remove .translate-x-full from #chat-view"
             hx-swap-oob="true">
            <div class="flex items-center justify-between mb-1">
                <div class="flex items-center">
                    <div class="w-2 h-2 {{if eq .Platform "facebook"}}bg-blue-500{{else}}bg-pink-500{{end}} rounded-full mr-2"></div>
                    <span class="text-sm font-medium {{if eq .Platform "facebook"}}text-blue-600{{else}}text-pink-600{{end}}">
                        {{if eq .Platform "facebook"}}Facebook{{else}}Instagram{{end}}
                    </span>
                </div>
                <div class="flex items-center space-x-3">
                    <button
                        class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-indigo-600 focus:ring-offset-2 {{if .BotEnabled}}bg-indigo-600{{else}}bg-gray-200{{end}}"
                        role="switch"
                        hx-post="/toggle-bot"
                        hx-vals='{"thread_id": "{{.ThreadID}}", "enabled": "{{not .BotEnabled}}"}'
                        hx-swap="none"
                        _="on click
                            halt the event
                            preventDefault(event)
                            stopPropagation(event)
                            toggle .bg-indigo-600 .bg-gray-200 on me
                            toggle .translate-x-5 .translate-x-0 on #toggle-button-{{.ThreadID}}"
                        aria-checked="{{.BotEnabled}}">
                        <span class="sr-only">Toggle bot</span>
                        <span
                            id="toggle-button-{{.ThreadID}}"
                            aria-hidden="true"
                            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out {{if .BotEnabled}}translate-x-5{{else}}translate-x-0{{end}}">
                        </span>
                    </button>
                    <span class="text-xs text-gray-500">{{.Timestamp.Format "15:04"}}</span>
                </div>
            </div>
            <div class="flex items-center">
                <div class="h-12 w-12 rounded-full bg-gray-200 flex-shrink-0 overflow-hidden">
                    <img src="{{if ne .ProfilePicURL ""}}{{.ProfilePicURL}}{{else}}/static/default-avatar.png{{end}}"
                         alt=""
                         class="h-full w-full object-cover">
                </div>
                <div class="ml-3 flex-1">
                    <div class="text-sm font-medium text-gray-900">{{.FromUser}}</div>
                    <div class="text-sm text-gray-500 truncate">{{.Content}}</div>
                </div>
            </div>
        </div>
    `))

	// Execute the preview template
	now := time.Now()
	err = previewTmpl.Execute(w, struct {
		ThreadID      string
		Content       string
		ProfilePicURL string
		Platform      string
		FromUser      string
		Timestamp     time.Time
		BotEnabled    bool
	}{
		ThreadID:      threadID,
		Content:       content,
		ProfilePicURL: profilePicURL.String,
		Platform:      platform.String,
		FromUser:      "Admin", // You might want to get this from somewhere else
		Timestamp:     now,
		BotEnabled:    botEnabled,
	})
	if err != nil {
		log.Printf("❌ Error rendering preview: %v", err)
		// Don't return error here as the message was already sent successfully
	}

	log.Printf("✅ Message response rendered successfully with profile picture: %v", profilePicURL.String)

	// Update cache with new thread preview
	newPreview := cache.ThreadPreview{
		ID:                "", // You might want to get this from the DB
		ThreadID:          threadID,
		FromUser:          "Admin",
		Content:           content,
		Timestamp:         now,
		Platform:          platform.String,
		BotEnabled:        botEnabled,
		ProfilePictureURL: profilePicURL.String,
	}

	if err := cache.CacheThreadPreview(threadID, newPreview); err != nil {
		log.Printf("⚠️ Failed to update thread preview in cache: %v", err)
	} else {
		log.Printf("✅ Thread preview updated in cache successfully")
	}
}

func GetThreadPreview(w http.ResponseWriter, r *http.Request) {
	threadID := r.URL.Query().Get("thread_id")

	query := `
        WITH thread_owner AS (
            SELECT DISTINCT ON (m.thread_id)
                m.thread_id,
                m.from_user AS original_sender
            FROM messages m
            WHERE m.thread_id = $1
            ORDER BY m.thread_id, m.timestamp ASC
        )
        SELECT
            m.id, m.client_id, m.page_id, m.platform,
            t.original_sender AS from_user,
            m.content, m.timestamp, m.thread_id, m.read,
            m.source,
            COALESCE(c.bot_enabled, TRUE) AS bot_enabled,
            c.profile_picture_url
        FROM messages m
        JOIN thread_owner t ON m.thread_id = t.thread_id
        LEFT JOIN conversations c ON m.thread_id = c.thread_id
        WHERE m.thread_id = $1
        ORDER BY m.timestamp DESC
        LIMIT 1
    `
	messages, err := db.FetchMessages(query, threadID)
	if err != nil {
		db.HandleError(w, err, "Error fetching thread preview", http.StatusInternalServerError)
		return
	}

	if len(messages) == 0 {
		http.Error(w, "No messages found", http.StatusNotFound)
		return
	}

	msg := messages[0]

	tmpl := template.Must(template.New("thread-preview").Parse(`
    <div class="p-4 hover:bg-gray-50 active:bg-gray-100 cursor-pointer"
         id="thread-preview-{{.ThreadID}}"
         hx-get="/chat?thread_id={{.ThreadID}}"
         hx-target="#chat-view"
         hx-trigger="click"
         _="on htmx:afterOnLoad remove .hidden from #chat-view then remove .translate-x-full from #chat-view">
        <div class="flex items-center justify-between mb-1">
            <div class="flex items-center">
                <div class="w-2 h-2 {{if eq .Platform "facebook"}}bg-blue-500{{else}}bg-pink-500{{end}} rounded-full mr-2"></div>
                <span class="text-sm font-medium {{if eq .Platform "facebook"}}text-blue-600{{else}}text-pink-600{{end}}">
                    {{if eq .Platform "facebook"}}Facebook{{else}}Instagram{{end}}
                </span>
            </div>
            <span class="text-xs text-gray-500">{{.Timestamp.Format "15:04"}}</span>
        </div>
        <div class="flex items-center">
            <div class="h-12 w-12 rounded-full bg-gray-200"></div>
            <div class="ml-3 flex-1">
                <div class="text-sm font-medium text-gray-900">{{.FromUser}}</div>
                <div class="text-sm text-gray-500 truncate">{{.Content}}</div>
            </div>
        </div>
    </div>
    `))

	tmpl.Execute(w, msg)
}

func ToggleBotStatus(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔄 Received toggle request")
	if r.Method != http.MethodPost {
		log.Printf("❌ Wrong method for toggle bot status, got: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	threadID := r.FormValue("thread_id")
	enabled := r.FormValue("enabled") == "true"
	log.Printf("🔄 Toggling bot status -> threadID: %s, enabled: %v", threadID, enabled)

	err := db.UpdateBotStatus(threadID, enabled)
	if err != nil {
		log.Printf("❌ Error toggling bot status, threadID: %s, %v", threadID, err)
		http.Error(w, "Failed to update bot status", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Successfully toggled bot status for thread: %s", threadID)
	w.WriteHeader(http.StatusOK)
}
