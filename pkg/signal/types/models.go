package types

import (
	"encoding/json"
	"strconv"
)

// FlexibleInt64 can unmarshal both string and int64 JSON values
type FlexibleInt64 int64

func (f *FlexibleInt64) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		// It's a string, try to parse as int64
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*f = FlexibleInt64(i)
		return nil
	}

	// Try as int64 directly
	var i int64
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	*f = FlexibleInt64(i)
	return nil
}

func (f FlexibleInt64) Int64() int64 {
	return int64(f)
}

// SendMessage types for REST API
type SendMessageRequest struct {
	Message           string   `json:"message"`
	Number            string   `json:"number"`
	Recipients        []string `json:"recipients"`
	Base64Attachments []string `json:"base64_attachments,omitempty"`
	TextMode          string   `json:"text_mode,omitempty"` // "normal" or "styled"
}

type SendMessageResponse struct {
	Timestamp int64  `json:"timestamp"`
	MessageID string `json:"messageId"`
}

type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        string `json:"data"` // base64 encoded
}

type SendResponse struct {
	Timestamp FlexibleInt64 `json:"timestamp"`
}

type AboutResponse struct {
	Versions     []string            `json:"versions"`
	Build        int                 `json:"build"`
	Mode         string              `json:"mode"`
	Version      string              `json:"version"`
	Capabilities map[string][]string `json:"capabilities"`
}

// SignalMessage represents a received Signal message
type SignalMessage struct {
	Timestamp     int64    `json:"timestamp"`
	Sender        string   `json:"sender"`
	MessageID     string   `json:"messageId"`
	Message       string   `json:"message"`
	Attachments   []string `json:"attachments"`
	QuotedMessage *struct {
		ID        string `json:"id"`
		Author    string `json:"author"`
		Text      string `json:"text"`
		Timestamp int64  `json:"timestamp"`
	} `json:"quotedMessage,omitempty"`
	Reaction    *SignalReaction `json:"reaction,omitempty"`
	Deletion    *SignalDeletion `json:"deletion,omitempty"`
	IsSentByMe  bool            `json:"isSentByMe,omitempty"`
	Destination string          `json:"destination,omitempty"`
}

// SignalDeletion represents a message deletion event
type SignalDeletion struct {
	TargetMessageID string `json:"targetMessageId"`
	TargetTimestamp int64  `json:"targetTimestamp"`
}

// SignalReaction represents a reaction to a message
type SignalReaction struct {
	Emoji           string `json:"emoji"`
	TargetAuthor    string `json:"targetAuthor"`
	TargetTimestamp int64  `json:"targetSentTimestamp"`
	TargetMessageID string `json:"targetMessageId,omitempty"`
	IsRemove        bool   `json:"isRemove"`
}

// RestDataMessage represents the data message content in Signal messages
type RestDataMessage struct {
	Timestamp    int64                   `json:"timestamp"`
	Message      string                  `json:"message"`
	Attachments  []RestMessageAttachment `json:"attachments"`
	Quote        *RestMessageQuote       `json:"quote,omitempty"`
	Reaction     *RestMessageReaction    `json:"reaction,omitempty"`
	RemoteDelete *struct {
		Timestamp int64 `json:"timestamp"`
	} `json:"remoteDelete,omitempty"`
}

// RestSentMessage represents a message sent by the user (received via sync)
type RestSentMessage struct {
	Destination       string                  `json:"destination,omitempty"`
	DestinationNumber string                  `json:"destinationNumber,omitempty"`
	DestinationUUID   string                  `json:"destinationUuid,omitempty"`
	Timestamp         int64                   `json:"timestamp"`
	Message           string                  `json:"message"`
	ExpiresInSeconds  int                     `json:"expiresInSeconds,omitempty"`
	Attachments       []RestMessageAttachment `json:"attachments,omitempty"`
	Quote             *RestMessageQuote       `json:"quote,omitempty"`
	GroupInfo         *RestGroupInfo          `json:"groupInfo,omitempty"`
}

// RestGroupInfo represents group information in a message
type RestGroupInfo struct {
	GroupID string `json:"groupId"`
	Type    string `json:"type,omitempty"`
}

// RestSyncMessage represents a sync message (messages sent by the user on another device)
type RestSyncMessage struct {
	SentMessage *RestSentMessage `json:"sentMessage,omitempty"`
}

// REST API message types for receiving messages
type RestMessage struct {
	Envelope struct {
		Source         string           `json:"source"`
		SourceNumber   string           `json:"sourceNumber"`
		SourceUUID     string           `json:"sourceUuid"`
		SourceName     string           `json:"sourceName"`
		Timestamp      int64            `json:"timestamp"`
		DataMessage    *RestDataMessage `json:"dataMessage,omitempty"`
		SyncMessage    *RestSyncMessage `json:"syncMessage,omitempty"`
		ReceiptMessage interface{}      `json:"receiptMessage,omitempty"`
		TypingMessage  interface{}      `json:"typingMessage,omitempty"`
	} `json:"envelope"`
	Account string `json:"account"`
}

type RestMessageAttachment struct {
	ContentType string `json:"contentType"`
	Filename    string `json:"filename"`
	ID          string `json:"id"`
	Size        int64  `json:"size"`
}

type RestMessageQuote struct {
	ID     int64  `json:"id"`
	Author string `json:"author"`
	Text   string `json:"text"`
}

type RestMessageReaction struct {
	Emoji           string `json:"emoji"`
	TargetAuthor    string `json:"targetAuthor"`
	TargetTimestamp int64  `json:"targetSentTimestamp"`
	IsRemove        bool   `json:"isRemove"`
}
