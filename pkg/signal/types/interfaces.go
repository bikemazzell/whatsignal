package types

import (
	"context"
)

type Client interface {
	SendMessage(ctx context.Context, recipient, message string, attachments []string) (*SendMessageResponse, error)
	ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]SignalMessage, error)
	InitializeDevice(ctx context.Context) error
}
