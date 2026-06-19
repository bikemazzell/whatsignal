package retry

import (
	"context"
	"errors"
	"testing"

	appErrors "whatsignal/internal/errors"

	"github.com/stretchr/testify/assert"
)

func TestClassifierUsesTypedRetryableErrorsBeforeLegacyStrings(t *testing.T) {
	retryable := appErrors.WrapRetryable(errors.New("validation failed"), appErrors.ErrCodeWhatsAppAPI, "temporary upstream failure")
	assert.True(t, IsRetryableWhatsAppError(retryable))

	nonRetryable := appErrors.Wrap(errors.New("connection refused"), appErrors.ErrCodeWhatsAppAPI, "permanent upstream failure")
	assert.False(t, IsRetryableWhatsAppError(nonRetryable))
}

func TestClassifierHandlesDomainSpecificContextErrors(t *testing.T) {
	assert.False(t, IsRetryableDatabaseError(context.DeadlineExceeded))
	assert.True(t, IsRetryablePollError(context.DeadlineExceeded))
	assert.False(t, IsRetryablePollError(context.Canceled))
}

func TestClassifierKeepsLegacySubstringFallbacks(t *testing.T) {
	assert.True(t, IsRetryableDatabaseError(errors.New("database is locked")))
	assert.False(t, IsRetryableDatabaseError(errors.New("UNIQUE constraint failed")))

	assert.False(t, IsRetryableSignalError(errors.New("Untrusted Identity")))
	assert.True(t, IsRetryableSignalError(errors.New("temporary network timeout")))

	assert.False(t, IsRetryableWhatsAppError(errors.New("status 404 chat not found")))
	assert.True(t, IsRetryableWhatsAppError(errors.New("status 503 service unavailable")))
}
