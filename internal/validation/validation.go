package validation

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"whatsignal/internal/constants"
	"whatsignal/internal/errors"
)

// ValidateE164PhoneNumber validates a Signal/config phone number with optional + prefix.
func ValidateE164PhoneNumber(phone string) error {
	if phone == "" {
		return errors.New(errors.ErrCodeInvalidInput, "phone number cannot be empty")
	}

	// Remove optional + prefix for validation.
	cleaned := strings.TrimPrefix(phone, "+")

	// Check length bounds
	if len(cleaned) < constants.MinPhoneNumberLength {
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("phone number must be at least %d digits", constants.MinPhoneNumberLength))
	}

	if len(cleaned) > 15 {
		return errors.New(errors.ErrCodeInvalidInput, "phone number too long (max 15 digits)")
	}

	// Check that it contains only digits
	for _, char := range cleaned {
		if !unicode.IsDigit(char) {
			return errors.New(errors.ErrCodeInvalidInput, "phone number must contain only digits")
		}
	}

	return nil
}

// ValidatePhoneNumber validates a Signal/config phone number.
func ValidatePhoneNumber(phone string) error {
	return ValidateE164PhoneNumber(phone)
}

// ValidateChatID validates WhatsApp chat IDs, group IDs, LIDs, and bare phone numbers.
func ValidateChatID(chatID string) error {
	if chatID == "" {
		return errors.New(errors.ErrCodeInvalidInput, "phone number cannot be empty")
	}

	isGroup := strings.HasSuffix(chatID, "@g.us") || strings.Contains(chatID, "@g.us")
	isLID := strings.HasSuffix(chatID, "@lid")

	cleaned := strings.TrimSuffix(chatID, "@c.us")
	cleaned = strings.TrimSuffix(cleaned, "@g.us")
	cleaned = strings.TrimSuffix(cleaned, "@lid")

	if strings.Contains(cleaned, "-") {
		parts := strings.Split(cleaned, "-")
		if len(parts) >= 2 && len(parts[0]) > 0 {
			for _, char := range parts[0] {
				if !unicode.IsDigit(char) {
					return errors.New(errors.ErrCodeInvalidInput, "invalid group ID format: non-numeric characters")
				}
			}
			return nil
		}
		return errors.New(errors.ErrCodeInvalidInput, "invalid group ID format")
	}

	digits := cleaned
	digits = strings.TrimPrefix(digits, "+")

	if len(digits) == 0 {
		return errors.New(errors.ErrCodeInvalidInput, "phone number must contain digits")
	}

	maxLength := 20
	if isGroup || isLID {
		maxLength = 25
	}

	if len(digits) < 7 || len(digits) > maxLength {
		if isGroup {
			return errors.New(errors.ErrCodeInvalidInput, fmt.Sprintf("group ID must be between 7 and %d digits, got %d", maxLength, len(digits)))
		}
		if isLID {
			return errors.New(errors.ErrCodeInvalidInput, fmt.Sprintf("linked ID must be between 7 and %d digits, got %d", maxLength, len(digits)))
		}
		return errors.New(errors.ErrCodeInvalidInput, fmt.Sprintf("phone number must be between 7 and %d digits, got %d", maxLength, len(digits)))
	}

	for _, char := range digits {
		if !unicode.IsDigit(char) {
			if isGroup {
				return errors.New(errors.ErrCodeInvalidInput, "group ID must contain only digits")
			}
			return errors.New(errors.ErrCodeInvalidInput, "phone number must contain only digits")
		}
	}

	return nil
}

// ValidateMessageID validates message ID format and length
func ValidateMessageID(messageID string) error {
	if messageID == "" {
		return errors.New(errors.ErrCodeInvalidInput, "message ID cannot be empty")
	}

	if len(messageID) > constants.MaxMessageIDLength {
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("message ID too long (max %d characters)", constants.MaxMessageIDLength))
	}

	// Check for control characters that could cause issues
	for _, char := range messageID {
		if char == '\x00' || char == '\n' || char == '\r' || char == '\t' {
			return errors.New(errors.ErrCodeInvalidInput, "message ID contains invalid characters")
		}
	}

	return nil
}

// ValidateSessionName validates session name format and length
func ValidateSessionName(sessionName string) error {
	if sessionName == "" {
		return errors.New(errors.ErrCodeInvalidInput, "session name cannot be empty")
	}

	if len(sessionName) > constants.MaxSessionNameLength {
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("session name too long (max %d characters)", constants.MaxSessionNameLength))
	}

	// Session names should be alphanumeric with underscores and dashes
	for _, char := range sessionName {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '_' && char != '-' {
			return errors.New(errors.ErrCodeInvalidInput,
				"session name must contain only letters, numbers, underscores, and dashes")
		}
	}

	return nil
}

// ValidateMediaSize validates media file size against configured limits
func ValidateMediaSize(sizeBytes int64, mediaType string, limits map[string]int) error {
	if sizeBytes < 0 {
		return errors.New(errors.ErrCodeInvalidInput, "media size cannot be negative")
	}

	if sizeBytes == 0 {
		return errors.New(errors.ErrCodeInvalidInput, "media file is empty")
	}

	maxSizeMB, exists := limits[mediaType]
	if !exists {
		return errors.New(errors.ErrCodeInvalidInput, fmt.Sprintf("unsupported media type: %s", mediaType))
	}

	maxSizeBytes := int64(maxSizeMB) * constants.BytesPerMegabyte
	if sizeBytes > maxSizeBytes {
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("media file too large: %d bytes (max %d MB)", sizeBytes, maxSizeMB))
	}

	return nil
}

// ValidateHTTPRequestSize validates incoming HTTP request size
func ValidateHTTPRequestSize(r *http.Request, maxSizeBytes int64) error {
	if r.ContentLength < 0 {
		return errors.New(errors.ErrCodeInvalidInput, "invalid content length")
	}

	if r.ContentLength > maxSizeBytes {
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("request too large: %d bytes (max %d bytes)", r.ContentLength, maxSizeBytes))
	}

	return nil
}

// ValidateStringLength validates string length against bounds
func ValidateStringLength(value, fieldName string, minLength, maxLength int) error {
	if len(value) < minLength {
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("%s too short (min %d characters)", fieldName, minLength))
	}

	if len(value) > maxLength {
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("%s too long (max %d characters)", fieldName, maxLength))
	}

	return nil
}

// ValidateNumericRange validates numeric values against bounds
func ValidateNumericRange(value int, fieldName string, min, max int) error {
	if value < min {
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("%s too small (min %d)", fieldName, min))
	}

	if value > max {
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("%s too large (max %d)", fieldName, max))
	}

	return nil
}

// ValidateTimeout validates timeout values
func ValidateTimeout(timeoutSec int, fieldName string) error {
	if timeoutSec < 1 {
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("%s must be at least 1 second", fieldName))
	}

	if timeoutSec > 3600 { // Max 1 hour
		return errors.New(errors.ErrCodeInvalidInput,
			fmt.Sprintf("%s too large (max 3600 seconds)", fieldName))
	}

	return nil
}

// ValidateConnectionPool validates database connection pool settings
func ValidateConnectionPool(maxOpen, maxIdle int) error {
	if maxOpen < 1 {
		return errors.New(errors.ErrCodeInvalidInput, "max open connections must be at least 1")
	}

	if maxOpen > 1000 {
		return errors.New(errors.ErrCodeInvalidInput, "max open connections too large (max 1000)")
	}

	if maxIdle < 0 {
		return errors.New(errors.ErrCodeInvalidInput, "max idle connections cannot be negative")
	}

	if maxIdle > maxOpen {
		return errors.New(errors.ErrCodeInvalidInput, "max idle connections cannot exceed max open connections")
	}

	return nil
}

// ValidateRetentionDays validates data retention period
func ValidateRetentionDays(days int) error {
	if days < 1 {
		return errors.New(errors.ErrCodeInvalidInput, "retention days must be at least 1")
	}

	if days > 3650 { // Max 10 years
		return errors.New(errors.ErrCodeInvalidInput, "retention days too large (max 3650)")
	}

	return nil
}
