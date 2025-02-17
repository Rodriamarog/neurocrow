// ws/client.go
package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// Client represents a WebSocket connection client
type Client struct {
	// Unique identifier for this connection
	ID string

	// The websocket connection
	conn *websocket.Conn

	// Client ID from authentication
	ClientID string

	// Buffered channel of outbound messages
	send chan *Message

	// Current thread being viewed
	CurrentThread string

	// Optional username/identifier for logging
	Username string
}

// NewClient creates a new client instance
func NewClient(conn *websocket.Conn, clientID string, username string) *Client {
	return &Client{
		ID:       uuid.New().String(),
		conn:     conn,
		ClientID: clientID,
		Username: username,
		send:     make(chan *Message, 256), // Buffer up to 256 messages
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		log.Printf("🔌 Client ReadPump closing - ID: %s, Username: %s", c.ID, c.Username)
		GlobalHub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		log.Printf("📍 Pong received from client - ID: %s", c.ID)
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("❌ Unexpected close error for client %s: %v", c.ID, err)
			} else {
				log.Printf("🔌 Connection closed normally for client %s", c.ID)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("❌ Error unmarshaling message from client %s: %v", c.ID, err)
			continue
		}

		log.Printf("📥 Message received from client %s - Type: %s", c.ID, msg.Type)

		switch msg.Type {
		case "register_global_observer":
			log.Printf("🌐 Registering client %s as global observer", c.ID)
			GlobalHub.RegisterGlobalObserver(c.ClientID)
			log.Printf("✅ Client registered as global observer - ID: %s", c.ID)

		case TypeChatOpened:
			if msg.ThreadID != "" {
				// Remove from old thread if any
				if c.CurrentThread != "" && c.CurrentThread != msg.ThreadID {
					log.Printf("👋 Client %s leaving thread: %s", c.ID, c.CurrentThread)
					GlobalHub.removeThreadObserver(c.ClientID, c.CurrentThread)
				}

				// Update current thread and add as observer
				c.CurrentThread = msg.ThreadID
				GlobalHub.AddThreadObserver(c.ClientID, msg.ThreadID)
				log.Printf("👀 Client %s joined thread: %s", c.ID, msg.ThreadID)

				// Notify client of successful thread join
				response := NewThreadMessage(TypeChatOpened, c.ClientID, msg.ThreadID, nil)
				select {
				case c.send <- response:
					log.Printf("✅ Sent thread join confirmation to client %s", c.ID)
				default:
					log.Printf("❌ Failed to send thread join confirmation to client %s", c.ID)
				}
			}

		case TypeChatClosed:
			if c.CurrentThread != "" {
				log.Printf("👋 Client %s closing thread: %s", c.ID, c.CurrentThread)
				GlobalHub.removeThreadObserver(c.ClientID, c.CurrentThread)
				c.CurrentThread = ""
			}

		case "ping":
			// Handle client ping messages
			log.Printf("📍 Ping received from client %s", c.ID)
			response := NewMessage("pong", c.ClientID, nil)
			select {
			case c.send <- response:
				log.Printf("📍 Pong sent to client %s", c.ID)
			default:
				log.Printf("❌ Failed to send pong to client %s", c.ID)
			}

		default:
			log.Printf("ℹ️ Unhandled message type from client %s: %s", c.ID, msg.Type)
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		log.Printf("🔌 Client WritePump closing - ID: %s, Username: %s", c.ID, c.Username)
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				log.Printf("❌ Hub closed channel for client %s", c.ID)
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("❌ Error getting writer for client %s: %v", c.ID, err)
				return
			}

			// Add timestamp if not set
			if message.Timestamp.IsZero() {
				message.Timestamp = time.Now()
			}

			log.Printf("📤 Sending message to client %s - Type: %s", c.ID, message.Type)
			if err := json.NewEncoder(w).Encode(message); err != nil {
				log.Printf("❌ Error encoding message for client %s: %v", c.ID, err)
				return
			}

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				if err := json.NewEncoder(w).Encode(<-c.send); err != nil {
					log.Printf("❌ Error encoding queued message for client %s: %v", c.ID, err)
					return
				}
			}

			if err := w.Close(); err != nil {
				log.Printf("❌ Error closing writer for client %s: %v", c.ID, err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("❌ Error sending ping to client %s: %v", c.ID, err)
				return
			}
			log.Printf("📍 Ping sent to client %s", c.ID)
		}
	}
}

// SendMessage sends a message to the client
func (c *Client) SendMessage(msg *Message) error {
	select {
	case c.send <- msg:
		log.Printf("✅ Message queued for client %s - Type: %s", c.ID, msg.Type)
		return nil
	default:
		log.Printf("❌ Message queue full for client %s", c.ID)
		return ErrMessageQueueFull
	}
}

// IsInThread checks if the client is currently in the specified thread
func (c *Client) IsInThread(threadID string) bool {
	return c.CurrentThread == threadID
}
