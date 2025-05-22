package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	sendMessageResp     *signal.SendMessageResponse
	sendMessageErr      error
	initializeDeviceErr error
}

func (m *mockSignalClient) SendMessage(ctx context.Context, recipient, message string, attachments []string) (*signal.SendMessageResponse, error) {
	if m.sendMessageResp != nil || m.sendMessageErr != nil {
		return m.sendMessageResp, m.sendMessageErr
	}
	args := m.Called(ctx, recipient, message, attachments)
	if args.Get(0) == nil && args.Error(1) == nil {
		return nil, nil
	}
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*signal.SendMessageResponse), args.Error(1)
}

func (m *mockSignalClient) ReceiveMessages(ctx context.Context, timeoutSeconds int) ([]signal.SignalMessage, error) {
	args := m.Called(ctx, timeoutSeconds)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]signal.SignalMessage), args.Error(1)
}

func (m *mockSignalClient) InitializeDevice(ctx context.Context) error {
	if m.initializeDeviceErr != nil {
		return m.initializeDeviceErr
	}
	args := m.Called(ctx)
	return args.Error(0)
}

// Mock media handler
type mockMediaHandler struct {
	mock.Mock
}

func (h *mockMediaHandler) ProcessMedia(sourcePath string) (string, error) {
	args := h.Called(sourcePath)
	return args.String(0), args.Error(1)
}

func (h *mockMediaHandler) CleanupOldFiles(maxAge int64) error {
	args := h.Called(maxAge)
	return args.Error(0)
}

// Mock database
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
	tmpDir, err := os.MkdirTemp("", "whatsignal-bridge-test")
	require.NoError(t, err)

	// Create mock database
	mockDB := &mockDatabase{}

	// Create mock clients
	mockWAClient := &mockWhatsAppClient{}
	mockSignalClient := &mockSignalClient{}

	// Create media handler with temp directory
	mediaHandler := &mockMediaHandler{}

	// Create bridge with mocks
	bridge := &bridge{
		db:        mockDB,
		waClient:  mockWAClient,
		sigClient: mockSignalClient,
		media:     mediaHandler,
		retryConfig: RetryConfig{
			InitialBackoff: 1,
			MaxBackoff:     5,
			MaxAttempts:    3,
		},
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return bridge, tmpDir, cleanup
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
		setup     func()
	}{
		{
			name:    "text message",
			chatID:  "chat123",
			msgID:   "msg123",
			sender:  "sender123",
			content: "Hello, World!",
			wantErr: false,
			setup: func() {
				bridge.sigClient.(*mockSignalClient).sendMessageResp = &signal.SendMessageResponse{
					Result: struct {
						Timestamp int64  `json:"timestamp"`
						MessageID string `json:"messageId"`
					}{
						MessageID: "sig123",
						Timestamp: time.Now().UnixMilli(),
					},
				}
				bridge.db.(*mockDatabase).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
					return m.WhatsAppChatID == "chat123" &&
						m.WhatsAppMsgID == "msg123" &&
						m.SignalMsgID == "sig123" &&
						m.DeliveryStatus == models.DeliveryStatusSent
				})).Return(nil).Once()
			},
		},
		{
			name:      "media message",
			chatID:    "chat123",
			msgID:     "msg124",
			sender:    "sender123",
			content:   "Check this out!",
			mediaPath: mediaPath,
			wantErr:   false,
			setup: func() {
				bridge.media.(*mockMediaHandler).On("ProcessMedia", mediaPath).Return(mediaPath, nil).Once()
				bridge.sigClient.(*mockSignalClient).sendMessageResp = &signal.SendMessageResponse{
					Result: struct {
						Timestamp int64  `json:"timestamp"`
						MessageID string `json:"messageId"`
					}{
						MessageID: "sig124",
						Timestamp: time.Now().UnixMilli(),
					},
				}
				bridge.db.(*mockDatabase).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
					return m.WhatsAppChatID == "chat123" &&
						m.WhatsAppMsgID == "msg124" &&
						m.SignalMsgID == "sig124" &&
						m.DeliveryStatus == models.DeliveryStatusSent &&
						*m.MediaPath == mediaPath
				})).Return(nil).Once()
			},
		},
		{
			name:      "media processing error",
			chatID:    "chat123",
			msgID:     "msg125",
			sender:    "sender123",
			content:   "Check this out!",
			mediaPath: mediaPath,
			wantErr:   true,
			setup: func() {
				bridge.media.(*mockMediaHandler).On("ProcessMedia", mediaPath).Return("", assert.AnError).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
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

	// Set up mock expectations for the initial message mapping
	bridge.db.(*mockDatabase).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
		return m.WhatsAppChatID == "chat123" &&
			m.WhatsAppMsgID == "msg123" &&
			m.SignalMsgID == "sig123" &&
			m.DeliveryStatus == models.DeliveryStatusSent
	})).Return(nil).Once()

	err = bridge.db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	tests := []struct {
		name    string
		msg     *signal.SignalMessage
		wantErr bool
		setup   func()
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
			setup: func() {
				bridge.db.(*mockDatabase).On("GetMessageMappingByWhatsAppID", ctx, "msg123").Return(mapping, nil).Once()
				bridge.waClient.(*mockWhatsAppClient).sendTextResp = &types.SendMessageResponse{
					MessageID: "msg124",
					Status:    "sent",
				}
				bridge.db.(*mockDatabase).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
					return m.WhatsAppChatID == "chat123" &&
						m.WhatsAppMsgID == "msg124" &&
						m.SignalMsgID == "sig124" &&
						m.DeliveryStatus == models.DeliveryStatusSent
				})).Return(nil).Once()
			},
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
			setup: func() {
				bridge.db.(*mockDatabase).On("GetMessageMappingByWhatsAppID", ctx, "msg123").Return(mapping, nil).Once()
				bridge.media.(*mockMediaHandler).On("ProcessMedia", mediaPath).Return(mediaPath, nil).Once()
				bridge.waClient.(*mockWhatsAppClient).sendImageResp = &types.SendMessageResponse{
					MessageID: "msg125",
					Status:    "sent",
				}
				bridge.db.(*mockDatabase).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
					return m.WhatsAppChatID == "chat123" &&
						m.WhatsAppMsgID == "msg125" &&
						m.SignalMsgID == "sig125" &&
						m.DeliveryStatus == models.DeliveryStatusSent &&
						*m.MediaPath == mediaPath &&
						m.MediaType == "image"
				})).Return(nil).Once()
			},
		},
		{
			name: "media processing error",
			msg: &signal.SignalMessage{
				MessageID:   "sig126",
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
			wantErr: true,
			setup: func() {
				bridge.db.(*mockDatabase).On("GetMessageMappingByWhatsAppID", ctx, "msg123").Return(mapping, nil).Once()
				bridge.media.(*mockMediaHandler).On("ProcessMedia", mediaPath).Return("", assert.AnError).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
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

	// Set up mock expectations
	bridge.db.(*mockDatabase).On("UpdateDeliveryStatus", ctx, "msg123", "delivered").Return(nil).Once()

	err := bridge.UpdateDeliveryStatus(ctx, "msg123", models.DeliveryStatusDelivered)
	assert.NoError(t, err)

	// Test error case
	bridge.db.(*mockDatabase).On("UpdateDeliveryStatus", ctx, "msg123", "delivered").Return(assert.AnError).Once()

	err = bridge.UpdateDeliveryStatus(ctx, "msg123", models.DeliveryStatusDelivered)
	assert.Error(t, err)
}

func TestMediaTypeDetection(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		isImage   bool
		isVideo   bool
		isDoc     bool
		mediaType string
	}{
		{
			name:      "JPEG image",
			path:      "test.jpg",
			isImage:   true,
			isVideo:   false,
			isDoc:     false,
			mediaType: "image",
		},
		{
			name:      "PNG image",
			path:      "test.png",
			isImage:   true,
			isVideo:   false,
			isDoc:     false,
			mediaType: "image",
		},
		{
			name:      "MP4 video",
			path:      "test.mp4",
			isImage:   false,
			isVideo:   true,
			isDoc:     false,
			mediaType: "video",
		},
		{
			name:      "MOV video",
			path:      "test.mov",
			isImage:   false,
			isVideo:   true,
			isDoc:     false,
			mediaType: "video",
		},
		{
			name:      "PDF document",
			path:      "test.pdf",
			isImage:   false,
			isVideo:   false,
			isDoc:     true,
			mediaType: "document",
		},
		{
			name:      "Word document",
			path:      "test.docx",
			isImage:   false,
			isVideo:   false,
			isDoc:     true,
			mediaType: "document",
		},
		{
			name:      "Unknown type",
			path:      "test.xyz",
			isImage:   false,
			isVideo:   false,
			isDoc:     false,
			mediaType: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isImage, isImageAttachment(tt.path))
			assert.Equal(t, tt.isVideo, isVideoAttachment(tt.path))
			assert.Equal(t, tt.isDoc, isDocumentAttachment(tt.path))
			assert.Equal(t, tt.mediaType, getMediaType(tt.path))
		})
	}
}

func TestCleanupOldRecords(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Test successful cleanup
	bridge.db.(*mockDatabase).On("CleanupOldRecords", 7).Return(nil).Once()
	bridge.media.(*mockMediaHandler).On("CleanupOldFiles", int64(7*24*60*60)).Return(nil).Once()

	err := bridge.CleanupOldRecords(ctx, 7)
	assert.NoError(t, err)

	// Test database cleanup error
	bridge.db.(*mockDatabase).On("CleanupOldRecords", 7).Return(assert.AnError).Once()
	err = bridge.CleanupOldRecords(ctx, 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to cleanup old records")

	// Test media cleanup error
	bridge.db.(*mockDatabase).On("CleanupOldRecords", 7).Return(nil).Once()
	bridge.media.(*mockMediaHandler).On("CleanupOldFiles", int64(7*24*60*60)).Return(assert.AnError).Once()
	err = bridge.CleanupOldRecords(ctx, 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to cleanup old media files")
}

func TestHandleSignalGroupMessage(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	msg := &signal.SignalMessage{
		MessageID: "group123",
		Sender:    "group.123",
		Message:   "Group message",
		Timestamp: time.Now().UnixMilli(),
	}

	err := bridge.handleSignalGroupMessage(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "group messages are not supported yet")
}

func TestHandleNewSignalThread(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	msg := &signal.SignalMessage{
		MessageID: "msg123",
		Sender:    "sender123",
		Message:   "New thread message",
		Timestamp: time.Now().UnixMilli(),
	}

	err := bridge.handleNewSignalThread(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "new thread creation is not supported yet")
}

func TestNewBridge(t *testing.T) {
	waClient := &mockWhatsAppClient{}
	sigClient := &mockSignalClient{}
	db := &mockDatabase{}
	mediaHandler := &mockMediaHandler{}
	retryConfig := RetryConfig{
		InitialBackoff: 1,
		MaxBackoff:     5,
		MaxAttempts:    3,
	}

	b := NewBridge(waClient, sigClient, db, mediaHandler, retryConfig)
	require.NotNil(t, b)

	// Test that the bridge implements the MessageBridge interface
	_, ok := b.(MessageBridge)
	assert.True(t, ok)

	// Test that the bridge has the correct fields
	bridgeImpl := b.(*bridge)
	assert.Equal(t, waClient, bridgeImpl.waClient)
	assert.Equal(t, sigClient, bridgeImpl.sigClient)
	assert.Equal(t, db, bridgeImpl.db)
	assert.Equal(t, mediaHandler, bridgeImpl.media)
	assert.Equal(t, retryConfig, bridgeImpl.retryConfig)
}
