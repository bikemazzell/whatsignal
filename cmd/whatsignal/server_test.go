package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"whatsignal/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockMessageService struct {
	mock.Mock
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Message), args.Error(1)
}

func (m *mockMessageService) GetMessageThread(ctx context.Context, threadID string) ([]*models.Message, error) {
	args := m.Called(ctx, threadID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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

func (m *mockMessageService) HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content string, mediaPath string) error {
	args := m.Called(ctx, chatID, msgID, sender, content, mediaPath)
	return args.Error(0)
}

func (m *mockMessageService) HandleSignalMessage(ctx context.Context, msg *models.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockMessageService) UpdateDeliveryStatus(ctx context.Context, msgID string, status string) error {
	args := m.Called(ctx, msgID, status)
	return args.Error(0)
}

func TestServer_Health(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	server := NewServer(msgService, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth()(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_WhatsAppWebhook(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	server := NewServer(msgService, logger)

	tests := []struct {
		name       string
		payload    interface{}
		setup      func()
		wantStatus int
	}{
		{
			name: "valid text message",
			payload: map[string]interface{}{
				"event": "message",
				"data": map[string]interface{}{
					"id":      "msg123",
					"chatId":  "chat123",
					"sender":  "sender123",
					"type":    "text",
					"content": "Hello, World!",
				},
			},
			setup: func() {
				msgService.On("HandleWhatsAppMessage",
					mock.Anything,
					"chat123",
					"msg123",
					"sender123",
					"Hello, World!",
					"",
				).Return(nil).Once()
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "valid media message",
			payload: map[string]interface{}{
				"event": "message",
				"data": map[string]interface{}{
					"id":        "msg124",
					"chatId":    "chat124",
					"sender":    "sender124",
					"type":      "image",
					"content":   "Check this out!",
					"mediaPath": "/path/to/image.jpg",
				},
			},
			setup: func() {
				msgService.On("HandleWhatsAppMessage",
					mock.Anything,
					"chat124",
					"msg124",
					"sender124",
					"Check this out!",
					"/path/to/image.jpg",
				).Return(nil).Once()
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid payload",
			payload: map[string]string{
				"invalid": "payload",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "service error",
			payload: map[string]interface{}{
				"event": "message",
				"data": map[string]interface{}{
					"id":      "msg125",
					"chatId":  "chat125",
					"sender":  "sender125",
					"type":    "text",
					"content": "Error message",
				},
			},
			setup: func() {
				msgService.On("HandleWhatsAppMessage",
					mock.Anything,
					"chat125",
					"msg125",
					"sender125",
					"Error message",
					"",
				).Return(assert.AnError).Once()
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			payload, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			server.handleWhatsAppWebhook()(w, req)

			resp := w.Result()
			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			msgService.AssertExpectations(t)
		})
	}
}

func TestServer_SignalWebhook(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	server := NewServer(msgService, logger)

	tests := []struct {
		name       string
		payload    interface{}
		setup      func()
		wantStatus int
	}{
		{
			name: "valid message",
			payload: map[string]interface{}{
				"messageId": "sig123",
				"sender":    "+1234567890",
				"message":   "Hello, Signal!",
				"timestamp": time.Now().UnixMilli(),
				"type":      "text",
				"threadId":  "thread123",
				"recipient": "+0987654321",
			},
			setup: func() {
				msgService.On("HandleSignalMessage",
					mock.Anything,
					mock.MatchedBy(func(msg *models.Message) bool {
						return msg.ID == "sig123" &&
							msg.Sender == "+1234567890" &&
							msg.Content == "Hello, Signal!" &&
							msg.Type == models.TextMessage &&
							msg.Platform == "signal"
					}),
				).Return(nil).Once()
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "media message",
			payload: map[string]interface{}{
				"messageId":   "sig124",
				"sender":      "+1234567890",
				"message":     "Check this out!",
				"timestamp":   time.Now().UnixMilli(),
				"type":        "image",
				"threadId":    "thread124",
				"recipient":   "+0987654321",
				"attachments": []string{"http://example.com/image.jpg"},
			},
			setup: func() {
				msgService.On("HandleSignalMessage",
					mock.Anything,
					mock.MatchedBy(func(msg *models.Message) bool {
						return msg.ID == "sig124" &&
							msg.Sender == "+1234567890" &&
							msg.Content == "Check this out!" &&
							msg.Type == models.ImageMessage &&
							msg.Platform == "signal" &&
							msg.MediaURL == "http://example.com/image.jpg"
					}),
				).Return(nil).Once()
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid payload",
			payload: map[string]string{
				"invalid": "payload",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "service error",
			payload: map[string]interface{}{
				"messageId": "sig125",
				"sender":    "+1234567890",
				"message":   "Error message",
				"timestamp": time.Now().UnixMilli(),
				"type":      "text",
				"threadId":  "thread125",
				"recipient": "+0987654321",
			},
			setup: func() {
				msgService.On("HandleSignalMessage",
					mock.Anything,
					mock.MatchedBy(func(msg *models.Message) bool {
						return msg.ID == "sig125" &&
							msg.Platform == "signal"
					}),
				).Return(assert.AnError).Once()
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			payload, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/webhook/signal", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			server.handleSignalWebhook()(w, req)

			resp := w.Result()
			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			msgService.AssertExpectations(t)
		})
	}
}

func TestServer_StartAndShutdown(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	server := NewServer(msgService, logger)

	// Override port for test
	os.Setenv("PORT", "8082")
	defer os.Unsetenv("PORT")

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint
	resp, err := http.Get("http://localhost:8082/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	assert.NoError(t, err)

	// Verify server stopped
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("Received unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Server did not shut down within timeout")
	}
}
