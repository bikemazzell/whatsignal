package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/whatsignal/internal/database"
	"github.com/whatsignal/internal/models"
	"github.com/whatsignal/pkg/media"
	"github.com/whatsignal/pkg/signal"
	"github.com/whatsignal/pkg/whatsapp"
)

type Bridge struct {
	db          *database.Database
	whatsapp    *whatsapp.Client
	signal      *signal.Client
	media       *media.Handler
	retryConfig RetryConfig
}

type RetryConfig struct {
	InitialBackoff int
	MaxBackoff     int
	MaxAttempts    int
}

func NewBridge(db *database.Database, wa *whatsapp.Client, sig *signal.Client, mh *media.Handler, rc RetryConfig) *Bridge {
	return &Bridge{
		db:          db,
		whatsapp:    wa,
		signal:      sig,
		media:       mh,
		retryConfig: rc,
	}
}

func (b *Bridge) HandleWhatsAppMessage(chatID, msgID, sender, content string, mediaPath string) error {
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

	resp, err := b.signal.SendMessage(chatID, message, attachments)
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

	if err := b.db.SaveMessageMapping(mapping); err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (b *Bridge) HandleSignalMessage(msg *signal.SignalMessage) error {
	if msg.QuotedMessage == nil {
		return fmt.Errorf("message is not a reply, ignoring")
	}

	mapping, err := b.db.GetMessageMappingByWhatsAppID(msg.QuotedMessage.ID)
	if err != nil {
		return fmt.Errorf("failed to get message mapping: %w", err)
	}

	if mapping == nil {
		return fmt.Errorf("no mapping found for quoted message")
	}

	var attachments []string
	if len(msg.Attachments) > 0 {
		for _, attachment := range msg.Attachments {
			processedPath, err := b.media.ProcessMedia(attachment)
			if err != nil {
				return fmt.Errorf("failed to process media: %w", err)
			}
			attachments = append(attachments, processedPath)
		}
	}

	var resp *whatsapp.SendMessageResponse
	var sendErr error

	if len(attachments) > 0 {
		resp, sendErr = b.whatsapp.SendMedia(mapping.WhatsAppChatID, attachments[0], msg.Message)
	} else {
		resp, sendErr = b.whatsapp.SendText(mapping.WhatsAppChatID, msg.Message)
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
	}

	if err := b.db.SaveMessageMapping(newMapping); err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (b *Bridge) UpdateDeliveryStatus(msgID string, status models.DeliveryStatus) error {
	return b.db.UpdateDeliveryStatus(msgID, status)
}

func (b *Bridge) CleanupOldRecords(retentionDays int) error {
	if err := b.db.CleanupOldRecords(retentionDays); err != nil {
		return fmt.Errorf("failed to cleanup old records: %w", err)
	}

	if err := b.media.CleanupOldFiles(int64(retentionDays * 24 * 60 * 60)); err != nil {
		return fmt.Errorf("failed to cleanup old media files: %w", err)
	}

	return nil
}
