package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"whatsignal/internal/constants"
)

// retryableDBOperationNoReturn executes a database operation that returns only an error with retry logic
func retryableDBOperationNoReturn(ctx context.Context, operation func() error, operationName string) error {
	var lastErr error

	maxAttempts := constants.DefaultDatabaseRetryAttempts
	initialBackoff := time.Duration(constants.DefaultRetryBackoffMs) * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on certain non-retryable errors
		if !isRetryableDBError(err) {
			return fmt.Errorf("%s failed (non-retryable): %w", operationName, err)
		}

		// Don't wait on the last attempt
		if attempt == maxAttempts {
			break
		}

		// Exponential backoff with jitter
		backoff := time.Duration(attempt) * initialBackoff
		if backoff > time.Duration(constants.DefaultMaxBackoffMs)*time.Millisecond {
			backoff = time.Duration(constants.DefaultMaxBackoffMs) * time.Millisecond
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operationName, maxAttempts, lastErr)
}

// isRetryableDBError determines if a database error is worth retrying
func isRetryableDBError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable SQLite errors
	errStr := err.Error()

	// Database is locked errors are typically retryable
	if strings.Contains(errStr, "database is locked") {
		return true
	}

	// Disk I/O errors might be transient
	if strings.Contains(errStr, "disk I/O error") {
		return true
	}

	// Temporary network issues (for network-mounted databases)
	if strings.Contains(errStr, "no such host") || strings.Contains(errStr, "connection refused") {
		return true
	}

	// Context timeout/cancellation are not retryable by us
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// SQL constraint violations are not retryable
	if strings.Contains(errStr, "UNIQUE constraint") || strings.Contains(errStr, "FOREIGN KEY constraint") {
		return false
	}

	// Schema errors are not retryable
	if strings.Contains(errStr, "no such table") || strings.Contains(errStr, "no such column") {
		return false
	}

	// For other errors, we'll be conservative and not retry
	return false
}
