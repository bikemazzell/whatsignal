package retry

import (
	"context"
	stdErrors "errors"
	"strings"

	appErrors "whatsignal/internal/errors"
)

var nonRetryableSignalErrors = []string{
	"Untrusted Identity",
	"Unregistered user",
	"Invalid registration",
	"Rate limit",
	"Invalid phone number",
	"Forbidden",
	"Not found",
}

var nonRetryableWhatsAppErrors = []string{
	"status 400",
	"status 401",
	"status 403",
	"status 404",
	"invalid chat",
	"not registered",
	"blocked",
	"session not found",
}

// IsRetryableDatabaseError determines if a database error is worth retrying.
func IsRetryableDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	if stdErrors.Is(err, context.Canceled) || stdErrors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if retryable, ok := typedRetryable(err); ok {
		return retryable
	}

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "database is locked") ||
		strings.Contains(errStr, "disk i/o error") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "connection refused") {
		return true
	}
	if strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "foreign key constraint") ||
		strings.Contains(errStr, "no such table") ||
		strings.Contains(errStr, "no such column") {
		return false
	}

	return false
}

// IsRetryableSignalError determines if a Signal API error should be retried.
func IsRetryableSignalError(err error) bool {
	if err == nil {
		return false
	}
	if retryable, ok := typedRetryable(err); ok {
		return retryable
	}

	errStr := err.Error()
	for _, nonRetryable := range nonRetryableSignalErrors {
		if strings.Contains(errStr, nonRetryable) {
			return false
		}
	}

	return true
}

// IsRetryableWhatsAppError determines if a WAHA/WhatsApp API error should be retried.
func IsRetryableWhatsAppError(err error) bool {
	if err == nil {
		return false
	}
	if retryable, ok := typedRetryable(err); ok {
		return retryable
	}

	errStr := strings.ToLower(err.Error())
	for _, nonRetryable := range nonRetryableWhatsAppErrors {
		if strings.Contains(errStr, strings.ToLower(nonRetryable)) {
			return false
		}
	}

	if isLegacyTransientError(errStr) {
		return true
	}

	return true
}

// IsRetryablePollError determines if a Signal polling error should be retried.
func IsRetryablePollError(err error) bool {
	if err == nil {
		return false
	}
	if stdErrors.Is(err, context.Canceled) {
		return false
	}
	if stdErrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if retryable, ok := typedRetryable(err); ok {
		return retryable
	}

	errStr := strings.ToLower(err.Error())
	if isLegacyTransientError(errStr) || strings.Contains(errStr, "no route to host") {
		return true
	}
	if strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "forbidden") ||
		strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "not authorized") ||
		strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "malformed") ||
		strings.Contains(errStr, "bad request") {
		return false
	}

	return true
}

func typedRetryable(err error) (bool, bool) {
	var appErr *appErrors.AppError
	if stdErrors.As(err, &appErr) {
		return appErr.Retryable, true
	}
	return false, false
}

func isLegacyTransientError(errStr string) bool {
	return strings.Contains(errStr, "status 500") ||
		strings.Contains(errStr, "status 502") ||
		strings.Contains(errStr, "status 503") ||
		strings.Contains(errStr, "status 504") ||
		strings.Contains(errStr, "markedunread") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection timeout") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "temporary error") ||
		strings.Contains(errStr, "eof") ||
		strings.Contains(errStr, "broken pipe")
}
