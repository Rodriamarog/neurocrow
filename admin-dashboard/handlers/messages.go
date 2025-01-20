package handlers

import (
	"admin-dashboard/db"
	"html/template"
	"log"
	"net/http"
	"strings"
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

	data := map[string]interface{}{
		"Messages": messages,
	}

	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		if !strings.Contains(err.Error(), "write: broken pipe") {
			log.Printf("Error executing template: %v", err)
		}
		return
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

	log.Printf("📤 Sending admin message to thread %s: %q", threadID, content)

	result, err := db.DB.Exec(`
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
        RETURNING id
    `, content, threadID)

	if err != nil {
		log.Printf("❌ Error storing admin message: %v", err)
		db.HandleError(w, err, "Error sending message", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("⚠️ Could not determine if message was stored: %v", err)
	} else if rowsAffected == 0 {
		log.Printf("❌ No message stored - thread %s might not exist", threadID)
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}

	log.Printf("✅ Successfully stored admin message in thread %s", threadID)

	// HTMX response template
	tmpl := template.Must(template.New("message-response").Parse(`
        <div class="flex items-start max-w-[85%] justify-end ml-auto"
             hx-get="/thread-preview?thread_id={{.ThreadID}}"
             hx-target="#thread-preview-{{.ThreadID}}"
             hx-trigger="load"
             hx-swap="outerHTML">
            <div class="bg-indigo-600 text-white rounded-lg px-4 py-2">
                <p class="text-sm">{{.Content}}</p>
            </div>
        </div>
    `))

	if err := tmpl.Execute(w, struct {
		ThreadID string
		Content  string
	}{
		ThreadID: threadID,
		Content:  content,
	}); err != nil {
		log.Printf("❌ Error rendering message response: %v", err)
		db.HandleError(w, err, "Error rendering message", http.StatusInternalServerError)
		return
	}
}

func GetMessageList(w http.ResponseWriter, r *http.Request) {
	query := `
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
	messages, err := db.FetchMessages(query)
	if err != nil {
		db.HandleError(w, err, "Error fetching messages", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		tmpl := template.Must(template.ParseFiles("templates/components/message-list.html"))
		tmpl.ExecuteTemplate(w, "message-list", map[string]interface{}{
			"Messages": messages,
		})
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/components/message-list.html"))
	tmpl.ExecuteTemplate(w, "message-list", map[string]interface{}{
		"Messages": messages,
	})
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

// ToggleBotStatus handles enabling/disabling the AI bot for a specific thread
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
