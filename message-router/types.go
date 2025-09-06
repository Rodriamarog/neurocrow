// types.go
package main

import (
	"log"
	"sync"
	"time"
)

// FacebookEvent represents the incoming webhook event from Facebook
type FacebookEvent struct {
	Object string      `json:"object"`
	Entry  []EntryData `json:"entry"`
}

// EntryData represents each entry in the webhook event
type EntryData struct {
	ID   string `json:"id"`
	Time int64  `json:"time"`
	// Handle messaging events
	Messaging []MessagingEntry `json:"messaging"`
}

// MessagingEntry represents a message in the Facebook webhook
type MessagingEntry struct {
	Sender struct {
		ID string `json:"id"`
	} `json:"sender"`
	Recipient struct {
		ID string `json:"id"`
	} `json:"recipient"`
	Message  *MessageData  `json:"message"`
	Delivery *DeliveryData `json:"delivery"`
}

// MessageData represents the actual message content
type MessageData struct {
	Mid    string `json:"mid"`
	Text   string `json:"text"`
	IsEcho bool   `json:"is_echo"`
	AppId  int64  `json:"app_id"`
}

// DeliveryData represents a delivery receipt from Facebook
type DeliveryData struct {
	Mids      []string `json:"mids"`
	Watermark int64    `json:"watermark"`
}

// BotpressResponse represents the webhook response from Botpress
type BotpressResponse struct {
	Type    string `json:"type"`
	Payload struct {
		Text string `json:"text"`
	} `json:"payload"`
	ConversationId         string `json:"conversationId"`
	BotpressUserId         string `json:"botpressUserId"`
	BotpressMessageId      string `json:"botpressMessageId"`
	BotpressConversationId string `json:"botpressConversationId"`
}

// BotpressRequest represents the request we send to Botpress
type BotpressRequest struct {
	ID             string                 `json:"id"`
	ConversationId string                 `json:"conversationId"`
	Channel        string                 `json:"channel"`
	Type           string                 `json:"type"`
	Content        string                 `json:"content"`
	Payload        BotpressRequestPayload `json:"payload"`
	Direction      string                 `json:"direction"`
}

// BotpressRequestPayload represents the payload in a Botpress request
type BotpressRequestPayload struct {
	Text     string `json:"text"`
	Type     string `json:"type"`
	PageId   string `json:"pageId"`
	SenderId string `json:"senderId"`
}

// FacebookResponse represents a response we send to Facebook
type FacebookResponse struct {
	Recipient struct {
		ID string `json:"id"`
	} `json:"recipient"`
	Message struct {
		Text string `json:"text"`
	} `json:"message"`
}

// InstagramMessage represents the Instagram-specific message structure
type InstagramMessage struct {
	ID        string         `json:"id"`
	From      *InstagramUser `json:"from"`
	Text      string         `json:"text"`
	Timestamp int64          `json:"timestamp"`
}

type InstagramUser struct {
	ID       string `json:"id"`
	Username string `json:"username,omitempty"`
}

type InstagramChanges struct {
	Field string `json:"field"`
	Value struct {
		Messages []InstagramMessage `json:"messages"`
	} `json:"value"`
}

type ConversationState struct {
	ThreadID           string
	PageID             string
	Platform           string
	BotEnabled         bool // Current bot control flag - true=bot enabled, false=human agent has control
	LastBotMessage     time.Time
	LastHumanMessage   time.Time // Used for 12-hour bot reactivation logic
	LastUserMessage    time.Time
	MessageCount       int
	DifyConversationID string // Dify conversation ID for maintaining context
}

type Config struct {
	DatabaseURL       string // Single database URL
	FacebookAppSecret string
	FacebookAppID     string // Added for OAuth functionality
	VerifyToken       string
	Port              string
	FireworksKey      string
	// Instagram OAuth credentials
	InstagramAppID        string // Added for Instagram OAuth
	InstagramAppSecretKey string // Added for Instagram OAuth
	// Facebook App IDs for echo message detection
	FacebookBotAppID       int64 // Your bot's Facebook App ID (1195277397801905) - used for echo detection
	FacebookPageInboxAppID int64 // Facebook Page Inbox App ID (263902037430900) - unused
	// Botpress integration (legacy - will be removed after migration)
	BotpressToken string // Botpress token (temporary during migration)
	// Note: Dify API keys are now stored per-page in database (multi-tenant)
}

// PageInfo represents essential page information retrieved from the database
type PageInfo struct {
	Platform    string
	PageID      string
	AccessToken string
}

// FireworksResponse represents the response structure from the LLM API
type FireworksResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// UserProfile represents the user profile information from Facebook
type UserProfile struct {
	Name     string `json:"name"`
	Username string `json:"username"` // For Instagram users
}

type FacebookProfile struct {
	Name string `json:"name"`
}

type InstagramProfile struct {
	Username string `json:"username"`
}

type UserCache struct {
	sync.RWMutex
	data map[string]cachedUser
}

type cachedUser struct {
	name      string
	expiresAt time.Time
}

var (
	userCache = &UserCache{
		data: make(map[string]cachedUser),
	}
	cacheDuration = 24 * time.Hour
)

type SendMessageRequest struct {
	PageID      string `json:"page_id"`
	RecipientID string `json:"recipient_id"`
	Platform    string `json:"platform"`
	Message     string `json:"message"`
}

func (c *UserCache) Get(userID string) (string, bool) {
	c.RLock()
	defer c.RUnlock()

	if user, exists := c.data[userID]; exists {
		if time.Now().Before(user.expiresAt) {
			log.Printf("🎯 Cache hit for user %s: %s (expires in %v)",
				userID, user.name, time.Until(user.expiresAt))
			return user.name, true
		}
		log.Printf("⌛ Cache entry expired for user %s", userID)
	}
	log.Printf("❌ Cache miss for user %s", userID)
	return "", false
}

func (c *UserCache) Set(userID, name string) {
	c.Lock()
	defer c.Unlock()

	expiresAt := time.Now().Add(cacheDuration)
	c.data[userID] = cachedUser{
		name:      name,
		expiresAt: expiresAt,
	}
	log.Printf("💾 Cached user %s name as %s (expires at %v)", userID, name, expiresAt)
}

// =============================================================================
// DIFY API TYPES - New integration replacing Botpress
// =============================================================================

// DifyRequest represents the request we send to Dify Chat API
type DifyRequest struct {
	Inputs         map[string]interface{} `json:"inputs"`                    // Additional inputs (usually empty for simple chat)
	Query          string                 `json:"query"`                     // The user's message text
	ResponseMode   string                 `json:"response_mode"`             // "blocking" or "streaming"
	User           string                 `json:"user"`                      // Unique user identifier
	ConversationId string                 `json:"conversation_id,omitempty"` // Optional, for conversation continuity
	Files          []interface{}          `json:"files,omitempty"`           // File attachments (if any)
}

// DifyResponse represents the response from Dify Chat API
type DifyResponse struct {
	Answer         string `json:"answer"`          // The AI's response text
	ConversationId string `json:"conversation_id"` // Conversation ID for continuity
	MessageId      string `json:"message_id"`      // Unique message identifier
	Mode           string `json:"mode"`            // Response mode used
	Metadata       struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		RetrieverResources []interface{} `json:"retriever_resources"`
	} `json:"metadata"`
	CreatedAt int64 `json:"created_at"` // Unix timestamp
}

// DifyErrorResponse represents error responses from Dify API
type DifyErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// DifyConversationState represents conversation state for Dify integration
type DifyConversationState struct {
	ConversationId string    // Dify's conversation ID
	UserId         string    // Our internal user ID (pageId-senderId)
	LastMessageId  string    // Last message ID from Dify
	CreatedAt      time.Time // When this conversation started
	UpdatedAt      time.Time // Last activity
}
