package errors

import (
	"fmt"
)

// ErrorCode represents a categorized error type
type ErrorCode string

const (
	// Configuration errors
	ErrCodeInvalidConfig ErrorCode = "INVALID_CONFIG"
	ErrCodeMissingConfig ErrorCode = "MISSING_CONFIG"

	// Database errors
	ErrCodeDatabaseConnection ErrorCode = "DATABASE_CONNECTION"
	ErrCodeDatabaseQuery      ErrorCode = "DATABASE_QUERY"
	ErrCodeDatabaseMigration  ErrorCode = "DATABASE_MIGRATION"

	// External service errors
	ErrCodeWhatsAppAPI   ErrorCode = "WHATSAPP_API"
	ErrCodeSignalAPI     ErrorCode = "SIGNAL_API"
	ErrCodeMediaDownload ErrorCode = "MEDIA_DOWNLOAD"

	// Validation errors
	ErrCodeInvalidInput     ErrorCode = "INVALID_INPUT"
	ErrCodeValidationFailed ErrorCode = "VALIDATION_FAILED"

	// Security errors
	ErrCodeAuthentication ErrorCode = "AUTHENTICATION"
	ErrCodeAuthorization  ErrorCode = "AUTHORIZATION"
	ErrCodeRateLimit      ErrorCode = "RATE_LIMIT"

	// Internal errors
	ErrCodeInternalError ErrorCode = "INTERNAL_ERROR"
	ErrCodeNotFound      ErrorCode = "NOT_FOUND"
	ErrCodeTimeout       ErrorCode = "TIMEOUT"
)

// AppError represents a structured application error
type AppError struct {
	Code        ErrorCode              `json:"code"`
	Message     string                 `json:"message"`
	Cause       error                  `json:"-"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Retryable   bool                   `json:"retryable"`
	UserMessage string                 `json:"user_message,omitempty"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithContext adds context to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithUserMessage sets a user-friendly message
func (e *AppError) WithUserMessage(msg string) *AppError {
	e.UserMessage = msg
	return e
}

// New creates a new AppError
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// WrapRetryable wraps an error and marks it as retryable
func WrapRetryable(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		Cause:     err,
		Retryable: true,
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Retryable
	}
	return false
}

// GetCode extracts the error code from an error
func GetCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return ErrCodeInternalError
}

// GetUserMessage extracts a user-friendly message from an error
func GetUserMessage(err error) string {
	if appErr, ok := err.(*AppError); ok && appErr.UserMessage != "" {
		return appErr.UserMessage
	}
	return "An internal error occurred"
}
