package handlers

import (
	"admin-dashboard/db"
	"admin-dashboard/models"
	"admin-dashboard/pkg/meta"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
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
            CASE 
                WHEN lm.source IN ('bot', 'admin', 'system') THEN '/static/default-avatar.png'
                WHEN c.profile_picture_url IS NULL THEN '/static/default-avatar.png'
                WHEN c.profile_picture_url = '' THEN '/static/default-avatar.png'
                ELSE c.profile_picture_url
            END as profile_picture_url
        FROM latest_messages lm
        LEFT JOIN conversations c ON c.thread_id = lm.thread_id
        WHERE 
            CASE 
                WHEN $1 != '' THEN 
                    lm.content ILIKE '%' || $1 || '%' 
                    OR lm.thread_owner ILIKE '%' || $1 || '%'
                ELSE TRUE
            END
        ORDER BY lm.timestamp DESC;
    `

	if searchQuery != "" {
		messages, err = db.FetchMessages(query, fmt.Sprintf("%%%s%%", searchQuery))
	} else {
		messages, err = db.FetchMessages(query)
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

func sendToMessageRouter(pageID, threadID, platform, message string) error {
	// Use the Render deployment URL
	messageRouterURL := "https://neurocrow-message-router.onrender.com"

	payload := map[string]interface{}{
		"page_id":      pageID,
		"recipient_id": threadID,
		"platform":     platform,
		"message":      message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error creating payload: %v", err)
	}

	// Create request with context for timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Longer timeout for Render's cold starts
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", messageRouterURL+"/send-message", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Use custom transport with longer timeouts
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	log.Printf("📤 Sending message to router: %s", messageRouterURL)
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("request timed out after 30 seconds")
		}
		return fmt.Errorf("error sending message: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	// Log response for debugging
	log.Printf("📥 Message router response (status %d): %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("message router error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// Update the SendMessage function to include the message routing
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

	// Get thread details from database
	var clientID, pageID, platform sql.NullString
	var botEnabled bool
	err = db.DB.QueryRow(`
        SELECT 
            m.client_id,
            m.page_id,
            m.platform,
            COALESCE(c.bot_enabled, TRUE) as bot_enabled
        FROM messages m
        LEFT JOIN conversations c ON c.thread_id = m.thread_id
        WHERE m.thread_id = $1 
        ORDER BY m.timestamp DESC
        LIMIT 1
    `, threadID).Scan(&clientID, &pageID, &platform, &botEnabled)
	if err != nil {
		log.Printf("❌ Error fetching thread details: %v", err)
		db.HandleError(w, err, "Error sending message", http.StatusInternalServerError)
		return
	}

	// Send message through the message router
	if pageID.Valid && platform.Valid {
		err = sendToMessageRouter(pageID.String, threadID, platform.String, content)
		if err != nil {
			log.Printf("❌ Error sending message through router: %v", err)
			db.HandleError(w, err, "Error sending message", http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Message sent through message router successfully")
	}

	// Store the message in the database
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

	// Set headers for HTMX to trigger a refresh of the chat view
	w.Header().Set("HX-Trigger", "refreshChat")
	w.Header().Set("HX-Trigger-After-Settle", "{\"refreshMessageList\": true}")
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
