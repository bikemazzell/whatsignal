package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"whatsignal/internal/database"
	"whatsignal/internal/models"
	"whatsignal/pkg/signal"
	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockWhatsAppClient struct {
	mock.Mock
	sendTextResp  *types.SendMessageResponse
	sendTextErr   error
	sendImageResp *types.SendMessageResponse
	sendImageErr  error
}

func (m *mockWhatsAppClient) SendText(ctx context.Context, chatID, text string) (*types.SendMessageResponse, error) {
	return m.sendTextResp, m.sendTextErr
}

func (m *mockWhatsAppClient) SendImage(ctx context.Context, chatID, imagePath, caption string) (*types.SendMessageResponse, error) {
	return m.sendImageResp, m.sendImageErr
}

func (m *mockWhatsAppClient) SendVideo(ctx context.Context, chatID, videoPath, caption string) (*types.SendMessageResponse, error) {
	return m.sendImageResp, m.sendImageErr
}

func (m *mockWhatsAppClient) SendDocument(ctx context.Context, chatID, docPath, caption string) (*types.SendMessageResponse, error) {
	return m.sendImageResp, m.sendImageErr
}

func (m *mockWhatsAppClient) SendFile(ctx context.Context, chatID, filePath, caption string) (*types.SendMessageResponse, error) {
	return m.sendImageResp, m.sendImageErr
}

func (m *mockWhatsAppClient) SendVoice(ctx context.Context, chatID, voicePath string) (*types.SendMessageResponse, error) {
	return m.sendImageResp, m.sendImageErr
}

func (m *mockWhatsAppClient) CreateSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWhatsAppClient) StartSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWhatsAppClient) StopSession(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockWhatsAppClient) GetSessionStatus(ctx context.Context) (*types.Session, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Session), args.Error(1)
}

func (m *mockWhatsAppClient) SendSeen(ctx context.Context, chatID string) error {
	args := m.Called(ctx, chatID)
	return args.Error(0)
}

func (m *mockWhatsAppClient) StartTyping(ctx context.Context, chatID string) error {
	args := m.Called(ctx, chatID)
	return args.Error(0)
}

func (m *mockWhatsAppClient) StopTyping(ctx context.Context, chatID string) error {
	args := m.Called(ctx, chatID)
	return args.Error(0)
}

type mockSignalClient struct {
	mock.Mock
	sendMessageResp *signal.SendMessageResponse
	sendMessageErr  error
}

func (m *mockSignalClient) SendMessage(recipient, message string, attachments []string) (*signal.SendMessageResponse, error) {
	return m.sendMessageResp, m.sendMessageErr
}

func (m *mockSignalClient) ReceiveMessages(timeoutSeconds int) ([]signal.SignalMessage, error) {
	args := m.Called(timeoutSeconds)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]signal.SignalMessage), args.Error(1)
}

func (m *mockSignalClient) Register() error {
	args := m.Called()
	return args.Error(0)
}

// Mock media handler
type mockMediaHandler struct {
	cacheDir string
}

func (h *mockMediaHandler) ProcessMedia(sourcePath string) (string, error) {
	// Simple mock that just copies the file to the cache directory
	fileName := filepath.Base(sourcePath)
	destPath := filepath.Join(h.cacheDir, fileName)

	if err := os.MkdirAll(h.cacheDir, 0755); err != nil {
		return "", err
	}

	input, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(destPath, input, 0644)
	if err != nil {
		return "", err
	}

	return destPath, nil
}

func (h *mockMediaHandler) CleanupOldFiles(maxAge int64) error {
	entries, err := os.ReadDir(h.cacheDir)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(h.cacheDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now-info.ModTime().Unix() > maxAge {
			os.Remove(path)
		}
	}

	return nil
}

type mockDatabase struct {
	mock.Mock
}

func (m *mockDatabase) SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error {
	args := m.Called(ctx, mapping)
	return args.Error(0)
}

func (m *mockDatabase) GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDatabase) GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error) {
	args := m.Called(ctx, whatsappID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDatabase) UpdateDeliveryStatus(ctx context.Context, id string, status string) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockDatabase) CleanupOldRecords(retentionDays int) error {
	args := m.Called(retentionDays)
	return args.Error(0)
}

func setupTestBridge(t *testing.T) (*bridge, string, func()) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "whatsignal-bridge-test")
	require.NoError(t, err)

	// Create a temporary database
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.New(dbPath)
	require.NoError(t, err)

	// Create media handler with temp directory
	mediaHandler := &mockMediaHandler{cacheDir: filepath.Join(tmpDir, "media")}

	// Create mock clients with default responses
	waClient := &mockWhatsAppClient{
		sendTextResp: &types.SendMessageResponse{
			MessageID: "test-wa-msg-id",
			Status:    "sent",
		},
		sendImageResp: &types.SendMessageResponse{
			MessageID: "test-wa-image-msg-id",
			Status:    "sent",
		},
	}

	// Mock session methods
	waClient.On("CreateSession", mock.Anything).Return(nil)
	waClient.On("StartSession", mock.Anything).Return(nil)
	waClient.On("StopSession", mock.Anything).Return(nil)
	waClient.On("GetSessionStatus", mock.Anything).Return(&types.Session{
		Status: types.SessionStatusRunning,
	}, nil)

	sigClient := &mockSignalClient{
		sendMessageResp: &signal.SendMessageResponse{
			Result: struct {
				Timestamp int64  `json:"timestamp"`
				MessageID string `json:"messageId"`
			}{
				MessageID: "test-sig-msg-id",
				Timestamp: time.Now().UnixMilli(),
			},
		},
	}

	b := NewBridge(waClient, sigClient, db, mediaHandler, RetryConfig{
		InitialBackoff: 1,
		MaxBackoff:     5,
		MaxAttempts:    3,
	}).(*bridge)

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return b, tmpDir, cleanup
}

func TestBridgeSendMessage(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Override mock clients for error cases
	bridge.waClient = &mockWhatsAppClient{
		sendTextErr:  assert.AnError,
		sendImageErr: assert.AnError,
	}
	bridge.sigClient = &mockSignalClient{
		sendMessageErr: assert.AnError,
	}

	tests := []struct {
		name      string
		message   *models.Message
		wantError bool
		setup     func()
	}{
		{
			name: "whatsapp error",
			message: &models.Message{
				ID:       "msg3",
				ChatID:   "chat3",
				Content:  "Error message",
				Platform: "whatsapp",
				Type:     models.TextMessage,
			},
			wantError: true,
		},
		{
			name: "signal error",
			message: &models.Message{
				ID:       "msg4",
				ThreadID: "thread1",
				Content:  "Error message",
				Platform: "signal",
				Type:     models.TextMessage,
			},
			wantError: true,
		},
		{
			name: "unsupported platform",
			message: &models.Message{
				ID:       "msg5",
				Content:  "Invalid platform",
				Platform: "invalid",
				Type:     models.TextMessage,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bridge.SendMessage(ctx, tt.message)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandleWhatsAppMessage(t *testing.T) {
	bridge, tmpDir, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test media file
	mediaContent := []byte("test media content")
	mediaPath := filepath.Join(tmpDir, "test.jpg")
	err := os.WriteFile(mediaPath, mediaContent, 0644)
	require.NoError(t, err)

	tests := []struct {
		name      string
		chatID    string
		msgID     string
		sender    string
		content   string
		mediaPath string
		wantErr   bool
	}{
		{
			name:    "text message",
			chatID:  "chat123",
			msgID:   "msg123",
			sender:  "sender123",
			content: "Hello, World!",
			wantErr: false,
		},
		{
			name:      "media message",
			chatID:    "chat123",
			msgID:     "msg124",
			sender:    "sender123",
			content:   "Check this out!",
			mediaPath: mediaPath,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bridge.HandleWhatsAppMessage(ctx, tt.chatID, tt.msgID, tt.sender, tt.content, tt.mediaPath)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandleSignalMessage(t *testing.T) {
	bridge, tmpDir, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test media file
	mediaContent := []byte("test media content")
	mediaPath := filepath.Join(tmpDir, "test.jpg")
	err := os.WriteFile(mediaPath, mediaContent, 0644)
	require.NoError(t, err)

	// First, create a WhatsApp message mapping
	mapping := &models.MessageMapping{
		WhatsAppChatID:  "chat123",
		WhatsAppMsgID:   "msg123",
		SignalMsgID:     "sig123",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
	}
	err = bridge.db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	tests := []struct {
		name    string
		msg     *signal.SignalMessage
		wantErr bool
	}{
		{
			name: "text reply",
			msg: &signal.SignalMessage{
				MessageID: "sig124",
				Sender:    "sender123",
				Message:   "This is a reply",
				QuotedMessage: &struct {
					ID        string `json:"id"`
					Author    string `json:"author"`
					Text      string `json:"text"`
					Timestamp int64  `json:"timestamp"`
				}{
					ID:     "msg123",
					Author: "sender123",
					Text:   "Original message",
				},
			},
			wantErr: false,
		},
		{
			name: "media reply",
			msg: &signal.SignalMessage{
				MessageID:   "sig125",
				Sender:      "sender123",
				Message:     "Check this out!",
				Attachments: []string{mediaPath},
				QuotedMessage: &struct {
					ID        string `json:"id"`
					Author    string `json:"author"`
					Text      string `json:"text"`
					Timestamp int64  `json:"timestamp"`
				}{
					ID:     "msg123",
					Author: "sender123",
					Text:   "Original message",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bridge.HandleSignalMessage(ctx, tt.msg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateDeliveryStatus(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test message mapping
	mapping := &models.MessageMapping{
		WhatsAppChatID:  "chat123",
		WhatsAppMsgID:   "msg123",
		SignalMsgID:     "sig123",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
	}
	err := bridge.db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	tests := []struct {
		name        string
		msgID       string
		newStatus   models.DeliveryStatus
		wantErr     bool
		wantMapping *models.MessageMapping
	}{
		{
			name:      "update to delivered",
			msgID:     "msg123",
			newStatus: models.DeliveryStatusDelivered,
			wantErr:   false,
			wantMapping: &models.MessageMapping{
				WhatsAppChatID: "chat123",
				WhatsAppMsgID:  "msg123",
				SignalMsgID:    "sig123",
				DeliveryStatus: models.DeliveryStatusDelivered,
			},
		},
		{
			name:      "update to failed",
			msgID:     "msg123",
			newStatus: models.DeliveryStatusFailed,
			wantErr:   false,
			wantMapping: &models.MessageMapping{
				WhatsAppChatID: "chat123",
				WhatsAppMsgID:  "msg123",
				SignalMsgID:    "sig123",
				DeliveryStatus: models.DeliveryStatusFailed,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bridge.UpdateDeliveryStatus(ctx, tt.msgID, tt.newStatus)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				mapping, err := bridge.db.GetMessageMappingByWhatsAppID(ctx, tt.msgID)
				require.NoError(t, err)
				assert.Equal(t, tt.wantMapping.DeliveryStatus, mapping.DeliveryStatus)
			}
		})
	}
}

func TestCleanupOldRecords(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Mock database error
	mockDB := new(mockDatabase)
	mockDB.On("CleanupOldRecords", 7).Return(assert.AnError)
	bridge.db = mockDB

	err := bridge.CleanupOldRecords(ctx, 7)
	assert.Error(t, err, "Expected error when database cleanup fails")
	mockDB.AssertExpectations(t)
}
