package errors

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("email", "invalid@", "must be a valid email address")

	assert.Equal(t, ErrCodeValidationFailed, err.Code)
	assert.Equal(t, "must be a valid email address", err.Message)
	assert.Equal(t, "Invalid email: must be a valid email address", err.UserMessage)
	assert.Equal(t, "email", err.Context["field"])
	assert.Equal(t, "invalid@", err.Context["value"])
}

func TestNewConfigError(t *testing.T) {
	err := NewConfigError("database.host", "missing required configuration")

	assert.Equal(t, ErrCodeInvalidConfig, err.Code)
	assert.Equal(t, "missing required configuration", err.Message)
	assert.Equal(t, "Configuration error", err.UserMessage)
	assert.Equal(t, "database.host", err.Context["config_key"])
}

func TestNewDatabaseError(t *testing.T) {
	originalErr := errors.New("connection failed")
	err := NewDatabaseError("insert", originalErr)

	assert.Equal(t, ErrCodeDatabaseQuery, err.Code)
	assert.Equal(t, "database insert failed", err.Message)
	assert.Equal(t, "Database operation failed", err.UserMessage)
	assert.Equal(t, originalErr, err.Cause)
	assert.Equal(t, "insert", err.Context["operation"])
}

func TestNewAPIError(t *testing.T) {
	tests := []struct {
		name         string
		service      string
		endpoint     string
		statusCode   int
		expectedCode ErrorCode
		retryable    bool
	}{
		{
			name:         "WhatsApp API 500 error",
			service:      "whatsapp",
			endpoint:     "/api/sendText",
			statusCode:   500,
			expectedCode: ErrCodeWhatsAppAPI,
			retryable:    true,
		},
		{
			name:         "Signal API 400 error",
			service:      "signal",
			endpoint:     "/v1/send",
			statusCode:   400,
			expectedCode: ErrCodeSignalAPI,
			retryable:    false,
		},
		{
			name:         "Unknown service 429 error",
			service:      "unknown",
			endpoint:     "/test",
			statusCode:   429,
			expectedCode: ErrCodeInternalError,
			retryable:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalErr := errors.New("API error")
			err := NewAPIError(tt.service, tt.endpoint, tt.statusCode, originalErr)

			assert.Equal(t, tt.expectedCode, err.Code)
			assert.Equal(t, tt.retryable, err.Retryable)
			assert.Equal(t, tt.service, err.Context["service"])
			assert.Equal(t, tt.endpoint, err.Context["endpoint"])
			assert.Equal(t, tt.statusCode, err.Context["status_code"])
			assert.Equal(t, originalErr, err.Cause)
		})
	}
}

func TestNewTimeoutError(t *testing.T) {
	err := NewTimeoutError("database query", "30s")

	assert.Equal(t, ErrCodeTimeout, err.Code)
	assert.Equal(t, "database query timed out after 30s", err.Message)
	assert.Equal(t, "Operation timed out, please try again", err.UserMessage)
	assert.Equal(t, "database query", err.Context["operation"])
	assert.Equal(t, "30s", err.Context["timeout"])
}

func TestNewAuthError(t *testing.T) {
	err := NewAuthError("invalid token")

	assert.Equal(t, ErrCodeAuthentication, err.Code)
	assert.Equal(t, "authentication failed", err.Message)
	assert.Equal(t, "Authentication failed", err.UserMessage)
	assert.Equal(t, "invalid token", err.Context["reason"])
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("user", "123")

	assert.Equal(t, ErrCodeNotFound, err.Code)
	assert.Equal(t, "user not found", err.Message)
	assert.Equal(t, "user not found", err.UserMessage)
	assert.Equal(t, "user", err.Context["resource"])
	assert.Equal(t, "123", err.Context["identifier"])
}

func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError(100, "1m")

	assert.Equal(t, ErrCodeRateLimit, err.Code)
	assert.Equal(t, "rate limit exceeded", err.Message)
	assert.Equal(t, "Too many requests, please try again later", err.UserMessage)
	assert.Equal(t, 100, err.Context["limit"])
	assert.Equal(t, "1m", err.Context["window"])
}

func TestNewMediaError(t *testing.T) {
	originalErr := errors.New("download failed")
	err := NewMediaError("download", "image/jpeg", originalErr)

	assert.Equal(t, ErrCodeMediaDownload, err.Code)
	assert.Equal(t, "media download failed", err.Message)
	assert.Equal(t, "Media processing failed", err.UserMessage)
	assert.Equal(t, originalErr, err.Cause)
	assert.Equal(t, "download", err.Context["operation"])
	assert.Equal(t, "image/jpeg", err.Context["media_type"])
}

func TestFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected map[string]interface{}
	}{
		{
			name:     "nil context",
			ctx:      nil,
			expected: nil,
		},
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: map[string]interface{}{},
		},
		{
			name: "context with values",
			ctx: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "request_id", "req_123") //nolint:staticcheck
				ctx = context.WithValue(ctx, "trace_id", "trace_456") //nolint:staticcheck
				ctx = context.WithValue(ctx, "user_id", "user_789")   //nolint:staticcheck
				return ctx
			}(),
			expected: map[string]interface{}{
				"request_id": "req_123",
				"trace_id":   "trace_456",
				"user_id":    "user_789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromContext(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWithContextFromRequest(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req_123")        //nolint:staticcheck
	ctx = context.WithValue(ctx, "session_name", "test-session") //nolint:staticcheck

	err := New(ErrCodeValidationFailed, "validation failed")
	result := WithContextFromRequest(err, ctx)

	assert.Equal(t, err, result) // Should return same instance
	assert.Equal(t, "req_123", err.Context["request_id"])
	assert.Equal(t, "test-session", err.Context["session_name"])
}

func TestHTTPStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode int
	}{
		{
			name:         "validation error",
			err:          New(ErrCodeValidationFailed, "validation failed"),
			expectedCode: 400,
		},
		{
			name:         "authentication error",
			err:          New(ErrCodeAuthentication, "auth failed"),
			expectedCode: 401,
		},
		{
			name:         "authorization error",
			err:          New(ErrCodeAuthorization, "access denied"),
			expectedCode: 403,
		},
		{
			name:         "not found error",
			err:          New(ErrCodeNotFound, "resource not found"),
			expectedCode: 404,
		},
		{
			name:         "timeout error",
			err:          New(ErrCodeTimeout, "operation timed out"),
			expectedCode: 408,
		},
		{
			name:         "rate limit error",
			err:          New(ErrCodeRateLimit, "rate limit exceeded"),
			expectedCode: 429,
		},
		{
			name:         "retryable API error",
			err:          WrapRetryable(errors.New("temp failure"), ErrCodeWhatsAppAPI, "WhatsApp API error"),
			expectedCode: 502,
		},
		{
			name:         "non-retryable API error",
			err:          New(ErrCodeSignalAPI, "Signal API error"),
			expectedCode: 500,
		},
		{
			name:         "database error",
			err:          New(ErrCodeDatabaseConnection, "database connection failed"),
			expectedCode: 503,
		},
		{
			name:         "internal error",
			err:          New(ErrCodeInternalError, "something went wrong"),
			expectedCode: 500,
		},
		{
			name:         "standard error",
			err:          errors.New("standard error"),
			expectedCode: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedCode, HTTPStatusCode(tt.err))
		})
	}
}

func TestToHTTPResponse(t *testing.T) {
	tests := []struct {
		name              string
		err               error
		requestID         string
		expectedCode      ErrorCode
		expectedMessage   string
		expectContext     bool
		expectedRequestID string
	}{
		{
			name: "AppError with context",
			err: New(ErrCodeValidationFailed, "validation failed").
				WithContext("field", "email").
				WithContext("password", "secret123"). // Should be filtered out
				WithUserMessage("Please enter a valid email"),
			requestID:         "req_123",
			expectedCode:      ErrCodeValidationFailed,
			expectedMessage:   "Please enter a valid email",
			expectContext:     true,
			expectedRequestID: "req_123",
		},
		{
			name:              "standard error",
			err:               errors.New("something went wrong"),
			requestID:         "req_456",
			expectedCode:      ErrCodeInternalError,
			expectedMessage:   "An internal error occurred",
			expectContext:     false,
			expectedRequestID: "req_456",
		},
		{
			name:              "AppError without user message",
			err:               New(ErrCodeNotFound, "user not found"),
			requestID:         "",
			expectedCode:      ErrCodeNotFound,
			expectedMessage:   "An internal error occurred",
			expectContext:     false,
			expectedRequestID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := ToHTTPResponse(tt.err, tt.requestID)

			assert.Equal(t, tt.expectedCode, response.Error.Code)
			assert.Equal(t, tt.expectedMessage, response.Error.Message)
			assert.Equal(t, tt.expectedRequestID, response.RequestID)

			if tt.expectContext {
				assert.NotNil(t, response.Error.Context)
				contextMap := response.Error.Context.(map[string]interface{})
				assert.Contains(t, contextMap, "field")
				assert.Equal(t, "email", contextMap["field"])
				// Sensitive field should be filtered out
				assert.NotContains(t, contextMap, "password")
			} else {
				assert.Nil(t, response.Error.Context)
			}
		})
	}
}

func TestChain(t *testing.T) {
	tests := []struct {
		name     string
		errors   []*AppError
		expected *AppError
	}{
		{
			name:     "no errors",
			errors:   []*AppError{},
			expected: nil,
		},
		{
			name: "single error",
			errors: []*AppError{
				New(ErrCodeValidationFailed, "validation failed"),
			},
			expected: New(ErrCodeValidationFailed, "validation failed"),
		},
		{
			name: "multiple errors",
			errors: []*AppError{
				New(ErrCodeValidationFailed, "email validation failed").WithContext("field", "email"),
				New(ErrCodeValidationFailed, "password validation failed").WithContext("field", "password"),
			},
			expected: &AppError{
				Code:    ErrCodeValidationFailed,
				Message: "multiple errors: [email validation failed (2) password validation failed]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Chain(tt.errors...)

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, tt.expected.Code, result.Code)

			if len(tt.errors) > 1 {
				assert.Contains(t, result.Message, "multiple errors")
				// Check that context from all errors is merged
				assert.NotNil(t, result.Context)
			}
		})
	}
}

func TestAPIError_RetryableStatusCodes(t *testing.T) {
	retryableCodes := []int{500, 502, 503, 504, 429, 408}
	nonRetryableCodes := []int{400, 401, 403, 404, 422}

	for _, code := range retryableCodes {
		t.Run(fmt.Sprintf("status_%d_should_be_retryable", code), func(t *testing.T) {
			err := NewAPIError("whatsapp", "/test", code, errors.New("api error"))
			assert.True(t, err.Retryable, "Status code %d should be retryable", code)
		})
	}

	for _, code := range nonRetryableCodes {
		t.Run(fmt.Sprintf("status_%d_should_not_be_retryable", code), func(t *testing.T) {
			err := NewAPIError("whatsapp", "/test", code, errors.New("api error"))
			assert.False(t, err.Retryable, "Status code %d should not be retryable", code)
		})
	}
}
