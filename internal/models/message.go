package models

import "time"

type MessageType string

const (
	TextMessage  MessageType = "text"
	ImageMessage MessageType = "image"
	VideoMessage MessageType = "video"
	AudioMessage MessageType = "audio"
)

// Message represents a message in the system, either from WhatsApp or Signal
type Message struct {
	ID              string        `json:"id"`
	ChatID          string        `json:"chatId"`
	ThreadID        string        `json:"threadId"`
	Sender          string        `json:"sender"`
	Recipient       string        `json:"recipient"`
	Content         string        `json:"content"`
	Type            MessageType   `json:"type"`
	Platform        string        `json:"platform"`
	MediaURL        string        `json:"mediaUrl"`
	MediaPath       string        `json:"mediaPath"`
	MediaMetadata   MediaMetadata `json:"mediaMetadata"`
	QuotedMessageID string        `json:"quotedMessageId"`
	Timestamp       time.Time     `json:"timestamp"`
	DeliveryStatus  string        `json:"deliveryStatus"`
}

type MediaMetadata struct {
	Size      int64  `json:"size"`
	MimeType  string `json:"mimeType"`
	Filename  string `json:"filename"`
	CachePath string `json:"cachePath"`
}
