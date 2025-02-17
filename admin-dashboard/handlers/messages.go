// handlers/messages.go
package handlers

import (
	"admin-dashboard/db"
	"admin-dashboard/pkg/auth"
	"admin-dashboard/pkg/template"
	"admin-dashboard/ws"
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

	messages, err := db.FetchMessages(user.ClientID, db.GetMessagesQuery)
	if err != nil {
		log.Printf("❌ Error in GetMessages: %v", err)
		db.HandleError(w, err, "Error fetching messages", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Successfully fetched messages for user %s", user.ID)
	log.Printf("  - Number of messages: %d", len(messages))

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
	log.Printf("GetChat called with thread_id: %s", threadID)

	messages, err := db.FetchMessages(user.ClientID, db.GetChatQuery, threadID)
	if err != nil {
		db.HandleError(w, err, "Error fetching chat", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d messages for thread %s", len(messages), threadID)

	data := map[string]interface{}{
		"Messages": messages,
		"User":     user,
	}

	if err := template.RenderTemplate(w, "chat-view", data); err != nil {
		db.HandleError(w, err, "Error rendering chat", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully rendered chat view for thread %s", threadID)
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
	messageRouterURL := "https://neurocrow-message-router.onrender.com"

	payload := map[string]interface{}{
		"page_id":        pageID,
		"recipient_id":   threadID,
		"platform":       platform,
		"message":        message,
		"messaging_type": "MESSAGE_TAG",
		"tag":            "HUMAN_AGENT",
		"source":         "human",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error creating payload: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", messageRouterURL+"/send-message", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	log.Printf("📥 Message router response (status %d): %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("message router error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func SendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	var clientID sql.NullString
	var pageUUID, metaPageID, platform sql.NullString
	var botEnabled bool
	err = db.DB.QueryRow(`
        SELECT 
            m.client_id,
            m.page_id,
            sp.page_id as meta_page_id,
            m.platform,
            COALESCE(c.bot_enabled, TRUE) as bot_enabled
        FROM messages m
        LEFT JOIN conversations c ON c.thread_id = m.thread_id
        LEFT JOIN social_pages sp ON sp.id = m.page_id
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

	if metaPageID.Valid && platform.Valid {
		err = sendToMessageRouter(metaPageID.String, threadID, platform.String, content)
		if err != nil {
			log.Printf("❌ Error sending message through router: %v", err)
			db.HandleError(w, err, "Error sending message", http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Message sent through message router successfully")
	}

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

	messages, err := db.FetchMessages(user.ClientID, db.GetLastMessageQuery, threadID)
	if err != nil {
		log.Printf("❌ Error fetching new message: %v", err)
		db.HandleError(w, err, "Error sending message", http.StatusInternalServerError)
		return
	}

	if len(messages) == 0 {
		log.Printf("❌ No messages found after insertion")
		http.Error(w, "Message not found after sending", http.StatusInternalServerError)
		return
	}

	// Broadcast the new message via WebSocket
	if clientID.Valid {
		log.Printf("📢 Broadcasting new message to WebSocket clients")
		ws.SendNewMessage(clientID.String, threadID, messages[0])
		log.Printf("✅ Message broadcast complete")
	}

	w.WriteHeader(http.StatusOK)
	if err := template.RenderTemplate(w, "message-bubble.html", messages[0]); err != nil {
		db.HandleError(w, err, "Error rendering new message", http.StatusInternalServerError)
		return
	}
}

func GetThreadPreview(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔍 GetThreadPreview called for thread_id: %s", r.URL.Query().Get("thread_id"))
	threadID := r.URL.Query().Get("thread_id")

	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
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
		return
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

	if err := db.UpdateBotStatus(threadID, enabled); err != nil {
		log.Printf("❌ Error toggling bot status, threadID: %s, %v", threadID, err)
		http.Error(w, "Failed to update bot status", http.StatusInternalServerError)
		return
	}

	// Broadcast the status change via WebSocket
	ws.SendThreadUpdate(threadID, map[string]interface{}{
		"bot_enabled": enabled,
	})

	log.Printf("✅ Successfully toggled bot status for thread: %s", threadID)
	w.WriteHeader(http.StatusOK)
}

func GetChatMessages(w http.ResponseWriter, r *http.Request) {
	// Retrieve the authenticated user from context.
	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		log.Printf("❌ No user found in context")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	threadID := r.URL.Query().Get("thread_id")
	log.Printf("⚠️ Full chat refresh requested for thread: %s", threadID)

	messages, err := db.FetchMessages(user.ClientID, db.GetChatQuery, threadID)
	if err != nil {
		log.Printf("❌ Error fetching chat messages: %v", err)
		db.HandleError(w, err, "Error fetching chat messages", http.StatusInternalServerError)
		return
	}

	log.Printf("Found %d messages for thread %s", len(messages), threadID)

	data := map[string]interface{}{
		"Messages": messages,
		"User":     user,
	}

	if err := template.RenderTemplate(w, "chat-messages", data); err != nil {
		log.Printf("❌ Error rendering chat messages: %v", err)
		db.HandleError(w, err, "Error rendering chat messages", http.StatusInternalServerError)
		return
	}

	// Notify WebSocket clients about updated messages
	if len(messages) > 0 {
		ws.SendThreadUpdate(threadID, map[string]interface{}{
			"messages_updated": true,
			"message_count":    len(messages),
			"last_message":     messages[len(messages)-1],
			"timestamp":        time.Now(),
		})
	}

	log.Printf("✅ Successfully rendered chat messages for thread %s", threadID)
	log.Printf("  - Message count: %d", len(messages))
	log.Printf("  - User ID: %s", user.ID)
	log.Printf("  - Client ID: %s", user.ClientID)
}
