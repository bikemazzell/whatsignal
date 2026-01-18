package service

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	// Create a test media config
	mediaConfig := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Image:    []string{"jpg", "jpeg", "png"},
			Video:    []string{"mp4", "mov"},
			Document: []string{"pdf", "doc", "docx"},
			Voice:    []string{"ogg", "aac", "m4a", "oga"},
		},
	}

	// Create a channel manager for the test
	channels := []models.Channel{
		{
			WhatsAppSessionName:          "default",
			SignalDestinationPhoneNumber: "+1234567890",
		},
	}
	channelManager, err := NewChannelManager(channels)
	require.NoError(t, err)

	// Create test logger
	testLogger := logrus.New()
	testLogger.SetOutput(io.Discard) // Suppress test output

	// Create bridge without contact service and group service for basic tests (those services have their own tests)
	bridge := NewBridge(mockWAClient, mockSignalClient, mockDB, mediaHandler, retryConfig, mediaConfig, channelManager, nil, nil, "", testLogger).(*bridge)

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
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
				bridge.sigClient.(*mockSignalClient).sendMessageResponse = &signaltypes.SendMessageResponse{
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
				bridge.sigClient.(*mockSignalClient).sendMessageResponse = &signaltypes.SendMessageResponse{
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
			err := bridge.HandleWhatsAppMessageWithSession(ctx, "default", tt.chatID, tt.msgID, tt.sender, "", tt.content, tt.mediaPath)
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
		{
			name: "reaction message",
			msg: &signaltypes.SignalMessage{
				MessageID: "sig127",
				Sender:    "sender123",
				Message:   "👍",
				Timestamp: time.Now().UnixMilli(),
				Reaction: &signaltypes.SignalReaction{
					Emoji:           "👍",
					TargetAuthor:    "+0987654321",
					TargetTimestamp: 1234567890000,
					IsRemove:        false,
				},
			},
			wantErr: false,
			setup: func() {
				// For reactions, we look up the target message by timestamp
				targetID := "1234567890000"
				targetMapping := &models.MessageMapping{
					WhatsAppChatID: "chat123",
					WhatsAppMsgID:  "wa_msg456",
					SignalMsgID:    targetID,
				}
				bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, targetID).Return(targetMapping, nil).Once()

				// Expect SendReaction to be called
				resp := &types.SendMessageResponse{
					MessageID: "reaction_msg_id",
					Status:    "sent",
				}
				bridge.waClient.(*mockWhatsAppClient).On("SendReactionWithSession", ctx, "chat123", "wa_msg456", "👍", "default").
					Return(resp, nil).Once()
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
			name:      "Unknown type (defaults to document)",
			path:      "test.xyz",
			isImage:   false,
			isVideo:   false,
			isVoice:   false,
			isDoc:     false,
			mediaType: "document",
		},
	}

	// Create a test bridge to access the methods
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isImage, bridge.mediaRouter.IsImageAttachment(tt.path))
			assert.Equal(t, tt.isVideo, bridge.mediaRouter.IsVideoAttachment(tt.path))
			assert.Equal(t, tt.isVoice, bridge.mediaRouter.IsVoiceAttachment(tt.path))
			assert.Equal(t, tt.isDoc, bridge.mediaRouter.IsDocumentAttachment(tt.path))
			assert.Equal(t, tt.mediaType, bridge.mediaRouter.GetMediaType(tt.path))
		})
	}
}

func TestCleanupOldRecords(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Test successful cleanup
	bridge.db.(*mockDatabaseService).On("CleanupOldRecords", ctx, 7).Return(nil).Once()
	bridge.media.(*mockMediaHandler).On("CleanupOldFiles", int64(7*24*60*60)).Return(nil).Once()

	err := bridge.CleanupOldRecords(ctx, 7)
	assert.NoError(t, err)

	// Test database cleanup error
	bridge.db.(*mockDatabaseService).On("CleanupOldRecords", ctx, 7).Return(assert.AnError).Once()
	err = bridge.CleanupOldRecords(ctx, 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to cleanup old records")

	// Test media cleanup error
	bridge.db.(*mockDatabaseService).On("CleanupOldRecords", ctx, 7).Return(nil).Once()
	bridge.media.(*mockMediaHandler).On("CleanupOldFiles", int64(7*24*60*60)).Return(assert.AnError).Once()
	err = bridge.CleanupOldRecords(ctx, 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to cleanup old media files")
}

func TestCleanupSignalAttachments(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "signal-attachments-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test files with different ages
	oldFile := filepath.Join(tmpDir, "old-attachment.jpg")
	newFile := filepath.Join(tmpDir, "new-attachment.jpg")

	err = os.WriteFile(oldFile, []byte("old content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(newFile, []byte("new content"), 0644)
	require.NoError(t, err)

	// Set old file's modification time to 10 days ago
	oldTime := time.Now().AddDate(0, 0, -10)
	err = os.Chtimes(oldFile, oldTime, oldTime)
	require.NoError(t, err)

	// Create bridge with signal attachments dir
	mockWAClient := new(mockWAClient)
	mockSignalClient := new(mockSignalClient)
	mockDB := new(mockDatabaseService)
	mockMedia := new(mockMediaHandler)

	channels := []models.Channel{
		{
			WhatsAppSessionName:          "test-session",
			SignalDestinationPhoneNumber: "+1234567890",
		},
	}
	channelManager, err := NewChannelManager(channels)
	require.NoError(t, err)

	testLogger := logrus.New()
	testLogger.SetOutput(io.Discard)

	bridge := NewBridge(mockWAClient, mockSignalClient, mockDB, mockMedia, models.RetryConfig{}, models.MediaConfig{}, channelManager, nil, nil, tmpDir, testLogger).(*bridge)

	// Setup mocks for successful cleanup
	mockDB.On("CleanupOldRecords", mock.Anything, 7).Return(nil).Once()
	mockMedia.On("CleanupOldFiles", int64(7*24*60*60)).Return(nil).Once()

	// Run cleanup with 7 days retention
	err = bridge.CleanupOldRecords(context.Background(), 7)
	require.NoError(t, err)

	// Verify old file was deleted
	_, err = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err), "old file should be deleted")

	// Verify new file still exists
	_, err = os.Stat(newFile)
	assert.NoError(t, err, "new file should still exist")
}

func TestCleanupSignalAttachmentsEmptyDir(t *testing.T) {
	// Test with empty signal attachments dir (should be no-op)
	mockWAClient := new(mockWAClient)
	mockSignalClient := new(mockSignalClient)
	mockDB := new(mockDatabaseService)
	mockMedia := new(mockMediaHandler)

	channels := []models.Channel{
		{
			WhatsAppSessionName:          "test-session",
			SignalDestinationPhoneNumber: "+1234567890",
		},
	}
	channelManager, err := NewChannelManager(channels)
	require.NoError(t, err)

	testLogger := logrus.New()
	testLogger.SetOutput(io.Discard)

	bridge := NewBridge(mockWAClient, mockSignalClient, mockDB, mockMedia, models.RetryConfig{}, models.MediaConfig{}, channelManager, nil, nil, "", testLogger).(*bridge)

	// Setup mocks for successful cleanup
	mockDB.On("CleanupOldRecords", mock.Anything, 7).Return(nil).Once()
	mockMedia.On("CleanupOldFiles", int64(7*24*60*60)).Return(nil).Once()

	// Run cleanup - should succeed even with empty attachments dir
	err = bridge.CleanupOldRecords(context.Background(), 7)
	require.NoError(t, err)
}

func TestCleanupSignalAttachmentsNonExistentDir(t *testing.T) {
	// Test with non-existent signal attachments dir (should be no-op)
	mockWAClient := new(mockWAClient)
	mockSignalClient := new(mockSignalClient)
	mockDB := new(mockDatabaseService)
	mockMedia := new(mockMediaHandler)

	channels := []models.Channel{
		{
			WhatsAppSessionName:          "test-session",
			SignalDestinationPhoneNumber: "+1234567890",
		},
	}
	channelManager, err := NewChannelManager(channels)
	require.NoError(t, err)

	testLogger := logrus.New()
	testLogger.SetOutput(io.Discard)

	bridge := NewBridge(mockWAClient, mockSignalClient, mockDB, mockMedia, models.RetryConfig{}, models.MediaConfig{}, channelManager, nil, nil, "/nonexistent/path/to/attachments", testLogger).(*bridge)

	// Setup mocks for successful cleanup
	mockDB.On("CleanupOldRecords", mock.Anything, 7).Return(nil).Once()
	mockMedia.On("CleanupOldFiles", int64(7*24*60*60)).Return(nil).Once()

	// Run cleanup - should succeed even with non-existent dir
	err = bridge.CleanupOldRecords(context.Background(), 7)
	require.NoError(t, err)
}

func TestHandleSignalGroupMessage(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Quoted message mapping pointing to a WhatsApp group chat
	mapping := &models.MessageMapping{
		WhatsAppChatID: "group123@g.us",
		WhatsAppMsgID:  "wa_msg_1",
		SignalMsgID:    "sig_orig",
		ForwardedAt:    time.Now(),
	}
	bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, "wa_msg_1").Return(mapping, nil).Once()

	// Expect a WhatsApp text send and mapping save
	bridge.waClient.(*mockWhatsAppClient).sendTextResp = &types.SendMessageResponse{
		MessageID: "wa_msg_reply",
		Status:    "sent",
	}
	bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.AnythingOfType("*models.MessageMapping")).Return(nil).Once()

	msg := &signaltypes.SignalMessage{
		MessageID: "sig_reply_1",
		Sender:    "group.123",
		Message:   "Group reply",
		Timestamp: time.Now().UnixMilli(),
		QuotedMessage: &struct {
			ID        string `json:"id"`
			Author    string `json:"author"`
			Text      string `json:"text"`
			Timestamp int64  `json:"timestamp"`
		}{
			ID: "wa_msg_1",
		},
	}

	err := bridge.HandleSignalMessage(ctx, msg)
	assert.NoError(t, err)
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

func TestHandleSignalReaction(t *testing.T) {
	tests := []struct {
		name          string
		msg           *signaltypes.SignalMessage
		mapping       *models.MessageMapping
		mappingError  error
		reactionError error
		expectError   bool
		errorContains string
	}{
		{
			name: "successful reaction",
			msg: &signaltypes.SignalMessage{
				MessageID: "msg123",
				Sender:    "sender123",
				Timestamp: time.Now().UnixMilli(),
				Reaction: &signaltypes.SignalReaction{
					Emoji:           "👍",
					TargetAuthor:    "+0987654321",
					TargetTimestamp: 1234567890000,
					IsRemove:        false,
				},
			},
			mapping: &models.MessageMapping{
				WhatsAppChatID: "chat123@c.us",
				WhatsAppMsgID:  "wa_msg456",
				SignalMsgID:    "1234567890000",
			},
			expectError: false,
		},
		{
			name: "remove reaction",
			msg: &signaltypes.SignalMessage{
				MessageID: "msg124",
				Sender:    "sender123",
				Timestamp: time.Now().UnixMilli(),
				Reaction: &signaltypes.SignalReaction{
					Emoji:           "👍",
					TargetAuthor:    "+0987654321",
					TargetTimestamp: 1234567890000,
					IsRemove:        true,
				},
			},
			mapping: &models.MessageMapping{
				WhatsAppChatID: "chat123@c.us",
				WhatsAppMsgID:  "wa_msg456",
				SignalMsgID:    "1234567890000",
			},
			expectError: false,
		},
		{
			name: "mapping not found",
			msg: &signaltypes.SignalMessage{
				MessageID: "msg125",
				Sender:    "sender123",
				Timestamp: time.Now().UnixMilli(),
				Reaction: &signaltypes.SignalReaction{
					Emoji:           "❤️",
					TargetAuthor:    "+0987654321",
					TargetTimestamp: 1234567890000,
					IsRemove:        false,
				},
			},
			mapping:       nil,
			expectError:   true,
			errorContains: "no mapping found",
		},
		{
			name: "database error",
			msg: &signaltypes.SignalMessage{
				MessageID: "msg126",
				Sender:    "sender123",
				Timestamp: time.Now().UnixMilli(),
				Reaction: &signaltypes.SignalReaction{
					Emoji:           "😊",
					TargetAuthor:    "+0987654321",
					TargetTimestamp: 1234567890000,
					IsRemove:        false,
				},
			},
			mappingError:  assert.AnError,
			expectError:   true,
			errorContains: "failed to get message mapping",
		},
		{
			name: "WhatsApp send error",
			msg: &signaltypes.SignalMessage{
				MessageID: "msg127",
				Sender:    "sender123",
				Timestamp: time.Now().UnixMilli(),
				Reaction: &signaltypes.SignalReaction{
					Emoji:           "🎉",
					TargetAuthor:    "+0987654321",
					TargetTimestamp: 1234567890000,
					IsRemove:        false,
				},
			},
			mapping: &models.MessageMapping{
				WhatsAppChatID: "chat123@c.us",
				WhatsAppMsgID:  "wa_msg456",
				SignalMsgID:    "1234567890000",
			},
			reactionError: assert.AnError,
			expectError:   true,
			errorContains: "failed to send reaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge, _, cleanup := setupTestBridge(t)
			defer cleanup()

			ctx := context.Background()

			// Setup database mock
			targetID := "1234567890000"
			mockDB := bridge.db.(*mockDatabaseService)
			mockDB.On("GetMessageMapping", ctx, targetID).Return(tt.mapping, tt.mappingError).Once()

			// Setup WhatsApp client mock if needed
			if tt.mapping != nil && tt.mappingError == nil {
				mockWA := bridge.waClient.(*mockWhatsAppClient)
				reaction := tt.msg.Reaction.Emoji
				if tt.msg.Reaction.IsRemove {
					reaction = ""
				}
				resp := &types.SendMessageResponse{
					MessageID: "reaction_msg_id",
					Status:    "sent",
				}
				mockWA.On("SendReactionWithSession", ctx, tt.mapping.WhatsAppChatID, tt.mapping.WhatsAppMsgID, reaction, "default").
					Return(resp, tt.reactionError).Once()
			}

			err := bridge.handleSignalReactionWithSession(ctx, tt.msg, "default")

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}

			mockDB.AssertExpectations(t)
			if tt.mapping != nil && tt.mappingError == nil {
				bridge.waClient.(*mockWhatsAppClient).AssertExpectations(t)
			}
		})
	}
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

	mediaConfig := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Image:    []string{"jpg", "jpeg", "png"},
			Video:    []string{"mp4", "mov"},
			Document: []string{"pdf", "doc", "docx"},
			Voice:    []string{"ogg", "aac", "m4a"},
		},
	}

	// Create a channel manager for the test
	channels := []models.Channel{
		{
			WhatsAppSessionName:          "default",
			SignalDestinationPhoneNumber: "+1234567890",
		},
	}
	channelManager, err := NewChannelManager(channels)
	require.NoError(t, err)

	// Create test logger
	testLogger := logrus.New()
	testLogger.SetOutput(io.Discard)

	// For constructor test, use nil contact service and group service to keep test simple
	b := NewBridge(waClient, sigClient, db, mediaHandler, retryConfig, mediaConfig, channelManager, nil, nil, "", testLogger)
	require.NotNil(t, b)

	// Test that the bridge implements the MessageBridge interface
	var _ = b

	// Test that the bridge has the correct fields
	bridgeImpl := b.(*bridge)
	assert.Equal(t, waClient, bridgeImpl.waClient)
	assert.Equal(t, sigClient, bridgeImpl.sigClient)
	assert.Equal(t, db, bridgeImpl.db)
	assert.Equal(t, mediaHandler, bridgeImpl.media)
	assert.Equal(t, retryConfig, bridgeImpl.retryConfig)
	assert.Equal(t, channelManager, bridgeImpl.channelManager)
}

func TestHandleSignalVoiceRecordingWithoutExtension(t *testing.T) {
	bridge, tmpDir, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Create a mock OGG voice recording file without extension (like Signal creates)
	voiceFile := filepath.Join(tmpDir, "signal_voice_recording")
	oggHeader := []byte("OggS")                              // OGG file signature
	voiceContent := append(oggHeader, make([]byte, 1000)...) // Simulate 1KB voice recording
	err := os.WriteFile(voiceFile, voiceContent, 0644)
	require.NoError(t, err)

	// Create Signal message with voice recording attachment (with quoted message to avoid new thread handling)
	msg := &signaltypes.SignalMessage{
		MessageID:   "voice123",
		Sender:      "+1234567890",
		Message:     "",
		Timestamp:   time.Now().UnixMilli(),
		Attachments: []string{voiceFile},
		QuotedMessage: &struct {
			ID        string `json:"id"`
			Author    string `json:"author"`
			Text      string `json:"text"`
			Timestamp int64  `json:"timestamp"`
		}{
			ID:        "quoted123",
			Author:    "+1234567890",
			Text:      "Previous message",
			Timestamp: time.Now().UnixMilli() - 1000,
		},
	}

	// Mock database to return a mapping for the quoted message
	mockMapping := &models.MessageMapping{
		WhatsAppChatID: "+1234567890@c.us",
	}
	bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, "quoted123").Return(mockMapping, nil)

	// Mock media handler to process the voice file and return path with .ogg extension
	processedPath := filepath.Join(tmpDir, "cached_voice_file.ogg")
	err = os.WriteFile(processedPath, voiceContent, 0644)
	require.NoError(t, err)
	bridge.media.(*mockMediaHandler).On("ProcessMedia", voiceFile).Return(processedPath, nil)

	// Mock WhatsApp client to return voice response
	expectedResponse := &types.SendMessageResponse{
		MessageID: "wa-voice-123",
		Status:    "sent",
	}
	bridge.waClient.(*mockWhatsAppClient).sendVoiceResp = expectedResponse

	// Mock database save
	bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.AnythingOfType("*models.MessageMapping")).Return(nil)

	// Process the Signal message
	err = bridge.HandleSignalMessage(ctx, msg)
	assert.NoError(t, err)

	// The fact that the test passed means SendVoice was successfully called with the processed path
	// and the voice recording was correctly identified and sent as a voice message
}

func TestHandleSignalMessageAutoReply(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Test: Signal message without quoted message should auto-reply to latest WhatsApp message from that chat
	msg := &signaltypes.SignalMessage{
		MessageID:     "auto_reply_123",
		Sender:        "+1234567890",
		Message:       "This is an auto-reply",
		Timestamp:     time.Now().UnixMilli(),
		QuotedMessage: nil, // No quoted message - should trigger auto-reply logic
	}

	// Mock database to return latest message mapping
	latestMapping := &models.MessageMapping{
		WhatsAppChatID: "+1234567890@c.us",
		WhatsAppMsgID:  "latest_wa_msg_123",
		SignalMsgID:    "latest_sig_msg_123",
		ForwardedAt:    time.Now().Add(-5 * time.Minute), // 5 minutes ago
	}
	bridge.db.(*mockDatabaseService).On("GetLatestMessageMappingBySession", ctx, "default").Return(latestMapping, nil)
	bridge.db.(*mockDatabaseService).On("GetLatestMessageMapping", ctx).Return(latestMapping, nil)

	// Mock Signal client to handle fallback routing notification
	bridge.sigClient.(*mockSignalClient).On("SendMessage", ctx, "+1234567890", mock.MatchedBy(func(msg string) bool {
		return strings.Contains(msg, "Message routed to last active chat")
	}), []string{}).Return(&signaltypes.SendMessageResponse{MessageID: "notif_123"}, nil)

	// Mock WhatsApp client to return text response
	expectedResponse := &types.SendMessageResponse{
		MessageID: "auto_reply_wa_msg",
		Status:    "sent",
	}
	bridge.waClient.(*mockWhatsAppClient).sendTextResp = expectedResponse

	// Mock database save
	bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.AnythingOfType("*models.MessageMapping")).Return(nil)

	// Process the Signal message
	err := bridge.HandleSignalMessage(ctx, msg)
	assert.NoError(t, err)

	// Verify that the auto-reply logic was triggered and the message was sent
	bridge.db.(*mockDatabaseService).AssertCalled(t, "GetLatestMessageMappingBySession", ctx, "default")
}

func TestHandleSignalMessageAutoReplyNoHistory(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Test: Signal message without quoted message and no chat history should be treated as new thread
	msg := &signaltypes.SignalMessage{
		MessageID:     "new_thread_123",
		Sender:        "+9999999999",
		Message:       "This should be a new thread",
		Timestamp:     time.Now().UnixMilli(),
		QuotedMessage: nil,
	}

	// Mock database to return no message mapping (new conversation)
	bridge.db.(*mockDatabaseService).On("GetLatestMessageMappingBySession", ctx, "default").Return(nil, nil)
	bridge.db.(*mockDatabaseService).On("GetLatestMessageMapping", ctx).Return(nil, nil)

	// Process the Signal message - should call handleNewSignalThread (which currently logs and returns nil)
	err := bridge.HandleSignalMessage(ctx, msg)
	assert.NoError(t, err) // Should not error, just log and ignore

	// Verify that the auto-reply logic was attempted but found no history
	bridge.db.(*mockDatabaseService).AssertCalled(t, "GetLatestMessageMappingBySession", ctx, "default")
}

func TestHandleSignalMessageDeletion(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Test: Signal message deletion should delete corresponding WhatsApp message
	targetMessageID := "target_msg_123"
	sender := "+1234567890"

	// Mock database to return message mapping for target message
	mapping := &models.MessageMapping{
		WhatsAppChatID: "+1234567890@c.us",
		WhatsAppMsgID:  "wa_msg_123",
		SignalMsgID:    targetMessageID,
		ForwardedAt:    time.Now().Add(-5 * time.Minute),
	}
	bridge.db.(*mockDatabaseService).On("GetMessageMappingBySignalID", ctx, targetMessageID).Return(mapping, nil)

	// Mock WhatsApp client to handle deletion
	bridge.waClient.(*mockWhatsAppClient).On("DeleteMessage", ctx, "+1234567890@c.us", "wa_msg_123").Return(nil)

	// Process the deletion
	err := bridge.HandleSignalMessageDeletion(ctx, targetMessageID, sender)
	assert.NoError(t, err)

	// Verify that the correct methods were called
	bridge.db.(*mockDatabaseService).AssertCalled(t, "GetMessageMappingBySignalID", ctx, targetMessageID)
	bridge.waClient.(*mockWhatsAppClient).AssertCalled(t, "DeleteMessage", ctx, "+1234567890@c.us", "wa_msg_123")
}

func TestHandleSignalMessageDeletionNoMapping(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Test: Signal message deletion with no mapping should return error
	targetMessageID := "nonexistent_msg_123"
	sender := "+1234567890"

	// Mock database to return no mapping
	bridge.db.(*mockDatabaseService).On("GetMessageMappingBySignalID", ctx, targetMessageID).Return(nil, nil)

	// Process the deletion
	err := bridge.HandleSignalMessageDeletion(ctx, targetMessageID, sender)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no mapping found for deletion target message")

	// Verify database was queried but WhatsApp delete was not called
	bridge.db.(*mockDatabaseService).AssertCalled(t, "GetMessageMappingBySignalID", ctx, targetMessageID)
	bridge.waClient.(*mockWhatsAppClient).AssertNotCalled(t, "DeleteMessage")
}

func TestHandleSignalMessageDeletionWhatsAppError(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Test: WhatsApp deletion error should be propagated
	targetMessageID := "target_msg_456"
	sender := "+1234567890"

	// Mock database to return message mapping
	mapping := &models.MessageMapping{
		WhatsAppChatID: "+1234567890@c.us",
		WhatsAppMsgID:  "wa_msg_456",
		SignalMsgID:    targetMessageID,
		ForwardedAt:    time.Now().Add(-5 * time.Minute),
	}
	bridge.db.(*mockDatabaseService).On("GetMessageMappingBySignalID", ctx, targetMessageID).Return(mapping, nil)

	// Mock WhatsApp client to return error
	bridge.waClient.(*mockWhatsAppClient).On("DeleteMessage", ctx, "+1234567890@c.us", "wa_msg_456").Return(assert.AnError)

	// Process the deletion
	err := bridge.HandleSignalMessageDeletion(ctx, targetMessageID, sender)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete message in WhatsApp")

	// Verify both methods were called
	bridge.db.(*mockDatabaseService).AssertCalled(t, "GetMessageMappingBySignalID", ctx, targetMessageID)
	bridge.waClient.(*mockWhatsAppClient).AssertCalled(t, "DeleteMessage", ctx, "+1234567890@c.us", "wa_msg_456")
}

func TestHandleSignalDeletion(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Test: Signal deletion message should trigger deletion handling
	msg := &signaltypes.SignalMessage{
		MessageID: "deletion_msg_123",
		Sender:    "+1234567890",
		Deletion: &signaltypes.SignalDeletion{
			TargetMessageID: "target_msg_789",
			TargetTimestamp: 1234567890000,
		},
	}

	// Mock database to return message mapping
	mapping := &models.MessageMapping{
		WhatsAppChatID: "+1234567890@c.us",
		WhatsAppMsgID:  "wa_msg_789",
		SignalMsgID:    "target_msg_789",
		ForwardedAt:    time.Now().Add(-10 * time.Minute),
	}
	bridge.db.(*mockDatabaseService).On("GetMessageMappingBySignalID", ctx, "target_msg_789").Return(mapping, nil)

	// Mock WhatsApp client to handle deletion
	bridge.waClient.(*mockWhatsAppClient).On("DeleteMessage", ctx, "+1234567890@c.us", "wa_msg_789").Return(nil)

	// Process the Signal message with deletion
	err := bridge.HandleSignalMessage(ctx, msg)
	assert.NoError(t, err)

	// Verify that deletion was processed
	bridge.db.(*mockDatabaseService).AssertCalled(t, "GetMessageMappingBySignalID", ctx, "target_msg_789")
	bridge.waClient.(*mockWhatsAppClient).AssertCalled(t, "DeleteMessage", ctx, "+1234567890@c.us", "wa_msg_789")
}

func TestHandleSignalReactionLegacy(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	msg := &signaltypes.SignalMessage{
		MessageID: "msg123",
		Sender:    "sender123",
		Reaction:  &signaltypes.SignalReaction{Emoji: "👍"},
	}

	err := bridge.handleSignalReaction(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handleSignalReaction called without session context")
}

func TestHandleSignalDeletionLegacy(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	msg := &signaltypes.SignalMessage{
		MessageID: "msg123",
		Sender:    "sender123",
		Deletion:  &signaltypes.SignalDeletion{TargetMessageID: "target123"},
	}

	err := bridge.handleSignalDeletion(ctx, msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handleSignalDeletion called without session context")
}

func TestSendSignalNotificationForSession(t *testing.T) {
	// Test the method by testing its parts separately since we can't easily mock the ChannelManager
	// This tests the behavior rather than the exact implementation

	t.Run("test channel manager integration", func(t *testing.T) {
		// Test that the method exists and works with valid data
		bridge, _, cleanup := setupTestBridge(t)
		defer cleanup()

		ctx := context.Background()

		// The setupTestBridge creates a channel manager with "default" session -> "+1234567890"
		// So we can test with this valid session
		bridge.sigClient.(*mockSignalClient).sendMessageResponse = &signaltypes.SendMessageResponse{
			MessageID: "notif123",
			Timestamp: time.Now().UnixMilli(),
		}

		err := bridge.SendSignalNotificationForSession(ctx, "default", "Test notification")
		assert.NoError(t, err)
	})

	t.Run("invalid session should error", func(t *testing.T) {
		bridge, _, cleanup := setupTestBridge(t)
		defer cleanup()

		ctx := context.Background()

		err := bridge.SendSignalNotificationForSession(ctx, "invalid_session", "Test notification")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get Signal destination")
	})

	t.Run("signal send error should propagate", func(t *testing.T) {
		bridge, _, cleanup := setupTestBridge(t)
		defer cleanup()

		ctx := context.Background()

		// Set up signal client to return error
		bridge.sigClient.(*mockSignalClient).sendMessageErr = assert.AnError

		err := bridge.SendSignalNotificationForSession(ctx, "default", "Test notification")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send Signal notification")
	})
}

func TestBridge_HandleWhatsAppMessage_GroupMessage(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	// Create a mock group service
	mockGroupService := new(mockGroupService)
	bridge.groupService = mockGroupService

	// Create a mock contact service
	mockContactService := new(mockContactService)
	bridge.contactService = mockContactService

	ctx := context.Background()

	t.Run("Group message with GroupService enabled", func(t *testing.T) {
		// Setup mocks
		mockContactService.On("GetContactDisplayName", ctx, "1234567890").Return("John Doe")
		mockGroupService.On("GetGroupName", ctx, "group123@g.us", "default").Return("Family Group")

		bridge.sigClient.(*mockSignalClient).sendMessageResponse = &signaltypes.SendMessageResponse{
			MessageID: "sig-msg-123",
			Timestamp: time.Now().Unix() * 1000,
		}
		bridge.sigClient.(*mockSignalClient).sendMessageErr = nil

		bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.AnythingOfType("*models.MessageMapping")).Return(nil)

		// Call with group chat ID
		err := bridge.HandleWhatsAppMessageWithSession(ctx, "default", "group123@g.us", "wa-msg-123", "1234567890@c.us", "", "Hello everyone", "")

		assert.NoError(t, err)
		mockContactService.AssertExpectations(t)
		mockGroupService.AssertExpectations(t)

		// Verify the message sent to Signal has the correct format
		mockSigClient := bridge.sigClient.(*mockSignalClient)
		assert.Contains(t, mockSigClient.lastMessage, "John Doe in Family Group:")
		assert.Contains(t, mockSigClient.lastMessage, "Hello everyone")
	})

	t.Run("Group message without GroupService (nil)", func(t *testing.T) {
		// Disable group service
		bridge.groupService = nil

		mockContactService.On("GetContactDisplayName", ctx, "9876543210").Return("Jane Smith")

		bridge.sigClient.(*mockSignalClient).sendMessageResponse = &signaltypes.SendMessageResponse{
			MessageID: "sig-msg-456",
			Timestamp: time.Now().Unix() * 1000,
		}
		bridge.sigClient.(*mockSignalClient).sendMessageErr = nil
		bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.AnythingOfType("*models.MessageMapping")).Return(nil)

		// Call with group chat ID but no group service
		err := bridge.HandleWhatsAppMessageWithSession(ctx, "default", "group456@g.us", "wa-msg-456", "9876543210@c.us", "", "Hi there", "")

		assert.NoError(t, err)
		mockContactService.AssertExpectations(t)

		// Verify the message sent to Signal falls back to direct message format
		mockSigClient := bridge.sigClient.(*mockSignalClient)
		assert.Contains(t, mockSigClient.lastMessage, "Jane Smith:")
		assert.NotContains(t, mockSigClient.lastMessage, " in ")
	})

	t.Run("Direct message still works normally", func(t *testing.T) {
		// Re-enable group service
		bridge.groupService = mockGroupService

		mockContactService.On("GetContactDisplayName", ctx, "5555555555").Return("Bob Wilson")

		bridge.sigClient.(*mockSignalClient).sendMessageResponse = &signaltypes.SendMessageResponse{
			MessageID: "sig-msg-789",
			Timestamp: time.Now().Unix() * 1000,
		}
		bridge.sigClient.(*mockSignalClient).sendMessageErr = nil
		bridge.db.(*mockDatabaseService).On("SaveMessageMapping", ctx, mock.AnythingOfType("*models.MessageMapping")).Return(nil)

		// Call with direct chat ID (not a group)
		err := bridge.HandleWhatsAppMessageWithSession(ctx, "default", "5555555555@c.us", "wa-msg-789", "5555555555@c.us", "", "Direct message", "")

		assert.NoError(t, err)
		mockContactService.AssertExpectations(t)

		// GroupService should NOT be called for direct messages
		// Verify the message sent to Signal uses direct message format
		mockSigClient := bridge.sigClient.(*mockSignalClient)
		assert.Equal(t, "Bob Wilson: Direct message", mockSigClient.lastMessage)
		assert.NotContains(t, mockSigClient.lastMessage, " in ")
	})
}

func TestExtractMappingFromQuotedText(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	tests := []struct {
		name       string
		quotedText string
		wantChatID string
		wantNil    bool
	}{
		{
			name:       "valid phone with emoji prefix",
			quotedText: "📱 15551234567: Hello there",
			wantChatID: "15551234567@c.us",
			wantNil:    false,
		},
		{
			name:       "valid phone without prefix",
			quotedText: "15559876543: Some message content",
			wantChatID: "15559876543@c.us",
			wantNil:    false,
		},
		{
			name:       "valid phone with country code",
			quotedText: "+1 555 123 4567: Message here",
			wantChatID: "15551234567@c.us",
			wantNil:    false,
		},
		{
			name:       "no colon separator",
			quotedText: "This has no separator",
			wantChatID: "",
			wantNil:    true,
		},
		{
			name:       "too short phone number",
			quotedText: "📱 123: Short",
			wantChatID: "",
			wantNil:    true,
		},
		{
			name:       "no digits at all",
			quotedText: "NoDigits: Some message",
			wantChatID: "",
			wantNil:    true,
		},
		{
			name:       "empty string",
			quotedText: "",
			wantChatID: "",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bridge.extractMappingFromQuotedText(tt.quotedText)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.wantChatID, result.WhatsAppChatID)
			}
		})
	}
}

func TestIsRetryableSignalError(t *testing.T) {
	tests := []struct {
		name      string
		errMsg    string
		wantRetry bool
	}{
		{"Untrusted Identity", `signal API error: status 400, body: {"error":"Untrusted Identity for \"+15555550100\""}`, false},
		{"Rate limit", `signal API error: status 429, body: {"error":"Rate limit exceeded"}`, false},
		{"Unregistered user", `signal API error: status 400, body: {"error":"Unregistered user"}`, false},
		{"Invalid phone number", `signal API error: status 400, body: {"error":"Invalid phone number format"}`, false},
		{"Forbidden", `signal API error: status 403, body: {"error":"Forbidden"}`, false},
		{"Not found", `signal API error: status 404, body: {"error":"Not found"}`, false},
		{"Connection refused", `dial tcp 127.0.0.1:8080: connect: connection refused`, true},
		{"Timeout", `context deadline exceeded`, true},
		{"Generic server error", `signal API error: status 500, body: {"error":"Internal server error"}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &testError{msg: tt.errMsg}
			result := isRetryableSignalError(err)
			assert.Equal(t, tt.wantRetry, result, "isRetryableSignalError(%q) = %v, want %v", tt.errMsg, result, tt.wantRetry)
		})
	}

	// Test nil case
	t.Run("nil error", func(t *testing.T) {
		result := isRetryableSignalError(nil)
		assert.False(t, result, "nil error should return false")
	})
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestIsRetryableWhatsAppError(t *testing.T) {
	tests := []struct {
		name      string
		errMsg    string
		wantRetry bool
	}{
		{"Status 400 - Bad request", `request failed with status 400: {"error":"invalid chatId"}`, false},
		{"Status 401 - Unauthorized", `request failed with status 401: {"error":"unauthorized"}`, false},
		{"Status 403 - Forbidden", `request failed with status 403: {"error":"forbidden"}`, false},
		{"Status 404 - Not found", `request failed with status 404: {"error":"chat not found"}`, false},
		{"Invalid chat", `request failed: invalid chat ID format`, false},
		{"Not registered", `request failed: user not registered on WhatsApp`, false},
		{"Blocked user", `request failed: user blocked`, false},
		{"Session not found", `session not found`, false},
		{"Session is not ready", `session is not ready`, false},
		{"Status 500 - Internal error", `request failed with status 500: {"error":"internal server error"}`, true},
		{"Status 500 - markedUnread", `request failed with status 500: Cannot read properties of undefined (reading 'markedUnread')`, true},
		{"Status 502 - Bad Gateway", `request failed with status 502: bad gateway`, true},
		{"Status 503 - Service Unavailable", `request failed with status 503: service unavailable`, true},
		{"Status 504 - Gateway Timeout", `request failed with status 504: gateway timeout`, true},
		{"Timeout error", `context deadline exceeded (timeout)`, true},
		{"Network error", `dial tcp: network unreachable`, true},
		{"Connection refused", `dial tcp 127.0.0.1:8080: connect: connection refused`, true},
		{"Generic unknown error", `some unknown error occurred`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &testError{msg: tt.errMsg}
			result := isRetryableWhatsAppError(err)
			assert.Equal(t, tt.wantRetry, result, "isRetryableWhatsAppError(%q) = %v, want %v", tt.errMsg, result, tt.wantRetry)
		})
	}

	t.Run("nil error", func(t *testing.T) {
		result := isRetryableWhatsAppError(nil)
		assert.False(t, result, "nil error should return false")
	})
}

func TestSendMessageToWhatsApp_RetriesOnTransientError(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()
	callCount := 0

	mockWA := &mockWhatsAppClient{
		sendTextFunc: func(ctx context.Context, chatID, text string) (*types.SendMessageResponse, error) {
			callCount++
			if callCount < 3 {
				return nil, &testError{msg: "request failed with status 500: markedUnread error"}
			}
			return &types.SendMessageResponse{
				MessageID: "wa_msg_success",
				Status:    "sent",
			}, nil
		},
	}
	bridge.waClient = mockWA

	resp, err := bridge.sendMessageToWhatsApp(ctx, "123456789@g.us", "Test message", nil, "", "default")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "wa_msg_success", resp.MessageID)
	assert.Equal(t, 3, callCount, "Expected 3 attempts (2 failures + 1 success)")
}

func TestSendMessageToWhatsApp_FailsOnNonRetryableError(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()
	callCount := 0

	mockWA := &mockWhatsAppClient{
		sendTextFunc: func(ctx context.Context, chatID, text string) (*types.SendMessageResponse, error) {
			callCount++
			return nil, &testError{msg: "request failed with status 404: chat not found"}
		},
	}
	bridge.waClient = mockWA

	resp, err := bridge.sendMessageToWhatsApp(ctx, "123456789@c.us", "Test message", nil, "", "default")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "non-retryable")
	assert.Equal(t, 1, callCount, "Should fail immediately on non-retryable error")
}

func TestSendMessageToWhatsApp_FailsAfterMaxRetries(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()
	callCount := 0

	mockWA := &mockWhatsAppClient{
		sendTextFunc: func(ctx context.Context, chatID, text string) (*types.SendMessageResponse, error) {
			callCount++
			return nil, &testError{msg: "request failed with status 500: internal server error"}
		},
	}
	bridge.waClient = mockWA

	resp, err := bridge.sendMessageToWhatsApp(ctx, "123456789@c.us", "Test message", nil, "", "default")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "after retries")
	assert.Equal(t, 3, callCount, "Should retry MaxAttempts times (configured as 3)")
}

func TestSendMessageToWhatsApp_EmptyMessage(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	resp, err := bridge.sendMessageToWhatsApp(ctx, "123456789@c.us", "   ", nil, "", "default")

	assert.NoError(t, err)
	assert.Nil(t, resp, "Empty message should return nil response")
}

func TestResolveGroupMessageMapping_WithQuotedMessage(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Setup: quoted message mapping pointing to a WhatsApp group chat
	expectedMapping := &models.MessageMapping{
		WhatsAppChatID: "group123@g.us",
		WhatsAppMsgID:  "wa_quoted_msg",
		SignalMsgID:    "sig_quoted_msg",
		ForwardedAt:    time.Now(),
	}

	bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, "quoted_msg_id").Return(expectedMapping, nil).Once()

	msg := &signaltypes.SignalMessage{
		MessageID: "sig_reply_1",
		Sender:    "group.123",
		Message:   "Group reply",
		Timestamp: time.Now().UnixMilli(),
		QuotedMessage: &struct {
			ID        string `json:"id"`
			Author    string `json:"author"`
			Text      string `json:"text"`
			Timestamp int64  `json:"timestamp"`
		}{
			ID: "quoted_msg_id",
		},
	}

	mapping, usedFallback, err := bridge.resolveGroupMessageMapping(ctx, msg, "default")

	assert.NoError(t, err)
	assert.NotNil(t, mapping)
	assert.False(t, usedFallback, "Should not use fallback when quoted message is provided")
	assert.Equal(t, "group123@g.us", mapping.WhatsAppChatID)
}

func TestResolveGroupMessageMapping_FallbackToLatestGroup(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Setup: no quoted message, fallback to latest group mapping
	fallbackMapping := &models.MessageMapping{
		WhatsAppChatID: "latestgroup@g.us",
		WhatsAppMsgID:  "wa_latest_msg",
		SignalMsgID:    "sig_latest_msg",
		ForwardedAt:    time.Now(),
	}

	bridge.db.(*mockDatabaseService).On("GetLatestGroupMessageMappingBySession", ctx, "default", 25).Return(fallbackMapping, nil).Once()

	// Message without quoted message
	msg := &signaltypes.SignalMessage{
		MessageID: "sig_msg_1",
		Sender:    "group.123",
		Message:   "Group message without quote",
		Timestamp: time.Now().UnixMilli(),
	}

	mapping, usedFallback, err := bridge.resolveGroupMessageMapping(ctx, msg, "default")

	assert.NoError(t, err)
	assert.NotNil(t, mapping)
	assert.True(t, usedFallback, "Should use fallback when no quoted message is provided")
	assert.Equal(t, "latestgroup@g.us", mapping.WhatsAppChatID)
}

func TestResolveGroupMessageMapping_NoGroupContext(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Setup: no quoted message and no group history
	bridge.db.(*mockDatabaseService).On("GetLatestGroupMessageMappingBySession", ctx, "default", 25).Return(nil, nil).Once()

	msg := &signaltypes.SignalMessage{
		MessageID: "sig_msg_1",
		Sender:    "group.123",
		Message:   "Group message with no context",
		Timestamp: time.Now().UnixMilli(),
	}

	mapping, usedFallback, err := bridge.resolveGroupMessageMapping(ctx, msg, "default")

	assert.Error(t, err)
	assert.Nil(t, mapping)
	assert.True(t, usedFallback, "Should indicate fallback was attempted")
	assert.Contains(t, err.Error(), "no group context")
	assert.Contains(t, err.Error(), "quote a group message")
}

func TestResolveGroupMessageMapping_QuotedMessageNotFound(t *testing.T) {
	bridge, _, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Setup: quoted message not found in database
	bridge.db.(*mockDatabaseService).On("GetMessageMapping", ctx, "nonexistent_msg").Return(nil, nil).Once()

	msg := &signaltypes.SignalMessage{
		MessageID: "sig_msg_1",
		Sender:    "group.123",
		Message:   "Reply to old message",
		Timestamp: time.Now().UnixMilli(),
		QuotedMessage: &struct {
			ID        string `json:"id"`
			Author    string `json:"author"`
			Text      string `json:"text"`
			Timestamp int64  `json:"timestamp"`
		}{
			ID: "nonexistent_msg",
		},
	}

	mapping, usedFallback, err := bridge.resolveGroupMessageMapping(ctx, msg, "default")

	assert.Error(t, err)
	assert.Nil(t, mapping)
	assert.False(t, usedFallback, "Should not indicate fallback when explicit quote fails")
	assert.Contains(t, err.Error(), "no mapping found for quoted message")
	assert.Contains(t, err.Error(), "try quoting a more recent message")
}
