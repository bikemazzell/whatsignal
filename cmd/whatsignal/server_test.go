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
	"whatsignal/pkg/signal"

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

func (m *mockMessageService) ProcessIncomingSignalMessage(ctx context.Context, rawSignalMsg *signal.SignalMessage) error {
	args := m.Called(ctx, rawSignalMsg)
	return args.Error(0)
}

func TestServer_Health(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{}
	server := NewServer(cfg, msgService, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth()(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_WhatsAppWebhook(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{}
	server := NewServer(cfg, msgService, logger)

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
	cfg := &models.Config{}
	server := NewServer(cfg, msgService, logger)

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
				msgService.On("ProcessIncomingSignalMessage",
					mock.Anything,
					mock.MatchedBy(func(msg *signal.SignalMessage) bool {
						return msg.MessageID == "sig123" &&
							msg.Sender == "+1234567890" &&
							msg.Message == "Hello, Signal!"
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
				msgService.On("ProcessIncomingSignalMessage",
					mock.Anything,
					mock.MatchedBy(func(msg *signal.SignalMessage) bool {
						return msg.MessageID == "sig124" &&
							msg.Sender == "+1234567890" &&
							msg.Message == "Check this out!" &&
							len(msg.Attachments) == 1 &&
							msg.Attachments[0] == "http://example.com/image.jpg"
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
				msgService.On("ProcessIncomingSignalMessage",
					mock.Anything,
					mock.MatchedBy(func(msg *signal.SignalMessage) bool {
						return msg.MessageID == "sig125"
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

func TestConvertWebhookPayloadToSignalMessage(t *testing.T) {
	tests := []struct {
		name     string
		payload  *SignalWebhookPayload
		expected *signal.SignalMessage
	}{
		{
			name: "basic message",
			payload: &SignalWebhookPayload{
				MessageID: "msg123",
				Sender:    "+1234567890",
				Message:   "Hello, World!",
				Timestamp: 1234567890,
				Type:      "text",
			},
			expected: &signal.SignalMessage{
				MessageID:     "msg123",
				Sender:        "+1234567890",
				Message:       "Hello, World!",
				Timestamp:     1234567890,
				Attachments:   []string{},
				QuotedMessage: nil,
			},
		},
		{
			name: "message with attachments",
			payload: &SignalWebhookPayload{
				MessageID:   "msg124",
				Sender:      "+1234567890",
				Message:     "Check this out!",
				Timestamp:   1234567890,
				Type:        "image",
				Attachments: []string{"http://example.com/image.jpg"},
			},
			expected: &signal.SignalMessage{
				MessageID:     "msg124",
				Sender:        "+1234567890",
				Message:       "Check this out!",
				Timestamp:     1234567890,
				Attachments:   []string{"http://example.com/image.jpg"},
				QuotedMessage: nil,
			},
		},
		{
			name: "message with media path",
			payload: &SignalWebhookPayload{
				MessageID: "msg125",
				Sender:    "+1234567890",
				Message:   "Media message",
				Timestamp: 1234567890,
				Type:      "image",
				MediaPath: "/path/to/media.jpg",
			},
			expected: &signal.SignalMessage{
				MessageID:     "msg125",
				Sender:        "+1234567890",
				Message:       "Media message",
				Timestamp:     1234567890,
				Attachments:   []string{"/path/to/media.jpg"},
				QuotedMessage: nil,
			},
		},
		{
			name: "message with both attachments and media path",
			payload: &SignalWebhookPayload{
				MessageID:   "msg126",
				Sender:      "+1234567890",
				Message:     "Multiple attachments",
				Timestamp:   1234567890,
				Type:        "image",
				Attachments: []string{"http://example.com/image1.jpg"},
				MediaPath:   "/path/to/media2.jpg",
			},
			expected: &signal.SignalMessage{
				MessageID:     "msg126",
				Sender:        "+1234567890",
				Message:       "Multiple attachments",
				Timestamp:     1234567890,
				Attachments:   []string{"http://example.com/image1.jpg", "/path/to/media2.jpg"},
				QuotedMessage: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertWebhookPayloadToSignalMessage(tt.payload)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestServer_StartAndShutdown(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{}
	server := NewServer(cfg, msgService, logger)

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
