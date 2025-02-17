// ws/hub.go
package ws

import (
	"log"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	// Registered clients, mapped by client ID for efficient broadcasting
	clients map[string]map[*Client]bool

	// Inbound message channels
	register        chan *Client
	unregister      chan *Client
	broadcast       chan *Message
	globalObservers map[string]bool

	// Thread tracking
	// Maps thread IDs to client IDs that are currently viewing them
	threadObservers map[string]map[string]bool

	// Mutex for thread-safe operations
	mu sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:         make(map[string]map[*Client]bool),
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		broadcast:       make(chan *Message),
		threadObservers: make(map[string]map[string]bool),
		globalObservers: make(map[string]bool),
	}
}

// Registers Global Observer (Message-List)
func (h *Hub) RegisterGlobalObserver(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.globalObservers[clientID] = true
	log.Printf("🌐 Registered global observer - Client ID: %s", clientID)
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	log.Printf("🚀 Starting WebSocket hub")
	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case message := <-h.broadcast:
			h.handleBroadcast(message)
		}
	}
}

func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("📥 Registering new client - ID: %s, Client ID: %s", client.ID, client.ClientID)

	// Initialize client map if needed
	if _, exists := h.clients[client.ClientID]; !exists {
		h.clients[client.ClientID] = make(map[*Client]bool)
		log.Printf("🔨 Created new client map for Client ID: %s", client.ClientID)
	}

	h.clients[client.ClientID][client] = true

	log.Printf("✅ Client registered successfully")
	log.Printf("📊 Total clients for Client ID %s: %d", client.ClientID, len(h.clients[client.ClientID]))
}

func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("📤 Unregistering client - ID: %s, Client ID: %s", client.ID, client.ClientID)

	// Remove from clients map
	if clients, exists := h.clients[client.ClientID]; exists {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.send)

			// Remove client map if empty
			if len(clients) == 0 {
				delete(h.clients, client.ClientID)
				log.Printf("🧹 Removed empty client map for Client ID: %s", client.ClientID)
			}
		}
	}

	// Remove from thread observers
	if client.CurrentThread != "" {
		h.removeThreadObserver(client.ClientID, client.CurrentThread)
	}

	log.Printf("✅ Client unregistered successfully")
}

func (h *Hub) handleBroadcast(message *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	log.Printf("📢 Broadcasting message - Type: %s, Client ID: %s", message.Type, message.ClientID)

	// Get all target clients
	targetClients := make(map[*Client]bool)

	// Add thread observers if it's a thread-specific message
	if message.ThreadID != "" {
		for client := range h.getThreadObservers(message.ThreadID) {
			targetClients[client] = true
		}
	}

	// Add global observers for new_message type
	if message.Type == "new_message" {
		for clientID := range h.globalObservers {
			if clients := h.clients[clientID]; clients != nil {
				for client := range clients {
					targetClients[client] = true
				}
			}
		}
	}

	// Add direct client recipients
	if clients := h.clients[message.ClientID]; clients != nil {
		for client := range clients {
			targetClients[client] = true
		}
	}

	// Send to all target clients
	for client := range targetClients {
		select {
		case client.send <- message:
			log.Printf("✅ Message sent to client - ID: %s", client.ID)
		default:
			log.Printf("❌ Failed to send message to client - ID: %s", client.ID)
			h.unregister <- client
		}
	}
}

// AddThreadObserver adds a client as an observer of a thread
func (h *Hub) AddThreadObserver(clientID, threadID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.threadObservers[threadID] == nil {
		h.threadObservers[threadID] = make(map[string]bool)
	}
	h.threadObservers[threadID][clientID] = true

	log.Printf("👀 Added thread observer - Client ID: %s, Thread ID: %s", clientID, threadID)
}

// RemoveThreadObserver removes a client as an observer of a thread
func (h *Hub) removeThreadObserver(clientID, threadID string) {
	if observers := h.threadObservers[threadID]; observers != nil {
		delete(observers, clientID)
		if len(observers) == 0 {
			delete(h.threadObservers, threadID)
		}
		log.Printf("👋 Removed thread observer - Client ID: %s, Thread ID: %s", clientID, threadID)
	}
}

// getThreadObservers returns all clients observing a thread
func (h *Hub) getThreadObservers(threadID string) map[*Client]bool {
	observers := make(map[*Client]bool)

	if threadObservers := h.threadObservers[threadID]; threadObservers != nil {
		for clientID := range threadObservers {
			if clients := h.clients[clientID]; clients != nil {
				for client := range clients {
					observers[client] = true
				}
			}
		}
	}

	return observers
}

// Broadcast sends a message to the hub for broadcasting
func (h *Hub) Broadcast(message *Message) {
	log.Printf("📨 Queuing message for broadcast - Type: %s, Client ID: %s",
		message.Type, message.ClientID)
	h.broadcast <- message
}

// Global hub instance
var GlobalHub = NewHub()
