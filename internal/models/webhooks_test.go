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
			ID        string `json:"id"`
			Timestamp int64  `json:"timestamp"`
			From      string `json:"from"`
			FromMe    bool   `json:"fromMe"`
			To        string `json:"to"`
			Body      string `json:"body"`
			HasMedia  bool   `json:"hasMedia"`
			Media     *struct {
				URL      string `json:"url"`
				MimeType string `json:"mimetype"`
				Filename string `json:"filename"`
			} `json:"media"`
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

func TestSignalWebhookPayload_JSONMarshaling(t *testing.T) {
	payload := SignalWebhookPayload{
		MessageID: "sig123",
		Sender:    "+1234567890",
		Message:   "Hello Signal!",
		Timestamp: 1234567890,
		Type:      "text",
		ThreadID:  "thread123",
		Recipient: "+0987654321",
	}

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), "Hello Signal!")
	assert.Contains(t, string(jsonData), "sig123")

	var unmarshaled SignalWebhookPayload
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, "sig123", unmarshaled.MessageID)
	assert.Equal(t, "+1234567890", unmarshaled.Sender)
}