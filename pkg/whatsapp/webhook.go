package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"whatsignal/pkg/whatsapp/types"
)

type webhookHandler struct {
	handlers map[string]func(context.Context, json.RawMessage) error
	mu       sync.RWMutex
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler() WebhookHandler {
	return &webhookHandler{
		handlers: make(map[string]func(context.Context, json.RawMessage) error),
	}
}

func (wh *webhookHandler) Handle(ctx context.Context, event *types.WebhookEvent) error {
	wh.mu.RLock()
	handler, exists := wh.handlers[event.Event]
	wh.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no handler registered for event type: %s", event.Event)
	}

	return handler(ctx, event.Payload)
}

func (wh *webhookHandler) RegisterEventHandler(eventType string, handler func(context.Context, json.RawMessage) error) {
	wh.mu.Lock()
	defer wh.mu.Unlock()

	wh.handlers[eventType] = handler
}

// Common event handler implementations
func HandleMessageEvent(ctx context.Context, payload json.RawMessage) error {
	var msg types.MessagePayload
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message payload: %w", err)
	}

	// Process the message based on its type
	switch msg.Type {
	case "text":
		// Handle text message
		return handleTextMessage(ctx, &msg)
	case "image":
		// Handle image message
		return handleImageMessage(ctx, &msg)
	default:
		return fmt.Errorf("unsupported message type: %s", msg.Type)
	}
}

func handleTextMessage(ctx context.Context, msg *types.MessagePayload) error {
	// Implementation for handling text messages
	return nil
}

func handleImageMessage(ctx context.Context, msg *types.MessagePayload) error {
	// Implementation for handling image messages
	return nil
}
