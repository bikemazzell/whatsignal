package service

import (
	"context"
	"fmt"
	"strings"

	"whatsignal/internal/constants"
	"whatsignal/internal/privacy"
	"whatsignal/internal/validation"

	"github.com/sirupsen/logrus"
)

// ContextKey is a package-local type to prevent context key collisions
// See staticcheck SA1029 guidance
type ContextKey string

// VerboseContextKey is the strongly-typed context key for verbose logging flag
const VerboseContextKey ContextKey = "verbose"

// IsVerboseLogging checks if verbose logging is enabled from context
func IsVerboseLogging(ctx context.Context) bool {
	if verbose, ok := ctx.Value(VerboseContextKey).(bool); ok {
		return verbose
	}
	return false
}

// SanitizePhoneNumber removes or masks phone numbers for privacy
func SanitizePhoneNumber(phone string) string {
	if phone == "" {
		return ""
	}
	if strings.Contains(phone, "@") {
		return privacy.MaskChatID(phone)
	}
	return privacy.MaskPhoneNumber(phone)
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

// SanitizeWhatsAppMessageID sanitizes WhatsApp message IDs that contain phone numbers
// Format: false_1234567890123@c.us_EXAMPLE1234567890ABCDEF1234567890
func SanitizeWhatsAppMessageID(msgID string) string {
	if msgID == "" {
		return ""
	}

	// Split by underscore to get parts
	parts := strings.Split(msgID, "_")
	if len(parts) >= 3 {
		// Sanitize the phone number part
		phonePart := parts[1]
		if idx := strings.Index(phonePart, "@"); idx > 0 {
			phoneNum := phonePart[:idx]
			domain := phonePart[idx:]
			sanitizedPhone := SanitizePhoneNumber(phoneNum)
			// Reconstruct: false_***7277@c.us_E844B47A...
			return parts[0] + "_" + sanitizedPhone + domain + "_" + SanitizeMessageID(parts[2])
		}
	}

	// Fallback to regular message ID sanitization
	return SanitizeMessageID(msgID)
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

// ValidatePhoneNumber performs basic phone number validation
// Accepts phone numbers with or without + prefix (WhatsApp API compatibility)
// Also accepts WhatsApp group IDs (numeric with @g.us suffix, up to 25 digits)
func ValidatePhoneNumber(phone string) error {
	return validation.ValidateChatID(phone)
}

// ValidateMessageID performs basic message ID validation
func ValidateMessageID(msgID string) error {
	if msgID == "" {
		return fmt.Errorf("message ID cannot be empty")
	}

	// Check length limits
	if len(msgID) > constants.MaxMessageIDLength {
		return fmt.Errorf("message ID too long (max %d characters)", constants.MaxMessageIDLength)
	}

	// Check for potentially dangerous characters
	if strings.ContainsAny(msgID, "\x00\n\r\t") {
		return fmt.Errorf("message ID contains invalid characters")
	}

	return nil
}

// ValidateSessionName performs session name validation
func ValidateSessionName(sessionName string) error {
	if sessionName == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	// Check length limits
	if len(sessionName) > constants.MaxSessionNameLength {
		return fmt.Errorf("session name too long (max %d characters)", constants.MaxSessionNameLength)
	}

	// Allow only alphanumeric characters, hyphens, and underscores
	for _, char := range sessionName {
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') &&
			char != '-' && char != '_' {
			return fmt.Errorf("session name must contain only alphanumeric characters, hyphens, and underscores")
		}
	}

	return nil
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
