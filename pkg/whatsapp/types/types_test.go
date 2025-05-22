package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientConfig(t *testing.T) {
	config := ClientConfig{
		BaseURL:     "http://localhost:8080",
		APIKey:      "test-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
		RetryCount:  3,
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded ClientConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, config.BaseURL, decoded.BaseURL)
	assert.Equal(t, config.APIKey, decoded.APIKey)
	assert.Equal(t, config.SessionName, decoded.SessionName)
	assert.Equal(t, config.Timeout, decoded.Timeout)
	assert.Equal(t, config.RetryCount, decoded.RetryCount)
}

func TestSession(t *testing.T) {
	session := Session{
		Name:      "test-session",
		Status:    SessionStatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test JSON marshaling
	data, err := json.Marshal(session)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded Session
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, session.Name, decoded.Name)
	assert.Equal(t, session.Status, decoded.Status)
}

func TestWebhookEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   WebhookEvent
		wantErr bool
	}{
		{
			name: "valid message event",
			event: WebhookEvent{
				Event: "message.any",
				Payload: json.RawMessage(`{
					"id": "test-msg-id",
					"chatId": "test-chat-id",
					"type": "text",
					"content": "test message"
				}`),
			},
			wantErr: false,
		},
		{
			name: "valid status event",
			event: WebhookEvent{
				Event: "status.update",
				Payload: json.RawMessage(`{
					"id": "test-status-id",
					"status": "delivered"
				}`),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			var decoded WebhookEvent
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.event.Event, decoded.Event)
			assert.JSONEq(t, string(tt.event.Payload), string(decoded.Payload))
		})
	}
}

func TestSendMessageResponse(t *testing.T) {
	resp := SendMessageResponse{
		MessageID: "test-msg-id",
		Status:    "sent",
		Error:     "test error",
	}

	// Test JSON marshaling
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded SendMessageResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.MessageID, decoded.MessageID)
	assert.Equal(t, resp.Status, decoded.Status)
	assert.Equal(t, resp.Error, decoded.Error)
}

func TestMessagePayload(t *testing.T) {
	msg := MessagePayload{
		ID:       "test-msg-id",
		ChatID:   "test-chat-id",
		Sender:   "test-sender",
		Type:     "text",
		Content:  "test message",
		MediaURL: "http://example.com/media.jpg",
	}

	// Test JSON marshaling
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded MessagePayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.ID, decoded.ID)
	assert.Equal(t, msg.ChatID, decoded.ChatID)
	assert.Equal(t, msg.Sender, decoded.Sender)
	assert.Equal(t, msg.Type, decoded.Type)
	assert.Equal(t, msg.Content, decoded.Content)
	assert.Equal(t, msg.MediaURL, decoded.MediaURL)
}
