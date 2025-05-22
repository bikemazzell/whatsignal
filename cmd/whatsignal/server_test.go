package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"whatsignal/internal/models"
	"whatsignal/pkg/signal"
	"whatsignal/pkg/whatsapp/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockMessageService struct {
	mock.Mock
}

func (m *MockMessageService) SendMessage(ctx context.Context, msg *models.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockMessageService) ReceiveMessage(ctx context.Context, msg *models.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockMessageService) GetMessageByID(ctx context.Context, id string) (*models.Message, error) {
	args := m.Called(ctx, id)
	if msg := args.Get(0); msg != nil {
		return msg.(*models.Message), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockMessageService) GetMessageThread(ctx context.Context, threadID string) ([]*models.Message, error) {
	args := m.Called(ctx, threadID)
	if msgs := args.Get(0); msgs != nil {
		return msgs.([]*models.Message), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockMessageService) MarkMessageDelivered(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMessageService) DeleteMessage(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMessageService) HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content, mediaPath string) error {
	args := m.Called(ctx, chatID, msgID, sender, content, mediaPath)
	return args.Error(0)
}

func (m *MockMessageService) HandleSignalMessage(ctx context.Context, msg *signal.SignalMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockMessageService) UpdateDeliveryStatus(ctx context.Context, msgID string, status string) error {
	args := m.Called(ctx, msgID, status)
	return args.Error(0)
}

func TestServer_HandleHealth(t *testing.T) {
	mockService := new(MockMessageService)
	logger := logrus.New()
	server := NewServer(mockService, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestServer_HandleWhatsAppWebhook(t *testing.T) {
	mockService := new(MockMessageService)
	logger := logrus.New()
	server := NewServer(mockService, logger)

	// Set up mock expectations
	mockService.On("HandleWhatsAppMessage",
		mock.Anything,  // ctx
		"test-chat-id", // chatID
		"test-msg-id",  // msgID
		"test-sender",  // sender
		"test message", // content
		"",             // mediaPath
	).Return(nil)

	// Create a valid webhook event
	event := types.WebhookEvent{
		Event: "message.any",
		Payload: json.RawMessage(`{
			"id": "test-msg-id",
			"chatId": "test-chat-id",
			"sender": "test-sender",
			"type": "text",
			"content": "test message"
		}`),
	}

	payload, err := json.Marshal(event)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

func TestServer_HandleSignalWebhook(t *testing.T) {
	mockService := new(MockMessageService)
	logger := logrus.New()
	server := NewServer(mockService, logger)

	// Set up mock expectations
	mockService.On("HandleSignalMessage", mock.Anything, mock.AnythingOfType("*signal.SignalMessage")).Return(nil)

	// Create a valid Signal message
	msg := signal.SignalMessage{
		Timestamp:   time.Now().UnixMilli(),
		Sender:      "+1234567890",
		MessageID:   "test-msg-id",
		Message:     "test message",
		Attachments: []string{},
	}

	payload, err := json.Marshal(msg)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/webhook/signal", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}
