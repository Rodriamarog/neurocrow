package handlers

import (
	"admin-dashboard/db"
	"admin-dashboard/models"
	"admin-dashboard/pkg/auth"
	"admin-dashboard/pkg/meta"
	"admin-dashboard/pkg/template"
	"admin-dashboard/pkg/views"
	"admin-dashboard/services"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Services struct {
	Messages *services.MessageService
	// Add other services as needed
}

// Updated GetMessages with extensive logging for debugging UUID issues
func GetMessages(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		log.Printf("❌ Wrong path requested: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		log.Printf("❌ No user found in context")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	log.Printf("🔍 GetMessages called for user:")
	log.Printf("  - UserID: %s", user.ID)
	log.Printf("  - ClientID: %s", user.ClientID)
	log.Printf("  - Role: %s", user.Role)

	// Log Supabase configuration
	supabaseURL := r.Context().Value("supabase_url")
	supabaseAPIKey := r.Context().Value("supabase_api_key")
	log.Printf("🔍 Supabase config in handler: URL=%v, API Key exists=%v",
		supabaseURL, supabaseAPIKey != nil)

	messages, err := db.FetchMessages(user.ClientID, db.GetMessagesQuery)
	if err != nil {
		log.Printf("❌ Error in GetMessages: %v", err)
		db.HandleError(w, err, "Error fetching messages", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Successfully fetched messages for user %s", user.ID)
	log.Printf("  - Number of messages: %d", len(messages))

	// Include the user in template data.
	if r.Header.Get("HX-Request") == "true" {
		if err := template.RenderTemplate(w, "message-list", map[string]interface{}{
			"Messages": messages,
			"User":     user,
		}); err != nil {
			db.HandleError(w, err, "Error rendering message list", http.StatusInternalServerError)
		}
		return
	}

	data := map[string]interface{}{
		"Messages": messages,
		"User":     user,
	}

	if err := template.RenderTemplate(w, "layout.html", data); err != nil {
		if !strings.Contains(err.Error(), "write: broken pipe") {
			log.Printf("Error executing template: %v", err)
		}
	}
}

func GetChat(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	threadID := r.URL.Query().Get("thread_id")
	if threadID == "" {
		http.Error(w, "Thread ID is required", http.StatusBadRequest)
		return
	}

	const messagesPerPage = 30
	page := r.URL.Query().Get("page")
	offset := 0
	if page != "" {
		// Add pagination logic here
	}

	messages, err := db.FetchMessages(
		user.ClientID,
		db.GetChatQuery,
		threadID,
		messagesPerPage,
		offset,
	)

	if err != nil {
		log.Printf("❌ Error fetching chat messages: %v", err)
		db.HandleError(w, err, "Error fetching chat", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Messages": messages,
		"User":     user,
	}

	if err := template.RenderTemplate(w, "chat-view", data); err != nil {
		db.HandleError(w, err, "Error rendering chat", http.StatusInternalServerError)
	}
}

func GetMessageList(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔍 Request received at: %s", r.URL.Path)

	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		log.Printf("❌ No user found in context")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	searchQuery := r.URL.Query().Get("search")
	log.Printf("🔍 Raw URL: %s", r.URL.String())
	log.Printf("🔍 Search query received: %q", searchQuery)

	// Pass both the client ID and search query
	messages, err := db.FetchMessages(user.ClientID, db.GetMessageListSearchQuery, searchQuery)
	if err != nil {
		log.Printf("❌ Error executing query: %v", err)
		db.HandleError(w, err, "Error fetching messages", http.StatusInternalServerError)
		return
	}

	log.Printf("✨ Found %d messages matching search: %q", len(messages), searchQuery)

	if err := template.RenderTemplate(w, "message-list", map[string]interface{}{
		"Messages": messages,
	}); err != nil {
		db.HandleError(w, err, "Error rendering message list", http.StatusInternalServerError)
	}
}

func sendToMessageRouter(pageID, threadID, platform, message string) error {
	// Use the Render deployment URL
	messageRouterURL := "https://neurocrow-message-router.onrender.com"

	// Updated payload with additional fields
	payload := map[string]interface{}{
		"page_id":        pageID,
		"recipient_id":   threadID,
		"platform":       platform,
		"message":        message,
		"messaging_type": "MESSAGE_TAG",
		"tag":            "HUMAN_AGENT",
		"source":         "human", // This helps the message router distinguish human vs bot messages
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
// Updated SendMessage signature: change *Request to *http.Request
func SendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Retrieve the authenticated user from context.
	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
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

	// Get thread details from database with the actual Facebook/Instagram page_id
	var clientID sql.NullString
	var pageUUID, metaPageID, platform sql.NullString
	var botEnabled bool
	err = db.DB.QueryRow(`
        SELECT 
            m.client_id,
            m.page_id,
            sp.page_id as meta_page_id,  -- Get the actual Facebook/Instagram page ID
            m.platform,
            COALESCE(c.bot_enabled, TRUE) as bot_enabled
        FROM messages m
        LEFT JOIN conversations c ON c.thread_id = m.thread_id
        LEFT JOIN social_pages sp ON sp.id = m.page_id  -- Join with social_pages to get meta_page_id
        WHERE m.thread_id = $1 
        ORDER BY m.timestamp DESC
        LIMIT 1
    `, threadID).Scan(&clientID, &pageUUID, &metaPageID, &platform, &botEnabled)
	if err != nil {
		log.Printf("❌ Error fetching thread details: %v", err)
		db.HandleError(w, err, "Error sending message", http.StatusInternalServerError)
		return
	}

	log.Printf("📤 Retrieved details - Platform: %v, Meta Page ID: %v",
		platform.String, metaPageID.String)

	// Send message through the message router using the Meta page ID
	if metaPageID.Valid && platform.Valid {
		err = sendToMessageRouter(metaPageID.String, threadID, platform.String, content)
		if err != nil {
			log.Printf("❌ Error sending message through router: %v", err)
			db.HandleError(w, err, "Error sending message", http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Message sent through message router successfully")
	}

	// Store the message in the database using our internal page UUID
	clientIDStr := ""
	if clientID.Valid {
		clientIDStr = clientID.String
	}

	pageIDStr := ""
	if pageUUID.Valid {
		pageIDStr = pageUUID.String
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

	// --- New refresh logic ---
	newMsgs, err := db.FetchMessages(db.GetLastMessageQuery, threadID)
	if err != nil || len(newMsgs) == 0 {
		log.Printf("❌ Error fetching new message: %v", err)
		db.HandleError(w, err, "Error sending message", http.StatusInternalServerError)
		return
	}

	// Right before template execution, add:
	log.Printf("🔄 Rendering single message bubble for thread: %s", threadID)

	// Return only the new message rendered with the message-bubble template
	w.WriteHeader(http.StatusOK)
	if err := template.RenderTemplate(w, "message-bubble.html", newMsgs[0]); err != nil {
		db.HandleError(w, err, "Error rendering new message", http.StatusInternalServerError)
		return
	}
	// --- End of refresh logic ---
}

func GetThreadPreview(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔍 GetThreadPreview called for thread_id: %s", r.URL.Query().Get("thread_id"))
	threadID := r.URL.Query().Get("thread_id")

	// Retrieve the authenticated user from context.
	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Check if this is a test thread ID from the realtime.js
	if strings.HasPrefix(threadID, "test-") {
		// Return a test thread preview for test IDs
		log.Printf("🧪 Creating test thread preview for: %s", threadID)
		testMessage := models.Message{
			ID:        threadID,
			ThreadID:  threadID,
			Content:   "Test message thread",
			Source:    "user",
			Timestamp: time.Now(),
		}

		if err := template.RenderTemplate(w, "thread-preview", testMessage); err != nil {
			db.HandleError(w, err, "Error rendering test thread preview", http.StatusInternalServerError)
		}
		return
	}

	messages, err := db.FetchMessages(user.ClientID, db.GetThreadPreviewQuery, threadID)
	if err != nil {
		db.HandleError(w, err, "Error fetching thread preview", http.StatusInternalServerError)
		return
	}

	if len(messages) == 0 {
		log.Printf("❌ No messages found for thread_id: %s", threadID)
		http.Error(w, "No messages found", http.StatusNotFound)
		return
	}

	log.Printf("✅ Rendering thread preview for thread_id: %s", threadID)
	if err := template.RenderTemplate(w, "thread-preview", messages[0]); err != nil {
		db.HandleError(w, err, "Error rendering thread preview", http.StatusInternalServerError)
	}
}

func ToggleBotStatus(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	log.Printf("🔄 Received toggle request")
	if r.Method != http.MethodPost {
		log.Printf("❌ Wrong method for toggle bot status, got: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	threadID := r.FormValue("thread_id")
	enabled := r.FormValue("enabled") == "true"
	log.Printf("🔄 Toggling bot status -> threadID: %s, enabled: %v", threadID, enabled)

	// Updated call: removed the user parameter
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
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	threadID := r.URL.Query().Get("thread_id")
	log.Printf("⚠️ Full chat refresh requested for thread: %s", threadID)

	opts := services.MessageOptions{
		Limit: 50,
		Order: "DESC",
	}

	svc := r.Context().Value("services").(*Services)
	messages, err := svc.Messages.GetThreadMessages(r.Context(), threadID, opts)
	if err != nil {
		db.HandleError(w, err, "Error fetching chat messages", http.StatusInternalServerError)
		return
	}

	view := &views.MessageListView{
		Messages: views.ToMessageViews(messages),
		User:     views.ToUserView(user),
	}

	if err := template.RenderTemplate(w, "chat-messages", view); err != nil {
		db.HandleError(w, err, "Error rendering chat messages", http.StatusInternalServerError)
	}
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

// GetMessageBubble returns HTML for a single message bubble
func GetMessageBubble(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	messageID := r.URL.Query().Get("id")
	if messageID == "" {
		http.Error(w, "Message ID is required", http.StatusBadRequest)
		return
	}

	message, err := db.FetchSingleMessage(messageID, user.ClientID)
	if err != nil {
		log.Printf("❌ Error fetching message: %v", err)
		db.HandleError(w, err, "Error fetching message", http.StatusInternalServerError)
		return
	}

	if err := template.RenderTemplate(w, "message-bubble.html", message); err != nil {
		db.HandleError(w, err, "Error rendering message bubble", http.StatusInternalServerError)
	}
}
