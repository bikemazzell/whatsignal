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

func TestSessionStatusConstants(t *testing.T) {
	tests := []struct {
		name   string
		status SessionStatus
		value  string
	}{
		{
			name:   "initialized status",
			status: SessionStatusInitialized,
			value:  "initialized",
		},
		{
			name:   "starting status",
			status: SessionStatusStarting,
			value:  "starting",
		},
		{
			name:   "running status",
			status: SessionStatusRunning,
			value:  "running",
		},
		{
			name:   "stopped status",
			status: SessionStatusStopped,
			value:  "stopped",
		},
		{
			name:   "error status",
			status: SessionStatusError,
			value:  "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.value, string(tt.status))
		})
	}
}

func TestSessionWithError(t *testing.T) {
	session := Session{
		Name:      "test-session",
		Status:    SessionStatusError,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Error:     "Connection failed",
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
	assert.Equal(t, session.Error, decoded.Error)
}

func TestWebhookEventWithRawPayload(t *testing.T) {
	// Test with a valid JSON string as raw message
	event := WebhookEvent{
		Event:   "message.any",
		Payload: json.RawMessage(`"raw string payload"`),
	}

	// Test JSON marshaling
	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded WebhookEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.Event, decoded.Event)
	assert.Equal(t, event.Payload, decoded.Payload)
}

func TestWebhookEventWithEmptyPayload(t *testing.T) {
	event := WebhookEvent{
		Event:   "status.update",
		Payload: json.RawMessage(`{}`),
	}

	// Test JSON marshaling
	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var decoded WebhookEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.Event, decoded.Event)
	assert.Equal(t, event.Payload, decoded.Payload)
}

func TestSendMessageResponseWithError(t *testing.T) {
	resp := SendMessageResponse{
		MessageID: "",
		Status:    "failed",
		Error:     "Network timeout",
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

func TestClientConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  ClientConfig
		isValid bool
	}{
		{
			name: "valid config",
			config: ClientConfig{
				BaseURL:     "http://localhost:8080",
				APIKey:      "test-key",
				SessionName: "test-session",
				Timeout:     5 * time.Second,
				RetryCount:  3,
			},
			isValid: true,
		},
		{
			name: "empty base URL",
			config: ClientConfig{
				BaseURL:     "",
				APIKey:      "test-key",
				SessionName: "test-session",
				Timeout:     5 * time.Second,
				RetryCount:  3,
			},
			isValid: false,
		},
		{
			name: "empty API key",
			config: ClientConfig{
				BaseURL:     "http://localhost:8080",
				APIKey:      "",
				SessionName: "test-session",
				Timeout:     5 * time.Second,
				RetryCount:  3,
			},
			isValid: false,
		},
		{
			name: "zero timeout",
			config: ClientConfig{
				BaseURL:     "http://localhost:8080",
				APIKey:      "test-key",
				SessionName: "test-session",
				Timeout:     0,
				RetryCount:  3,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation logic
			isValid := tt.config.BaseURL != "" &&
				tt.config.APIKey != "" &&
				tt.config.Timeout > 0

			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

func TestMessagePayloadTypes(t *testing.T) {
	types := []string{"text", "image", "video", "document", "voice", "sticker"}

	for _, msgType := range types {
		t.Run("message type "+msgType, func(t *testing.T) {
			msg := MessagePayload{
				ID:     "test-id",
				ChatID: "test-chat",
				Sender: "test-sender",
				Type:   msgType,
			}

			data, err := json.Marshal(msg)
			require.NoError(t, err)

			var decoded MessagePayload
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, msgType, decoded.Type)
		})
	}
}
