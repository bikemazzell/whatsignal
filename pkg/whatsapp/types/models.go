package types

import (
	"encoding/json"
	"strings"
	"time"
)

// SessionStatus represents the current state of a WhatsApp session
type SessionStatus string

const (
	SessionStatusInitialized SessionStatus = "initialized"
	SessionStatusStarting    SessionStatus = "starting"
	SessionStatusRunning     SessionStatus = "running"
	SessionStatusStopped     SessionStatus = "stopped"
	SessionStatusError       SessionStatus = "error"
)

// Session represents a WhatsApp session
type Session struct {
	Name      string        `json:"name"`
	Status    SessionStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Error     string        `json:"error,omitempty"`
}

// WebhookEvent represents a webhook event from WAHA
type WebhookEvent struct {
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

// ReactionRequest represents the request to send a reaction
type ReactionRequest struct {
	Session   string `json:"session"`
	MessageID string `json:"messageId"`
	Reaction  string `json:"reaction"`
}

// SeenRequest represents the request to mark messages as seen
type SeenRequest struct {
	ChatID  string `json:"chatId"`
	Session string `json:"session"`
}

// TypingRequest represents the request to start/stop typing indicator
type TypingRequest struct {
	ChatID  string `json:"chatId"`
	Session string `json:"session"`
}

// SendMessageRequest represents the base request for sending messages
type SendMessageRequest struct {
	ChatID  string `json:"chatId"`
	Text    string `json:"text"`
	Session string `json:"session"`
}

// FileData represents file information for media messages
type FileData struct {
	Mimetype string `json:"mimetype"`
	Filename string `json:"filename"`
	Data     string `json:"data"`
}

// MediaMessageRequest represents the request for sending media messages
type MediaMessageRequest struct {
	ChatID  string   `json:"chatId"`
	File    FileData `json:"file"`
	Caption string   `json:"caption,omitempty"`
	Session string   `json:"session"`
	ReplyTo string   `json:"reply_to,omitempty"`
	Convert *bool    `json:"convert,omitempty"` // For video conversion
	AsNote  *bool    `json:"asNote,omitempty"`  // For video notes (rounded videos)
}

// MessagePayload represents a message payload in a webhook
type MessagePayload struct {
	ID        string    `json:"id"`
	ChatID    string    `json:"chatId"`
	Sender    string    `json:"sender"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Content   string    `json:"content,omitempty"`
	MediaURL  string    `json:"mediaUrl,omitempty"`
}

// SendMessageResponse represents the response from send message operations
type SendMessageResponse struct {
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// WAHAMessageResponse represents the actual WAHA API response format
type WAHAMessageResponse struct {
	Data *struct {
		ID *struct {
			FromMe     bool   `json:"fromMe"`
			Remote     string `json:"remote"`
			ID         string `json:"id"`
			Serialized string `json:"_serialized"`
		} `json:"id"`
	} `json:"_data"`
	ID *struct {
		FromMe     bool   `json:"fromMe"`
		Remote     string `json:"remote"`
		ID         string `json:"id"`
		Serialized string `json:"_serialized"`
	} `json:"id"`
}

// WAHAErrorResponse represents error responses from WAHA API
type WAHAErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// Contact represents a WhatsApp contact from WAHA API
type Contact struct {
	ID          string `json:"id"`
	Number      string `json:"number"`
	Name        string `json:"name"`
	PushName    string `json:"pushname"`
	ShortName   string `json:"shortName"`
	IsMe        bool   `json:"isMe"`
	IsGroup     bool   `json:"isGroup"`
	IsWAContact bool   `json:"isWAContact"`
	IsMyContact bool   `json:"isMyContact"`
	IsBlocked   bool   `json:"isBlocked"`
}

// GetDisplayName returns the best available display name for the contact
func (c *Contact) GetDisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	if c.PushName != "" {
		return c.PushName
	}
	return c.Number
}

// ContactsResponse represents the response from contacts API calls
type ContactsResponse struct {
	Contacts []Contact `json:"contacts,omitempty"`
	Contact  *Contact  `json:",omitempty"` // For single contact responses
}

// ClientConfig represents the configuration for WhatsApp client
type ClientConfig struct {
	BaseURL     string        `json:"base_url" validate:"required,url"`
	APIKey      string        `json:"api_key" validate:"required"`
	SessionName string        `json:"session_name" validate:"required"`
	Timeout     time.Duration `json:"timeout" validate:"required"`
	RetryCount  int           `json:"retry_count" validate:"min=1,max=10"`
}

// ServerVersion represents WAHA server version info from /api/server/version
type ServerVersion struct {
	Version string `json:"version"`
	Engine  string `json:"engine"`
	Tier    string `json:"tier"`
	Browser string `json:"browser"`
}

// Group represents a WhatsApp group from WAHA API
type Group struct {
	ID           string             `json:"id"`
	Subject      string             `json:"subject"`
	Description  string             `json:"description"`
	Participants []GroupParticipant `json:"participants"`
	InviteLink   string             `json:"invite"`
	CreatedAt    int64              `json:"createdAt"`
}

// GroupParticipant represents a participant in a WhatsApp group
type GroupParticipant struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	IsAdmin bool   `json:"isAdmin"`
}

// GetDisplayName returns the best available display name for the group
func (g *Group) GetDisplayName() string {
	if g.Subject != "" {
		return g.Subject
	}
	return g.ID
}

// IsGroupMessage returns true if the message is from a group chat
func (m *MessagePayload) IsGroupMessage() bool {
	return strings.HasSuffix(m.ChatID, "@g.us")
}

// ExtractGroupID extracts and validates a group ID from the chatID field
func (m *MessagePayload) ExtractGroupID() string {
	if m.IsGroupMessage() {
		return m.ChatID
	}
	return ""
}
