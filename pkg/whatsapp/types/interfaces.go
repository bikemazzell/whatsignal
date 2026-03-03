package types

import (
	"context"
	"time"
)

type WAClient interface {
	SendTextWithSession(ctx context.Context, chatID, message, replyTo, sessionName string) (*SendMessageResponse, error)
	SendImageWithSession(ctx context.Context, chatID, imagePath, caption, replyTo, sessionName string) (*SendMessageResponse, error)
	SendVideoWithSession(ctx context.Context, chatID, videoPath, caption, replyTo, sessionName string) (*SendMessageResponse, error)
	SendDocumentWithSession(ctx context.Context, chatID, docPath, caption, replyTo, sessionName string) (*SendMessageResponse, error)
	SendVoiceWithSession(ctx context.Context, chatID, voicePath, replyTo, sessionName string) (*SendMessageResponse, error)
	SendReactionWithSession(ctx context.Context, chatID, messageID, reaction, sessionName string) (*SendMessageResponse, error)
	DeleteMessage(ctx context.Context, chatID, messageID string) error
	CreateSession(ctx context.Context) error
	StartSession(ctx context.Context) error
	StopSession(ctx context.Context) error
	RestartSession(ctx context.Context) error
	GetSessionStatus(ctx context.Context) (*Session, error)
	GetSessionStatusByName(ctx context.Context, sessionName string) (*Session, error)
	WaitForSessionReady(ctx context.Context, maxWaitTime time.Duration) error
	GetSessionName() string

	// Contact methods
	GetContact(ctx context.Context, contactID string) (*Contact, error)
	GetAllContacts(ctx context.Context, limit, offset int) ([]Contact, error)

	// Group methods
	GetGroup(ctx context.Context, groupID string) (*Group, error)
	GetAllGroups(ctx context.Context, limit, offset int) ([]Group, error)

	// Message acknowledgment
	AckMessage(ctx context.Context, chatID, sessionName string) error

	// Health check
	HealthCheck(ctx context.Context) error
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
