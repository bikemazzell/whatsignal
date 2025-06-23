package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"whatsignal/internal/models"
	signaltypes "whatsignal/pkg/signal/types"
	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)


func setupTestBridge(t *testing.T) (*bridge, string, func()) {
	tmpDir, err := os.MkdirTemp("", "whatsignal-bridge-test")
	require.NoError(t, err)

	// Create mock database
	mockDB := &mockDatabaseService{}

	// Create mock clients
	mockWAClient := &mockWhatsAppClient{}
	mockSignalClient := &mockSignalClient{}

	// Create media handler with temp directory
	mediaHandler := &mockMediaHandler{}

	// Create bridge with mocks using the constructor
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 1,
		MaxBackoffMs:     5,
		MaxAttempts:      3,
	}
	// Create bridge without contact service for basic tests (contact service has its own tests)
	bridge := NewBridge(mockWAClient, mockSignalClient, mockDB, mediaHandler, retryConfig, "+1234567890", nil).(*bridge)

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
				bridge.sigClient.(*mockSignalClient).sendMessageResp = &signaltypes.SendMessageResponse{
					MessageID: "sig123",
					Timestamp: time.Now().UnixMilli(),
				}
				bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
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
				bridge.sigClient.(*mockSignalClient).sendMessageResp = &signaltypes.SendMessageResponse{
					MessageID: "sig124",
					Timestamp: time.Now().UnixMilli(),
				}
				bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
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
	bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
		return m.WhatsAppChatID == "chat123" &&
			m.WhatsAppMsgID == "msg123" &&
			m.SignalMsgID == "sig123" &&
			m.DeliveryStatus == models.DeliveryStatusSent
	})).Return(nil).Once()

	err = bridge.db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	tests := []struct {
		name    string
		msg     *signaltypes.SignalMessage
		wantErr bool
		setup   func()
	}{
		{
			name: "text reply",
			msg: &signaltypes.SignalMessage{
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
				bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, "msg123").Return(mapping, nil).Once()
				bridge.waClient.(*mockWhatsAppClient).sendTextResp = &types.SendMessageResponse{
					MessageID: "msg124",
					Status:    "sent",
				}
				bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
					return m.WhatsAppChatID == "chat123" &&
						m.WhatsAppMsgID == "msg124" &&
						m.SignalMsgID == "sig124" &&
						m.DeliveryStatus == models.DeliveryStatusSent
				})).Return(nil).Once()
			},
		},
		{
			name: "media reply",
			msg: &signaltypes.SignalMessage{
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
				bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, "msg123").Return(mapping, nil).Once()
				bridge.media.(*mockMediaHandler).On("ProcessMedia", mediaPath).Return(mediaPath, nil).Once()
				bridge.waClient.(*mockWhatsAppClient).sendImageResp = &types.SendMessageResponse{
					MessageID: "msg125",
					Status:    "sent",
				}
				bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
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
			name: "voice message reply",
			msg: &signaltypes.SignalMessage{
				MessageID:   "sig126",
				Sender:      "sender123",
				Message:     "Listen to this!",
				Attachments: []string{filepath.Join(tmpDir, "test.ogg")},
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
				// Create a test voice file
				voicePath := filepath.Join(tmpDir, "test.ogg")
				err := os.WriteFile(voicePath, []byte("test voice content"), 0644)
				require.NoError(t, err)

				bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, "msg123").Return(mapping, nil).Once()
				bridge.media.(*mockMediaHandler).On("ProcessMedia", voicePath).Return(voicePath, nil).Once()
				bridge.waClient.(*mockWhatsAppClient).sendVoiceResp = &types.SendMessageResponse{
					MessageID: "msg126",
					Status:    "sent",
				}
				bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
					return m.WhatsAppChatID == "chat123" &&
						m.WhatsAppMsgID == "msg126" &&
						m.SignalMsgID == "sig126" &&
						m.DeliveryStatus == models.DeliveryStatusSent &&
						*m.MediaPath == voicePath &&
						m.MediaType == "voice"
				})).Return(nil).Once()
			},
		},
		{
			name: "video message reply",
			msg: &signaltypes.SignalMessage{
				MessageID:   "sig127",
				Sender:      "sender123",
				Message:     "Watch this!",
				Attachments: []string{filepath.Join(tmpDir, "test.mp4")},
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
				// Create a test video file
				videoPath := filepath.Join(tmpDir, "test.mp4")
				err := os.WriteFile(videoPath, []byte("test video content"), 0644)
				require.NoError(t, err)

				bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, "msg123").Return(mapping, nil).Once()
				bridge.media.(*mockMediaHandler).On("ProcessMedia", videoPath).Return(videoPath, nil).Once()
				bridge.waClient.(*mockWhatsAppClient).sendVideoResp = &types.SendMessageResponse{
					MessageID: "msg127",
					Status:    "sent",
				}
				bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
					return m.WhatsAppChatID == "chat123" &&
						m.WhatsAppMsgID == "msg127" &&
						m.SignalMsgID == "sig127" &&
						m.DeliveryStatus == models.DeliveryStatusSent &&
						*m.MediaPath == videoPath &&
						m.MediaType == "video"
				})).Return(nil).Once()
			},
		},
		{
			name: "document message reply",
			msg: &signaltypes.SignalMessage{
				MessageID:   "sig128",
				Sender:      "sender123",
				Message:     "Read this document!",
				Attachments: []string{filepath.Join(tmpDir, "test.pdf")},
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
				// Create a test document file
				docPath := filepath.Join(tmpDir, "test.pdf")
				err := os.WriteFile(docPath, []byte("test document content"), 0644)
				require.NoError(t, err)

				bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, "msg123").Return(mapping, nil).Once()
				bridge.media.(*mockMediaHandler).On("ProcessMedia", docPath).Return(docPath, nil).Once()
				bridge.waClient.(*mockWhatsAppClient).sendDocumentResp = &types.SendMessageResponse{
					MessageID: "msg128",
					Status:    "sent",
				}
				bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.MatchedBy(func(m *models.MessageMapping) bool {
					return m.WhatsAppChatID == "chat123" &&
						m.WhatsAppMsgID == "msg128" &&
						m.SignalMsgID == "sig128" &&
						m.DeliveryStatus == models.DeliveryStatusSent &&
						*m.MediaPath == docPath &&
						m.MediaType == "document"
				})).Return(nil).Once()
			},
		},
		{
			name: "media processing error",
			msg: &signaltypes.SignalMessage{
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
			wantErr: false, // Changed: media processing errors are now handled gracefully
			setup: func() {
				bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, "msg123").Return(mapping, nil).Once()
				bridge.media.(*mockMediaHandler).On("ProcessMedia", mediaPath).Return("", assert.AnError).Once()
				// Set up text response for when media processing fails
				bridge.waClient.(*mockWhatsAppClient).sendTextResp = &types.SendMessageResponse{
					MessageID: "msg124", // Use consistent ID with other tests
					Status:    "sent",
				}
				// Expect message mapping to be saved even when media processing fails
				bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.MatchedBy(func(mapping *models.MessageMapping) bool {
					return mapping.WhatsAppChatID == "chat123" &&
						   mapping.WhatsAppMsgID == "msg124" &&
						   mapping.SignalMsgID == "sig126" &&
						   mapping.MediaPath == nil // No media path when processing fails
				})).Return(nil).Once()
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
	bridge.db.(*mockDatabaseService).On("UpdateDeliveryStatus", ctx, "msg123", "delivered").Return(nil).Once()

	err := bridge.UpdateDeliveryStatus(ctx, "msg123", models.DeliveryStatusDelivered)
	assert.NoError(t, err)

	// Test error case
	bridge.db.(*mockDatabaseService).On("UpdateDeliveryStatus", ctx, "msg123", "delivered").Return(assert.AnError).Once()

	err = bridge.UpdateDeliveryStatus(ctx, "msg123", models.DeliveryStatusDelivered)
	assert.Error(t, err)
}

func TestMediaTypeDetection(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		isImage   bool
		isVideo   bool
		isVoice   bool
		isDoc     bool
		mediaType string
	}{
		{
			name:      "JPEG image",
			path:      "test.jpg",
			isImage:   true,
			isVideo:   false,
			isVoice:   false,
			isDoc:     false,
			mediaType: "image",
		},
		{
			name:      "PNG image",
			path:      "test.png",
			isImage:   true,
			isVideo:   false,
			isVoice:   false,
			isDoc:     false,
			mediaType: "image",
		},
		{
			name:      "JPEG uppercase",
			path:      "test.JPEG",
			isImage:   true,
			isVideo:   false,
			isVoice:   false,
			isDoc:     false,
			mediaType: "image",
		},
		{
			name:      "MP4 video",
			path:      "test.mp4",
			isImage:   false,
			isVideo:   true,
			isVoice:   false,
			isDoc:     false,
			mediaType: "video",
		},
		{
			name:      "MOV video",
			path:      "test.mov",
			isImage:   false,
			isVideo:   true,
			isVoice:   false,
			isDoc:     false,
			mediaType: "video",
		},
		{
			name:      "OGG voice",
			path:      "test.ogg",
			isImage:   false,
			isVideo:   false,
			isVoice:   true,
			isDoc:     false,
			mediaType: "voice",
		},
		{
			name:      "AAC voice",
			path:      "test.aac",
			isImage:   false,
			isVideo:   false,
			isVoice:   true,
			isDoc:     false,
			mediaType: "voice",
		},
		{
			name:      "M4A voice",
			path:      "test.m4a",
			isImage:   false,
			isVideo:   false,
			isVoice:   true,
			isDoc:     false,
			mediaType: "voice",
		},
		{
			name:      "OGG uppercase",
			path:      "test.OGG",
			isImage:   false,
			isVideo:   false,
			isVoice:   true,
			isDoc:     false,
			mediaType: "voice",
		},
		{
			name:      "PDF document",
			path:      "test.pdf",
			isImage:   false,
			isVideo:   false,
			isVoice:   false,
			isDoc:     true,
			mediaType: "document",
		},
		{
			name:      "Word document",
			path:      "test.docx",
			isImage:   false,
			isVideo:   false,
			isVoice:   false,
			isDoc:     true,
			mediaType: "document",
		},
		{
			name:      "DOC document",
			path:      "test.doc",
			isImage:   false,
			isVideo:   false,
			isVoice:   false,
			isDoc:     true,
			mediaType: "document",
		},
		{
			name:      "Unknown type",
			path:      "test.xyz",
			isImage:   false,
			isVideo:   false,
			isVoice:   false,
			isDoc:     false,
			mediaType: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isImage, isImageAttachment(tt.path))
			assert.Equal(t, tt.isVideo, isVideoAttachment(tt.path))
			assert.Equal(t, tt.isVoice, isVoiceAttachment(tt.path))
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
	bridge.db.(*mockDatabaseService).On("CleanupOldRecords", 7).Return(nil).Once()
	bridge.media.(*mockMediaHandler).On("CleanupOldFiles", int64(7*24*60*60)).Return(nil).Once()

	err := bridge.CleanupOldRecords(ctx, 7)
	assert.NoError(t, err)

	// Test database cleanup error
	bridge.db.(*mockDatabaseService).On("CleanupOldRecords", 7).Return(assert.AnError).Once()
	err = bridge.CleanupOldRecords(ctx, 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to cleanup old records")

	// Test media cleanup error
	bridge.db.(*mockDatabaseService).On("CleanupOldRecords", 7).Return(nil).Once()
	bridge.media.(*mockMediaHandler).On("CleanupOldFiles", int64(7*24*60*60)).Return(assert.AnError).Once()
	err = bridge.CleanupOldRecords(ctx, 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to cleanup old media files")
}

func TestHandleSignalGroupMessage(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	msg := &signaltypes.SignalMessage{
		MessageID: "group123",
		Sender:    "group.123",
		Message:   "Group message",
		Timestamp: time.Now().UnixMilli(),
	}

	err := bridge.handleSignalGroupMessage(ctx, msg)
	assert.NoError(t, err) // Should not error with graceful degradation
}

func TestHandleNewSignalThread(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	msg := &signaltypes.SignalMessage{
		MessageID: "msg123",
		Sender:    "sender123",
		Message:   "New thread message",
		Timestamp: time.Now().UnixMilli(),
	}

	err := bridge.handleNewSignalThread(ctx, msg)
	assert.NoError(t, err) // Should not error with graceful degradation
}

func TestNewBridge(t *testing.T) {
	waClient := &mockWhatsAppClient{}
	sigClient := &mockSignalClient{}
	db := &mockDatabaseService{}
	mediaHandler := &mockMediaHandler{}
	retryConfig := models.RetryConfig{
		InitialBackoffMs: 1,
		MaxBackoffMs:     5,
		MaxAttempts:      3,
	}

	destinationPhoneNumber := "+1234567890"
	// For constructor test, use nil contact service to keep test simple
	b := NewBridge(waClient, sigClient, db, mediaHandler, retryConfig, destinationPhoneNumber, nil)
	require.NotNil(t, b)

	// Test that the bridge implements the MessageBridge interface
	var _ MessageBridge = b

	// Test that the bridge has the correct fields
	bridgeImpl := b.(*bridge)
	assert.Equal(t, waClient, bridgeImpl.waClient)
	assert.Equal(t, sigClient, bridgeImpl.sigClient)
	assert.Equal(t, db, bridgeImpl.db)
	assert.Equal(t, mediaHandler, bridgeImpl.media)
	assert.Equal(t, retryConfig, bridgeImpl.retryConfig)
	assert.Equal(t, destinationPhoneNumber, bridgeImpl.signalDestinationPhoneNumber)
}
