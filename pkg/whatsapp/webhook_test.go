package whatsapp

import (
	"context"
	"encoding/json"
	"testing"

	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookHandler_RegisterAndHandle(t *testing.T) {
	wh := NewWebhookHandler()
	ctx := context.Background()

	// Test registering and handling an event
	eventType := "test.event"
	var handlerCalled bool
	var receivedPayload string

	wh.RegisterEventHandler(eventType, func(ctx context.Context, payload json.RawMessage) error {
		handlerCalled = true
		receivedPayload = string(payload)
		return nil
	})

	testPayload := json.RawMessage(`{"test":"data"}`)
	event := &types.WebhookEvent{
		Event:   eventType,
		Payload: testPayload,
	}

	err := wh.Handle(ctx, event)
	require.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.Equal(t, string(testPayload), receivedPayload)

	// Test handling unregistered event
	unregisteredEvent := &types.WebhookEvent{
		Event:   "unregistered.event",
		Payload: testPayload,
	}

	err = wh.Handle(ctx, unregisteredEvent)
	assert.Error(t, err)
}

func TestHandleMessageEvent(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		payload     json.RawMessage
		expectError bool
	}{
		{
			name: "valid text message",
			payload: json.RawMessage(`{
				"id": "msg123",
				"chatId": "chat123",
				"sender": "sender123",
				"timestamp": "2024-01-01T00:00:00Z",
				"type": "text",
				"content": "Hello, World!"
			}`),
			expectError: false,
		},
		{
			name: "valid image message",
			payload: json.RawMessage(`{
				"id": "msg123",
				"chatId": "chat123",
				"sender": "sender123",
				"timestamp": "2024-01-01T00:00:00Z",
				"type": "image",
				"mediaUrl": "http://example.com/image.jpg"
			}`),
			expectError: false,
		},
		{
			name: "unsupported message type",
			payload: json.RawMessage(`{
				"id": "msg123",
				"chatId": "chat123",
				"sender": "sender123",
				"timestamp": "2024-01-01T00:00:00Z",
				"type": "unsupported"
			}`),
			expectError: true,
		},
		{
			name:        "invalid json",
			payload:     json.RawMessage(`invalid json`),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := HandleMessageEvent(ctx, tt.payload)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
