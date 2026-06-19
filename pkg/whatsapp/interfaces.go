package whatsapp

import (
	"context"

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
