// ws/message.go
package ws

import (
	"errors"
	"time"
)

var (
	// ErrMessageQueueFull is returned when a client's message queue is full
	ErrMessageQueueFull = errors.New("message queue is full")
)

// Message represents a WebSocket message
type Message struct {
	Type      string      `json:"type"`
	ThreadID  string      `json:"thread_id,omitempty"`
	Content   interface{} `json:"content,omitempty"`
	ClientID  string      `json:"client_id,omitempty"`
	Timestamp time.Time   `json:"timestamp,omitempty"`
}

// MessageType constants
const (
	// Client -> Server message types
	TypeChatOpened = "chat_opened"
	TypeChatClosed = "chat_closed"

	// Server -> Client message types
	TypeNewMessage     = "new_message"
	TypeMessageUpdated = "message_updated"
	TypeThreadUpdated  = "thread_updated"
	TypeError          = "error"
)

// NewMessage creates a new message with the current timestamp
func NewMessage(msgType string, clientID string, content interface{}) *Message {
	return &Message{
		Type:      msgType,
		ClientID:  clientID,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewThreadMessage creates a new thread-specific message
func NewThreadMessage(msgType string, clientID string, threadID string, content interface{}) *Message {
	msg := NewMessage(msgType, clientID, content)
	msg.ThreadID = threadID
	return msg
}
