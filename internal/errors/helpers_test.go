package errors

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				ctx = context.WithValue(ctx, requestIDKey, "req_123")
				ctx = context.WithValue(ctx, traceIDKey, "trace_456")
				ctx = context.WithValue(ctx, userIDKey, "user_789")
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
	ctx = context.WithValue(ctx, requestIDKey, "req_123")
	ctx = context.WithValue(ctx, sessionNameKey, "test-session")

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
