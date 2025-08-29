package models

import "time"

type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "pending"
	DeliveryStatusSent      DeliveryStatus = "sent"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusRead      DeliveryStatus = "read"
	DeliveryStatusFailed    DeliveryStatus = "failed"
)

// MessageMapping represents a bidirectional mapping between WhatsApp and Signal messages
type MessageMapping struct {
	ID              int64          `json:"id"`
	WhatsAppChatID  string         `json:"whatsappChatId"`
	WhatsAppMsgID   string         `json:"whatsappMsgId"`
	SignalMsgID     string         `json:"signalMsgId"`
	SignalTimestamp time.Time      `json:"signalTimestamp"`
	ForwardedAt     time.Time      `json:"forwardedAt"`
	DeliveryStatus  DeliveryStatus `json:"deliveryStatus"`
	MediaPath       *string        `json:"mediaPath,omitempty"`
	MediaType       string         `json:"mediaType"`
	SessionName     string         `json:"sessionName"` // WhatsApp session name for multi-channel support
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

// MessageMetadata contains metadata about a message that needs to be preserved
// when bridging between platforms
type MessageMetadata struct {
	Sender   string    `json:"sender"`
	Chat     string    `json:"chat"`
	Time     time.Time `json:"time"`
	MsgID    string    `json:"msgId"`
	ThreadID string    `json:"threadId"`
}
