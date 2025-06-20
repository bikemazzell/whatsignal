package types

import (
	"encoding/json"
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
	BaseURL     string
	APIKey      string
	SessionName string
	Timeout     time.Duration
	RetryCount  int
}
