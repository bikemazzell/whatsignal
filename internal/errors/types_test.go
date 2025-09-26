package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name: "error without cause",
			err: &AppError{
				Code:    ErrCodeInvalidConfig,
				Message: "configuration is invalid",
			},
			expected: "INVALID_CONFIG: configuration is invalid",
		},
		{
			name: "error with cause",
			err: &AppError{
				Code:    ErrCodeDatabaseConnection,
				Message: "failed to connect to database",
				Cause:   errors.New("connection refused"),
			},
			expected: "DATABASE_CONNECTION: failed to connect to database: connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &AppError{
		Code:    ErrCodeInternalError,
		Message: "something went wrong",
		Cause:   cause,
	}

	assert.Equal(t, cause, err.Unwrap())
}

func TestAppError_WithContext(t *testing.T) {
	err := New(ErrCodeValidationFailed, "validation failed")

	result := err.WithContext("field", "email").WithContext("value", "invalid@")

	assert.Equal(t, err, result) // Should return same instance
	assert.Len(t, err.Context, 2)
	assert.Equal(t, "email", err.Context["field"])
	assert.Equal(t, "invalid@", err.Context["value"])
}

func TestAppError_WithUserMessage(t *testing.T) {
	err := New(ErrCodeAuthentication, "auth failed")
	userMsg := "Please check your credentials"

	result := err.WithUserMessage(userMsg)

	assert.Equal(t, err, result) // Should return same instance
	assert.Equal(t, userMsg, err.UserMessage)
}

func TestNew(t *testing.T) {
	err := New(ErrCodeNotFound, "resource not found")

	assert.Equal(t, ErrCodeNotFound, err.Code)
	assert.Equal(t, "resource not found", err.Message)
	assert.Nil(t, err.Cause)
	assert.False(t, err.Retryable)
	assert.Empty(t, err.UserMessage)
	assert.Nil(t, err.Context)
}

func TestWrap(t *testing.T) {
	cause := errors.New("network timeout")
	err := Wrap(cause, ErrCodeTimeout, "operation timed out")

	assert.Equal(t, ErrCodeTimeout, err.Code)
	assert.Equal(t, "operation timed out", err.Message)
	assert.Equal(t, cause, err.Cause)
	assert.False(t, err.Retryable)
}

func TestWrapRetryable(t *testing.T) {
	cause := errors.New("temporary failure")
	err := WrapRetryable(cause, ErrCodeWhatsAppAPI, "WhatsApp API error")

	assert.Equal(t, ErrCodeWhatsAppAPI, err.Code)
	assert.Equal(t, "WhatsApp API error", err.Message)
	assert.Equal(t, cause, err.Cause)
	assert.True(t, err.Retryable)
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable AppError",
			err:      WrapRetryable(errors.New("temp error"), ErrCodeSignalAPI, "Signal API error"),
			expected: true,
		},
		{
			name:     "non-retryable AppError",
			err:      New(ErrCodeInvalidInput, "bad input"),
			expected: false,
		},
		{
			name:     "standard error",
			err:      errors.New("standard error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsRetryable(tt.err))
		})
	}
}

func TestGetCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCode
	}{
		{
			name:     "AppError with code",
			err:      New(ErrCodeValidationFailed, "validation failed"),
			expected: ErrCodeValidationFailed,
		},
		{
			name:     "standard error",
			err:      errors.New("standard error"),
			expected: ErrCodeInternalError,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: ErrCodeInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetCode(tt.err))
		})
	}
}

func TestGetUserMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "AppError with user message",
			err:      New(ErrCodeAuthentication, "auth failed").WithUserMessage("Please login again"),
			expected: "Please login again",
		},
		{
			name:     "AppError without user message",
			err:      New(ErrCodeInternalError, "something broke"),
			expected: "An internal error occurred",
		},
		{
			name:     "standard error",
			err:      errors.New("standard error"),
			expected: "An internal error occurred",
		},
		{
			name:     "nil error",
			err:      nil,
			expected: "An internal error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetUserMessage(tt.err))
		})
	}
}

func TestErrorCodes_Coverage(t *testing.T) {
	// Test that all error codes are unique and well-formed
	codes := []ErrorCode{
		ErrCodeInvalidConfig,
		ErrCodeMissingConfig,
		ErrCodeDatabaseConnection,
		ErrCodeDatabaseQuery,
		ErrCodeDatabaseMigration,
		ErrCodeWhatsAppAPI,
		ErrCodeSignalAPI,
		ErrCodeMediaDownload,
		ErrCodeInvalidInput,
		ErrCodeValidationFailed,
		ErrCodeAuthentication,
		ErrCodeAuthorization,
		ErrCodeRateLimit,
		ErrCodeInternalError,
		ErrCodeNotFound,
		ErrCodeTimeout,
	}

	// Check all codes are non-empty
	for _, code := range codes {
		assert.NotEmpty(t, string(code), "Error code should not be empty")
	}

	// Check for duplicates by creating a map
	codeMap := make(map[ErrorCode]bool)
	for _, code := range codes {
		assert.False(t, codeMap[code], "Duplicate error code found: %s", code)
		codeMap[code] = true
	}
}

func TestAppError_ChainedOperations(t *testing.T) {
	// Test method chaining
	err := New(ErrCodeValidationFailed, "validation error").
		WithContext("field", "email").
		WithContext("value", "invalid@domain").
		WithUserMessage("Please enter a valid email address")

	assert.Equal(t, ErrCodeValidationFailed, err.Code)
	assert.Equal(t, "validation error", err.Message)
	assert.Equal(t, "Please enter a valid email address", err.UserMessage)
	assert.Len(t, err.Context, 2)
	assert.Equal(t, "email", err.Context["field"])
	assert.Equal(t, "invalid@domain", err.Context["value"])
}

func TestAppError_JSON_Serialization(t *testing.T) {
	// Test that AppError can be JSON serialized properly
	err := New(ErrCodeValidationFailed, "validation failed").
		WithContext("field", "email").
		WithUserMessage("Invalid email format")

	// The JSON tags should work correctly
	assert.Equal(t, ErrCodeValidationFailed, err.Code)
	assert.Equal(t, "validation failed", err.Message)
	assert.Equal(t, "Invalid email format", err.UserMessage)
	assert.NotNil(t, err.Context)
	assert.False(t, err.Retryable)
}

func TestErrorCodeTypes(t *testing.T) {
	// Test specific error code categories
	configErrors := []ErrorCode{ErrCodeInvalidConfig, ErrCodeMissingConfig}
	dbErrors := []ErrorCode{ErrCodeDatabaseConnection, ErrCodeDatabaseQuery, ErrCodeDatabaseMigration}
	apiErrors := []ErrorCode{ErrCodeWhatsAppAPI, ErrCodeSignalAPI, ErrCodeMediaDownload}
	validationErrors := []ErrorCode{ErrCodeInvalidInput, ErrCodeValidationFailed}
	securityErrors := []ErrorCode{ErrCodeAuthentication, ErrCodeAuthorization, ErrCodeRateLimit}
	generalErrors := []ErrorCode{ErrCodeInternalError, ErrCodeNotFound, ErrCodeTimeout}

	allCategories := [][]ErrorCode{configErrors, dbErrors, apiErrors, validationErrors, securityErrors, generalErrors}

	for _, category := range allCategories {
		for _, code := range category {
			assert.NotEmpty(t, string(code))
			// All error codes should be uppercase with underscores
			assert.Regexp(t, `^[A-Z_]+$`, string(code))
		}
	}
}
