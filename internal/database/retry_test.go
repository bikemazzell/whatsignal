package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryableDBOperationNoReturn_Success(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	operation := func() error {
		callCount++
		return nil // Success on first attempt
	}

	err := retryableDBOperationNoReturn(ctx, operation, "test operation")
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRetryableDBOperationNoReturn_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	operation := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("database is locked") // Retryable error
		}
		return nil // Success on third attempt
	}

	err := retryableDBOperationNoReturn(ctx, operation, "test operation")
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestRetryableDBOperationNoReturn_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	operation := func() error {
		callCount++
		return errors.New("UNIQUE constraint failed") // Non-retryable error
	}

	err := retryableDBOperationNoReturn(ctx, operation, "test operation")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-retryable")
	assert.Equal(t, 1, callCount) // Should not retry
}

func TestRetryableDBOperationNoReturn_MaxAttemptsReached(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	operation := func() error {
		callCount++
		return errors.New("database is locked") // Always retryable error
	}

	err := retryableDBOperationNoReturn(ctx, operation, "test operation")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after")
	assert.Contains(t, err.Error(), "attempts")
	assert.Equal(t, 3, callCount) // Should use default retry attempts (3)
}

func TestRetryableDBOperationNoReturn_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	operation := func() error {
		callCount++
		if callCount == 1 {
			cancel() // Cancel context on first call
		}
		return errors.New("database is locked")
	}

	err := retryableDBOperationNoReturn(ctx, operation, "test operation")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, 1, callCount)
}

func TestRetryableDBOperationNoReturn_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	callCount := 0
	operation := func() error {
		callCount++
		return errors.New("database is locked") // Retryable error
	}

	err := retryableDBOperationNoReturn(ctx, operation, "test operation")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	// callCount might be 1 or 2 depending on timing
	assert.True(t, callCount >= 1)
}

func TestIsRetryableDBError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "database is locked",
			err:      errors.New("database is locked"),
			expected: true,
		},
		{
			name:     "disk I/O error",
			err:      errors.New("disk I/O error"),
			expected: true,
		},
		{
			name:     "no such host",
			err:      errors.New("no such host"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "UNIQUE constraint failed",
			err:      errors.New("UNIQUE constraint failed: duplicate key"),
			expected: false,
		},
		{
			name:     "FOREIGN KEY constraint failed",
			err:      errors.New("FOREIGN KEY constraint failed"),
			expected: false,
		},
		{
			name:     "no such table",
			err:      errors.New("no such table: users"),
			expected: false,
		},
		{
			name:     "no such column",
			err:      errors.New("no such column: email"),
			expected: false,
		},
		{
			name:     "random error",
			err:      errors.New("some random error"),
			expected: false,
		},
		{
			name:     "wrapped context error",
			err:      fmt.Errorf("operation failed: %w", context.Canceled),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableDBError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRetryableDBError_CaseSensitive(t *testing.T) {
	// Test that error matching is case-sensitive (as implemented)
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Database Is Locked (mixed case)",
			err:      errors.New("Database Is Locked"),
			expected: false, // Case sensitive - should not match
		},
		{
			name:     "DATABASE IS LOCKED (uppercase)",
			err:      errors.New("DATABASE IS LOCKED"),
			expected: false, // Case sensitive - should not match
		},
		{
			name:     "Disk I/O Error (mixed case)",
			err:      errors.New("Disk I/O Error"),
			expected: false, // Case sensitive - should not match
		},
		{
			name:     "database is locked (lowercase)",
			err:      errors.New("database is locked"),
			expected: true, // Exact match
		},
		{
			name:     "disk I/O error (lowercase)",
			err:      errors.New("disk I/O error"),
			expected: true, // Exact match
		},
		{
			name:     "UNIQUE Constraint Failed (mixed case)",
			err:      errors.New("UNIQUE Constraint Failed"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableDBError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryableDBOperationNoReturn_BackoffProgression(t *testing.T) {
	ctx := context.Background()
	callCount := 0
	var callTimes []time.Time

	operation := func() error {
		callCount++
		callTimes = append(callTimes, time.Now())
		return errors.New("database is locked") // Always fail to test backoff
	}

	err := retryableDBOperationNoReturn(ctx, operation, "test operation")
	assert.Error(t, err)
	assert.Equal(t, 3, callCount)
	assert.Len(t, callTimes, 3)

	// Verify that there was some delay between calls (backoff)
	if len(callTimes) >= 2 {
		firstDelay := callTimes[1].Sub(callTimes[0])
		// Be more lenient with timing in tests
		assert.True(t, firstDelay > 1*time.Millisecond, "Expected some backoff delay, got %v", firstDelay)
	}

	if len(callTimes) >= 3 {
		secondDelay := callTimes[2].Sub(callTimes[1])
		// Second delay should be longer than first (exponential backoff)
		firstDelay := callTimes[1].Sub(callTimes[0])
		assert.True(t, secondDelay >= firstDelay, "Expected exponential backoff, first: %v, second: %v", firstDelay, secondDelay)
	}
}

func TestRetryableDBOperationNoReturn_ErrorFormatting(t *testing.T) {
	ctx := context.Background()

	// Test non-retryable error formatting
	operation := func() error {
		return errors.New("UNIQUE constraint failed")
	}

	err := retryableDBOperationNoReturn(ctx, operation, "insert user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert user failed (non-retryable)")
	assert.Contains(t, err.Error(), "UNIQUE constraint failed")

	// Test max attempts reached error formatting
	operation = func() error {
		return errors.New("database is locked")
	}

	err = retryableDBOperationNoReturn(ctx, operation, "update record")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update record failed after")
	assert.Contains(t, err.Error(), "attempts")
	assert.Contains(t, err.Error(), "database is locked")
}

func TestRetryableDBOperationNoReturn_ContextCheckBeforeOperation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	callCount := 0
	operation := func() error {
		callCount++
		return nil
	}

	err := retryableDBOperationNoReturn(ctx, operation, "test operation")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, 0, callCount) // Operation should never be called
}

func TestRetryableDBOperationNoReturn_ContextCheckDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	operation := func() error {
		callCount++
		if callCount == 1 {
			// Cancel context after first call, during backoff
			go func() {
				time.Sleep(10 * time.Millisecond)
				cancel()
			}()
		}
		return errors.New("database is locked")
	}

	err := retryableDBOperationNoReturn(ctx, operation, "test operation")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, 1, callCount) // Should stop after first call due to cancellation
}
