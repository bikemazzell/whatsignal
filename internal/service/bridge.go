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
	HandleSignalMessageDeletion(ctx context.Context, targetMessageID string, sender string) error
	UpdateDeliveryStatus(ctx context.Context, msgID string, status models.DeliveryStatus) error
	CleanupOldRecords(ctx context.Context, retentionDays int) error
}

type DatabaseService interface {
	SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error
	GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error)
	GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error)
	GetMessageMappingBySignalID(ctx context.Context, signalID string) (*models.MessageMapping, error)
	GetLatestMessageMappingByWhatsAppChatID(ctx context.Context, whatsappChatID string) (*models.MessageMapping, error)
	GetLatestMessageMapping(ctx context.Context) (*models.MessageMapping, error)
	UpdateDeliveryStatus(ctx context.Context, id string, status string) error
	CleanupOldRecords(retentionDays int) error
}

type bridge struct {
	waClient                   types.WAClient
	sigClient                  signal.Client
	db                         DatabaseService
	media                      media.Handler
	retryConfig                models.RetryConfig
	mediaConfig                models.MediaConfig
	logger                     *logrus.Logger
	signalDestinationPhoneNumber string
	contactService             ContactServiceInterface
}

func NewBridge(waClient types.WAClient, sigClient signal.Client, db DatabaseService, mh media.Handler, rc models.RetryConfig, mc models.MediaConfig, signalDestinationPhoneNumber string, contactService ContactServiceInterface) MessageBridge {
	return &bridge{
		waClient:                     waClient,
		sigClient:                    sigClient,
		db:                           db,
		media:                        mh,
		retryConfig:                  rc,
		mediaConfig:                  mc,
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
	// Enhanced debugging: Log the complete message structure for analysis
	b.logger.WithFields(logrus.Fields{
		"messageID":     msg.MessageID,
		"sender":        msg.Sender,
		"timestamp":     msg.Timestamp,
		"message":       msg.Message,
		"attachments":   msg.Attachments,
		"quotedMessage": msg.QuotedMessage,
		"reaction":      msg.Reaction,
		"deletion":      msg.Deletion,
		"messageLength": len(msg.Message),
		"hasReaction":   msg.Reaction != nil,
		"hasDeletion":   msg.Deletion != nil,
		"hasQuoted":     msg.QuotedMessage != nil,
		"hasAttachments": len(msg.Attachments) > 0,
	}).Debug("Received Signal message - full structure analysis")

	if strings.HasPrefix(msg.Sender, "group.") {
		return b.handleSignalGroupMessage(ctx, msg)
	}

	// Handle reactions
	if msg.Reaction != nil {
		return b.handleSignalReaction(ctx, msg)
	}

	// Handle message deletions
	if msg.Deletion != nil {
		return b.handleSignalDeletion(ctx, msg)
	}

	var mapping *models.MessageMapping
	var err error
	
	if msg.QuotedMessage == nil {
		// No quoted message - find the latest message that was sent to the Signal user (auto-reply to last sender)
		b.logger.WithFields(logrus.Fields{
			"signalSender": msg.Sender,
		}).Debug("No quoted message, looking up latest message for auto-reply to last WhatsApp sender")
		
		mapping, err = b.db.GetLatestMessageMapping(ctx)
		if err != nil {
			b.logger.WithFields(logrus.Fields{
				"signalSender": msg.Sender,
				"error": err,
			}).Error("Failed to get latest message mapping for auto-reply")
			return fmt.Errorf("failed to get latest message mapping for auto-reply: %w", err)
		}
		
		if mapping == nil {
			// No previous messages found - this is a new conversation
			return b.handleNewSignalThread(ctx, msg)
		}
		
		b.logger.WithFields(logrus.Fields{
			"signalSender": msg.Sender,
			"replyToWhatsAppChat": mapping.WhatsAppChatID,
			"latestMessageID": mapping.WhatsAppMsgID,
			"latestMessageTime": mapping.ForwardedAt,
		}).Debug("Found latest message for auto-reply - will reply to last WhatsApp sender")
	} else {
		// Has quoted message - use existing logic
		b.logger.WithFields(logrus.Fields{
			"quotedMessageID": msg.QuotedMessage.ID,
		}).Debug("Looking up message mapping for quoted message")
		
		mapping, err = b.db.GetMessageMapping(ctx, msg.QuotedMessage.ID)
		if err != nil {
			b.logger.WithFields(logrus.Fields{
				"quotedMessageID": msg.QuotedMessage.ID,
				"error": err,
			}).Error("Failed to get message mapping for quoted message")
			return fmt.Errorf("failed to get message mapping for quoted message: %w", err)
		}
	}
	
	if mapping != nil {
		if msg.QuotedMessage != nil {
			b.logger.WithFields(logrus.Fields{
				"quotedMessageID": msg.QuotedMessage.ID,
			}).Debug("Found message mapping in database")
		} else {
			b.logger.WithFields(logrus.Fields{
				"whatsappChatID": mapping.WhatsAppChatID,
			}).Debug("Found latest message mapping for auto-reply")
		}
	} else {
		if msg.QuotedMessage != nil {
			b.logger.WithFields(logrus.Fields{
				"quotedMessageID": msg.QuotedMessage.ID,
			}).Debug("No message mapping found in database, trying fallback")
		}
	}

	if mapping == nil && msg.QuotedMessage != nil {
		// Try fallback extraction from quoted message text only if we have a quoted message
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

	b.logger.WithFields(logrus.Fields{
		"messageID":       msg.MessageID,
		"sender":          msg.Sender,
		"attachmentCount": len(msg.Attachments),
	}).Debug("Processing Signal message with attachments")

	attachments, err := b.processSignalAttachments(msg.Attachments)
	if err != nil {
		return fmt.Errorf("failed to process attachments: %w", err)
	}

	b.logger.WithFields(logrus.Fields{
		"messageID":      msg.MessageID,
		"processedCount": len(attachments),
	}).Debug("Processed Signal attachments")

	var resp *types.SendMessageResponse
	var sendErr error



	switch {
	case len(attachments) > 0 && b.isImageAttachment(attachments[0]):
		b.logger.WithFields(logrus.Fields{
			"messageID": msg.MessageID,
			"method":    "SendImage",
		}).Debug("Sending image to WhatsApp")
		resp, sendErr = b.waClient.SendImage(ctx, whatsappChatID, attachments[0], msg.Message)
	case len(attachments) > 0 && b.isVideoAttachment(attachments[0]):
		b.logger.WithFields(logrus.Fields{
			"messageID": msg.MessageID,
			"method":    "SendVideo",
		}).Debug("Sending video to WhatsApp")
		resp, sendErr = b.waClient.SendVideo(ctx, whatsappChatID, attachments[0], msg.Message)
	case len(attachments) > 0 && b.isVoiceAttachment(attachments[0]):
		b.logger.WithFields(logrus.Fields{
			"messageID": msg.MessageID,
			"method":    "SendVoice",
		}).Debug("Sending voice to WhatsApp")
		resp, sendErr = b.waClient.SendVoice(ctx, whatsappChatID, attachments[0])
	case len(attachments) > 0:
		// Default: treat all other attachments (including configured documents and unrecognized files) as documents
		b.logger.WithFields(logrus.Fields{
			"messageID": msg.MessageID,
			"method":    "SendDocument",
			"filePath":  attachments[0],
		}).Debug("Sending attachment as document to WhatsApp")
		resp, sendErr = b.waClient.SendDocument(ctx, whatsappChatID, attachments[0], msg.Message)
	default:
		// Only send text if there's actually text content
		if msg.Message != "" {
			b.logger.WithFields(logrus.Fields{
				"messageID": msg.MessageID,
				"method":    "SendText",
			}).Debug("Sending text to WhatsApp")
			resp, sendErr = b.waClient.SendText(ctx, whatsappChatID, msg.Message)
		} else {
			// Log detailed information about empty messages for debugging potential deletion events
			b.logger.WithFields(logrus.Fields{
				"messageID":     msg.MessageID,
				"sender":        msg.Sender,
				"timestamp":     msg.Timestamp,
				"message":       msg.Message,
				"attachments":   msg.Attachments,
				"quotedMessage": msg.QuotedMessage,
				"reaction":      msg.Reaction,
				"deletion":      msg.Deletion,
			}).Debug("Detailed analysis of empty message - potential deletion event")
			
			b.logger.WithFields(logrus.Fields{
				"messageID": msg.MessageID,
			}).Warn("Skipping empty message with no attachments")
			return nil // Skip empty messages
		}
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
	
	b.logger.WithFields(logrus.Fields{
		"whatsappMsgID": resp.MessageID,
		"signalMsgID": msg.MessageID,
	}).Debug("Saving message mapping for Signal to WhatsApp")

	if len(attachments) > 0 {
		newMapping.MediaPath = &attachments[0]
		newMapping.MediaType = b.getMediaType(attachments[0])
	}

	if err := b.db.SaveMessageMapping(ctx, newMapping); err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (b *bridge) HandleSignalMessageDeletion(ctx context.Context, targetMessageID string, sender string) error {
	b.logger.WithFields(logrus.Fields{
		"targetMessageID": targetMessageID,
		"sender":          sender,
	}).Debug("Processing Signal message deletion")

	// Look up the message mapping for the target message by Signal ID
	mapping, err := b.db.GetMessageMappingBySignalID(ctx, targetMessageID)
	if err != nil {
		b.logger.WithFields(logrus.Fields{
			"targetMessageID": targetMessageID,
			"error":           err,
		}).Error("Failed to get message mapping for deletion")
		return fmt.Errorf("failed to get message mapping for deletion: %w", err)
	}

	if mapping == nil {
		b.logger.WithField("targetMessageID", targetMessageID).Warn("No mapping found for deletion target message")
		return fmt.Errorf("no mapping found for deletion target message: %s", targetMessageID)
	}

	// Delete the message in WhatsApp
	err = b.waClient.DeleteMessage(ctx, mapping.WhatsAppChatID, mapping.WhatsAppMsgID)
	if err != nil {
		b.logger.WithFields(logrus.Fields{
			"whatsappChatID": mapping.WhatsAppChatID,
			"whatsappMsgID":  mapping.WhatsAppMsgID,
			"error":          err,
		}).Error("Failed to delete message in WhatsApp")
		return fmt.Errorf("failed to delete message in WhatsApp: %w", err)
	}

	b.logger.WithFields(logrus.Fields{
		"whatsappChatID":  mapping.WhatsAppChatID,
		"whatsappMsgID":   mapping.WhatsAppMsgID,
		"targetMessageID": targetMessageID,
	}).Info("Successfully deleted message in WhatsApp")

	return nil
}

func (b *bridge) processSignalAttachments(attachments []string) ([]string, error) {
	if len(attachments) == 0 {
		return nil, nil
	}

	b.logger.WithField("attachments", attachments).Debug("Processing Signal attachments")

	var processed []string
	for i, attachment := range attachments {
		b.logger.WithFields(logrus.Fields{
			"attachment": attachment,
			"index":      i + 1,
			"total":      len(attachments),
		}).Debug("Processing individual attachment")

		processedPath, err := b.media.ProcessMedia(attachment)
		if err != nil {
			b.logger.WithFields(logrus.Fields{
				"attachment": attachment,
				"error":      err.Error(),
			}).Error("Failed to process attachment, skipping")
			// Continue processing other attachments instead of failing completely
			continue
		}

		b.logger.WithFields(logrus.Fields{
			"original":  attachment,
			"processed": processedPath,
		}).Debug("Successfully processed attachment")

		processed = append(processed, processedPath)
	}

	// If no attachments were successfully processed, log a warning but don't fail
	if len(processed) == 0 && len(attachments) > 0 {
		b.logger.WithField("originalCount", len(attachments)).Error("No attachments could be processed successfully")
	} else if len(attachments) > 0 {
		b.logger.WithFields(logrus.Fields{
			"originalCount":  len(attachments),
			"processedCount": len(processed),
		}).Debug("Attachment processing completed successfully")
	}

	return processed, nil
}

func (b *bridge) isImageAttachment(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	for _, allowedExt := range b.mediaConfig.AllowedTypes.Image {
		if ext == strings.ToLower(allowedExt) {
			return true
		}
	}
	return false
}

func (b *bridge) isVideoAttachment(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	for _, allowedExt := range b.mediaConfig.AllowedTypes.Video {
		if ext == strings.ToLower(allowedExt) {
			return true
		}
	}
	return false
}

func (b *bridge) isDocumentAttachment(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	for _, allowedExt := range b.mediaConfig.AllowedTypes.Document {
		if ext == strings.ToLower(allowedExt) {
			return true
		}
	}
	return false
}

func (b *bridge) isVoiceAttachment(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	for _, allowedExt := range b.mediaConfig.AllowedTypes.Voice {
		if ext == strings.ToLower(allowedExt) {
			return true
		}
	}
	return false
}

func (b *bridge) getMediaType(path string) string {
	switch {
	case b.isImageAttachment(path):
		return "image"
	case b.isVideoAttachment(path):
		return "video"
	case b.isVoiceAttachment(path):
		return "voice"
	default:
		return "document" // Default everything else to document
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

func (b *bridge) handleSignalReaction(ctx context.Context, msg *signaltypes.SignalMessage) error {
	b.logger.WithFields(logrus.Fields{
		"messageID": msg.MessageID,
		"sender":    msg.Sender,
		"reaction":  msg.Reaction.Emoji,
		"targetTimestamp": msg.Reaction.TargetTimestamp,
		"isRemove": msg.Reaction.IsRemove,
	}).Debug("Processing Signal reaction")

	// Find the original message mapping by Signal timestamp
	targetID := fmt.Sprintf("%d", msg.Reaction.TargetTimestamp)
	mapping, err := b.db.GetMessageMapping(ctx, targetID)
	if err != nil {
		b.logger.WithError(err).Error("Failed to get message mapping for reaction target")
		return fmt.Errorf("failed to get message mapping for reaction target: %w", err)
	}

	if mapping == nil {
		b.logger.WithField("targetID", targetID).Warn("No mapping found for reaction target message")
		return fmt.Errorf("no mapping found for reaction target message: %s", targetID)
	}

	// Send reaction to WhatsApp
	reaction := msg.Reaction.Emoji
	if msg.Reaction.IsRemove {
		// Empty string removes the reaction in WAHA
		reaction = ""
	}


	resp, err := b.waClient.SendReaction(ctx, mapping.WhatsAppChatID, mapping.WhatsAppMsgID, reaction)
	if err != nil {
		b.logger.WithError(err).Error("Failed to send reaction to WhatsApp")
		return fmt.Errorf("failed to send reaction to WhatsApp: %w", err)
	}

	b.logger.WithFields(logrus.Fields{
		"whatsappMsgID": mapping.WhatsAppMsgID,
		"reaction": reaction,
		"response": resp,
	}).Info("Successfully forwarded reaction to WhatsApp")

	return nil
}

func (b *bridge) handleSignalDeletion(ctx context.Context, msg *signaltypes.SignalMessage) error {
	b.logger.WithFields(logrus.Fields{
		"messageID":        msg.MessageID,
		"sender":           msg.Sender,
		"targetMessageID":  msg.Deletion.TargetMessageID,
		"targetTimestamp":  msg.Deletion.TargetTimestamp,
	}).Debug("Processing Signal message deletion")

	// Use the target message ID or timestamp to find the message to delete
	var targetID string
	if msg.Deletion.TargetMessageID != "" {
		targetID = msg.Deletion.TargetMessageID
	} else {
		// Fallback to timestamp if message ID is not available
		targetID = fmt.Sprintf("%d", msg.Deletion.TargetTimestamp)
	}

	return b.HandleSignalMessageDeletion(ctx, targetID, msg.Sender)
}
