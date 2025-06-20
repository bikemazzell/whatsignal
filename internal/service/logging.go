package service

import (
	"context"
	"strings"

	"whatsignal/internal/constants"

	"github.com/sirupsen/logrus"
)

// IsVerboseLogging checks if verbose logging is enabled from context
func IsVerboseLogging(ctx context.Context) bool {
	if verbose, ok := ctx.Value("verbose").(bool); ok {
		return verbose
	}
	return false
}

// SanitizePhoneNumber removes or masks phone numbers for privacy
func SanitizePhoneNumber(phone string) string {
	if phone == "" {
		return ""
	}
	
	// Remove @c.us suffix if present
	cleaned := strings.TrimSuffix(phone, "@c.us")
	
	// For privacy, show only last N digits
	if len(cleaned) > constants.DefaultPhoneMaskLength {
		return "***" + cleaned[len(cleaned)-constants.DefaultPhoneMaskLength:]
	}
	return "***"
}

// SanitizeMessageID removes or shortens message IDs for privacy
func SanitizeMessageID(msgID string) string {
	if msgID == "" {
		return ""
	}
	
	// Show only first N characters
	if len(msgID) > constants.DefaultMessageIDLength {
		return msgID[:constants.DefaultMessageIDLength] + "..."
	}
	return msgID
}

// SanitizeContent completely hides message content for privacy
func SanitizeContent(content string) string {
	if content == "" {
		return ""
	}
	return "[hidden]"
}

// LogWithContext creates a logger entry with optional sensitive information
func LogWithContext(ctx context.Context, logger *logrus.Logger) *logrus.Entry {
	return logger.WithField("verbose", IsVerboseLogging(ctx))
}

// LogMessageProcessing logs message processing with appropriate privacy controls
func LogMessageProcessing(ctx context.Context, logger *logrus.Logger, msgType string, chatID, msgID, sender, content string) {
	if IsVerboseLogging(ctx) {
		logger.WithFields(logrus.Fields{
			"type":    msgType,
			"chatID":  chatID,
			"msgID":   msgID,
			"sender":  sender,
			"content": content,
		}).Info("Processing message")
	} else {
		logger.WithFields(logrus.Fields{
			"type":   msgType,
			"chatID": SanitizePhoneNumber(chatID),
			"msgID":  SanitizeMessageID(msgID),
			"sender": SanitizePhoneNumber(sender),
		}).Info("Processing message")
	}
}

// LogSignalPolling logs Signal polling activity with privacy controls
func LogSignalPolling(ctx context.Context, logger *logrus.Logger, messageCount int) {
	if messageCount > 0 {
		if IsVerboseLogging(ctx) {
			logger.WithField("count", messageCount).Info("Found new Signal messages")
		} else {
			logger.Info("Found new Signal messages")
		}
	} else {
		logger.Debug("No new Signal messages found")
	}
}