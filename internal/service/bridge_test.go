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
	// Create a test media config
	mediaConfig := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Image:    []string{"jpg", "jpeg", "png"},
			Video:    []string{"mp4", "mov"},
			Document: []string{"pdf", "doc", "docx"},
			Voice:    []string{"ogg", "aac", "m4a", "oga"},
		},
	}

	// Create bridge without contact service for basic tests (contact service has its own tests)
	bridge := NewBridge(mockWAClient, mockSignalClient, mockDB, mediaHandler, retryConfig, mediaConfig, "+1234567890", nil).(*bridge)

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
				bridge.waClient.(*mockWhatsAppClient).On("SendReaction", ctx, "chat123", "wa_msg456", "👍").
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
			assert.Equal(t, tt.isImage, bridge.isImageAttachment(tt.path))
			assert.Equal(t, tt.isVideo, bridge.isVideoAttachment(tt.path))
			assert.Equal(t, tt.isVoice, bridge.isVoiceAttachment(tt.path))
			assert.Equal(t, tt.isDoc, bridge.isDocumentAttachment(tt.path))
			assert.Equal(t, tt.mediaType, bridge.getMediaType(tt.path))
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
				mockWA.On("SendReaction", ctx, tt.mapping.WhatsAppChatID, tt.mapping.WhatsAppMsgID, reaction).
					Return(resp, tt.reactionError).Once()
			}

			err := bridge.handleSignalReaction(ctx, tt.msg)

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

	destinationPhoneNumber := "+1234567890"
	mediaConfig := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Image:    []string{"jpg", "jpeg", "png"},
			Video:    []string{"mp4", "mov"},
			Document: []string{"pdf", "doc", "docx"},
			Voice:    []string{"ogg", "aac", "m4a"},
		},
	}
	// For constructor test, use nil contact service to keep test simple
	b := NewBridge(waClient, sigClient, db, mediaHandler, retryConfig, mediaConfig, destinationPhoneNumber, nil)
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

func TestHandleSignalVoiceRecordingWithoutExtension(t *testing.T) {
	bridge, tmpDir, cleanup := setupTestBridge(t)
	defer cleanup()

	ctx := context.Background()

	// Create a mock OGG voice recording file without extension (like Signal creates)
	voiceFile := filepath.Join(tmpDir, "signal_voice_recording")
	oggHeader := []byte("OggS") // OGG file signature
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
	bridge.db.(*mockDatabaseService).On("GetLatestMessageMapping", ctx).Return(latestMapping, nil)

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
	bridge.db.(*mockDatabaseService).AssertCalled(t, "GetLatestMessageMapping", ctx)
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
	bridge.db.(*mockDatabaseService).On("GetLatestMessageMapping", ctx).Return(nil, nil)

	// Process the Signal message - should call handleNewSignalThread (which currently logs and returns nil)
	err := bridge.HandleSignalMessage(ctx, msg)
	assert.NoError(t, err) // Should not error, just log and ignore

	// Verify that the auto-reply logic was attempted but found no history
	bridge.db.(*mockDatabaseService).AssertCalled(t, "GetLatestMessageMapping", ctx)
}
