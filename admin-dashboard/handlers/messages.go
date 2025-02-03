package handlers

import (
	"admin-dashboard/cache"
	"admin-dashboard/db"
	"admin-dashboard/models"
	"admin-dashboard/pkg/meta"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

var tmpl *template.Template

func init() {
	// Create a new template with NO name first
	t := template.New("")

	// Read message-bubble.html content first
	messageBubbleContent, err := os.ReadFile("templates/components/message-bubble.html")
	if err != nil {
		log.Fatalf("Could not read message-bubble.html: %v", err)
	}

	// Parse it first to make sure it's available
	t, err = t.Parse(string(messageBubbleContent))
	if err != nil {
		log.Fatalf("Could not parse message-bubble.html: %v", err)
	}

	// Then parse everything else
	tmpl = template.Must(t.ParseFiles(
		"templates/layout.html",
		"templates/messages.html",
		"templates/components/chat-view.html",
		"templates/components/message-list.html",
		"templates/components/thread-preview.html",
		"templates/components/chat-messages.html", // Add this line
	))

	// Print ALL defined templates for debugging
	log.Printf("✅ Available templates: %v", tmpl.DefinedTemplates())
}

func GetMessages(w http.ResponseWriter, r *http.Request) {
	messages, err := db.FetchMessages(db.GetMessagesQuery)
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

	messages, err := db.FetchMessages(db.GetChatQuery, threadID)
	if err != nil {
		db.HandleError(w, err, "Error fetching chat", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d messages for thread %s", len(messages), threadID)

	data := map[string]interface{}{
		"Messages": messages,
	}

	// Use the global tmpl variable instead of creating a new one
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

	var messages []models.Message
	var err error

	if searchQuery != "" {
		messages, err = db.FetchMessages(db.GetMessageListSearchQuery, fmt.Sprintf("%%%s%%", searchQuery))
	} else {
		messages, err = db.FetchMessages(db.GetMessagesQuery)
	}

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

	// Get the values, handling NULL cases
	clientIDStr := ""
	if clientID.Valid {
		clientIDStr = clientID.String
	}

	pageIDStr := ""
	if pageID.Valid {
		pageIDStr = pageID.String
	}

	platformStr := ""
	if platform.Valid {
		platformStr = platform.String
	}

	// Insert the message using the query from queries.go
	result, err := db.DB.Exec(db.InsertMessageQuery,
		clientIDStr,
		pageIDStr,
		platformStr,
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

	// Set headers for HTMX to trigger a refresh of the chat view
	w.Header().Set("HX-Trigger", "refreshChat")

	// Also trigger a refresh of the message list to update the preview
	w.Header().Set("HX-Trigger-After-Settle", "{\"refreshMessageList\": true}")

	// Send a 200 OK status
	w.WriteHeader(http.StatusOK)
}

func GetThreadPreview(w http.ResponseWriter, r *http.Request) {
	threadID := r.URL.Query().Get("thread_id")

	messages, err := db.FetchMessages(db.GetThreadPreviewQuery, threadID)
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

func GetChatMessages(w http.ResponseWriter, r *http.Request) {
	threadID := r.URL.Query().Get("thread_id")
	log.Printf("GetChatMessages called with thread_id: %s", threadID)

	messages, err := db.FetchMessages(db.GetChatQuery, threadID)
	if err != nil {
		db.HandleError(w, err, "Error fetching chat messages", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d messages for thread %s", len(messages), threadID)

	data := map[string]interface{}{
		"Messages": messages,
	}

	if err := tmpl.ExecuteTemplate(w, "chat-messages", data); err != nil {
		db.HandleError(w, err, "Error rendering chat messages", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully rendered chat messages for thread %s", threadID)
}

func RefreshProfilePictures(w http.ResponseWriter, r *http.Request) {
	// Allow only POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Fetch unique thread IDs that aren't test threads
	rows, err := db.DB.Query(`
        SELECT DISTINCT thread_id
        FROM messages
        WHERE thread_id NOT LIKE 'thread_%'
    `)
	if err != nil {
		db.HandleError(w, err, "Error fetching threads", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var threadIDs []string
	for rows.Next() {
		var threadID string
		if err := rows.Scan(&threadID); err != nil {
			continue
		}
		threadIDs = append(threadIDs, threadID)
	}

	// For each thread, refresh the profile picture
	for _, id := range threadIDs {
		err := meta.RefreshProfilePicture(db.DB, id)
		if err != nil {
			log.Printf("Failed to refresh profile picture for thread %s: %v", id, err)
		}
	}

	w.WriteHeader(http.StatusOK)
}
