package models

import "time"

// PendingSignalMessage represents a Signal message that has been received but not yet
// successfully processed and forwarded.
type PendingSignalMessage struct {
	ID          int64     `json:"id"`
	MessageID   string    `json:"messageId"`
	Sender      string    `json:"sender"`
	Message     string    `json:"message"`
	GroupID     string    `json:"groupId"`
	Timestamp   int64     `json:"timestamp"`
	RawJSON     string    `json:"rawJson"`
	Destination string    `json:"destination"`
	RetryCount  int       `json:"retryCount"`
	CreatedAt   time.Time `json:"createdAt"`
}
