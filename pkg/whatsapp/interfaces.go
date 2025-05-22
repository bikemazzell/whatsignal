package whatsapp

import (
	"context"
	"encoding/json"

	"whatsignal/pkg/whatsapp/types"
)

// SessionManager handles WhatsApp session lifecycle
type SessionManager interface {
	Create(ctx context.Context, name string) (*types.Session, error)
	Get(ctx context.Context, name string) (*types.Session, error)
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Delete(ctx context.Context, name string) error
}

// WebhookHandler handles incoming webhook events
type WebhookHandler interface {
	Handle(ctx context.Context, event *types.WebhookEvent) error
	RegisterEventHandler(eventType string, handler func(context.Context, json.RawMessage) error)
}
