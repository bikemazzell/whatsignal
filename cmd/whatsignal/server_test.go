package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"whatsignal/internal/constants"
	"whatsignal/internal/models"
	"whatsignal/internal/service"
	signaltypes "whatsignal/pkg/signal/types"
	"whatsignal/pkg/whatsapp/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockDatabase implements DatabaseInterface for testing
type mockDatabase struct {
	mock.Mock
}

func (m *mockDatabase) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// For tests, we'll use nil for signal client since the code has nil checks

// Helper function to create a test channel manager
func createTestChannelManager() *service.ChannelManager {
	channels := []models.Channel{
		{
			WhatsAppSessionName:          "default",
			SignalDestinationPhoneNumber: "+1234567890",
		},
	}
	cm, _ := service.NewChannelManager(channels)
	return cm
}

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

// mockWAClient implements WAClient interface for testing
type mockWAClient struct {
	mock.Mock
}

func (m *mockWAClient) SendText(ctx context.Context, chatID, message string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, message)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendTextWithSession(ctx context.Context, chatID, message, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, message, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendImage(ctx context.Context, chatID, imagePath, caption string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, imagePath, caption)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendImageWithSession(ctx context.Context, chatID, imagePath, caption, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, imagePath, caption, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendVideo(ctx context.Context, chatID, videoPath, caption string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, videoPath, caption)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendVideoWithSession(ctx context.Context, chatID, videoPath, caption, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, videoPath, caption, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendDocument(ctx context.Context, chatID, docPath, caption string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, docPath, caption)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendDocumentWithSession(ctx context.Context, chatID, docPath, caption, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, docPath, caption, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendFile(ctx context.Context, chatID, filePath, caption string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, filePath, caption)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendVoice(ctx context.Context, chatID, voicePath string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, voicePath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendVoiceWithSession(ctx context.Context, chatID, voicePath, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, voicePath, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendTextWithSessionReply(ctx context.Context, chatID, message, replyTo, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, message, replyTo, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendImageWithSessionReply(ctx context.Context, chatID, imagePath, caption, replyTo, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, imagePath, caption, replyTo, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendVideoWithSessionReply(ctx context.Context, chatID, videoPath, caption, replyTo, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, videoPath, caption, replyTo, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendDocumentWithSessionReply(ctx context.Context, chatID, docPath, caption, replyTo, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, docPath, caption, replyTo, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendVoiceWithSessionReply(ctx context.Context, chatID, voicePath, replyTo, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, voicePath, replyTo, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) CreateSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWAClient) StartSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWAClient) StopSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWAClient) GetSessionStatus(ctx context.Context) (*types.Session, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *mockWAClient) GetSessionStatusByName(ctx context.Context, sessionName string) (*types.Session, error) {
	args := m.Called(ctx, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *mockWAClient) RestartSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWAClient) WaitForSessionReady(ctx context.Context, maxWaitTime time.Duration) error {
	args := m.Called(ctx, maxWaitTime)
	return args.Error(0)
}

func (m *mockWAClient) GetContact(ctx context.Context, contactID string) (*types.Contact, error) {
	args := m.Called(ctx, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Contact), args.Error(1)
}

func (m *mockWAClient) GetAllContacts(ctx context.Context, limit, offset int) ([]types.Contact, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Contact), args.Error(1)
}

func (m *mockWAClient) GetGroup(ctx context.Context, groupID string) (*types.Group, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Group), args.Error(1)
}

func (m *mockWAClient) GetAllGroups(ctx context.Context, limit, offset int) ([]types.Group, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Group), args.Error(1)
}

func (m *mockWAClient) SendReaction(ctx context.Context, chatID, messageID, reaction string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, messageID, reaction)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) SendReactionWithSession(ctx context.Context, chatID, messageID, reaction, sessionName string) (*types.SendMessageResponse, error) {
	args := m.Called(ctx, chatID, messageID, reaction, sessionName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.SendMessageResponse), args.Error(1)
}

func (m *mockWAClient) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	args := m.Called(ctx, chatID, messageID)
	return args.Error(0)
}

func (m *mockWAClient) GetSessionName() string {
	return "test-session"
}

func (m *mockWAClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWAClient) AckMessage(ctx context.Context, chatID, sessionName string) error {
	args := m.Called(ctx, chatID, sessionName)
	return args.Error(0)
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

func (m *mockMessageService) HandleWhatsAppMessageWithSession(ctx context.Context, sessionName, chatID, msgID, sender, senderDisplayName, content string, mediaPath string) error {
	args := m.Called(ctx, sessionName, chatID, msgID, sender, senderDisplayName, content, mediaPath)
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

func (m *mockMessageService) PollSignalMessages(ctx context.Context) error {
	args := m.Called(ctx)
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

			body, err := verifySignatureWithSkew(req, tt.secretKey, "X-Test-Signature", time.Duration(constants.DefaultWebhookMaxSkewSec)*time.Second)

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

func TestVerifySignatureWithSkew_WAHA(t *testing.T) {
	secretKey := "test-secret"
	payload := []byte("{\"event\":\"message.any\"}")

	// Helper to compute WAHA signature (sha512 of body)
	computeSig := func(b []byte) string {
		mac := hmac.New(sha512.New, []byte(secretKey))
		mac.Write(b)
		return hex.EncodeToString(mac.Sum(nil))
	}

	t.Run("millisecond timestamp conversion", func(t *testing.T) {
		// This test specifically verifies that millisecond timestamps from WAHA
		// are correctly converted to seconds for validation
		req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(payload))
		req.Header.Set(XWahaSignatureHeader, computeSig(payload))
		// Use current time in milliseconds (as WAHA sends)
		currentTimeMs := time.Now().UnixMilli()
		req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", currentTimeMs))

		body, err := verifySignatureWithSkew(req, secretKey, XWahaSignatureHeader, 5*time.Minute)
		assert.NoError(t, err, "Should accept valid millisecond timestamp")
		assert.Equal(t, payload, body)
	})

	t.Run("edge case: timestamp exactly at max skew boundary", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(payload))
		req.Header.Set(XWahaSignatureHeader, computeSig(payload))
		// Timestamp exactly at the edge of acceptable skew (5 minutes ago in milliseconds)
		edgeTime := time.Now().Add(-time.Duration(constants.DefaultWebhookMaxSkewSec)*time.Second + 1*time.Second).UnixMilli()
		req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", edgeTime))

		body, err := verifySignatureWithSkew(req, secretKey, XWahaSignatureHeader, time.Duration(constants.DefaultWebhookMaxSkewSec)*time.Second)
		assert.NoError(t, err, "Should accept timestamp at edge of acceptable range")
		assert.Equal(t, payload, body)
	})

	t.Run("wrong timestamp format: seconds instead of milliseconds", func(t *testing.T) {
		// This test verifies what happens if someone accidentally sends seconds
		// (which would appear as a timestamp from 1970)
		req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(payload))
		req.Header.Set(XWahaSignatureHeader, computeSig(payload))
		// Send seconds instead of milliseconds - this would be interpreted as early 1970
		wrongFormatTime := time.Now().Unix() // seconds, not milliseconds
		req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", wrongFormatTime))

		body, err := verifySignatureWithSkew(req, secretKey, XWahaSignatureHeader, 5*time.Minute)
		assert.Error(t, err, "Should reject timestamp in wrong format (seconds instead of ms)")
		assert.Contains(t, err.Error(), "timestamp out of acceptable range")
		assert.Nil(t, body)
	})

	t.Run("missing timestamp header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(payload))
		req.Header.Set(XWahaSignatureHeader, computeSig(payload))

		body, err := verifySignatureWithSkew(req, secretKey, XWahaSignatureHeader, time.Duration(constants.DefaultWebhookMaxSkewSec)*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing X-Webhook-Timestamp")
		assert.Nil(t, body)
	})

	t.Run("timestamp too old", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(payload))
		req.Header.Set(XWahaSignatureHeader, computeSig(payload))
		old := time.Now().Add(-(time.Duration(constants.DefaultWebhookMaxSkewSec) + 10) * time.Second).UnixMilli()
		req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", old))

		body, err := verifySignatureWithSkew(req, secretKey, XWahaSignatureHeader, time.Duration(constants.DefaultWebhookMaxSkewSec)*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timestamp out of acceptable range")
		assert.Nil(t, body)
	})

	t.Run("timestamp in future too far", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(payload))
		req.Header.Set(XWahaSignatureHeader, computeSig(payload))
		future := time.Now().Add((time.Duration(constants.DefaultWebhookMaxSkewSec) + 10) * time.Second).UnixMilli()
		req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", future))

		body, err := verifySignatureWithSkew(req, secretKey, XWahaSignatureHeader, time.Duration(constants.DefaultWebhookMaxSkewSec)*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timestamp out of acceptable range")
		assert.Nil(t, body)
	})

	t.Run("valid WAHA signature and timestamp", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(payload))
		req.Header.Set(XWahaSignatureHeader, computeSig(payload))
		req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

		body, err := verifySignatureWithSkew(req, secretKey, XWahaSignatureHeader, time.Duration(constants.DefaultWebhookMaxSkewSec)*time.Second)
		assert.NoError(t, err)
		assert.Equal(t, payload, body)
	})
}

func TestServer_Health(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{}
	mockWAClient := &mockWAClient{}
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}

	// Setup mock expectations for health checks
	mockDB.On("HealthCheck", mock.Anything).Return(nil)
	mockWAClient.On("HealthCheck", mock.Anything).Return(nil)

	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth()(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	mockDB.AssertExpectations(t)
	mockWAClient.AssertExpectations(t)
}

func TestServer_SessionStatus(t *testing.T) {
	tests := []struct {
		name           string
		sessionStatus  *types.Session
		sessionError   error
		expectedStatus int
		expectedFields []string
	}{
		{
			name: "healthy session",
			sessionStatus: &types.Session{
				Name:      "test-session",
				Status:    "WORKING",
				UpdatedAt: time.Now(),
			},
			expectedStatus: http.StatusOK,
			expectedFields: []string{"name", "status", "healthy", "updated_at", "config"},
		},
		{
			name: "unhealthy session",
			sessionStatus: &types.Session{
				Name:      "test-session",
				Status:    "STOPPED",
				UpdatedAt: time.Now(),
			},
			expectedStatus: http.StatusOK,
			expectedFields: []string{"name", "status", "healthy", "updated_at", "config"},
		},
		{
			name:           "session error",
			sessionError:   assert.AnError,
			expectedStatus: http.StatusServiceUnavailable,
			expectedFields: []string{"error", "details"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgService := &mockMessageService{}
			logger := logrus.New()
			cfg := &models.Config{
				WhatsApp: models.WhatsAppConfig{
					SessionAutoRestart:    true,
					SessionHealthCheckSec: 30,
				},
			}
			mockWAClient := &mockWAClient{}

			// Setup mock expectations
			mockWAClient.On("GetSessionStatus", mock.Anything).Return(tt.sessionStatus, tt.sessionError).Once()

			channelManager := createTestChannelManager()
			mockDB := &mockDatabase{}
			server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

			req := httptest.NewRequest(http.MethodGet, "/session/status", nil)
			w := httptest.NewRecorder()

			server.handleSessionStatus()(w, req)

			resp := w.Result()
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

			// Parse response body
			var responseBody map[string]interface{}
			err := json.NewDecoder(resp.Body).Decode(&responseBody)
			assert.NoError(t, err)

			// Check expected fields are present
			for _, field := range tt.expectedFields {
				assert.Contains(t, responseBody, field, "Response should contain field: %s", field)
			}

			// Additional checks for successful responses
			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, tt.sessionStatus.Name, responseBody["name"])
				assert.Equal(t, string(tt.sessionStatus.Status), responseBody["status"])

				expectedHealthy := string(tt.sessionStatus.Status) == "WORKING"
				assert.Equal(t, expectedHealthy, responseBody["healthy"])

				// Check config section
				config, ok := responseBody["config"].(map[string]interface{})
				assert.True(t, ok, "Config should be a map")
				assert.Equal(t, true, config["auto_restart_enabled"])
				assert.Equal(t, float64(30), config["health_check_interval_sec"])
			}

			mockWAClient.AssertExpectations(t)
		})
	}
}

func TestServer_WhatsAppWebhook(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			WebhookSecret: "test-secret",
		},
	}
	mockWAClient := &mockWAClient{}
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}
	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

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
				"event":   "message",
				"session": "default",
				"payload": map[string]interface{}{
					"id":       "msg123",
					"from":     "+1234567890",
					"fromMe":   false,
					"body":     "Hello, World!",
					"hasMedia": false,
				},
			},
			setup: func() {
				msgService.On("HandleWhatsAppMessageWithSession",
					mock.Anything,
					"default", // session name
					"+1234567890",
					"msg123",
					"+1234567890",
					"", // senderDisplayName
					"Hello, World!",
					"",
				).Return(nil).Once()
			},
			wantStatus:   http.StatusOK,
			useSignature: true,
		},
		{
			name: "status/broadcast message should be ignored",
			payload: map[string]interface{}{
				"event":   "message",
				"session": "default",
				"payload": map[string]interface{}{
					"id":       "false_status@broadcast_3A732FBEB4228EB0DCB0_393382105411@c.us",
					"from":     "false_status@broadcast_3A732FBEB4228EB0DCB0_393382105411@c.us",
					"fromMe":   false,
					"body":     "Status update",
					"hasMedia": false,
				},
			},
			setup: func() {
				// No message service call should be made for status messages
			},
			wantStatus:   http.StatusOK,
			useSignature: true,
		},
		{
			name: "invalid sender phone number (newsletter/system message) should be ignored",
			payload: map[string]interface{}{
				"event":   "message",
				"session": "default",
				"payload": map[string]interface{}{
					"id":       "msg_newsletter_123",
					"from":     "1",
					"fromMe":   false,
					"body":     "Newsletter content",
					"hasMedia": false,
				},
			},
			setup: func() {
				// No message service call should be made for invalid sender messages
			},
			wantStatus:   http.StatusOK,
			useSignature: true,
		},
		{
			name: "valid media message",
			payload: map[string]interface{}{
				"event":   "message",
				"session": "default",
				"payload": map[string]interface{}{
					"id":       "msg124",
					"from":     "+1234567891",
					"fromMe":   false,
					"body":     "Check this out!",
					"hasMedia": true,
					"media": map[string]interface{}{
						"url": "/path/to/image.jpg",
					},
				},
			},
			setup: func() {
				msgService.On("HandleWhatsAppMessageWithSession",
					mock.Anything,
					"default", // session name
					"+1234567891",
					"msg124",
					"+1234567891",
					"", // senderDisplayName
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
				"event":   "status",
				"session": "default",
				"payload": map[string]interface{}{
					"id": "status123",
				},
			},
			wantStatus:   http.StatusOK,
			useSignature: true,
		},
		{
			name: "missing payload field",
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
				"payload": map[string]interface{}{
					"body": "Hello",
				},
			},
			wantStatus:   http.StatusBadRequest,
			useSignature: true,
		},
		{
			name: "invalid payload - message event with missing ID",
			payload: map[string]interface{}{
				"event": "message",
				"payload": map[string]interface{}{
					"from":     "+1234567892",
					"fromMe":   false,
					"body":     "Hello",
					"hasMedia": false,
					// Missing required "id" field
				},
			},
			wantStatus:   http.StatusBadRequest,
			useSignature: true,
		},
		{
			name: "service error",
			payload: map[string]interface{}{
				"event":   "message",
				"session": "default",
				"payload": map[string]interface{}{
					"id":       "msg125",
					"from":     "+1234567893",
					"fromMe":   false,
					"body":     "Error message",
					"hasMedia": false,
				},
			},
			setup: func() {
				msgService.On("HandleWhatsAppMessageWithSession",
					mock.Anything,
					"default", // session name
					"+1234567893",
					"msg125",
					"+1234567893",
					"", // senderDisplayName
					"Error message",
					"",
				).Return(assert.AnError).Once()
			},
			wantStatus:   http.StatusInternalServerError,
			useSignature: true,
		},
		{
			name: "message with fromMe=true is dropped",
			payload: map[string]interface{}{
				"event":   "message",
				"session": "default",
				"payload": map[string]interface{}{
					"id":       "msg_echo_ours",
					"from":     "+1234567890",
					"fromMe":   true,
					"body":     "Our own message echoed back",
					"hasMedia": false,
				},
			},
			setup: func() {
				// No service calls should be made for our own messages
			},
			wantStatus:   http.StatusOK,
			useSignature: true,
		},
		{
			name: "invalid signature",
			payload: map[string]interface{}{
				"event": "message",
				"payload": map[string]interface{}{
					"id":       "msg126",
					"from":     "+1234567894",
					"fromMe":   false,
					"body":     "Test message",
					"hasMedia": false,
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
				// Create valid signature for WAHA webhook (uses SHA-512 and requires timestamp)
				mac := hmac.New(sha512.New, []byte("test-secret"))
				mac.Write(payload)
				signature := hex.EncodeToString(mac.Sum(nil))
				req.Header.Set(XWahaSignatureHeader, signature)
				req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
			} else {
				// Create invalid signature
				req.Header.Set(XWahaSignatureHeader, "invalidsignature")
				req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
			}

			w := httptest.NewRecorder()

			server.handleWhatsAppWebhook()(w, req)

			resp := w.Result()
			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			msgService.AssertExpectations(t)
		})
	}
}

func TestWebhookProcessingDetachedContext(t *testing.T) {
	// Test that webhook event processing continues even when the HTTP client disconnects
	// Fix: webhook processing now uses context.WithTimeout(context.Background(), 60*time.Second)
	// instead of r.Context(), allowing processing to survive HTTP connection timeouts

	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			WebhookSecret: "test-secret",
		},
	}
	mockWAClient := &mockWAClient{}
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}
	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

	t.Run("webhook processing completes with detached context", func(t *testing.T) {
		// Create a payload that will trigger message handling
		payload := map[string]interface{}{
			"event":   "message",
			"session": "default",
			"payload": map[string]interface{}{
				"id":       "msg123",
				"from":     "+1234567890",
				"fromMe":   false,
				"body":     "Test message",
				"hasMedia": false,
			},
		}

		// Mock the message service to verify it gets called with a detached context
		// The key is that processing should succeed even if we simulate an early HTTP disconnect
		msgService.On("HandleWhatsAppMessageWithSession",
			mock.Anything, // This will be the detached context with timeout
			"default",     // session name
			"+1234567890",
			"msg123",
			"+1234567890",
			"", // senderDisplayName
			"Test message",
			"", // mediaPath
		).Run(func(args mock.Arguments) {
			// Simulate a slow operation that might exceed the original request timeout
			// This demonstrates that the processing context is independent of HTTP request context
			ctx := args.Get(0).(context.Context)
			_, hasDeadline := ctx.Deadline()
			// Verify the context has a deadline (from WithTimeout)
			assert.True(t, hasDeadline, "processing context should have a deadline set")

			// Verify the deadline is reasonable (60 seconds)
			deadline, _ := ctx.Deadline()
			now := time.Now()
			timeUntilDeadline := deadline.Sub(now)
			// Should be close to 60 seconds, allowing some variance for test execution
			assert.Greater(t, timeUntilDeadline.Seconds(), float64(55), "deadline should be approximately 60 seconds from now")
			assert.Less(t, timeUntilDeadline.Seconds(), float64(65), "deadline should be approximately 60 seconds from now")
		}).Return(nil).Once()

		// Calculate webhook signature using SHA-512 (WAHA uses this)
		bodyBytes, _ := json.Marshal(payload)
		mac := hmac.New(sha512.New, []byte(cfg.WhatsApp.WebhookSecret))
		mac.Write(bodyBytes)
		signature := hex.EncodeToString(mac.Sum(nil))

		// Create request
		req := httptest.NewRequest("POST", "/webhook/whatsapp", bytes.NewBuffer(bodyBytes))
		req.Header.Set("X-Webhook-Hmac", signature)
		req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		// Call webhook handler
		server.handleWhatsAppWebhook()(w, req)

		// Verify success
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		msgService.AssertExpectations(t)
	})
}

func TestServer_SignalWebhook(t *testing.T) {
	t.Skip("Signal webhook functionality removed - Signal uses polling instead")
	/*
		msgService := &mockMessageService{}
		logger := logrus.New()
		cfg := &models.Config{
			Signal: models.SignalConfig{},
		}
		mockWAClient := &mockWAClient{}
		channelManager := createTestChannelManager()
		server := NewServer(cfg, msgService, logger, mockWAClient, channelManager)

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
					msgService.On("ProcessIncomingSignalMessageWithDestination",
						mock.Anything,
						mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
							return msg.MessageID == "sig123" &&
								msg.Sender == "+1234567890" &&
								msg.Message == "Hello, Signal!"
						}),
						"+0987654321", // destination
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
					msgService.On("ProcessIncomingSignalMessageWithDestination",
						mock.Anything,
						mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
							return msg.MessageID == "sig124" &&
								msg.Sender == "+1234567890" &&
								msg.Message == "Check this out!" &&
								len(msg.Attachments) == 1 &&
								msg.Attachments[0] == "http://example.com/image.jpg"
						}),
						"+0987654321", // destination
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
					msgService.On("ProcessIncomingSignalMessageWithDestination",
						mock.Anything,
						mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
							return msg.MessageID == "sig125"
						}),
						"+0987654321", // destination
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
					req.Header.Set("X-Signal-Signature-256", signature)
				} else {
					// Create invalid signature
					req.Header.Set("X-Signal-Signature-256", "sha256=invalidsignature")
				}

				w := httptest.NewRecorder()

				server.handleSignalWebhook()(w, req)

				resp := w.Result()
				assert.Equal(t, tt.wantStatus, resp.StatusCode)
				msgService.AssertExpectations(t)
			})
		}
	*/
}

func TestConvertWebhookPayloadToSignalMessage(t *testing.T) {
	t.Skip("Signal webhook functionality removed - Signal uses polling instead")
	/*
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
	*/
}

func TestServer_WhatsAppEventHandlers(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
	cfg := &models.Config{
		Signal: models.SignalConfig{
			IntermediaryPhoneNumber: "+1234567890",
		},
		WhatsApp: models.WhatsAppConfig{
			WebhookSecret: "test-secret",
		},
		Channels: []models.Channel{
			{
				WhatsAppSessionName:          "default",
				SignalDestinationPhoneNumber: "+1234567890",
			},
		},
	}
	mockWAClient := &mockWAClient{}
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}
	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

	tests := []struct {
		name    string
		event   string
		payload map[string]interface{}
		setup   func()
	}{
		{
			name:  "message.reaction event",
			event: models.EventMessageReaction,
			payload: map[string]interface{}{
				"event":     models.EventMessageReaction,
				"timestamp": time.Now().UnixMilli(),
				"session":   "default",
				"payload": map[string]interface{}{
					"id":     "reaction123",
					"from":   "+0987654321",
					"fromMe": false,
					"reaction": map[string]interface{}{
						"text":      "👍",
						"messageId": "original_msg_123",
					},
				},
			},
			setup: func() {
				// Mock finding the original message mapping
				msgService.On("GetMessageMappingByWhatsAppID", mock.Anything, "original_msg_123").
					Return(&models.MessageMapping{
						WhatsAppMsgID:  "original_msg_123",
						SignalMsgID:    "sig_123",
						WhatsAppChatID: "+0987654321@c.us",
						SessionName:    "default",
						DeliveryStatus: models.DeliveryStatusSent,
					}, nil).Once()

				// Mock sending reaction notification to Signal
				msgService.On("SendSignalNotification", mock.Anything, "default", "👍 Reacted with 👍").Return(nil).Once()
			},
		},
		{
			name:  "message.edited event",
			event: models.EventMessageEdited,
			payload: map[string]interface{}{
				"event":     models.EventMessageEdited,
				"timestamp": time.Now().UnixMilli(),
				"session":   "default",
				"payload": map[string]interface{}{
					"id":              "edit123",
					"from":            "+0987654321",
					"fromMe":          false,
					"body":            "This is the edited message",
					"editedMessageId": "original_msg_124",
				},
			},
			setup: func() {
				// Mock finding the original message mapping
				msgService.On("GetMessageMappingByWhatsAppID", mock.Anything, "original_msg_124").
					Return(&models.MessageMapping{
						WhatsAppMsgID:  "original_msg_124",
						SignalMsgID:    "sig_124",
						WhatsAppChatID: "+0987654321@c.us",
						SessionName:    "default",
						DeliveryStatus: models.DeliveryStatusSent,
					}, nil).Once()

				// Mock sending edit notification to Signal
				msgService.On("SendSignalNotification", mock.Anything, "default", "✏️ Message edited: This is the edited message").Return(nil).Once()
			},
		},
		{
			name:  "message.ack event",
			event: models.EventMessageACK,
			payload: map[string]interface{}{
				"event":     models.EventMessageACK,
				"timestamp": time.Now().UnixMilli(),
				"session":   "default",
				"payload": map[string]interface{}{
					"id":   "msg_ack_123",
					"from": "+0987654321",
					"to":   "+1234567890",
					"ack":  3, // ACKRead - this is now a simple integer
				},
			},
			setup: func() {
				// Mock finding the message mapping for ACK
				msgService.On("GetMessageMappingByWhatsAppID", mock.Anything, "msg_ack_123").
					Return(&models.MessageMapping{
						WhatsAppMsgID:  "msg_ack_123",
						SignalMsgID:    "sig_ack_123",
						WhatsAppChatID: "+0987654321@c.us",
						SessionName:    "default",
						DeliveryStatus: models.DeliveryStatusSent,
					}, nil).Once()

				// Mock updating delivery status
				msgService.On("UpdateDeliveryStatus", mock.Anything, "msg_ack_123", "read").
					Return(nil).Once()
			},
		},
		{
			name:  "message.ack event with fromMe=true is not skipped",
			event: models.EventMessageACK,
			payload: map[string]interface{}{
				"event":     models.EventMessageACK,
				"timestamp": time.Now().UnixMilli(),
				"session":   "default",
				"payload": map[string]interface{}{
					"id":     "msg_ack_fromme",
					"from":   "+0987654321",
					"to":     "+1234567890",
					"fromMe": true,
					"ack":    2, // ACKDevice (Delivered)
				},
			},
			setup: func() {
				msgService.On("GetMessageMappingByWhatsAppID", mock.Anything, "msg_ack_fromme").
					Return(&models.MessageMapping{
						WhatsAppMsgID:  "msg_ack_fromme",
						SignalMsgID:    "sig_ack_fromme",
						WhatsAppChatID: "+0987654321@c.us",
						SessionName:    "default",
						DeliveryStatus: models.DeliveryStatusSent,
					}, nil).Once()
				msgService.On("UpdateDeliveryStatus", mock.Anything, "msg_ack_fromme", "delivered").
					Return(nil).Once()
			},
		},
		{
			name:  "message.waiting event",
			event: models.EventMessageWaiting,
			payload: map[string]interface{}{
				"event":     models.EventMessageWaiting,
				"timestamp": time.Now().UnixMilli(),
				"session":   "default",
				"payload": map[string]interface{}{
					"id":     "waiting123",
					"from":   "+0987654321",
					"fromMe": false,
				},
			},
			setup: func() {
				// Mock sending waiting notification to Signal
				msgService.On("SendSignalNotification", mock.Anything, "default", "⏳ WhatsApp is waiting for a message").Return(nil).Once()
			},
		},
		{
			name:  "message.waiting event with fromMe=true is not skipped",
			event: models.EventMessageWaiting,
			payload: map[string]interface{}{
				"event":     models.EventMessageWaiting,
				"timestamp": time.Now().UnixMilli(),
				"session":   "default",
				"payload": map[string]interface{}{
					"id":     "waiting_fromme",
					"from":   "+0987654321",
					"fromMe": true,
				},
			},
			setup: func() {
				msgService.On("SendSignalNotification", mock.Anything, "default", "⏳ WhatsApp is waiting for a message").Return(nil).Once()
			},
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

			// Create valid signature
			mac := hmac.New(sha512.New, []byte("test-secret"))
			mac.Write(payload)
			signature := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set(XWahaSignatureHeader, signature)
			req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

			w := httptest.NewRecorder()

			server.handleWhatsAppWebhook()(w, req)

			resp := w.Result()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			msgService.AssertExpectations(t)
		})
	}
}

func TestServer_StartAndShutdown(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{}
	mockWAClient := &mockWAClient{}
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}

	// Set up mock expectations for health check
	mockDB.On("HealthCheck", mock.Anything).Return(nil)
	mockWAClient.On("HealthCheck", mock.Anything).Return(nil)

	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Logf("Warning: failed to close listener: %v", err)
	}

	// Override port for test
	_ = os.Setenv("PORT", fmt.Sprintf("%d", port))
	defer func() { _ = os.Unsetenv("PORT") }()

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
	mockWAClient := &mockWAClient{}
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}
	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

	// Test shutdown without starting server
	ctx := context.Background()
	err := server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestVerifySignature_BodyReadError(t *testing.T) {
	// Create a request with a body that will cause a read error
	req := httptest.NewRequest(http.MethodPost, "/test", &errorReader{})
	req.Header.Set("X-Test-Signature", "sha256=test")

	_, err := verifySignatureWithSkew(req, "secret", "X-Test-Signature", 5*time.Minute)
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

	mockWAClient := &mockWAClient{}
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}
	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.router)
	assert.NotNil(t, server.logger)
	assert.NotNil(t, server.msgService)
	// waWebhook removed
	assert.Equal(t, cfg, server.cfg)
}

func TestRequestSizeLimit(t *testing.T) {
	tests := []struct {
		name         string
		maxBytes     int
		bodySize     int
		expectStatus int
		expectError  bool
	}{
		{
			name:         "request within limit",
			maxBytes:     1024,
			bodySize:     512,
			expectStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "request exactly at limit",
			maxBytes:     2048,
			bodySize:     1024,
			expectStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "request exceeds limit",
			maxBytes:     1024,
			bodySize:     2048,
			expectStatus: http.StatusRequestEntityTooLarge,
			expectError:  true,
		},
		{
			name:         "very large request",
			maxBytes:     1024 * 1024,     // 1MB
			bodySize:     5 * 1024 * 1024, // 5MB
			expectStatus: http.StatusRequestEntityTooLarge,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgService := &mockMessageService{}
			logger := logrus.New()
			cfg := &models.Config{
				Server: models.ServerConfig{
					WebhookMaxBytes: tt.maxBytes,
				},
				WhatsApp: models.WhatsAppConfig{
					WebhookSecret: "test-secret",
				},
			}

			mockWAClient := &mockWAClient{}
			channelManager := createTestChannelManager()
			mockDB := &mockDatabase{}
			server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

			// Create request with body of specific size
			body := make([]byte, tt.bodySize)
			for i := range body {
				body[i] = 'a'
			}

			// Wrap the body in a valid JSON structure
			jsonBody := fmt.Sprintf(`{"event":"test","data":"%s"}`, string(body))

			req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewBufferString(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			// Add valid signature for the request
			mac := hmac.New(sha512.New, []byte(cfg.WhatsApp.WebhookSecret))
			mac.Write([]byte(jsonBody))
			signature := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Webhook-Hmac", signature)
			req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

			rr := httptest.NewRecorder()

			// Process the request through the middleware and handler
			handler := server.securityMiddleware(server.router)
			handler.ServeHTTP(rr, req)

			// Check the response
			if tt.expectError {
				assert.Equal(t, tt.expectStatus, rr.Code)
			} else {
				// For successful requests, we might get 200 or 400 depending on payload validity
				assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusBadRequest)
			}
		})
	}
}

func TestRequestSizeLimit_NoLimit(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{
		Server: models.ServerConfig{
			WebhookMaxBytes: 0, // No limit set, should use default
		},
		WhatsApp: models.WhatsAppConfig{
			WebhookSecret: "test-secret",
		},
	}

	mockWAClient := &mockWAClient{}
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}
	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

	// Create a request larger than typical default (5MB)
	bodySize := 3 * 1024 * 1024 // 3MB (should be within default 5MB limit)
	body := make([]byte, bodySize)
	for i := range body {
		body[i] = 'x'
	}

	jsonBody := fmt.Sprintf(`{"event":"test","data":"%s"}`, string(body))

	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	// Add valid signature
	mac := hmac.New(sha512.New, []byte(cfg.WhatsApp.WebhookSecret))
	mac.Write([]byte(jsonBody))
	signature := hex.EncodeToString(mac.Sum(nil))
	req.Header.Set("X-Webhook-Hmac", signature)
	req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

	rr := httptest.NewRecorder()

	handler := server.securityMiddleware(server.router)
	handler.ServeHTTP(rr, req)

	// Should accept since it's within default limit (5MB)
	assert.NotEqual(t, http.StatusRequestEntityTooLarge, rr.Code)
}

func TestServer_RequireSessionName(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			WebhookSecret: "test-secret",
		},
	}
	mockWAClient := &mockWAClient{}
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}
	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
		wantError  bool
	}{
		{
			name: "message with session name",
			payload: map[string]interface{}{
				"event":   "message",
				"session": "default",
				"payload": map[string]interface{}{
					"id":       "msg123",
					"from":     "+1234567890",
					"body":     "Hello",
					"hasMedia": false,
				},
			},
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name: "message without session name - should fail",
			payload: map[string]interface{}{
				"event": "message",
				// No session field
				"payload": map[string]interface{}{
					"id":       "msg124",
					"from":     "+1234567891",
					"body":     "Hello",
					"hasMedia": false,
				},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "reaction without session name - should fail",
			payload: map[string]interface{}{
				"event": "message.reaction",
				// No session field
				"payload": map[string]interface{}{
					"from": "+1234567892",
					"reaction": map[string]interface{}{
						"messageId": "msg125",
						"text":      "👍",
					},
				},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name: "edited message without session name - should fail",
			payload: map[string]interface{}{
				"event": "message.edited",
				// No session field
				"payload": map[string]interface{}{
					"from":            "+1234567893",
					"body":            "Edited",
					"editedMessageId": "msg126",
				},
			},
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.wantError {
				// Set up expectation for successful messages
				msgService.On("HandleWhatsAppMessageWithSession",
					mock.Anything,
					"default",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything, // senderDisplayName
					mock.Anything,
					mock.Anything,
				).Return(nil).Once()
			}

			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Add valid signature
			mac := hmac.New(sha512.New, []byte(cfg.WhatsApp.WebhookSecret))
			mac.Write(body)
			signature := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Webhook-Hmac", signature)
			req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

			rr := httptest.NewRecorder()
			server.router.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantError {
				// Check that the error message mentions session requirement
				responseBody := rr.Body.String()
				assert.Contains(t, responseBody, "session name is required")
			}
		})
	}
}

func TestServer_UndefinedSessionHandling(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	cfg := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			WebhookSecret: "test-secret",
		},
	}
	mockWAClient := &mockWAClient{}
	// Channel manager only has "default" session configured
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}
	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

	tests := []struct {
		name          string
		payload       interface{}
		session       string
		expectHandled bool
		wantStatus    int
	}{
		{
			name:    "message from configured session",
			session: "default",
			payload: map[string]interface{}{
				"event":   "message",
				"session": "default",
				"payload": map[string]interface{}{
					"id":       "msg123",
					"from":     "+1234567895",
					"fromMe":   false,
					"body":     "Hello from default session",
					"hasMedia": false,
				},
			},
			expectHandled: true,
			wantStatus:    http.StatusOK,
		},
		{
			name:    "message from undefined session",
			session: "jo",
			payload: map[string]interface{}{
				"event":   "message",
				"session": "jo",
				"payload": map[string]interface{}{
					"id":       "msg456",
					"from":     "+1234567896",
					"fromMe":   false,
					"body":     "Hello from jo session",
					"hasMedia": false,
				},
			},
			expectHandled: false,
			wantStatus:    http.StatusOK, // Still returns OK but doesn't process
		},
		{
			name:    "reaction from undefined session",
			session: "unknown",
			payload: map[string]interface{}{
				"event":   "message.reaction",
				"session": "unknown",
				"payload": map[string]interface{}{
					"id":   "reaction123",
					"from": "+1234567897",
					"reaction": map[string]interface{}{
						"messageId": "msg789",
						"text":      "👍",
					},
				},
			},
			expectHandled: false,
			wantStatus:    http.StatusOK,
		},
		{
			name:    "edited message from undefined session",
			session: "another",
			payload: map[string]interface{}{
				"event":   "message.edited",
				"session": "another",
				"payload": map[string]interface{}{
					"id":              "edited123",
					"from":            "+1234567898",
					"body":            "Edited message",
					"editedMessageId": "msg999",
				},
			},
			expectHandled: false,
			wantStatus:    http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectHandled {
				// Set up expectation for handled messages
				msgService.On("HandleWhatsAppMessageWithSession",
					mock.Anything,
					tt.session,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything, // senderDisplayName
					mock.Anything,
					mock.Anything,
				).Return(nil).Once()
			}
			// For unhandled messages, no expectation is set

			// Create request with webhook payload
			body, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Add timestamp header (required for WAHA webhooks)
			timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
			req.Header.Set("X-Webhook-Timestamp", timestamp)

			// Add signature for authentication (using SHA512 for WAHA)
			mac := hmac.New(sha512.New, []byte(cfg.WhatsApp.WebhookSecret))
			mac.Write(body)
			signature := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Webhook-Hmac", signature)

			// Send request
			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, req)

			// Check response
			assert.Equal(t, tt.wantStatus, recorder.Code)

			// Verify expectations
			if tt.expectHandled {
				msgService.AssertExpectations(t)
			} else {
				// Verify that HandleWhatsAppMessageWithSession was NOT called
				msgService.AssertNotCalled(t, "HandleWhatsAppMessageWithSession",
					mock.Anything,
					tt.session,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything, // senderDisplayName
					mock.Anything,
					mock.Anything,
				)
			}
		})
	}
}

func TestWhatsAppWebhook_InvalidJSON_NoRawBodyLogged(t *testing.T) {
	msgService := &mockMessageService{}
	logger := logrus.New()
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.DebugLevel)

	cfg := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			WebhookSecret: "test-secret",
		},
		Channels: []models.Channel{{
			WhatsAppSessionName:          "default",
			SignalDestinationPhoneNumber: "+1234567890",
		}},
	}
	mockWAClient := &mockWAClient{}
	channelManager := createTestChannelManager()
	mockDB := &mockDatabase{}
	server := NewServer(cfg, msgService, logger, mockWAClient, channelManager, mockDB, nil)

	// Craft invalid JSON payload that contains a sensitive token
	payload := []byte("{\"event\":\"message\",\"payload\":{\"id\":\"msg\",\"from\":\"+10000000000\",\"fromMe\":false,\"body\":\"supersecret-token") // missing closing quotes/braces

	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	// Add timestamp header (required for WAHA webhooks)
	req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))
	// Add valid WAHA signature (SHA-512)
	mac := hmac.New(sha512.New, []byte(cfg.WhatsApp.WebhookSecret))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))
	req.Header.Set(XWahaSignatureHeader, signature)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, req)

	resp := recorder.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Ensure raw sensitive token is not present in logs
	logOutput := buf.String()
	assert.NotContains(t, logOutput, "supersecret-token")
}

func makeACKPayload(msgID string, ack int) *models.WhatsAppWebhookPayload {
	p := &models.WhatsAppWebhookPayload{
		Event: models.EventMessageACK,
	}
	p.Payload.ID = msgID
	p.Payload.From = "+1234567890"
	p.Payload.To = "+0987654321"
	p.Payload.ACK = &ack
	return p
}

func TestHandleWhatsAppACK(t *testing.T) {
	tests := []struct {
		name       string
		payload    *models.WhatsAppWebhookPayload
		setupMocks func(ms *mockMessageService)
		expectErr  bool
	}{
		{
			name: "missing ACK data returns validation error",
			payload: &models.WhatsAppWebhookPayload{
				Event: models.EventMessageACK,
			},
			setupMocks: func(ms *mockMessageService) {},
			expectErr:  true,
		},
		{
			name:    "lookup error returns nil but does not call update",
			payload: makeACKPayload("msg_err_lookup", models.ACKRead),
			setupMocks: func(ms *mockMessageService) {
				ms.On("GetMessageMappingByWhatsAppID", mock.Anything, "msg_err_lookup").
					Return(nil, fmt.Errorf("database connection error")).Once()
			},
			expectErr: false,
		},
		{
			name:    "no mapping found returns nil without update",
			payload: makeACKPayload("msg_no_mapping", models.ACKDevice),
			setupMocks: func(ms *mockMessageService) {
				ms.On("GetMessageMappingByWhatsAppID", mock.Anything, "msg_no_mapping").
					Return(nil, nil).Once()
			},
			expectErr: false,
		},
		{
			name:    "ACK read updates delivery status to read",
			payload: makeACKPayload("msg_read", models.ACKRead),
			setupMocks: func(ms *mockMessageService) {
				ms.On("GetMessageMappingByWhatsAppID", mock.Anything, "msg_read").
					Return(&models.MessageMapping{
						WhatsAppMsgID:  "msg_read",
						WhatsAppChatID: "+0987654321@c.us",
						DeliveryStatus: models.DeliveryStatusSent,
					}, nil).Once()
				ms.On("UpdateDeliveryStatus", mock.Anything, "msg_read", "read").
					Return(nil).Once()
			},
			expectErr: false,
		},
		{
			name:    "ACK device updates delivery status to delivered",
			payload: makeACKPayload("msg_delivered", models.ACKDevice),
			setupMocks: func(ms *mockMessageService) {
				ms.On("GetMessageMappingByWhatsAppID", mock.Anything, "msg_delivered").
					Return(&models.MessageMapping{
						WhatsAppMsgID:  "msg_delivered",
						WhatsAppChatID: "+0987654321@c.us",
						DeliveryStatus: models.DeliveryStatusSent,
					}, nil).Once()
				ms.On("UpdateDeliveryStatus", mock.Anything, "msg_delivered", "delivered").
					Return(nil).Once()
			},
			expectErr: false,
		},
		{
			name:    "ACK error updates delivery status to failed",
			payload: makeACKPayload("msg_failed", models.ACKError),
			setupMocks: func(ms *mockMessageService) {
				ms.On("GetMessageMappingByWhatsAppID", mock.Anything, "msg_failed").
					Return(&models.MessageMapping{
						WhatsAppMsgID:  "msg_failed",
						WhatsAppChatID: "+0987654321@c.us",
						DeliveryStatus: models.DeliveryStatusSent,
					}, nil).Once()
				ms.On("UpdateDeliveryStatus", mock.Anything, "msg_failed", "failed").
					Return(nil).Once()
			},
			expectErr: false,
		},
		{
			name:    "ACK server updates delivery status to sent",
			payload: makeACKPayload("msg_server", models.ACKServer),
			setupMocks: func(ms *mockMessageService) {
				ms.On("GetMessageMappingByWhatsAppID", mock.Anything, "msg_server").
					Return(&models.MessageMapping{
						WhatsAppMsgID:  "msg_server",
						WhatsAppChatID: "+0987654321@c.us",
						DeliveryStatus: models.DeliveryStatusSent,
					}, nil).Once()
				ms.On("UpdateDeliveryStatus", mock.Anything, "msg_server", "sent").
					Return(nil).Once()
			},
			expectErr: false,
		},
		{
			name:    "ACK played updates delivery status to read",
			payload: makeACKPayload("msg_played", models.ACKPlayed),
			setupMocks: func(ms *mockMessageService) {
				ms.On("GetMessageMappingByWhatsAppID", mock.Anything, "msg_played").
					Return(&models.MessageMapping{
						WhatsAppMsgID:  "msg_played",
						WhatsAppChatID: "+0987654321@c.us",
						DeliveryStatus: models.DeliveryStatusSent,
					}, nil).Once()
				ms.On("UpdateDeliveryStatus", mock.Anything, "msg_played", "read").
					Return(nil).Once()
			},
			expectErr: false,
		},
		{
			name:    "update failure does not return error",
			payload: makeACKPayload("msg_update_fail", models.ACKDevice),
			setupMocks: func(ms *mockMessageService) {
				ms.On("GetMessageMappingByWhatsAppID", mock.Anything, "msg_update_fail").
					Return(&models.MessageMapping{
						WhatsAppMsgID:  "msg_update_fail",
						WhatsAppChatID: "+0987654321@c.us",
						DeliveryStatus: models.DeliveryStatusSent,
					}, nil).Once()
				ms.On("UpdateDeliveryStatus", mock.Anything, "msg_update_fail", "delivered").
					Return(fmt.Errorf("database write error")).Once()
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgSvc := &mockMessageService{}
			logger := logrus.New()
			cfg := &models.Config{}
			waClient := &mockWAClient{}
			channelManager := createTestChannelManager()
			db := &mockDatabase{}

			server := NewServer(cfg, msgSvc, logger, waClient, channelManager, db, nil)

			tt.setupMocks(msgSvc)

			err := server.handleWhatsAppACK(context.Background(), tt.payload)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			msgSvc.AssertExpectations(t)
		})
	}
}
