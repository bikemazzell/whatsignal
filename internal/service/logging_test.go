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