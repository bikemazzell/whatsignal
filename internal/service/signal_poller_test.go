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
	retryConfig := models.RetryConfig{}
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
	retryConfig := models.RetryConfig{}
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
	retryConfig := models.RetryConfig{}
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
	signalConfig := models.SignalConfig{}
	retryConfig := models.RetryConfig{}
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
