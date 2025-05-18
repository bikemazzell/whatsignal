package models

import (
	"time"
)

type DeliveryStatus string

const (
	DeliveryStatusSent      DeliveryStatus = "sent"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusRead      DeliveryStatus = "read"
	DeliveryStatusFailed    DeliveryStatus = "failed"
)

type MessageMapping struct {
	ID              int64          `db:"id"`
	WhatsAppChatID  string         `db:"whatsapp_chat_id"`
	WhatsAppMsgID   string         `db:"whatsapp_msg_id"`
	SignalMsgID     string         `db:"signal_msg_id"`
	SignalTimestamp time.Time      `db:"signal_timestamp"`
	ForwardedAt     time.Time      `db:"forwarded_at"`
	DeliveryStatus  DeliveryStatus `db:"delivery_status"`
	MediaPath       *string        `db:"media_path"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

type MessageMetadata struct {
	Sender   string    `json:"sender"`
	Chat     string    `json:"chat"`
	Time     time.Time `json:"time"`
	MsgID    string    `json:"msgId"`
	ThreadID string    `json:"threadId"`
}

type Message struct {
	Metadata  MessageMetadata
	Content   string
	MediaType string
	MediaURL  string
}
