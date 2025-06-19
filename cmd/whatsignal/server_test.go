package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"whatsignal/internal/models"
	signaltypes "whatsignal/pkg/signal/types"
	"whatsignal/pkg/whatsapp/types"

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

func (m *mockMessageService) ProcessIncomingSignalMessage(ctx context.Context, rawSignalMsg *signaltypes.SignalMessage) error {
	args := m.Called(ctx, rawSignalMsg)
	return args.Error(0)
}

func TestVerifySignature(t *testing.T) {
	secretKey := "test-secret-key"
	payload := []byte(`{"test": "data"}`)

	// Create valid signature
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write(payload)
	validSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name        string
		secretKey   string
		signature   string
		payload     []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid signature",
			secretKey:   secretKey,
			signature:   validSignature,
			payload:     payload,
			expectError: false,
		},
		{
			name:        "empty secret key (skip verification)",
			secretKey:   "",
			signature:   "",
			payload:     payload,
			expectError: false,
		},
		{
			name:        "missing signature header",
			secretKey:   secretKey,
			signature:   "",
			payload:     payload,
			expectError: true,
			errorMsg:    "missing signature header",
		},
		{
			name:        "invalid signature format - no equals",
			secretKey:   secretKey,
			signature:   "invalidformat",
			payload:     payload,
			expectError: true,
			errorMsg:    "invalid signature format",
		},
		{
			name:        "invalid signature format - wrong prefix",
			secretKey:   secretKey,
			signature:   "md5=abcdef",
			payload:     payload,
			expectError: true,
			errorMsg:    "invalid signature format",
		},
		{
			name:        "signature mismatch",
			secretKey:   secretKey,
			signature:   "sha256=wrongsignature",
			payload:     payload,
			expectError: true,
			errorMsg:    "signature mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(tt.payload))
			if tt.signature != "" {
				req.Header.Set("X-Test-Signature", tt.signature)
			}

			body, err := verifySignature(req, tt.secretKey, "X-Test-Signature")

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, body)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.payload, body)
			}
		})
	}
}

func TestSetupWebhookHandlers(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{}
	server := NewServer(cfg, msgService, logger)

	// Test that webhook handlers are properly set up
	assert.NotNil(t, server.waWebhook)

	// Test webhook handler registration by triggering a message event
	msgService.On("HandleWhatsAppMessage",
		mock.Anything,
		"test-chat",
		"test-msg",
		"test-sender",
		"test content",
		"",
	).Return(nil).Once()

	// Create a mock webhook event with message payload
	payload := map[string]interface{}{
		"id":      "test-msg",
		"chatId":  "test-chat",
		"sender":  "test-sender",
		"type":    "text",
		"content": "test content",
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	// Create a webhook event
	event := &types.WebhookEvent{
		Event:   "message.any",
		Payload: payloadBytes,
	}

	// Test the registered handler
	err = server.waWebhook.Handle(context.Background(), event)
	assert.NoError(t, err)

	msgService.AssertExpectations(t)
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
	cfg := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			WebhookSecret: "test-secret",
		},
	}
	server := NewServer(cfg, msgService, logger)

	tests := []struct {
		name         string
		payload      interface{}
		setup        func()
		wantStatus   int
		useSignature bool
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
			wantStatus:   http.StatusOK,
			useSignature: true,
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
			wantStatus:   http.StatusOK,
			useSignature: true,
		},
		{
			name: "non-message event",
			payload: map[string]interface{}{
				"event": "status",
				"data": map[string]interface{}{
					"id": "status123",
				},
			},
			wantStatus:   http.StatusOK,
			useSignature: true,
		},
		{
			name: "missing data field",
			payload: map[string]interface{}{
				"event": "message",
			},
			wantStatus:   http.StatusBadRequest,
			useSignature: true,
		},
		{
			name: "missing required fields",
			payload: map[string]interface{}{
				"event": "message",
				"data": map[string]interface{}{
					"content": "Hello",
				},
			},
			wantStatus:   http.StatusBadRequest,
			useSignature: true,
		},
		{
			name: "invalid payload - message event with missing ID",
			payload: map[string]interface{}{
				"event": "message",
				"data": map[string]interface{}{
					"chatId":  "chat123",
					"sender":  "sender123",
					"type":    "text",
					"content": "Hello",
					// Missing required "id" field
				},
			},
			wantStatus:   http.StatusBadRequest,
			useSignature: true,
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
			wantStatus:   http.StatusInternalServerError,
			useSignature: true,
		},
		{
			name: "invalid signature",
			payload: map[string]interface{}{
				"event": "message",
				"data": map[string]interface{}{
					"id":      "msg126",
					"chatId":  "chat126",
					"sender":  "sender126",
					"type":    "text",
					"content": "Test message",
				},
			},
			wantStatus:   http.StatusUnauthorized,
			useSignature: false, // This will create an invalid signature
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

			if tt.useSignature {
				// Create valid signature
				mac := hmac.New(sha256.New, []byte("test-secret"))
				mac.Write(payload)
				signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
				req.Header.Set(XWahaSignatureHeader, signature)
			} else {
				// Create invalid signature
				req.Header.Set(XWahaSignatureHeader, "sha256=invalidsignature")
			}

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
	cfg := &models.Config{
		Signal: models.SignalConfig{
			WebhookSecret: "test-secret",
		},
	}
	server := NewServer(cfg, msgService, logger)

	tests := []struct {
		name         string
		payload      interface{}
		setup        func()
		wantStatus   int
		useSignature bool
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
					mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
						return msg.MessageID == "sig123" &&
							msg.Sender == "+1234567890" &&
							msg.Message == "Hello, Signal!"
					}),
				).Return(nil).Once()
			},
			wantStatus:   http.StatusOK,
			useSignature: true,
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
					mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
						return msg.MessageID == "sig124" &&
							msg.Sender == "+1234567890" &&
							msg.Message == "Check this out!" &&
							len(msg.Attachments) == 1 &&
							msg.Attachments[0] == "http://example.com/image.jpg"
					}),
				).Return(nil).Once()
			},
			wantStatus:   http.StatusOK,
			useSignature: true,
		},
		{
			name: "missing required fields",
			payload: map[string]interface{}{
				"message": "Hello",
			},
			wantStatus:   http.StatusBadRequest,
			useSignature: true,
		},
		{
			name: "invalid payload",
			payload: map[string]string{
				"invalid": "payload",
			},
			wantStatus:   http.StatusBadRequest,
			useSignature: true,
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
					mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
						return msg.MessageID == "sig125"
					}),
				).Return(assert.AnError).Once()
			},
			wantStatus:   http.StatusInternalServerError,
			useSignature: true,
		},
		{
			name: "invalid signature",
			payload: map[string]interface{}{
				"messageId": "sig126",
				"sender":    "+1234567890",
				"message":   "Test message",
				"timestamp": time.Now().UnixMilli(),
				"type":      "text",
			},
			wantStatus:   http.StatusUnauthorized,
			useSignature: false,
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

			if tt.useSignature {
				// Create valid signature
				mac := hmac.New(sha256.New, []byte("test-secret"))
				mac.Write(payload)
				signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
				req.Header.Set(XSignalSignatureHeader, signature)
			} else {
				// Create invalid signature
				req.Header.Set(XSignalSignatureHeader, "sha256=invalidsignature")
			}

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
		payload  *models.SignalWebhookPayload
		expected *signaltypes.SignalMessage
	}{
		{
			name: "basic message",
			payload: &models.SignalWebhookPayload{
				MessageID: "msg123",
				Sender:    "+1234567890",
				Message:   "Hello, World!",
				Timestamp: 1234567890,
				Type:      "text",
			},
			expected: &signaltypes.SignalMessage{
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
			payload: &models.SignalWebhookPayload{
				MessageID:   "msg124",
				Sender:      "+1234567890",
				Message:     "Check this out!",
				Timestamp:   1234567890,
				Type:        "image",
				Attachments: []string{"http://example.com/image.jpg"},
			},
			expected: &signaltypes.SignalMessage{
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
			payload: &models.SignalWebhookPayload{
				MessageID: "msg125",
				Sender:    "+1234567890",
				Message:   "Media message",
				Timestamp: 1234567890,
				Type:      "image",
				MediaPath: "/path/to/media.jpg",
			},
			expected: &signaltypes.SignalMessage{
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
			payload: &models.SignalWebhookPayload{
				MessageID:   "msg126",
				Sender:      "+1234567890",
				Message:     "Multiple attachments",
				Timestamp:   1234567890,
				Type:        "image",
				Attachments: []string{"http://example.com/image1.jpg"},
				MediaPath:   "/path/to/media2.jpg",
			},
			expected: &signaltypes.SignalMessage{
				MessageID:     "msg126",
				Sender:        "+1234567890",
				Message:       "Multiple attachments",
				Timestamp:     1234567890,
				Attachments:   []string{"http://example.com/image1.jpg", "/path/to/media2.jpg"},
				QuotedMessage: nil,
			},
		},
		{
			name: "message with nil attachments",
			payload: &models.SignalWebhookPayload{
				MessageID:   "msg127",
				Sender:      "+1234567890",
				Message:     "No attachments",
				Timestamp:   1234567890,
				Type:        "text",
				Attachments: nil,
			},
			expected: &signaltypes.SignalMessage{
				MessageID:     "msg127",
				Sender:        "+1234567890",
				Message:       "No attachments",
				Timestamp:     1234567890,
				Attachments:   []string{},
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

	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Override port for test
	os.Setenv("PORT", fmt.Sprintf("%d", port))
	defer os.Unsetenv("PORT")

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
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

func TestServer_ShutdownNilServer(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{}
	server := NewServer(cfg, msgService, logger)

	// Test shutdown without starting server
	ctx := context.Background()
	err := server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestVerifySignature_BodyReadError(t *testing.T) {
	// Create a request with a body that will cause a read error
	req := httptest.NewRequest(http.MethodPost, "/test", &errorReader{})
	req.Header.Set("X-Test-Signature", "sha256=test")

	_, err := verifySignature(req, "secret", "X-Test-Signature")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read request body")
}

// errorReader is a helper type that always returns an error when Read is called
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("simulated read error")
}

func TestNewServer(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{}

	server := NewServer(cfg, msgService, logger)

	assert.NotNil(t, server)
	assert.NotNil(t, server.router)
	assert.NotNil(t, server.logger)
	assert.NotNil(t, server.msgService)
	assert.NotNil(t, server.waWebhook)
	assert.Equal(t, cfg, server.cfg)
}
