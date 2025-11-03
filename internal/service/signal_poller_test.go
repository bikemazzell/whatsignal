package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"whatsignal/internal/models"
	signaltypes "whatsignal/pkg/signal/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockMessageService struct {
	mock.Mock
	mu        sync.Mutex
	pollCalls int
}

func (m *mockMessageService) PollSignalMessages(ctx context.Context) error {
	m.mu.Lock()
	m.pollCalls++
	m.mu.Unlock()
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockMessageService) GetPollCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pollCalls
}

func (m *mockMessageService) SendMessage(ctx context.Context, msg *models.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockMessageService) ReceiveMessage(ctx context.Context, msg *models.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockMessageService) GetMessageByID(ctx context.Context, id string) (*models.Message, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models.Message), args.Error(1)
}

func (m *mockMessageService) GetMessageThread(ctx context.Context, threadID string) ([]*models.Message, error) {
	args := m.Called(ctx, threadID)
	return args.Get(0).([]*models.Message), args.Error(1)
}

func (m *mockMessageService) MarkMessageDelivered(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockMessageService) DeleteMessage(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Deprecated: Use HandleWhatsAppMessageWithSession instead
func (m *mockMessageService) HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content string, mediaPath string) error {
	args := m.Called(ctx, chatID, msgID, sender, content, mediaPath)
	return args.Error(0)
}

func (m *mockMessageService) HandleSignalMessage(ctx context.Context, msg *models.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockMessageService) ProcessIncomingSignalMessage(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage) error {
	args := m.Called(ctx, rawSignalMsg)
	return args.Error(0)
}

func (m *mockMessageService) UpdateDeliveryStatus(ctx context.Context, msgID string, status string) error {
	args := m.Called(ctx, msgID, status)
	return args.Error(0)
}

func (m *mockMessageService) HandleWhatsAppMessageWithSession(ctx context.Context, sessionName, chatID, msgID, sender, content string, mediaPath string) error {
	args := m.Called(ctx, sessionName, chatID, msgID, sender, content, mediaPath)
	return args.Error(0)
}

func (m *mockMessageService) ProcessIncomingSignalMessageWithDestination(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage, destination string) error {
	args := m.Called(ctx, rawSignalMsg, destination)
	return args.Error(0)
}

func (m *mockMessageService) SendSignalNotification(ctx context.Context, sessionName, message string) error {
	args := m.Called(ctx, sessionName, message)
	return args.Error(0)
}

func (m *mockMessageService) GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error) {
	args := m.Called(ctx, whatsappID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func TestSignalPoller_NewSignalPoller(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
		PollingEnabled:  true,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 1000,
		MaxBackoffMs:     5000,
		MaxAttempts:      3,
	}
	logger := logrus.New()

	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, logger)

	assert.NotNil(t, poller)
	assert.Equal(t, signalConfig, poller.config)
	assert.Equal(t, retryConfig, poller.retryConfig)
	assert.False(t, poller.IsRunning())
}

func TestSignalPoller_Start_Disabled(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
		PollingEnabled:  false,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 100,
		MaxBackoffMs:     500,
		MaxAttempts:      3,
	}
	logger := logrus.New()

	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, logger)

	ctx := context.Background()
	err := poller.Start(ctx)

	assert.NoError(t, err)
	assert.False(t, poller.IsRunning())
}

func TestSignalPoller_Start_InitializationFailure(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
		PollingEnabled:  true,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 100,
		MaxBackoffMs:     500,
		MaxAttempts:      3,
	}
	logger := logrus.New()

	expectedError := errors.New("initialization failed")
	mockSignalClient.On("InitializeDevice", mock.Anything).Return(expectedError)

	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, logger)

	ctx := context.Background()
	err := poller.Start(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to Signal CLI")
	assert.False(t, poller.IsRunning())
	mockSignalClient.AssertExpectations(t)
}

func TestSignalPoller_Start_Success(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 1, // Use shorter interval for testing
		PollingEnabled:  true,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 100,
		MaxBackoffMs:     500,
		MaxAttempts:      2,
	}
	logger := logrus.New()

	mockSignalClient.On("InitializeDevice", mock.Anything).Return(nil)
	mockMessageService.On("PollSignalMessages", mock.Anything).Return(nil)

	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := poller.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, poller.IsRunning())

	time.Sleep(2500 * time.Millisecond)

	poller.Stop()
	assert.False(t, poller.IsRunning())

	pollCalls := mockMessageService.GetPollCalls()
	assert.GreaterOrEqual(t, pollCalls, 1, "Should have made at least 1 poll call")
	assert.LessOrEqual(t, pollCalls, 3, "Should not have made too many poll calls")

	mockSignalClient.AssertExpectations(t)
}

func TestSignalPoller_Start_AlreadyRunning(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
		PollingEnabled:  true,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 100,
		MaxBackoffMs:     500,
		MaxAttempts:      3,
	}
	logger := logrus.New()

	mockSignalClient.On("InitializeDevice", mock.Anything).Return(nil)
	mockMessageService.On("PollSignalMessages", mock.Anything).Return(nil)

	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, logger)

	ctx := context.Background()
	err := poller.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, poller.IsRunning())

	err = poller.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	poller.Stop()
}

func TestSignalPoller_Stop_NotRunning(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 100,
		MaxBackoffMs:     500,
		MaxAttempts:      3,
	}
	logger := logrus.New()

	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, logger)

	assert.False(t, poller.IsRunning())
	poller.Stop()
	assert.False(t, poller.IsRunning())
}

func TestSignalPoller_RetryLogic(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 1,
		PollingEnabled:  true,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 50,
		MaxBackoffMs:     200,
		MaxAttempts:      3,
	}
	logger := logrus.New()

	mockSignalClient.On("InitializeDevice", mock.Anything).Return(nil)

	// First two calls fail, subsequent calls succeed
	mockMessageService.On("PollSignalMessages", mock.Anything).Return(errors.New("temporary failure")).Twice()
	mockMessageService.On("PollSignalMessages", mock.Anything).Return(nil)

	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := poller.Start(ctx)
	assert.NoError(t, err)

	time.Sleep(1500 * time.Millisecond)
	poller.Stop()

	// Verify that retries happened
	mockMessageService.AssertExpectations(t)
	mockSignalClient.AssertExpectations(t)
}

// New comprehensive tests for optimization fixes

func TestSignalPoller_NilLogger(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
		PollingEnabled:  false,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 100,
		MaxBackoffMs:     500,
		MaxAttempts:      3,
	}

	// Should not panic with nil logger
	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, nil)
	assert.NotNil(t, poller)
	assert.NotNil(t, poller.logger)

	// Should be able to start without panic
	ctx := context.Background()
	err := poller.Start(ctx)
	assert.NoError(t, err)
}

func TestSignalPoller_ConfigValidation(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	logger := logrus.New()

	tests := []struct {
		name         string
		signalConfig models.SignalConfig
		retryConfig  models.RetryConfig
		expectError  bool
		errorMsg     string
	}{
		{
			name: "Valid configuration",
			signalConfig: models.SignalConfig{
				PollIntervalSec: 5,
				PollTimeoutSec:  10,
				PollingEnabled:  true,
			},
			retryConfig: models.RetryConfig{
				InitialBackoffMs: 100,
				MaxBackoffMs:     500,
				MaxAttempts:      3,
			},
			expectError: false,
		},
		{
			name: "Zero poll interval",
			signalConfig: models.SignalConfig{
				PollIntervalSec: 0,
				PollingEnabled:  true,
			},
			retryConfig: models.RetryConfig{
				InitialBackoffMs: 100,
				MaxBackoffMs:     500,
				MaxAttempts:      3,
			},
			expectError: true,
			errorMsg:    "poll interval must be positive",
		},
		{
			name: "Negative poll timeout",
			signalConfig: models.SignalConfig{
				PollIntervalSec: 5,
				PollTimeoutSec:  -1,
				PollingEnabled:  true,
			},
			retryConfig: models.RetryConfig{
				InitialBackoffMs: 100,
				MaxBackoffMs:     500,
				MaxAttempts:      3,
			},
			expectError: true,
			errorMsg:    "poll timeout cannot be negative",
		},
		{
			name: "Zero max attempts",
			signalConfig: models.SignalConfig{
				PollIntervalSec: 5,
				PollingEnabled:  true,
			},
			retryConfig: models.RetryConfig{
				InitialBackoffMs: 100,
				MaxBackoffMs:     500,
				MaxAttempts:      0,
			},
			expectError: true,
			errorMsg:    "max retry attempts must be positive",
		},
		{
			name: "Negative initial backoff",
			signalConfig: models.SignalConfig{
				PollIntervalSec: 5,
				PollingEnabled:  true,
			},
			retryConfig: models.RetryConfig{
				InitialBackoffMs: -100,
				MaxBackoffMs:     500,
				MaxAttempts:      3,
			},
			expectError: true,
			errorMsg:    "initial backoff cannot be negative",
		},
		{
			name: "Max backoff less than initial",
			signalConfig: models.SignalConfig{
				PollIntervalSec: 5,
				PollingEnabled:  true,
			},
			retryConfig: models.RetryConfig{
				InitialBackoffMs: 500,
				MaxBackoffMs:     100,
				MaxAttempts:      3,
			},
			expectError: true,
			errorMsg:    "max backoff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSignalClient.On("InitializeDevice", mock.Anything).Return(nil).Maybe()

			poller := NewSignalPoller(mockSignalClient, mockMessageService, tt.signalConfig, tt.retryConfig, logger)

			ctx := context.Background()
			err := poller.Start(ctx)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				poller.Stop()
			}
		})
	}
}

func TestSignalPoller_ErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "Nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "Context cancelled",
			err:       context.Canceled,
			retryable: false,
		},
		{
			name:      "Context deadline exceeded",
			err:       context.DeadlineExceeded,
			retryable: true,
		},
		{
			name:      "Connection refused",
			err:       errors.New("connection refused"),
			retryable: true,
		},
		{
			name:      "Connection reset",
			err:       errors.New("connection reset by peer"),
			retryable: true,
		},
		{
			name:      "Timeout error",
			err:       errors.New("request timeout"),
			retryable: true,
		},
		{
			name:      "EOF error",
			err:       errors.New("unexpected EOF"),
			retryable: true,
		},
		{
			name:      "Unauthorized error",
			err:       errors.New("unauthorized access"),
			retryable: false,
		},
		{
			name:      "Forbidden error",
			err:       errors.New("forbidden resource"),
			retryable: false,
		},
		{
			name:      "Invalid request",
			err:       errors.New("invalid request format"),
			retryable: false,
		},
		{
			name:      "Malformed data",
			err:       errors.New("malformed JSON"),
			retryable: false,
		},
		{
			name:      "Unknown error",
			err:       errors.New("some unknown error"),
			retryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			assert.Equal(t, tt.retryable, result, "Error: %v", tt.err)
		})
	}
}

func TestSignalPoller_ContextCancellationDuringRetry(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 1,
		PollingEnabled:  true,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 100,
		MaxBackoffMs:     500,
		MaxAttempts:      10, // Many retries
	}
	logger := logrus.New()

	mockSignalClient.On("InitializeDevice", mock.Anything).Return(nil)

	// Always fail to trigger retries
	mockMessageService.On("PollSignalMessages", mock.Anything).Return(errors.New("temporary failure"))

	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := poller.Start(ctx)
	assert.NoError(t, err)

	// Wait a bit for polling to start
	time.Sleep(500 * time.Millisecond)

	// Stop the poller (cancels context)
	poller.Stop()

	// Should stop gracefully without hanging
	assert.False(t, poller.IsRunning())
}

func TestSignalPoller_IdempotentStop(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
		PollingEnabled:  false,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 100,
		MaxBackoffMs:     500,
		MaxAttempts:      3,
	}
	logger := logrus.New()

	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, logger)

	// Stop without starting - should not panic
	assert.NotPanics(t, func() {
		poller.Stop()
	})

	// Stop multiple times - should not panic
	assert.NotPanics(t, func() {
		poller.Stop()
		poller.Stop()
		poller.Stop()
	})
}

func TestSignalPoller_NonRetryableErrorStopsRetries(t *testing.T) {
	mockSignalClient := &mockSignalClient{}
	mockMessageService := &mockMessageService{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 1,
		PollingEnabled:  true,
	}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 50,
		MaxBackoffMs:     200,
		MaxAttempts:      10, // Many retries configured
	}
	logger := logrus.New()

	mockSignalClient.On("InitializeDevice", mock.Anything).Return(nil)

	// Return non-retryable error
	mockMessageService.On("PollSignalMessages", mock.Anything).Return(errors.New("unauthorized access")).Once()
	// Should not be called again
	mockMessageService.On("PollSignalMessages", mock.Anything).Return(nil)

	poller := NewSignalPoller(mockSignalClient, mockMessageService, signalConfig, retryConfig, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := poller.Start(ctx)
	assert.NoError(t, err)

	// Wait for one poll cycle
	time.Sleep(1500 * time.Millisecond)
	poller.Stop()

	// Should have been called only once (non-retryable error)
	mockMessageService.AssertNumberOfCalls(t, "PollSignalMessages", 1)
}
