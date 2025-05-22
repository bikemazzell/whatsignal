package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"whatsignal/internal/models"
	"whatsignal/pkg/media"
	"whatsignal/pkg/signal"
	"whatsignal/pkg/whatsapp"
	"whatsignal/pkg/whatsapp/types"
)

type MessageBridge interface {
	SendMessage(ctx context.Context, msg *models.Message) error
	HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content string, mediaPath string) error
	HandleSignalMessage(ctx context.Context, msg *signal.SignalMessage) error
	UpdateDeliveryStatus(ctx context.Context, msgID string, status models.DeliveryStatus) error
	CleanupOldRecords(ctx context.Context, retentionDays int) error
}

type DatabaseService interface {
	SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error
	GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error)
	GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error)
	UpdateDeliveryStatus(ctx context.Context, id string, status string) error
	CleanupOldRecords(retentionDays int) error
}

type bridge struct {
	waClient    whatsapp.WAClient
	sigClient   signal.Client
	db          DatabaseService
	media       media.Handler
	retryConfig RetryConfig
}

type RetryConfig struct {
	InitialBackoff int
	MaxBackoff     int
	MaxAttempts    int
}

func NewBridge(waClient whatsapp.WAClient, sigClient signal.Client, db DatabaseService, mh media.Handler, rc RetryConfig) MessageBridge {
	return &bridge{
		waClient:    waClient,
		sigClient:   sigClient,
		db:          db,
		media:       mh,
		retryConfig: rc,
	}
}

func (b *bridge) SendMessage(ctx context.Context, msg *models.Message) error {
	switch msg.Platform {
	case "whatsapp":
		resp, err := b.waClient.SendText(ctx, msg.ChatID, msg.Content)
		if err != nil {
			return fmt.Errorf("failed to send WhatsApp message: %w", err)
		}
		if resp.Status != "sent" {
			return fmt.Errorf("WhatsApp message not sent: %s", resp.Error)
		}
		return nil

	case "signal":
		attachments := []string{}
		if msg.MediaPath != "" {
			attachments = append(attachments, msg.MediaPath)
		}
		_, err := b.sigClient.SendMessage(msg.ThreadID, msg.Content, attachments)
		if err != nil {
			return fmt.Errorf("failed to send Signal message: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported platform: %s", msg.Platform)
	}
}

func (b *bridge) HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content string, mediaPath string) error {
	metadata := models.MessageMetadata{
		Sender:   sender,
		Chat:     chatID,
		Time:     time.Now(),
		MsgID:    msgID,
		ThreadID: fmt.Sprintf("wa-%s", msgID),
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	message := fmt.Sprintf("%s\n---\n%s", string(metadataJSON), content)
	var attachments []string

	if mediaPath != "" {
		processedPath, err := b.media.ProcessMedia(mediaPath)
		if err != nil {
			return fmt.Errorf("failed to process media: %w", err)
		}
		attachments = append(attachments, processedPath)
	}

	resp, err := b.sigClient.SendMessage(chatID, message, attachments)
	if err != nil {
		return fmt.Errorf("failed to send signal message: %w", err)
	}

	mapping := &models.MessageMapping{
		WhatsAppChatID:  chatID,
		WhatsAppMsgID:   msgID,
		SignalMsgID:     resp.Result.MessageID,
		SignalTimestamp: time.Unix(resp.Result.Timestamp/1000, 0),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
	}

	if len(attachments) > 0 {
		mapping.MediaPath = &attachments[0]
	}

	if err := b.db.SaveMessageMapping(ctx, mapping); err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (b *bridge) HandleSignalMessage(ctx context.Context, msg *signal.SignalMessage) error {
	// Handle group messages
	if strings.HasPrefix(msg.Sender, "group.") {
		return b.handleSignalGroupMessage(ctx, msg)
	}

	// Handle direct messages
	if msg.QuotedMessage == nil {
		// For messages without quotes, create a new thread
		return b.handleNewSignalThread(ctx, msg)
	}

	// Handle replies
	mapping, err := b.db.GetMessageMappingByWhatsAppID(ctx, msg.QuotedMessage.ID)
	if err != nil {
		return fmt.Errorf("failed to get message mapping: %w", err)
	}

	if mapping == nil {
		return fmt.Errorf("no mapping found for quoted message")
	}

	// Process attachments
	attachments, err := b.processSignalAttachments(msg.Attachments)
	if err != nil {
		return fmt.Errorf("failed to process attachments: %w", err)
	}

	var resp *types.SendMessageResponse
	var sendErr error

	// Handle different message types
	switch {
	case len(attachments) > 0 && isImageAttachment(attachments[0]):
		resp, sendErr = b.waClient.SendImage(ctx, mapping.WhatsAppChatID, attachments[0], msg.Message)
	case len(attachments) > 0 && isVideoAttachment(attachments[0]):
		resp, sendErr = b.waClient.SendVideo(ctx, mapping.WhatsAppChatID, attachments[0], msg.Message)
	case len(attachments) > 0 && isDocumentAttachment(attachments[0]):
		resp, sendErr = b.waClient.SendDocument(ctx, mapping.WhatsAppChatID, attachments[0], msg.Message)
	default:
		resp, sendErr = b.waClient.SendText(ctx, mapping.WhatsAppChatID, msg.Message)
	}

	if sendErr != nil {
		return fmt.Errorf("failed to send whatsapp message: %w", sendErr)
	}

	newMapping := &models.MessageMapping{
		WhatsAppChatID:  mapping.WhatsAppChatID,
		WhatsAppMsgID:   resp.MessageID,
		SignalMsgID:     msg.MessageID,
		SignalTimestamp: time.Unix(msg.Timestamp/1000, 0),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
	}

	if len(attachments) > 0 {
		newMapping.MediaPath = &attachments[0]
		newMapping.MediaType = getMediaType(attachments[0])
	}

	if err := b.db.SaveMessageMapping(ctx, newMapping); err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (b *bridge) processSignalAttachments(attachments []string) ([]string, error) {
	if len(attachments) == 0 {
		return nil, nil
	}

	var processed []string
	for _, attachment := range attachments {
		processedPath, err := b.media.ProcessMedia(attachment)
		if err != nil {
			return nil, fmt.Errorf("failed to process media %s: %w", attachment, err)
		}
		processed = append(processed, processedPath)
	}
	return processed, nil
}

func isImageAttachment(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png"
}

func isVideoAttachment(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".mp4" || ext == ".mov"
}

func isDocumentAttachment(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".pdf" || ext == ".doc" || ext == ".docx"
}

func getMediaType(path string) string {
	switch {
	case isImageAttachment(path):
		return "image"
	case isVideoAttachment(path):
		return "video"
	case isDocumentAttachment(path):
		return "document"
	default:
		return "unknown"
	}
}

func (b *bridge) UpdateDeliveryStatus(ctx context.Context, msgID string, status models.DeliveryStatus) error {
	return b.db.UpdateDeliveryStatus(ctx, msgID, string(status))
}

func (b *bridge) CleanupOldRecords(ctx context.Context, retentionDays int) error {
	if err := b.db.CleanupOldRecords(retentionDays); err != nil {
		return fmt.Errorf("failed to cleanup old records: %w", err)
	}

	if err := b.media.CleanupOldFiles(int64(retentionDays * 24 * 60 * 60)); err != nil {
		return fmt.Errorf("failed to cleanup old media files: %w", err)
	}

	return nil
}

func (b *bridge) handleSignalGroupMessage(ctx context.Context, msg *signal.SignalMessage) error {
	// For now, we'll just log group messages as they require special handling
	return fmt.Errorf("group messages are not supported yet")
}

func (b *bridge) handleNewSignalThread(ctx context.Context, msg *signal.SignalMessage) error {
	// For now, we'll just log new threads as they require special handling
	return fmt.Errorf("new thread creation is not supported yet")
}
