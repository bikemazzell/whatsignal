package database

import (
	"context"
	"fmt"
	"time"
	"whatsignal/internal/constants"
	"whatsignal/internal/retry"
)

// retryableDBOperationNoReturn executes a database operation that returns only an error with retry logic
func retryableDBOperationNoReturn(ctx context.Context, operation func() error, operationName string) error {
	var lastErr error

	maxAttempts := constants.DefaultDatabaseRetryAttempts
	backoff := retry.NewBackoff(retry.BackoffConfig{
		InitialDelay: time.Duration(constants.DefaultRetryBackoffMs) * time.Millisecond,
		MaxDelay:     time.Duration(constants.DefaultMaxBackoffMs) * time.Millisecond,
		Multiplier:   2.0,
		MaxAttempts:  maxAttempts,
		Jitter:       false,
	})

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

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff.GetNextDelay(attempt)):
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operationName, maxAttempts, lastErr)
}

// isRetryableDBError determines if a database error is worth retrying
func isRetryableDBError(err error) bool {
	return retry.IsRetryableDatabaseError(err)
}
