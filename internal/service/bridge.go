package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"whatsignal/internal/constants"
	intmedia "whatsignal/internal/media"
	"whatsignal/internal/metrics"
	"whatsignal/internal/models"
	"whatsignal/internal/privacy"
	"whatsignal/internal/retry"
	"whatsignal/internal/tracing"
	"whatsignal/pkg/media"
	"whatsignal/pkg/signal"
	signaltypes "whatsignal/pkg/signal/types"
	"whatsignal/pkg/whatsapp/types"

	"github.com/sirupsen/logrus"
)

// nonRetryableSignalErrors contains error substrings that indicate non-retryable Signal errors.
// These errors require manual intervention and should not be retried.
var nonRetryableSignalErrors = []string{
	"Untrusted Identity",   // Identity key changed, requires trust command
	"Unregistered user",    // User not on Signal
	"Invalid registration", // Registration issue
	"Rate limit",           // Rate limited - could be retryable but with longer delays
	"Invalid phone number", // Bad phone number format
	"Forbidden",            // Permission denied
	"Not found",            // User/resource not found
}

// nonRetryableWhatsAppErrors contains error substrings that indicate non-retryable WAHA/WhatsApp errors.
// These errors require manual intervention and should not be retried.
// Note: "session is not ready" errors are now retryable (session status validation with backoff)
var nonRetryableWhatsAppErrors = []string{
	"status 400",        // Bad request - invalid parameters
	"status 401",        // Unauthorized - auth issue
	"status 403",        // Forbidden - permission denied
	"status 404",        // Not found - chat/resource doesn't exist
	"invalid chat",      // Invalid chat ID format
	"not registered",    // User not on WhatsApp
	"blocked",           // User blocked
	"session not found", // Session doesn't exist
}

// isRetryableSignalError determines if a Signal API error should be retried.
// Returns false for errors that require manual intervention or cannot succeed with retries.
func isRetryableSignalError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	for _, nonRetryable := range nonRetryableSignalErrors {
		if strings.Contains(errStr, nonRetryable) {
			return false
		}
	}

	// Default to retryable for unknown/network errors
	return true
}

// isRetryableWhatsAppError determines if a WAHA/WhatsApp API error should be retried.
// Returns true for transient errors like 500 (markedUnread, JS errors), network issues.
// Returns false for errors that require manual intervention.
func isRetryableWhatsAppError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	for _, nonRetryable := range nonRetryableWhatsAppErrors {
		if strings.Contains(errStr, strings.ToLower(nonRetryable)) {
			return false
		}
	}

	// Specifically retryable WAHA errors (500 with transient JS errors)
	if strings.Contains(errStr, "status 500") {
		return true
	}
	if strings.Contains(errStr, "status 502") || strings.Contains(errStr, "status 503") || strings.Contains(errStr, "status 504") {
		return true
	}
	if strings.Contains(errStr, "markedunread") {
		return true
	}

	// Retryable network and timeout errors
	if strings.Contains(errStr, "timeout") {
		return true
	}
	if strings.Contains(errStr, "network") {
		return true
	}
	if strings.Contains(errStr, "connection refused") {
		return true
	}
	if strings.Contains(errStr, "connection reset") {
		return true
	}
	if strings.Contains(errStr, "connection timeout") {
		return true
	}
	if strings.Contains(errStr, "temporary failure") {
		return true
	}
	if strings.Contains(errStr, "temporary error") {
		return true
	}
	if strings.Contains(errStr, "EOF") {
		return true
	}
	if strings.Contains(errStr, "broken pipe") {
		return true
	}

	// Default to retryable for unknown/network errors
	return true
}

// RecordCleaner provides cleanup operations for old records.
// This interface enables components like Scheduler to depend only on the cleanup capability
// rather than the full MessageBridge, following the Interface Segregation Principle.
type RecordCleaner interface {
	CleanupOldRecords(ctx context.Context, retentionDays int) error
}

// MessageBridge provides the full message routing capability between Signal and WhatsApp.
// It embeds RecordCleaner to maintain backward compatibility while supporting ISP.
type MessageBridge interface {
	RecordCleaner
	SendMessage(ctx context.Context, msg *models.Message) error
	HandleWhatsAppMessageWithSession(ctx context.Context, sessionName, chatID, msgID, sender, senderDisplayName, content string, mediaPath string) error
	HandleSignalMessage(ctx context.Context, msg *signaltypes.SignalMessage) error
	HandleSignalMessageWithDestination(ctx context.Context, msg *signaltypes.SignalMessage, destination string) error
	HandleSignalMessageDeletion(ctx context.Context, targetMessageID string, sender string) error
	UpdateDeliveryStatus(ctx context.Context, msgID string, status models.DeliveryStatus) error
	SendSignalNotificationForSession(ctx context.Context, sessionName, message string) error
}

type DatabaseService interface {
	SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error
	GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error)
	GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error)
	GetMessageMappingBySignalID(ctx context.Context, signalID string) (*models.MessageMapping, error)
	GetLatestMessageMappingByWhatsAppChatID(ctx context.Context, whatsappChatID string) (*models.MessageMapping, error)
	GetLatestMessageMapping(ctx context.Context) (*models.MessageMapping, error)
	GetLatestMessageMappingBySession(ctx context.Context, sessionName string) (*models.MessageMapping, error)
	GetLatestGroupMessageMappingBySession(ctx context.Context, sessionName string, searchLimit int) (*models.MessageMapping, error)
	HasMessageHistoryBetween(ctx context.Context, sessionName, signalSender string) (bool, error)
	UpdateDeliveryStatus(ctx context.Context, id string, status string) error
	CleanupOldRecords(ctx context.Context, retentionDays int) error
	GetStaleMessageCount(ctx context.Context, threshold time.Duration) (int, error)
}

type bridge struct {
	waClient             types.WAClient
	sigClient            signal.Client
	db                   DatabaseService
	media                media.Handler
	retryConfig          models.RetryConfig
	mediaConfig          models.MediaConfig
	mediaRouter          intmedia.Router
	logger               *logrus.Logger
	contactService       ContactServiceInterface
	groupService         GroupServiceInterface
	channelManager       *ChannelManager
	signalAttachmentsDir string
}

// NewBridge creates a new bridge with channel manager (channels are required)
func NewBridge(waClient types.WAClient, sigClient signal.Client, db DatabaseService, mh media.Handler, rc models.RetryConfig, mc models.MediaConfig, channelManager *ChannelManager, contactService ContactServiceInterface, groupService GroupServiceInterface, signalAttachmentsDir string, logger *logrus.Logger) MessageBridge {
	return &bridge{
		waClient:             waClient,
		sigClient:            sigClient,
		db:                   db,
		media:                mh,
		retryConfig:          rc,
		mediaConfig:          mc,
		mediaRouter:          intmedia.NewRouter(mc),
		logger:               logger,
		contactService:       contactService,
		groupService:         groupService,
		channelManager:       channelManager,
		signalAttachmentsDir: signalAttachmentsDir,
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

func (b *bridge) HandleWhatsAppMessageWithSession(ctx context.Context, sessionName, chatID, msgID, sender, senderDisplayName, content string, mediaPath string) error {
	startTime := time.Now()
	requestInfo := tracing.GetRequestInfo(ctx)

	// Record message processing attempt
	metrics.IncrementCounter("message_processing_total", map[string]string{
		"direction": "whatsapp_to_signal",
		"session":   sessionName,
		"has_media": fmt.Sprintf("%t", mediaPath != ""),
	}, "Total message processing attempts")

	// Structured logging with privacy masking
	logFields := privacy.MaskSensitiveFields(map[string]interface{}{
		LogFieldRequestID: requestInfo.RequestID,
		LogFieldTraceID:   requestInfo.TraceID,
		LogFieldSession:   sessionName,
		LogFieldChatID:    chatID,
		LogFieldMessageID: msgID,
		LogFieldPlatform:  "whatsapp",
		LogFieldDirection: "incoming",
		"sender":          sender,
		"has_media":       mediaPath != "",
	})

	logrusFields := make(logrus.Fields)
	for k, v := range logFields {
		logrusFields[k] = v
	}

	b.logger.WithFields(logrusFields).Info("Processing WhatsApp message")

	// Extract phone number from sender ID
	// Formats: "12345@c.us" (user), "12345@lid" (linked ID), "12345@g.us" (group - shouldn't happen after server.go fix)
	senderPhone := sender
	if strings.HasSuffix(sender, "@c.us") {
		senderPhone = strings.TrimSuffix(sender, "@c.us")
	} else if strings.HasSuffix(sender, "@lid") {
		// LID (Linked ID) format used by WhatsApp for some user identifiers
		senderPhone = strings.TrimSuffix(sender, "@lid")
	} else if strings.HasSuffix(sender, "@g.us") {
		// Legacy fallback: group ID as sender (should not happen with updated server.go)
		senderPhone = strings.TrimSuffix(sender, "@g.us")
	}

	// Use provided display name if available, otherwise fall back to contact service lookup
	displayName := senderPhone // final fallback
	if senderDisplayName != "" {
		displayName = senderDisplayName
	} else if b.contactService != nil {
		displayName = b.contactService.GetContactDisplayName(ctx, senderPhone)
	}

	// Detect if this is a group message and format accordingly
	var message string
	isGroupMsg := strings.HasSuffix(chatID, "@g.us")
	if isGroupMsg && b.groupService != nil {
		// Get group name
		groupName := b.groupService.GetGroupName(ctx, chatID, sessionName)
		// Format: "John Doe in Family Group: hi there"
		message = fmt.Sprintf("%s in %s: %s", displayName, groupName, content)
	} else {
		// Direct message formatting (existing behavior)
		message = fmt.Sprintf("%s: %s", displayName, content)
	}
	var attachments []string

	if mediaPath != "" {
		processedPath, err := b.media.ProcessMedia(mediaPath)
		if err != nil {
			return fmt.Errorf("failed to process media: %w", err)
		}
		attachments = append(attachments, processedPath)
	}

	// Get the Signal destination based on session
	dest, err := b.channelManager.GetSignalDestination(sessionName)
	if err != nil {
		return fmt.Errorf("failed to get Signal destination for session %s: %w", sessionName, err)
	}
	destinationNumber := dest

	// Prepare retry configuration
	backoffConfig := retry.BackoffConfig{
		InitialDelay: time.Duration(b.retryConfig.InitialBackoffMs) * time.Millisecond,
		MaxDelay:     time.Duration(b.retryConfig.MaxBackoffMs) * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  b.retryConfig.MaxAttempts,
		Jitter:       true,
	}

	b.logger.WithFields(logrus.Fields{
		"InitialBackoffMs": b.retryConfig.InitialBackoffMs,
		"MaxBackoffMs":     b.retryConfig.MaxBackoffMs,
		"MaxAttempts":      b.retryConfig.MaxAttempts,
	}).Info("Retry configuration for Signal send")

	backoff := retry.NewBackoff(backoffConfig)

	var resp *signaltypes.SendMessageResponse
	retryErr := backoff.RetryWithPredicate(ctx, func() error {
		var sendErr error
		resp, sendErr = b.sigClient.SendMessage(ctx, destinationNumber, message, attachments)
		return sendErr
	}, isRetryableSignalError)

	if retryErr != nil {
		// Check if this was a non-retryable error to provide better error context
		if !isRetryableSignalError(retryErr) {
			return fmt.Errorf("signal message failed (non-retryable): %w", retryErr)
		}
		return fmt.Errorf("failed to send signal message after retries: %w", retryErr)
	}

	if resp == nil {
		return fmt.Errorf("received nil response from Signal client after successful retry")
	}

	mapping := &models.MessageMapping{
		WhatsAppChatID:  chatID,
		WhatsAppMsgID:   msgID,
		SignalMsgID:     resp.MessageID,
		SignalTimestamp: time.Unix(resp.Timestamp/constants.MillisecondsPerSecond, 0),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusDelivered,
		SessionName:     sessionName,
	}

	if len(attachments) > 0 {
		mapping.MediaPath = &attachments[0]
	}

	if err := b.db.SaveMessageMapping(ctx, mapping); err != nil {
		// Record failure metrics
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction": "whatsapp_to_signal",
			"session":   sessionName,
			"stage":     "save_mapping",
		}, "Message processing failures by stage")

		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	// Record success metrics and timing
	processingDuration := time.Since(startTime)
	metrics.IncrementCounter("message_processing_success", map[string]string{
		"direction": "whatsapp_to_signal",
		"session":   sessionName,
		"has_media": fmt.Sprintf("%t", mediaPath != ""),
	}, "Successful message processing operations")

	metrics.RecordTimer("message_processing_duration", processingDuration, map[string]string{
		"direction": "whatsapp_to_signal",
		"session":   sessionName,
	}, "Message processing duration")

	// Log successful completion
	completionFields := privacy.MaskSensitiveFields(map[string]interface{}{
		LogFieldRequestID: requestInfo.RequestID,
		LogFieldTraceID:   requestInfo.TraceID,
		LogFieldSession:   sessionName,
		LogFieldChatID:    chatID,
		LogFieldMessageID: msgID,
		LogFieldPlatform:  "whatsapp",
		LogFieldDirection: "incoming",
		LogFieldDuration:  processingDuration.Milliseconds(),
	})

	completionLogrusFields := make(logrus.Fields)
	for k, v := range completionFields {
		completionLogrusFields[k] = v
	}

	b.logger.WithFields(completionLogrusFields).Info("WhatsApp message processing completed successfully")

	return nil
}

func (b *bridge) HandleSignalMessage(ctx context.Context, msg *signaltypes.SignalMessage) error {
	// Try to infer destination from the message context
	// If there's only one channel configured, use it
	if b.channelManager.GetChannelCount() == 1 {
		destinations := b.channelManager.GetAllSignalDestinations()
		if len(destinations) > 0 {
			return b.HandleSignalMessageWithDestination(ctx, msg, destinations[0])
		}
	}
	// For multiple channels, this method should not be used - use HandleSignalMessageWithDestination instead
	return fmt.Errorf("cannot handle Signal message without destination context when multiple channels are configured")
}

func (b *bridge) HandleSignalMessageWithDestination(ctx context.Context, msg *signaltypes.SignalMessage, destination string) error {
	startTime := time.Now()

	// Delegate group messages to specialized handler
	if strings.HasPrefix(msg.Sender, "group.") {
		return b.handleSignalGroupMessage(ctx, msg, destination)
	}

	// Determine the WhatsApp session based on the Signal destination
	sessionName, err := b.channelManager.GetWhatsAppSession(destination)
	if err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      "unknown",
			"message_type": "unknown",
			"stage":        "session_lookup",
		}, "Message processing failures by stage")
		return fmt.Errorf("failed to determine WhatsApp session for Signal destination %s: %w", destination, err)
	}

	// Handle special message types
	if msg.Reaction != nil {
		return b.handleSignalReactionWithSession(ctx, msg, sessionName)
	}
	if msg.Deletion != nil {
		return b.handleSignalDeletionWithSession(ctx, msg, sessionName)
	}

	hasMedia := fmt.Sprintf("%t", len(msg.Attachments) > 0)
	metrics.IncrementCounter("message_processing_total", map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "direct",
		"has_media":    hasMedia,
	}, "Total message processing attempts")

	// Resolve target WhatsApp chat
	mapping, usedFallback, err := b.resolveMessageMapping(ctx, msg, sessionName)
	if err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "direct",
			"stage":        "resolve_mapping",
		}, "Message processing failures by stage")
		return err
	}
	if mapping == nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "direct",
			"stage":        "resolve_mapping",
		}, "Message processing failures by stage")
		return b.handleNewSignalThread(ctx, msg)
	}

	// Send warning if fallback routing was used (no quoted message)
	if usedFallback {
		notice := fmt.Sprintf("⚠️ Message routed to last active chat: %s\nTip: Quote a message to reply to a specific chat.", mapping.WhatsAppChatID)
		if notifyErr := b.SendSignalNotificationForSession(ctx, sessionName, notice); notifyErr != nil {
			b.logger.WithError(notifyErr).Warn("Failed to send fallback routing notification")
		}
	}

	// Process attachments
	attachments, err := b.processSignalAttachments(msg.Attachments)
	if err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "direct",
			"stage":        "process_attachments",
		}, "Message processing failures by stage")
		return fmt.Errorf("failed to process attachments: %w", err)
	}

	// Determine reply target when quoting
	replyTo := ""
	if msg.QuotedMessage != nil && mapping.WhatsAppMsgID != "" {
		replyTo = mapping.WhatsAppMsgID
	}

	// Send message to WhatsApp
	resp, err := b.sendMessageToWhatsApp(ctx, mapping.WhatsAppChatID, msg.Message, attachments, replyTo, sessionName)
	if err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "direct",
			"stage":        "send_whatsapp",
		}, "Message processing failures by stage")
		return err
	}
	if resp == nil {
		return nil
	}

	// Save message mapping
	if err := b.saveSignalToWhatsAppMapping(ctx, msg, resp, mapping.WhatsAppChatID, attachments, sessionName); err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "direct",
			"stage":        "save_mapping",
		}, "Message processing failures by stage")
		return err
	}

	metrics.IncrementCounter("message_processing_success", map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "direct",
		"has_media":    hasMedia,
	}, "Successful message processing operations")
	metrics.RecordTimer("message_processing_duration", time.Since(startTime), map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "direct",
	}, "Message processing duration")

	b.logger.WithFields(logrus.Fields{
		LogFieldChatID:    SanitizePhoneNumber(mapping.WhatsAppChatID),
		LogFieldMessageID: SanitizeMessageID(resp.MessageID),
		LogFieldDirection: "outgoing",
		LogFieldPlatform:  "whatsapp",
		LogFieldSession:   sessionName,
		LogFieldDuration:  time.Since(startTime).Milliseconds(),
		"signal_msg_id":   SanitizeMessageID(msg.MessageID),
	}).Info("Signal message forwarded to WhatsApp successfully")

	return nil
}

// saveSignalToWhatsAppMapping creates and persists the message mapping for a Signal-to-WhatsApp message.
func (b *bridge) saveSignalToWhatsAppMapping(ctx context.Context, msg *signaltypes.SignalMessage, resp *types.SendMessageResponse, whatsappChatID string, attachments []string, sessionName string) error {
	newMapping := &models.MessageMapping{
		WhatsAppChatID:  whatsappChatID,
		WhatsAppMsgID:   resp.MessageID,
		SignalMsgID:     msg.MessageID,
		SignalTimestamp: time.Unix(msg.Timestamp/constants.MillisecondsPerSecond, 0),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     sessionName,
	}

	if len(attachments) > 0 {
		newMapping.MediaPath = &attachments[0]
		newMapping.MediaType = b.mediaRouter.GetMediaType(attachments[0])
	}

	if err := b.db.SaveMessageMapping(ctx, newMapping); err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (b *bridge) HandleSignalMessageDeletion(ctx context.Context, targetMessageID string, sender string) error {
	b.logger.WithFields(logrus.Fields{
		"targetMessageID": SanitizeMessageID(targetMessageID),
		"sender":          SanitizePhoneNumber(sender),
	}).Debug("Processing Signal message deletion")

	// Look up the message mapping for the target message by Signal ID
	mapping, err := b.db.GetMessageMappingBySignalID(ctx, targetMessageID)
	if err != nil {
		b.logger.WithFields(logrus.Fields{
			"targetMessageID": SanitizeMessageID(targetMessageID),
			"error":           err,
		}).Error("Failed to get message mapping for deletion")
		return fmt.Errorf("failed to get message mapping for deletion: %w", err)
	}

	if mapping == nil {
		b.logger.WithField("targetMessageID", SanitizeMessageID(targetMessageID)).Warn("No mapping found for deletion target message")
		return fmt.Errorf("no mapping found for deletion target message: %s", targetMessageID)
	}

	// Delete the message in WhatsApp
	err = b.waClient.DeleteMessage(ctx, mapping.WhatsAppChatID, mapping.WhatsAppMsgID)
	if err != nil {
		b.logger.WithFields(logrus.Fields{
			"whatsappChatID": SanitizePhoneNumber(mapping.WhatsAppChatID),
			"whatsappMsgID":  SanitizeWhatsAppMessageID(mapping.WhatsAppMsgID),
			"error":          err,
		}).Error("Failed to delete message in WhatsApp")
		return fmt.Errorf("failed to delete message in WhatsApp: %w", err)
	}

	b.logger.WithFields(logrus.Fields{
		"whatsappChatID":  SanitizePhoneNumber(mapping.WhatsAppChatID),
		"whatsappMsgID":   SanitizeWhatsAppMessageID(mapping.WhatsAppMsgID),
		"targetMessageID": SanitizeMessageID(targetMessageID),
	}).Info("Successfully deleted message in WhatsApp")

	return nil
}

// sendMessageToWhatsApp sends a message to WhatsApp with proper media type routing.
// This consolidates the send logic used by both direct and group message handlers.
// Uses exponential backoff retry for transient WAHA errors (e.g., markedUnread, 500 errors).
func (b *bridge) sendMessageToWhatsApp(ctx context.Context, chatID string, message string, attachments []string, replyTo string, sessionName string) (*types.SendMessageResponse, error) {
	trimmedMessage := strings.TrimSpace(message)
	if len(attachments) == 0 && trimmedMessage == "" {
		return nil, nil
	}

	sendStart := time.Now()

	backoffConfig := retry.BackoffConfig{
		InitialDelay: time.Duration(b.retryConfig.InitialBackoffMs) * time.Millisecond,
		MaxDelay:     time.Duration(b.retryConfig.MaxBackoffMs) * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  b.retryConfig.MaxAttempts,
		Jitter:       true,
	}
	backoff := retry.NewBackoff(backoffConfig)

	var resp *types.SendMessageResponse
	attempt := 0
	retryErr := backoff.RetryWithPredicate(ctx, func() error {
		attempt++
		var sendErr error

		switch {
		case len(attachments) > 0 && b.mediaRouter.IsImageAttachment(attachments[0]):
			b.logger.WithFields(logrus.Fields{
				"method":      "SendImage",
				"sessionName": sessionName,
				"attempt":     attempt,
			}).Debug("Sending image to WhatsApp")
			if replyTo != "" {
				resp, sendErr = b.waClient.SendImageWithSessionReply(ctx, chatID, attachments[0], message, replyTo, sessionName)
			} else {
				resp, sendErr = b.waClient.SendImageWithSession(ctx, chatID, attachments[0], message, sessionName)
			}

		case len(attachments) > 0 && b.mediaRouter.IsVideoAttachment(attachments[0]):
			b.logger.WithFields(logrus.Fields{
				"method":      "SendVideo",
				"sessionName": sessionName,
				"attempt":     attempt,
			}).Debug("Sending video to WhatsApp")
			if replyTo != "" {
				resp, sendErr = b.waClient.SendVideoWithSessionReply(ctx, chatID, attachments[0], message, replyTo, sessionName)
			} else {
				resp, sendErr = b.waClient.SendVideoWithSession(ctx, chatID, attachments[0], message, sessionName)
			}

		case len(attachments) > 0 && b.mediaRouter.IsVoiceAttachment(attachments[0]):
			b.logger.WithFields(logrus.Fields{
				"method":      "SendVoice",
				"sessionName": sessionName,
				"attempt":     attempt,
			}).Debug("Sending voice to WhatsApp")
			if replyTo != "" {
				resp, sendErr = b.waClient.SendVoiceWithSessionReply(ctx, chatID, attachments[0], replyTo, sessionName)
			} else {
				resp, sendErr = b.waClient.SendVoiceWithSession(ctx, chatID, attachments[0], sessionName)
			}

		case len(attachments) > 0:
			b.logger.WithFields(logrus.Fields{
				"method":      "SendDocument",
				"filePath":    attachments[0],
				"sessionName": sessionName,
				"attempt":     attempt,
			}).Debug("Sending attachment as document to WhatsApp")
			if replyTo != "" {
				resp, sendErr = b.waClient.SendDocumentWithSessionReply(ctx, chatID, attachments[0], message, replyTo, sessionName)
			} else {
				resp, sendErr = b.waClient.SendDocumentWithSession(ctx, chatID, attachments[0], message, sessionName)
			}

		default:
			b.logger.WithFields(logrus.Fields{
				"method":        "SendText",
				"sessionName":   sessionName,
				"messageLength": len(trimmedMessage),
				"attempt":       attempt,
			}).Debug("Sending text to WhatsApp")
			if replyTo != "" {
				resp, sendErr = b.waClient.SendTextWithSessionReply(ctx, chatID, trimmedMessage, replyTo, sessionName)
			} else {
				resp, sendErr = b.waClient.SendTextWithSession(ctx, chatID, trimmedMessage, sessionName)
			}
		}

		if sendErr != nil {
			isRetryable := isRetryableWhatsAppError(sendErr)
			b.logger.WithFields(logrus.Fields{
				"sessionName": sessionName,
				"chatID":      SanitizePhoneNumber(chatID),
				"attempt":     attempt,
				"maxAttempts": b.retryConfig.MaxAttempts,
				"error":       sendErr.Error(),
				"retryable":   isRetryable,
			}).Warn("WhatsApp send attempt failed")
		}

		return sendErr
	}, isRetryableWhatsAppError)

	if retryErr != nil {
		retryable := fmt.Sprintf("%t", isRetryableWhatsAppError(retryErr))
		metrics.IncrementCounter("whatsapp_send_total", map[string]string{
			"session": sessionName,
			"status":  "failure",
		}, "WhatsApp send outcomes")
		metrics.IncrementCounter("whatsapp_send_failures", map[string]string{
			"session":   sessionName,
			"retryable": retryable,
		}, "WhatsApp send failures by retryability")
		metrics.RecordTimer("whatsapp_send_duration", time.Since(sendStart), map[string]string{
			"session": sessionName,
			"status":  "failure",
		}, "WhatsApp send duration including retries")
		b.logger.WithFields(logrus.Fields{
			"sessionName":   sessionName,
			"chatID":        SanitizePhoneNumber(chatID),
			"totalAttempts": attempt,
			"error":         retryErr.Error(),
		}).Error("WhatsApp message failed after all retry attempts")
		if !isRetryableWhatsAppError(retryErr) {
			return nil, fmt.Errorf("whatsapp message failed (non-retryable): %w", retryErr)
		}
		return nil, fmt.Errorf("failed to send whatsapp message after retries: %w", retryErr)
	}

	metrics.IncrementCounter("whatsapp_send_total", map[string]string{
		"session": sessionName,
		"status":  "success",
	}, "WhatsApp send outcomes")
	metrics.RecordTimer("whatsapp_send_duration", time.Since(sendStart), map[string]string{
		"session": sessionName,
		"status":  "success",
	}, "WhatsApp send duration including retries")

	b.logger.WithFields(logrus.Fields{
		"sessionName": sessionName,
		"chatID":      SanitizePhoneNumber(chatID),
		"attempts":    attempt,
	}).Debug("WhatsApp message sent successfully")

	return resp, nil
}

// resolveMessageMapping finds the target WhatsApp chat for a Signal message.
// It handles both quoted message lookups and auto-reply to latest sender.
// Returns the mapping, whether the fallback was used, and any error.
func (b *bridge) resolveMessageMapping(ctx context.Context, msg *signaltypes.SignalMessage, sessionName string) (*models.MessageMapping, bool, error) {
	var mapping *models.MessageMapping
	var err error

	if msg.QuotedMessage == nil {
		// No quoted message - find the latest message for auto-reply (fallback)
		mapping, err = b.db.GetLatestMessageMappingBySession(ctx, sessionName)
		if err != nil {
			b.logger.WithFields(logrus.Fields{
				"signalSender": msg.Sender,
				"sessionName":  sessionName,
				"error":        err,
			}).Error("Failed to get latest message mapping for auto-reply")
			return nil, false, fmt.Errorf("failed to get latest message mapping for auto-reply: %w", err)
		}
		if mapping != nil {
			b.logger.WithFields(logrus.Fields{
				"whatsappChatID": mapping.WhatsAppChatID,
			}).Debug("Found latest message mapping for auto-reply (fallback)")
		}
		return mapping, true, nil // true = used fallback
	}

	// Has quoted message - look it up
	b.logger.WithFields(logrus.Fields{
		"quotedMessageID": msg.QuotedMessage.ID,
	}).Debug("Looking up message mapping for quoted message")

	mapping, err = b.db.GetMessageMapping(ctx, msg.QuotedMessage.ID)
	if err != nil {
		b.logger.WithFields(logrus.Fields{
			"quotedMessageID": msg.QuotedMessage.ID,
			"error":           err,
		}).Error("Failed to get message mapping for quoted message")
		return nil, false, fmt.Errorf("failed to get message mapping for quoted message: %w", err)
	}

	if mapping != nil {
		b.logger.WithFields(logrus.Fields{
			"quotedMessageID": msg.QuotedMessage.ID,
		}).Debug("Found message mapping in database")
		return mapping, false, nil // false = explicit quote, not fallback
	}

	// Try fallback extraction from quoted message text
	b.logger.WithFields(logrus.Fields{
		"quotedMessageID": msg.QuotedMessage.ID,
	}).Debug("No message mapping found in database, trying fallback")

	mapping = b.extractMappingFromQuotedText(msg.QuotedMessage.Text)
	if mapping == nil {
		return nil, false, fmt.Errorf("no mapping found for quoted message: %s", msg.QuotedMessage.ID)
	}

	return mapping, false, nil // false = extracted from quote text
}

// extractMappingFromQuotedText attempts to extract a WhatsApp chat ID from quoted message text.
// This is a fallback for when the message mapping is not found in the database.
func (b *bridge) extractMappingFromQuotedText(quotedText string) *models.MessageMapping {
	if !strings.Contains(quotedText, ": ") {
		return nil
	}

	parts := strings.SplitN(quotedText, ": ", 2)
	if len(parts) != 2 {
		return nil
	}

	senderInfo := parts[0]
	b.logger.Debug("Attempting fallback extraction from quoted text")

	senderInfo = strings.TrimPrefix(senderInfo, "📱 ")

	if len(senderInfo) < constants.MinPhoneNumberLength || !strings.ContainsAny(senderInfo, "0123456789") {
		return nil
	}

	var phoneNumber string
	for _, char := range senderInfo {
		if char >= '0' && char <= '9' {
			phoneNumber += string(char)
		}
	}

	if len(phoneNumber) < constants.MinPhoneNumberLength {
		return nil
	}

	b.logger.Debug("Extracted phone number from quoted text for fallback")
	return &models.MessageMapping{
		WhatsAppChatID: phoneNumber + "@c.us",
	}
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

// Removed wrapper methods - use b.mediaRouter directly

func (b *bridge) UpdateDeliveryStatus(ctx context.Context, msgID string, status models.DeliveryStatus) error {
	return b.db.UpdateDeliveryStatus(ctx, msgID, string(status))
}

func (b *bridge) CleanupOldRecords(ctx context.Context, retentionDays int) error {
	if err := b.db.CleanupOldRecords(ctx, retentionDays); err != nil {
		return fmt.Errorf("failed to cleanup old records: %w", err)
	}

	if err := b.media.CleanupOldFiles(int64(retentionDays * constants.SecondsPerDay)); err != nil {
		return fmt.Errorf("failed to cleanup old media files: %w", err)
	}

	if err := b.cleanupSignalAttachments(retentionDays); err != nil {
		return fmt.Errorf("failed to cleanup signal attachments: %w", err)
	}

	return nil
}

func (b *bridge) cleanupSignalAttachments(retentionDays int) error {
	if b.signalAttachmentsDir == "" {
		return nil
	}

	entries, err := os.ReadDir(b.signalAttachmentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read signal attachments directory: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			b.logger.WithError(err).WithField("file", entry.Name()).Warn("Failed to get file info for signal attachment")
			continue
		}

		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(b.signalAttachmentsDir, info.Name())
			if err := os.Remove(filePath); err != nil {
				b.logger.WithError(err).WithField("file", filePath).Warn("Failed to remove old signal attachment")
				continue
			}
			b.logger.WithField("file", filePath).Debug("Removed old signal attachment")
		}
	}

	return nil
}

func (b *bridge) handleSignalGroupMessage(ctx context.Context, msg *signaltypes.SignalMessage, destination string) error {
	startTime := time.Now()

	// Resolve WhatsApp session from Signal destination
	sessionName, err := b.channelManager.GetWhatsAppSession(destination)
	if err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      "unknown",
			"message_type": "group",
			"stage":        "session_lookup",
		}, "Message processing failures by stage")
		return fmt.Errorf("failed to determine WhatsApp session for Signal destination %s: %w", destination, err)
	}

	hasMedia := fmt.Sprintf("%t", len(msg.Attachments) > 0)
	metrics.IncrementCounter("message_processing_total", map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "group",
		"has_media":    hasMedia,
	}, "Total message processing attempts")

	b.logger.WithFields(logrus.Fields{
		"messageID": msg.MessageID,
		"sender":    msg.Sender,
		"session":   sessionName,
	}).Debug("Processing Signal group message")

	// Resolve target WhatsApp group chat
	mapping, usedFallback, err := b.resolveGroupMessageMapping(ctx, msg, sessionName)
	if err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "group",
			"stage":        "resolve_mapping",
		}, "Message processing failures by stage")
		// Provide helpful guidance when group routing fails
		if strings.Contains(err.Error(), "no group context") {
			// Send notification to Signal about how to route group messages
			guidance := "Unable to determine target group. Please quote a message from the group you want to reply to."
			if notifyErr := b.SendSignalNotificationForSession(ctx, sessionName, guidance); notifyErr != nil {
				b.logger.WithError(notifyErr).Warn("Failed to send group routing guidance notification")
			}
		}
		return err
	}

	// Verify it's actually a group
	if !strings.HasSuffix(mapping.WhatsAppChatID, "@g.us") {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "group",
			"stage":        "not_group",
		}, "Message processing failures by stage")
		return fmt.Errorf("resolved chat is not a group: %s", mapping.WhatsAppChatID)
	}

	// Send warning if fallback routing was used (no quoted message)
	if usedFallback {
		// Extract group name for clearer notification
		groupName := mapping.WhatsAppChatID
		if b.groupService != nil {
			if name := b.groupService.GetGroupName(ctx, mapping.WhatsAppChatID, sessionName); name != "" {
				groupName = name
			}
		}
		notice := fmt.Sprintf("Group message routed to: %s\nTip: Quote a message to reply to a specific group.", groupName)
		if notifyErr := b.SendSignalNotificationForSession(ctx, sessionName, notice); notifyErr != nil {
			b.logger.WithError(notifyErr).Warn("Failed to send group fallback routing notification")
		}
	}

	// Process attachments
	attachments, err := b.processSignalAttachments(msg.Attachments)
	if err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "group",
			"stage":        "process_attachments",
		}, "Message processing failures by stage")
		return fmt.Errorf("failed to process attachments: %w", err)
	}

	// Determine reply target when quoting
	replyTo := ""
	if msg.QuotedMessage != nil && mapping.WhatsAppMsgID != "" {
		replyTo = mapping.WhatsAppMsgID
	}

	// Send message to WhatsApp
	resp, err := b.sendMessageToWhatsApp(ctx, mapping.WhatsAppChatID, msg.Message, attachments, replyTo, sessionName)
	if err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "group",
			"stage":        "send_whatsapp",
		}, "Message processing failures by stage")
		return err
	}
	if resp == nil {
		// Empty message was skipped
		return nil
	}

	// Save message mapping
	if err := b.saveSignalToWhatsAppMapping(ctx, msg, resp, mapping.WhatsAppChatID, attachments, sessionName); err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "group",
			"stage":        "save_mapping",
		}, "Message processing failures by stage")
		return err
	}

	metrics.IncrementCounter("message_processing_success", map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "group",
		"has_media":    hasMedia,
	}, "Successful message processing operations")
	metrics.RecordTimer("message_processing_duration", time.Since(startTime), map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "group",
	}, "Message processing duration")

	b.logger.WithFields(logrus.Fields{
		LogFieldChatID:    SanitizePhoneNumber(mapping.WhatsAppChatID),
		LogFieldMessageID: SanitizeMessageID(resp.MessageID),
		LogFieldDirection: "outgoing",
		LogFieldPlatform:  "whatsapp",
		LogFieldSession:   sessionName,
		LogFieldDuration:  time.Since(startTime).Milliseconds(),
		"signal_msg_id":   SanitizeMessageID(msg.MessageID),
		"group":           true,
	}).Info("Signal group message forwarded to WhatsApp successfully")

	return nil
}

// resolveGroupMessageMapping finds the target WhatsApp group chat for a Signal group message.
// Returns the mapping, whether fallback was used, and any error.
func (b *bridge) resolveGroupMessageMapping(ctx context.Context, msg *signaltypes.SignalMessage, sessionName string) (*models.MessageMapping, bool, error) {
	var mapping *models.MessageMapping
	var err error

	if msg.QuotedMessage != nil {
		b.logger.WithField("quotedMessageID", msg.QuotedMessage.ID).Debug("Looking up mapping for quoted message in group")
		mapping, err = b.db.GetMessageMapping(ctx, msg.QuotedMessage.ID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get message mapping for quoted message: %w", err)
		}
		if mapping == nil {
			return nil, false, fmt.Errorf("no mapping found for quoted message: %s (try quoting a more recent message)", msg.QuotedMessage.ID)
		}
		return mapping, false, nil // explicit quote, not fallback
	}

	// Fallback: use latest group message mapping for the session
	// WARNING: This can route to wrong group under concurrent load
	b.logger.WithField("sessionName", sessionName).Debug("No quoted message - using fallback to latest group mapping")
	mapping, err = b.db.GetLatestGroupMessageMappingBySession(ctx, sessionName, 25)
	if err != nil {
		return nil, true, fmt.Errorf("failed to get latest group message mapping for session: %w", err)
	}
	if mapping == nil {
		return nil, true, fmt.Errorf("no group context found for session %s - quote a group message to establish context", sessionName)
	}

	b.logger.WithFields(logrus.Fields{
		"sessionName":    sessionName,
		"whatsappChatID": SanitizePhoneNumber(mapping.WhatsAppChatID),
		"fallback":       true,
	}).Warn("Using fallback routing to latest group chat")

	return mapping, true, nil // fallback was used
}

func (b *bridge) handleNewSignalThread(ctx context.Context, msg *signaltypes.SignalMessage) error {
	b.logger.WithFields(logrus.Fields{
		"messageID": msg.MessageID,
		"sender":    msg.Sender,
	}).Error("Cannot start new conversation from Signal to WhatsApp - no message mapping found")

	return fmt.Errorf("cannot start new conversations from Signal to WhatsApp. Please send a message from WhatsApp first, or quote an existing message to reply to a specific conversation")
}

func (b *bridge) handleSignalReaction(ctx context.Context, msg *signaltypes.SignalMessage) error {
	// Legacy method - should not be used when sessions are configured
	return fmt.Errorf("handleSignalReaction called without session context")
}

func (b *bridge) handleSignalReactionWithSession(ctx context.Context, msg *signaltypes.SignalMessage, sessionName string) error {
	startTime := time.Now()

	metrics.IncrementCounter("message_processing_total", map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "reaction",
		"has_media":    "false",
	}, "Total message processing attempts")

	b.logger.WithFields(logrus.Fields{
		"messageID":       msg.MessageID,
		"sender":          msg.Sender,
		"reaction":        msg.Reaction.Emoji,
		"targetTimestamp": msg.Reaction.TargetTimestamp,
		"isRemove":        msg.Reaction.IsRemove,
	}).Debug("Processing Signal reaction")

	// Find the original message mapping by Signal timestamp
	targetID := fmt.Sprintf("%d", msg.Reaction.TargetTimestamp)
	mapping, err := b.db.GetMessageMapping(ctx, targetID)
	if err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "reaction",
			"stage":        "resolve_mapping",
		}, "Message processing failures by stage")
		b.logger.WithError(err).Error("Failed to get message mapping for reaction target")
		return fmt.Errorf("failed to get message mapping for reaction target: %w", err)
	}

	if mapping == nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "reaction",
			"stage":        "resolve_mapping",
		}, "Message processing failures by stage")
		b.logger.WithField("targetID", targetID).Warn("No mapping found for reaction target message")
		return fmt.Errorf("no mapping found for reaction target message: %s", targetID)
	}

	// Send reaction to WhatsApp
	reaction := msg.Reaction.Emoji
	if msg.Reaction.IsRemove {
		// Empty string removes the reaction in WAHA
		reaction = ""
	}

	resp, err := b.waClient.SendReactionWithSession(ctx, mapping.WhatsAppChatID, mapping.WhatsAppMsgID, reaction, sessionName)
	if err != nil {
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "reaction",
			"stage":        "send_whatsapp",
		}, "Message processing failures by stage")
		b.logger.WithError(err).Error("Failed to send reaction to WhatsApp")
		return fmt.Errorf("failed to send reaction to WhatsApp: %w", err)
	}

	metrics.IncrementCounter("message_processing_success", map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "reaction",
		"has_media":    "false",
	}, "Successful message processing operations")
	metrics.RecordTimer("message_processing_duration", time.Since(startTime), map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "reaction",
	}, "Message processing duration")

	b.logger.WithFields(logrus.Fields{
		"whatsappMsgID": SanitizeWhatsAppMessageID(mapping.WhatsAppMsgID),
		"reaction":      reaction,
		"response":      resp,
	}).Info("Successfully forwarded reaction to WhatsApp")

	return nil
}

func (b *bridge) handleSignalDeletion(ctx context.Context, msg *signaltypes.SignalMessage) error {
	// Legacy method - should not be used when sessions are configured
	return fmt.Errorf("handleSignalDeletion called without session context")
}

func (b *bridge) handleSignalDeletionWithSession(ctx context.Context, msg *signaltypes.SignalMessage, sessionName string) error {
	startTime := time.Now()

	metrics.IncrementCounter("message_processing_total", map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "deletion",
		"has_media":    "false",
	}, "Total message processing attempts")

	b.logger.WithFields(logrus.Fields{
		"messageID":       msg.MessageID,
		"sender":          msg.Sender,
		"targetMessageID": msg.Deletion.TargetMessageID,
		"targetTimestamp": msg.Deletion.TargetTimestamp,
	}).Debug("Processing Signal message deletion")

	// Use the target message ID or timestamp to find the message to delete
	var targetID string
	if msg.Deletion.TargetMessageID != "" {
		targetID = msg.Deletion.TargetMessageID
	} else {
		// Fallback to timestamp if message ID is not available
		targetID = fmt.Sprintf("%d", msg.Deletion.TargetTimestamp)
	}

	if err := b.HandleSignalMessageDeletion(ctx, targetID, msg.Sender); err != nil {
		stage := "send_whatsapp"
		if strings.Contains(err.Error(), "no mapping found") || strings.Contains(err.Error(), "failed to get message mapping") {
			stage = "resolve_mapping"
		}
		metrics.IncrementCounter("message_processing_failures", map[string]string{
			"direction":    "signal_to_whatsapp",
			"session":      sessionName,
			"message_type": "deletion",
			"stage":        stage,
		}, "Message processing failures by stage")
		return err
	}

	metrics.IncrementCounter("message_processing_success", map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "deletion",
		"has_media":    "false",
	}, "Successful message processing operations")
	metrics.RecordTimer("message_processing_duration", time.Since(startTime), map[string]string{
		"direction":    "signal_to_whatsapp",
		"session":      sessionName,
		"message_type": "deletion",
	}, "Message processing duration")

	return nil
}

func (b *bridge) SendSignalNotificationForSession(ctx context.Context, sessionName, message string) error {
	// Get the Signal destination based on session
	dest, err := b.channelManager.GetSignalDestination(sessionName)
	if err != nil {
		return fmt.Errorf("failed to get Signal destination for session %s: %w", sessionName, err)
	}

	// Send the notification message to Signal
	_, err = b.sigClient.SendMessage(ctx, dest, message, []string{})
	if err != nil {
		return fmt.Errorf("failed to send Signal notification: %w", err)
	}

	b.logger.WithFields(logrus.Fields{
		"sessionName": sessionName,
		"destination": dest,
		"message":     message,
	}).Debug("Sent Signal notification for session")

	return nil
}
