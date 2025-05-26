package types

import (
	"context"
)

type WAClient interface {
	SendText(ctx context.Context, chatID, message string) (*SendMessageResponse, error)
	SendImage(ctx context.Context, chatID, imagePath, caption string) (*SendMessageResponse, error)
	SendVideo(ctx context.Context, chatID, videoPath, caption string) (*SendMessageResponse, error)
	SendDocument(ctx context.Context, chatID, docPath, caption string) (*SendMessageResponse, error)
	SendFile(ctx context.Context, chatID, filePath, caption string) (*SendMessageResponse, error)
	SendVoice(ctx context.Context, chatID, voicePath string) (*SendMessageResponse, error)
	CreateSession(ctx context.Context) error
	StartSession(ctx context.Context) error
	StopSession(ctx context.Context) error
	GetSessionStatus(ctx context.Context) (*Session, error)
}

type SessionManager interface {
	Create(ctx context.Context, name string) (*Session, error)
	Get(ctx context.Context, name string) (*Session, error)
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Delete(ctx context.Context, name string) error
}

type WebhookHandler interface {
	Handle(ctx context.Context, event *WebhookEvent) error
	RegisterEventHandler(eventType string, handler func(context.Context, []byte) error)
}
