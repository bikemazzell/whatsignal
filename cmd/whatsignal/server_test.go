package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"whatsignal/internal/models"

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

	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_HandleSignalWebhook(t *testing.T) {
	mockService := new(MockMessageService)
	logger := logrus.New()
	server := NewServer(mockService, logger)

	req := httptest.NewRequest(http.MethodPost, "/webhook/signal", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
