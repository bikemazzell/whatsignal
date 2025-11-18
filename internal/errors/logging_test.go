package errors

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger()

	assert.NotNil(t, logger)
	assert.NotNil(t, logger.Logger)

	// Check that JSON formatter is set
	_, ok := logger.Formatter.(*logrus.JSONFormatter)
	assert.True(t, ok, "Logger should use JSON formatter")
}

func TestLogger_LogError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name             string
		err              error
		message          string
		fields           []logrus.Fields
		expectedInOutput []string
		expectedLevel    string
	}{
		{
			name:    "AppError with context",
			err:     New(ErrCodeValidationFailed, "validation failed").WithContext("field", "email"),
			message: "User input validation failed",
			fields:  []logrus.Fields{{"user_id": "123"}},
			expectedInOutput: []string{
				`"level":"error"`,
				`"error_code":"VALIDATION_FAILED"`,
				`"retryable":false`,
				`"field":"email"`,
				`"user_id":"123"`,
				`"msg":"User input validation failed"`,
			},
			expectedLevel: "error",
		},
		{
			name:    "standard error",
			err:     errors.New("something went wrong"),
			message: "Operation failed",
			expectedInOutput: []string{
				`"level":"error"`,
				`"msg":"Operation failed"`,
				`"error":"something went wrong"`,
			},
			expectedLevel: "error",
		},
		{
			name:    "retryable AppError",
			err:     WrapRetryable(errors.New("network error"), ErrCodeWhatsAppAPI, "WhatsApp API call failed"),
			message: "External service error",
			expectedInOutput: []string{
				`"level":"error"`,
				`"error_code":"WHATSAPP_API"`,
				`"retryable":true`,
			},
			expectedLevel: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			logger.LogError(tt.err, tt.message, tt.fields...)

			output := buf.String()
			for _, expected := range tt.expectedInOutput {
				assert.Contains(t, output, expected)
			}
		})
	}
}

func TestLogger_LogWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.DebugLevel)

	appErr := New(ErrCodeTimeout, "operation timed out").WithContext("timeout", "30s")

	logger.LogWarn(appErr, "Operation timeout occurred")

	output := buf.String()
	assert.Contains(t, output, `"level":"warning"`)
	assert.Contains(t, output, `"error_code":"TIMEOUT"`)
	assert.Contains(t, output, `"timeout":"30s"`)
	assert.Contains(t, output, `"msg":"Operation timeout occurred"`)
}

func TestLogger_LogRetryableError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name          string
		err           error
		expectedLevel string
	}{
		{
			name:          "retryable error logs at warn level",
			err:           WrapRetryable(errors.New("temp failure"), ErrCodeSignalAPI, "Signal API error"),
			expectedLevel: "warning",
		},
		{
			name:          "non-retryable error logs at error level",
			err:           New(ErrCodeInvalidInput, "bad input"),
			expectedLevel: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			logger.LogRetryableError(tt.err, "Test message")

			output := buf.String()
			assert.Contains(t, output, `"level":"`+tt.expectedLevel+`"`)
		})
	}
}

func TestLogger_WithContext(t *testing.T) {
	logger := NewLogger()

	entry := logger.WithContext(logrus.Fields{
		"request_id": "123",
		"user_id":    "456",
	})

	assert.NotNil(t, entry)
	assert.Equal(t, "123", entry.Data["request_id"])
	assert.Equal(t, "456", entry.Data["user_id"])
}

func TestLogger_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name             string
		err              error
		expectedInOutput []string
	}{
		{
			name: "AppError with full context",
			err: New(ErrCodeDatabaseQuery, "query failed").
				WithContext("table", "users").
				WithContext("query_time", "500ms"),
			expectedInOutput: []string{
				`"error_code":"DATABASE_QUERY"`,
				`"retryable":false`,
				`"table":"users"`,
				`"query_time":"500ms"`,
			},
		},
		{
			name: "standard error",
			err:  errors.New("simple error"),
			expectedInOutput: []string{
				`"error":"simple error"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			entry := logger.WithError(tt.err)
			entry.Info("Test message")

			output := buf.String()
			for _, expected := range tt.expectedInOutput {
				assert.Contains(t, output, expected)
			}
		})
	}
}

func TestLogger_StructuredLogging_Integration(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.DebugLevel)

	// Create a complex error scenario
	originalErr := errors.New("connection refused")
	appErr := Wrap(originalErr, ErrCodeDatabaseConnection, "failed to connect to database").
		WithContext("host", "localhost").
		WithContext("port", 5432).
		WithContext("database", "whatsignal").
		WithUserMessage("Database is currently unavailable")

	// Log the error with additional fields
	logger.LogError(appErr, "Database connection failed during startup", logrus.Fields{
		"retry_attempt": 3,
		"service":       "bridge",
	})

	output := buf.String()

	// Parse JSON to verify structure
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	// Verify all expected fields are present
	assert.Equal(t, "error", logEntry["level"])
	assert.Equal(t, "DATABASE_CONNECTION", logEntry["error_code"])
	assert.Equal(t, false, logEntry["retryable"])
	assert.Equal(t, "localhost", logEntry["host"])
	assert.Equal(t, float64(5432), logEntry["port"]) // JSON numbers are float64
	assert.Equal(t, "whatsignal", logEntry["database"])
	assert.Equal(t, float64(3), logEntry["retry_attempt"])
	assert.Equal(t, "bridge", logEntry["service"])
	assert.Equal(t, "Database connection failed during startup", logEntry["msg"])
	assert.Contains(t, logEntry["error"].(string), "connection refused")
}

func TestLogger_ErrorContext_Preservation(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)

	// Test that context is preserved across multiple operations
	baseErr := New(ErrCodeValidationFailed, "field validation failed")

	// Add context incrementally
	baseErr = baseErr.WithContext("field", "email")
	baseErr = baseErr.WithContext("value", "invalid@")
	baseErr = baseErr.WithContext("rule", "email_format")

	logger.LogError(baseErr, "Validation error occurred")

	output := buf.String()
	assert.Contains(t, output, `"field":"email"`)
	assert.Contains(t, output, `"value":"invalid@"`)
	assert.Contains(t, output, `"rule":"email_format"`)
}

func TestLogger_NilError_Handling(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger()
	logger.SetOutput(&buf)

	// Test logging with nil error (should not crash)
	logger.LogError(nil, "Something happened without an error")

	output := buf.String()
	assert.Contains(t, output, `"level":"error"`)
	assert.Contains(t, output, `"msg":"Something happened without an error"`)
	// Should not contain error-specific fields when error is nil
	assert.NotContains(t, output, `"error_code"`)
}
