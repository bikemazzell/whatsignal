package service

import (
	"bytes"
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestIsVerboseLogging(t *testing.T) {
	tests := []struct {
		name     string
		verbose  bool
		expected bool
	}{
		{
			name:     "verbose enabled",
			verbose:  true,
			expected: true,
		},
		{
			name:     "verbose disabled",
			verbose:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), "verbose", tt.verbose)
			result := IsVerboseLogging(ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
	
	// Test with no verbose value in context
	t.Run("no verbose in context", func(t *testing.T) {
		ctx := context.Background()
		result := IsVerboseLogging(ctx)
		assert.False(t, result)
	})
}

func TestSanitizePhoneNumber(t *testing.T) {
	tests := []struct {
		name     string
		phone    string
		expected string
	}{
		{
			name:     "valid phone number",
			phone:    "+1234567890",
			expected: "***7890",
		},
		{
			name:     "short phone number",
			phone:    "+123",
			expected: "***",
		},
		{
			name:     "empty phone number",
			phone:    "",
			expected: "",
		},
		{
			name:     "phone with @c.us suffix",
			phone:    "1234567890@c.us",
			expected: "***7890",
		},
		{
			name:     "exactly 4 chars after cleaning",
			phone:    "1234@c.us",
			expected: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizePhoneNumber(tt.phone)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeMessageID(t *testing.T) {
	tests := []struct {
		name     string
		msgID    string
		expected string
	}{
		{
			name:     "valid message ID",
			msgID:    "msg123456789",
			expected: "msg12345...",
		},
		{
			name:     "short message ID",
			msgID:    "msg",
			expected: "msg",
		},
		{
			name:     "empty message ID",
			msgID:    "",
			expected: "",
		},
		{
			name:     "exactly 8 chars",
			msgID:    "12345678",
			expected: "12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeMessageID(tt.msgID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeWhatsAppMessageID(t *testing.T) {
	tests := []struct {
		name     string
		msgID    string
		expected string
	}{
		{
			name:     "full WhatsApp message ID",
			msgID:    "false_555222777156@c.us_E8XYZ450FD81FABC9C5DA1",
			expected: "false_***7156@c.us_E8XYZ450...",
		},
		{
			name:     "WhatsApp message ID with short hash",
			msgID:    "true_1234567890@c.us_ABC123",
			expected: "true_***7890@c.us_ABC123",
		},
		{
			name:     "WhatsApp group message ID",
			msgID:    "false_120363044444444444@g.us_3EB0589C54321",
			expected: "false_***4444@g.us_3EB0589C...",
		},
		{
			name:     "invalid format - no underscores",
			msgID:    "invalidmessageid",
			expected: "invalidm...",
		},
		{
			name:     "invalid format - missing phone part",
			msgID:    "false__ABC123",
			expected: "false__A...",
		},
		{
			name:     "empty message ID",
			msgID:    "",
			expected: "",
		},
		{
			name:     "WhatsApp ID without domain",
			msgID:    "false_1234567890_ABC123",
			expected: "false_12...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeWhatsAppMessageID(tt.msgID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "short content",
			content:  "Hello",
			expected: "[hidden]",
		},
		{
			name:     "long content",
			content:  "This is a very long message that exceeds the maximum allowed length for logging purposes",
			expected: "[hidden]",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeContent(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogWithContext(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})

	ctx := context.WithValue(context.Background(), "verbose", true)
	
	entry := LogWithContext(ctx, logger)
	entry.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "verbose=true")
}

func TestLogMessageProcessing(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})

	// Test verbose logging
	ctx := context.WithValue(context.Background(), "verbose", true)
	
	LogMessageProcessing(ctx, logger, "text", "chat123", "msg123456789", "+1234567890", "Hello world")
	
	output := buf.String()
	assert.Contains(t, output, "Processing message")
	assert.Contains(t, output, "type=text")
	assert.Contains(t, output, "chatID=chat123")
	assert.Contains(t, output, "msgID=msg123456789")
	assert.Contains(t, output, "sender=+1234567890")
	assert.Contains(t, output, "content=\"Hello world\"")

	// Test non-verbose logging
	buf.Reset()
	ctx = context.Background()
	
	LogMessageProcessing(ctx, logger, "text", "chat123", "msg123456789", "+1234567890", "Hello world")
	
	output = buf.String()
	assert.Contains(t, output, "Processing message")
	assert.Contains(t, output, "type=text")
	assert.Contains(t, output, "***")  // Should contain sanitized phone numbers
	assert.Contains(t, output, "msgID=msg12345...")
	assert.NotContains(t, output, "content=")
}

func TestLogSignalPolling(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})

	// Test with messages found - verbose logging
	ctx := context.WithValue(context.Background(), "verbose", true)
	
	LogSignalPolling(ctx, logger, 5)
	
	output := buf.String()
	assert.Contains(t, output, "Found new Signal messages")
	assert.Contains(t, output, "count=5")

	// Test with messages found - non-verbose logging
	buf.Reset()
	ctx = context.Background()
	
	LogSignalPolling(ctx, logger, 3)
	
	output = buf.String()
	assert.Contains(t, output, "Found new Signal messages")
	assert.NotContains(t, output, "count=")

	// Test with no messages found
	buf.Reset()
	logger.SetLevel(logrus.DebugLevel)
	
	LogSignalPolling(ctx, logger, 0)
	
	output = buf.String()
	assert.Contains(t, output, "No new Signal messages found")
}

func TestValidatePhoneNumber(t *testing.T) {
	tests := []struct {
		name      string
		phone     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid international format",
			phone:     "+1234567890",
			expectErr: false,
		},
		{
			name:      "valid WhatsApp format",
			phone:     "1234567890@c.us",
			expectErr: false,
		},
		{
			name:      "valid WhatsApp format with plus",
			phone:     "+1234567890@c.us",
			expectErr: false,
		},
		{
			name:      "valid local format",
			phone:     "1234567890",
			expectErr: false,
		},
		{
			name:      "valid minimum length",
			phone:     "1234567",
			expectErr: false,
		},
		{
			name:      "valid maximum length",
			phone:     "123456789012345",
			expectErr: false,
		},
		{
			name:      "empty phone number",
			phone:     "",
			expectErr: true,
			errMsg:    "phone number cannot be empty",
		},
		{
			name:      "too short",
			phone:     "123456",
			expectErr: true,
			errMsg:    "phone number must be between 7 and 15 digits",
		},
		{
			name:      "too long",
			phone:     "1234567890123456",
			expectErr: true,
			errMsg:    "phone number must be between 7 and 15 digits",
		},
		{
			name:      "contains letters",
			phone:     "+123abc7890",
			expectErr: true,
			errMsg:    "phone number must contain only digits",
		},
		{
			name:      "contains special characters",
			phone:     "+123-456-7890",
			expectErr: true,
			errMsg:    "phone number must contain only digits",
		},
		{
			name:      "contains spaces",
			phone:     "+123 456 7890",
			expectErr: true,
			errMsg:    "phone number must contain only digits",
		},
		{
			name:      "WhatsApp format with invalid digits",
			phone:     "123abc7890@c.us",
			expectErr: true,
			errMsg:    "phone number must contain only digits",
		},
		{
			name:      "plus only",
			phone:     "+",
			expectErr: true,
			errMsg:    "phone number must be between 7 and 15 digits",
		},
		{
			name:      "WhatsApp suffix only",
			phone:     "@c.us",
			expectErr: true,
			errMsg:    "phone number must be between 7 and 15 digits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePhoneNumber(tt.phone)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMessageID(t *testing.T) {
	tests := []struct {
		name      string
		msgID     string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid message ID",
			msgID:     "msg_123_456",
			expectErr: false,
		},
		{
			name:      "valid alphanumeric",
			msgID:     "ABC123DEF456",
			expectErr: false,
		},
		{
			name:      "valid with dots and dashes",
			msgID:     "msg.123-456_789",
			expectErr: false,
		},
		{
			name:      "valid UUID format",
			msgID:     "550e8400-e29b-41d4-a716-446655440000",
			expectErr: false,
		},
		{
			name:      "empty message ID",
			msgID:     "",
			expectErr: true,
			errMsg:    "message ID cannot be empty",
		},
		{
			name:      "contains null byte",
			msgID:     "msg\x00123",
			expectErr: true,
			errMsg:    "message ID contains invalid characters",
		},
		{
			name:      "contains newline",
			msgID:     "msg\n123",
			expectErr: true,
			errMsg:    "message ID contains invalid characters",
		},
		{
			name:      "contains carriage return",
			msgID:     "msg\r123",
			expectErr: true,
			errMsg:    "message ID contains invalid characters",
		},
		{
			name:      "contains tab",
			msgID:     "msg\t123",
			expectErr: true,
			errMsg:    "message ID contains invalid characters",
		},
		{
			name:      "single character",
			msgID:     "a",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessageID(tt.msgID)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSessionName(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		expectErr   bool
		errMsg      string
	}{
		{
			name:        "valid session name",
			sessionName: "business",
			expectErr:   false,
		},
		{
			name:        "valid with underscores",
			sessionName: "business_session",
			expectErr:   false,
		},
		{
			name:        "valid with dashes",
			sessionName: "business-session",
			expectErr:   false,
		},
		{
			name:        "valid with numbers",
			sessionName: "session123",
			expectErr:   false,
		},
		{
			name:        "valid alphanumeric mixed",
			sessionName: "Session1_Test-2",
			expectErr:   false,
		},
		{
			name:        "empty session name",
			sessionName: "",
			expectErr:   true,
			errMsg:      "session name cannot be empty",
		},
		{
			name:        "single character",
			sessionName: "a",
			expectErr:   false,
		},
		{
			name:        "default session",
			sessionName: "default",
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSessionName(tt.sessionName)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}