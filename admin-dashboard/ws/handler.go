// ws/handler.go
package ws

import (
	"log"
	"net/http"

	"admin-dashboard/pkg/auth"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, you should implement proper origin checking
		return true
	},
}

// HandleWebSocket upgrades HTTP connection to WebSocket and handles the connection
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	connID := r.Header.Get("X-Connection-ID")
	log.Printf("[WS-%s] 🔌 New WebSocket connection request from %s", connID, r.RemoteAddr)

	// Get user from context
	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		log.Printf("[WS-%s] ❌ No user found in context", connID)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	log.Printf("[WS-%s] 👤 User authenticated:", connID)
	log.Printf("[WS-%s]    - User ID: %s", connID, user.ID)
	log.Printf("[WS-%s]    - Client ID: %s", connID, user.ClientID)
	log.Printf("[WS-%s]    - Role: %s", connID, user.Role)

	// Log headers for debugging
	log.Printf("[WS-%s] 📨 Request Headers:", connID)
	for key, values := range r.Header {
		log.Printf("[WS-%s]    - %s: %v", connID, key, values)
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS-%s] ❌ Failed to upgrade connection: %v", connID, err)
		return
	}

	// Create new client
	client := NewClient(conn, user.ClientID, user.Email)
	log.Printf("[WS-%s] ✨ Created new WebSocket client:", connID)
	log.Printf("[WS-%s]    - Client ID: %s", connID, client.ID)
	log.Printf("[WS-%s]    - User Client ID: %s", connID, client.ClientID)

	// Register with hub
	GlobalHub.register <- client

	// Start the client pumps
	go client.WritePump()
	go client.ReadPump()

	log.Printf("[WS-%s] ✅ WebSocket connection established successfully", connID)
}

// SendThreadUpdate broadcasts a thread update to all relevant clients
func SendThreadUpdate(threadID string, content interface{}) {
	msg := NewThreadMessage(TypeThreadUpdated, "", threadID, content)
	log.Printf("📢 Broadcasting thread update:")
	log.Printf("  - Thread ID: %s", threadID)
	GlobalHub.Broadcast(msg)
}

// SendNewMessage broadcasts a new message to relevant clients
func SendNewMessage(clientID string, threadID string, content interface{}) {
	msg := NewThreadMessage(TypeNewMessage, clientID, threadID, content)
	log.Printf("📢 Broadcasting new message:")
	log.Printf("  - Client ID: %s", clientID)
	log.Printf("  - Thread ID: %s", threadID)
	GlobalHub.Broadcast(msg)
}

// SendError sends an error message to a specific client
func SendError(clientID string, errorMsg string) {
	msg := NewMessage(TypeError, clientID, errorMsg)
	log.Printf("❌ Sending error message:")
	log.Printf("  - Client ID: %s", clientID)
	log.Printf("  - Error: %s", errorMsg)
	GlobalHub.Broadcast(msg)
}
