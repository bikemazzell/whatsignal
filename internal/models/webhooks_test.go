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
			ID          string            `json:"id"`
			Timestamp   FlexibleTimestamp `json:"timestamp"`
			From        string            `json:"from"`
			FromMe      bool              `json:"fromMe"`
			To          string            `json:"to"`
			Body        string            `json:"body"`
			HasMedia    bool              `json:"hasMedia"`
			Participant string            `json:"participant,omitempty"`
			NotifyName  string            `json:"notifyName,omitempty"`
			Media       *struct {
				URL      string `json:"url"`
				MimeType string `json:"mimetype"`
				Filename string `json:"filename"`
			} `json:"media"`
			Reaction *struct {
				Text      string `json:"text"`
				MessageID string `json:"messageId"`
			} `json:"reaction"`
			Data *struct {
				NotifyName string `json:"notifyName,omitempty"`
				PushName   string `json:"pushName,omitempty"`
			} `json:"_data,omitempty"`
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

func TestFlexibleTimestamp_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected int64
		wantErr  bool
	}{
		{
			name:     "integer timestamp",
			json:     `{"timestamp": 1770567080}`,
			expected: 1770567080,
		},
		{
			name:     "float timestamp truncated",
			json:     `{"timestamp": 1770567080.597}`,
			expected: 1770567080,
		},
		{
			name:     "zero",
			json:     `{"timestamp": 0}`,
			expected: 0,
		},
		{
			name:     "negative integer",
			json:     `{"timestamp": -100}`,
			expected: -100,
		},
		{
			name:    "string value rejected",
			json:    `{"timestamp": "not-a-number"}`,
			wantErr: true,
		},
		{
			name:     "null value defaults to zero",
			json:     `{"timestamp": null}`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result struct {
				Timestamp FlexibleTimestamp `json:"timestamp"`
			}
			err := json.Unmarshal([]byte(tt.json), &result)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.Timestamp.Int64())
		})
	}
}

func TestFlexibleTimestamp_FullWebhookPayload(t *testing.T) {
	// Simulate the exact WAHA message.waiting payload that was failing
	wahaJSON := `{
		"id": "evt_123",
		"timestamp": 1770567080.597,
		"event": "message.waiting",
		"session": "default",
		"payload": {
			"id": "msg_456",
			"timestamp": 1770567080.597,
			"from": "1234567890@c.us",
			"fromMe": false,
			"to": "0987654321@c.us",
			"body": "test",
			"hasMedia": false
		}
	}`

	var payload WhatsAppWebhookPayload
	err := json.Unmarshal([]byte(wahaJSON), &payload)
	require.NoError(t, err)

	assert.Equal(t, int64(1770567080), payload.Timestamp.Int64())
	assert.Equal(t, int64(1770567080), payload.Payload.Timestamp.Int64())
	assert.Equal(t, "message.waiting", payload.Event)
	assert.Equal(t, "msg_456", payload.Payload.ID)
}
