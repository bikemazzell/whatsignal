package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhatsAppWebhookPayload_JSONMarshaling(t *testing.T) {
	payload := WhatsAppWebhookPayload{
		Event: "message",
		Payload: struct {
			ID          string `json:"id"`
			Timestamp   int64  `json:"timestamp"`
			From        string `json:"from"`
			FromMe      bool   `json:"fromMe"`
			To          string `json:"to"`
			Body        string `json:"body"`
			HasMedia    bool   `json:"hasMedia"`
			Participant string `json:"participant,omitempty"`
			Media       *struct {
				URL      string `json:"url"`
				MimeType string `json:"mimetype"`
				Filename string `json:"filename"`
			} `json:"media"`
			Reaction *struct {
				Text      string `json:"text"`
				MessageID string `json:"messageId"`
			} `json:"reaction"`
			EditedMessageID *string `json:"editedMessageId,omitempty"`
			ACK             *int    `json:"ack,omitempty"`
		}{
			ID:       "msg123",
			From:     "1234567890@c.us",
			To:       "0987654321@c.us",
			Body:     "Hello World",
			HasMedia: false,
		},
	}

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), "Hello World")
	assert.Contains(t, string(jsonData), "message")

	var unmarshaled WhatsAppWebhookPayload
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, "message", unmarshaled.Event)
	assert.Equal(t, "msg123", unmarshaled.Payload.ID)
}
