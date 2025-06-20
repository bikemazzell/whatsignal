package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"whatsignal/internal/models"
	"whatsignal/pkg/media"
	"whatsignal/pkg/signal"
	signaltypes "whatsignal/pkg/signal/types"
	"whatsignal/pkg/whatsapp/types"

	"github.com/sirupsen/logrus"
)

type MessageBridge interface {
	SendMessage(ctx context.Context, msg *models.Message) error
	HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content string, mediaPath string) error
	HandleSignalMessage(ctx context.Context, msg *signaltypes.SignalMessage) error
	UpdateDeliveryStatus(ctx context.Context, msgID string, status models.DeliveryStatus) error
	CleanupOldRecords(ctx context.Context, retentionDays int) error
}

type DatabaseService interface {
	SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error
	GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error)
	GetMessageMappingBySignalID(ctx context.Context, signalID string) (*models.MessageMapping, error)
	UpdateDeliveryStatus(ctx context.Context, id string, status string) error
	CleanupOldRecords(retentionDays int) error
}

type bridge struct {
	waClient                   types.WAClient
	sigClient                  signal.Client
	db                         DatabaseService
	media                      media.Handler
	retryConfig                models.RetryConfig
	logger                     *logrus.Logger
	signalDestinationPhoneNumber string
	contactService             ContactServiceInterface
}

func NewBridge(waClient types.WAClient, sigClient signal.Client, db DatabaseService, mh media.Handler, rc models.RetryConfig, signalDestinationPhoneNumber string, contactService ContactServiceInterface) MessageBridge {
	return &bridge{
		waClient:                     waClient,
		sigClient:                    sigClient,
		db:                           db,
		media:                        mh,
		retryConfig:                  rc,
		logger:                       logrus.New(),
		signalDestinationPhoneNumber: signalDestinationPhoneNumber,
		contactService:               contactService,
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
		_, err := b.sigClient.SendMessage(ctx, msg.ThreadID, msg.Content, attachments)
		if err != nil {
			return fmt.Errorf("failed to send Signal message: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported platform: %s", msg.Platform)
	}
}

func (b *bridge) HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content string, mediaPath string) error {
	senderPhone := sender
	if strings.HasSuffix(sender, "@c.us") {
		senderPhone = strings.TrimSuffix(sender, "@c.us")
	}

	displayName := senderPhone // fallback
	if b.contactService != nil {
		displayName = b.contactService.GetContactDisplayName(ctx, senderPhone)
	}

	message := fmt.Sprintf("%s: %s", displayName, content)
	var attachments []string

	if mediaPath != "" {
		processedPath, err := b.media.ProcessMedia(mediaPath)
		if err != nil {
			return fmt.Errorf("failed to process media: %w", err)
		}
		attachments = append(attachments, processedPath)
	}

	resp, err := b.sigClient.SendMessage(ctx, b.signalDestinationPhoneNumber, message, attachments)
	if err != nil {
		return fmt.Errorf("failed to send signal message: %w", err)
	}
	
	b.logger.WithFields(logrus.Fields{
		"whatsappMsgID": msgID,
		"signalMsgID": resp.MessageID,
	}).Debug("Saving message mapping - WhatsApp to Signal")

	mapping := &models.MessageMapping{
		WhatsAppChatID:  chatID,
		WhatsAppMsgID:   msgID,
		SignalMsgID:     resp.MessageID,
		SignalTimestamp: time.Unix(resp.Timestamp/1000, 0),
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

func (b *bridge) HandleSignalMessage(ctx context.Context, msg *signaltypes.SignalMessage) error {
	if strings.HasPrefix(msg.Sender, "group.") {
		return b.handleSignalGroupMessage(ctx, msg)
	}

	if msg.QuotedMessage == nil {
		return b.handleNewSignalThread(ctx, msg)
	}

	b.logger.WithFields(logrus.Fields{
		"quotedMessageID": msg.QuotedMessage.ID,
	}).Debug("Looking up message mapping for quoted message")
	
	mapping, err := b.db.GetMessageMappingBySignalID(ctx, msg.QuotedMessage.ID)
	if err != nil {
		b.logger.WithFields(logrus.Fields{
			"quotedMessageID": msg.QuotedMessage.ID,
			"error": err,
		}).Error("Failed to get message mapping for quoted message")
		return fmt.Errorf("failed to get message mapping for quoted message: %w", err)
	}
	
	if mapping != nil {
		b.logger.WithFields(logrus.Fields{
			"quotedMessageID": msg.QuotedMessage.ID,
		}).Debug("Found message mapping in database")
	} else {
		b.logger.WithFields(logrus.Fields{
			"quotedMessageID": msg.QuotedMessage.ID,
		}).Debug("No message mapping found in database, trying fallback")
	}

	if mapping == nil {
		if strings.Contains(msg.QuotedMessage.Text, ": ") {
			parts := strings.SplitN(msg.QuotedMessage.Text, ": ", 2)
			if len(parts) == 2 {
				senderInfo := parts[0]
				
				b.logger.Debug("Attempting fallback extraction from quoted text")
				
				senderInfo = strings.TrimPrefix(senderInfo, "📱 ")
				
				var phoneNumber string
				if len(senderInfo) >= 10 && strings.ContainsAny(senderInfo, "0123456789") {
					for _, char := range senderInfo {
						if char >= '0' && char <= '9' {
							phoneNumber += string(char)
						}
					}
					if len(phoneNumber) >= 10 {
						whatsappChatID := phoneNumber + "@c.us"
						b.logger.Debug("Extracted phone number from quoted text for fallback")
						mapping = &models.MessageMapping{
							WhatsAppChatID: whatsappChatID,
						}
					}
				}
				
			}
		}
		
		if mapping == nil {
			b.logger.WithField("quotedMessageID", msg.QuotedMessage.ID).Error("No mapping found for quoted message")
			return fmt.Errorf("no mapping found for quoted message: %s", msg.QuotedMessage.ID)
		}
	}
	
	whatsappChatID := mapping.WhatsAppChatID

	attachments, err := b.processSignalAttachments(msg.Attachments)
	if err != nil {
		return fmt.Errorf("failed to process attachments: %w", err)
	}

	var resp *types.SendMessageResponse
	var sendErr error

	switch {
	case len(attachments) > 0 && isImageAttachment(attachments[0]):
		resp, sendErr = b.waClient.SendImage(ctx, whatsappChatID, attachments[0], msg.Message)
	case len(attachments) > 0 && isVideoAttachment(attachments[0]):
		resp, sendErr = b.waClient.SendVideo(ctx, whatsappChatID, attachments[0], msg.Message)
	case len(attachments) > 0 && isDocumentAttachment(attachments[0]):
		resp, sendErr = b.waClient.SendDocument(ctx, whatsappChatID, attachments[0], msg.Message)
	default:
		resp, sendErr = b.waClient.SendText(ctx, whatsappChatID, msg.Message)
	}

	if sendErr != nil {
		return fmt.Errorf("failed to send whatsapp message: %w", sendErr)
	}

	newMapping := &models.MessageMapping{
		WhatsAppChatID:  whatsappChatID,
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

func (b *bridge) handleSignalGroupMessage(ctx context.Context, msg *signaltypes.SignalMessage) error {
	b.logger.WithFields(logrus.Fields{
		"messageID": msg.MessageID,
		"sender":    msg.Sender,
	}).Warn("Group messages are not yet supported - message ignored")

	return nil
}

func (b *bridge) handleNewSignalThread(ctx context.Context, msg *signaltypes.SignalMessage) error {
	b.logger.WithFields(logrus.Fields{
		"messageID": msg.MessageID,
		"sender":    msg.Sender,
	}).Warn("New thread creation is not yet supported - message ignored")

	return nil
}
