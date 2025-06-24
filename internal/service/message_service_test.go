package service

import (
	"context"
	"testing"
	"time"

	"whatsignal/internal/models"
	signaltypes "whatsignal/pkg/signal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockBridge struct {
	mock.Mock
}

func (m *mockBridge) SendMessage(ctx context.Context, msg *models.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockBridge) HandleWhatsAppMessage(ctx context.Context, chatID, msgID, sender, content string, mediaPath string) error {
	args := m.Called(ctx, chatID, msgID, sender, content, mediaPath)
	return args.Error(0)
}

func (m *mockBridge) HandleSignalMessage(ctx context.Context, msg *signaltypes.SignalMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockBridge) HandleSignalMessageDeletion(ctx context.Context, targetMessageID string, sender string) error {
	args := m.Called(ctx, targetMessageID, sender)
	return args.Error(0)
}

func (m *mockBridge) UpdateDeliveryStatus(ctx context.Context, msgID string, status models.DeliveryStatus) error {
	args := m.Called(ctx, msgID, status)
	return args.Error(0)
}

func (m *mockBridge) CleanupOldRecords(ctx context.Context, retentionDays int) error {
	args := m.Called(ctx, retentionDays)
	return args.Error(0)
}

type mockDB struct {
	mock.Mock
}

func (m *mockDB) SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error {
	args := m.Called(ctx, mapping)
	return args.Error(0)
}

func (m *mockDB) GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDB) GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error) {
	args := m.Called(ctx, whatsappID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDB) GetMessageMappingBySignalID(ctx context.Context, signalID string) (*models.MessageMapping, error) {
	args := m.Called(ctx, signalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.MessageMapping), args.Error(1)
}

func (m *mockDB) UpdateDeliveryStatus(ctx context.Context, id string, status string) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

type mockMediaCache struct {
	mock.Mock
}

func (m *mockMediaCache) ProcessMedia(path string) (string, error) {
	args := m.Called(path)
	return args.String(0), args.Error(1)
}

func (m *mockMediaCache) CleanupOldFiles(maxAge int64) error {
	args := m.Called(maxAge)
	return args.Error(0)
}

func setupTestService(t *testing.T) (MessageService, context.Context) {
	ctx := context.Background()
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	signalClient := &mockSignalClient{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
		PollTimeoutSec: 10,
	}
	service := NewMessageService(bridge, db, mediaCache, signalClient, signalConfig)
	require.NotNil(t, service)
	return service, ctx
}

func createTestMessageService(bridge *mockBridge, db *mockDB, mediaCache *mockMediaCache) MessageService {
	signalClient := &mockSignalClient{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
		PollTimeoutSec: 10,
	}
	return NewMessageService(bridge, db, mediaCache, signalClient, signalConfig)
}

func TestNewMessageService(t *testing.T) {
	service, _ := setupTestService(t)
	assert.NotNil(t, service)
}

func TestSendMessage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		message   *models.Message
		wantError bool
		setup     func(bridge *mockBridge, db *mockDB, mediaCache *mockMediaCache)
	}{
		{
			name: "bridge error",
			message: &models.Message{
				ID:       "msg3",
				ChatID:   "chat3",
				Content:  "Error message",
				Platform: "whatsapp",
				Type:     models.TextMessage,
			},
			setup: func(bridge *mockBridge, db *mockDB, mediaCache *mockMediaCache) {
				bridge.On("SendMessage", ctx, mock.AnythingOfType("*models.Message")).Return(assert.AnError)
			},
			wantError: true,
		},
		{
			name: "successful text message",
			message: &models.Message{
				ID:        "msg1",
				ChatID:    "chat1",
				Content:   "Hello, World!",
				Platform:  "whatsapp",
				Type:      models.TextMessage,
				Timestamp: time.Now(),
			},
			setup: func(bridge *mockBridge, db *mockDB, mediaCache *mockMediaCache) {
				bridge.On("SendMessage", ctx, mock.AnythingOfType("*models.Message")).Return(nil)
				db.On("SaveMessageMapping", ctx, mock.AnythingOfType("*models.MessageMapping")).Return(nil)
			},
			wantError: false,
		},
		{
			name: "successful media message",
			message: &models.Message{
				ID:       "msg2",
				ChatID:   "chat2",
				Content:  "Check this out!",
				Platform: "whatsapp",
				Type:     models.ImageMessage,
				MediaURL: "http://example.com/image.jpg",
			},
			setup: func(bridge *mockBridge, db *mockDB, mediaCache *mockMediaCache) {
				mediaCache.On("ProcessMedia", "http://example.com/image.jpg").Return("/cache/image.jpg", nil)
				bridge.On("SendMessage", ctx, mock.AnythingOfType("*models.Message")).Return(nil)
				db.On("SaveMessageMapping", ctx, mock.AnythingOfType("*models.MessageMapping")).Return(nil)
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := new(mockBridge)
			db := new(mockDB)
			mediaCache := new(mockMediaCache)
			service := createTestMessageService(bridge, db, mediaCache)

			if tt.setup != nil {
				tt.setup(bridge, db, mediaCache)
			}

			err := service.SendMessage(ctx, tt.message)
			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to send message")
			} else {
				assert.NoError(t, err)
			}

			bridge.AssertExpectations(t)
			db.AssertExpectations(t)
			mediaCache.AssertExpectations(t)
		})
	}
}

func TestReceiveMessage(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	service := createTestMessageService(bridge, db, mediaCache)

	ctx := context.Background()

	tests := []struct {
		name      string
		message   *models.Message
		wantError bool
		setup     func()
	}{
		{
			name: "successful new message",
			message: &models.Message{
				ID:        "msg1",
				ChatID:    "chat1",
				Content:   "Hello, World!",
				Platform:  "whatsapp",
				Type:      models.TextMessage,
				Timestamp: time.Now(),
			},
			setup: func() {
				db.On("GetMessageMapping", ctx, "msg1").Return(nil, nil)
				bridge.On("SendMessage", ctx, mock.AnythingOfType("*models.Message")).Return(nil)
				db.On("SaveMessageMapping", ctx, mock.AnythingOfType("*models.MessageMapping")).Return(nil)
			},
		},
		{
			name: "duplicate message",
			message: &models.Message{
				ID:        "msg2",
				ChatID:    "chat2",
				Content:   "Duplicate message",
				Platform:  "whatsapp",
				Type:      models.TextMessage,
				Timestamp: time.Now(),
			},
			setup: func() {
				db.On("GetMessageMapping", ctx, "msg2").Return(&models.MessageMapping{
					WhatsAppMsgID:  "msg2",
					DeliveryStatus: "delivered",
				}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := service.ReceiveMessage(ctx, tt.message)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			bridge.AssertExpectations(t)
			db.AssertExpectations(t)
			mediaCache.AssertExpectations(t)
		})
	}
}

func TestGetMessageByID(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	service := createTestMessageService(bridge, db, mediaCache)

	ctx := context.Background()

	tests := []struct {
		name       string
		messageID  string
		wantError  bool
		wantResult *models.Message
		setup      func()
	}{
		{
			name:      "existing message",
			messageID: "msg1",
			setup: func() {
				db.On("GetMessageMapping", ctx, "msg1").Return(&models.MessageMapping{
					WhatsAppMsgID:   "msg1",
					WhatsAppChatID:  "chat1",
					SignalTimestamp: time.Now(),
					DeliveryStatus:  "delivered",
				}, nil)
			},
			wantResult: &models.Message{
				ID:             "msg1",
				ChatID:         "chat1",
				Type:           models.TextMessage,
				Platform:       "whatsapp",
				DeliveryStatus: "delivered",
			},
		},
		{
			name:      "non-existent message",
			messageID: "msg2",
			setup: func() {
				db.On("GetMessageMapping", ctx, "msg2").Return(nil, nil)
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			msg, err := service.GetMessageByID(ctx, tt.messageID)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, msg)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, msg)
				assert.Equal(t, tt.wantResult.ID, msg.ID)
				assert.Equal(t, tt.wantResult.ChatID, msg.ChatID)
				assert.Equal(t, tt.wantResult.Type, msg.Type)
				assert.Equal(t, tt.wantResult.Platform, msg.Platform)
				assert.Equal(t, tt.wantResult.DeliveryStatus, msg.DeliveryStatus)
			}
			db.AssertExpectations(t)
		})
	}
}

func TestGetMessageThread(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	service := createTestMessageService(bridge, db, mediaCache)
	ctx := context.Background()

	// Test getting non-existent thread
	db.On("GetMessageMapping", ctx, "nonexistent").Return(nil, assert.AnError)
	messages, err := service.GetMessageThread(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Nil(t, messages)
	db.AssertExpectations(t)
}

func TestMarkMessageDelivered(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	service := createTestMessageService(bridge, db, mediaCache)

	ctx := context.Background()

	tests := []struct {
		name      string
		messageID string
		wantError bool
		setup     func()
	}{
		{
			name:      "successful update",
			messageID: "msg1",
			setup: func() {
				db.On("UpdateDeliveryStatus", ctx, "msg1", "delivered").Return(nil)
			},
		},
		{
			name:      "non-existent message",
			messageID: "msg2",
			setup: func() {
				db.On("UpdateDeliveryStatus", ctx, "msg2", "delivered").Return(assert.AnError)
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := service.MarkMessageDelivered(ctx, tt.messageID)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			db.AssertExpectations(t)
		})
	}
}

func TestDeleteMessage(t *testing.T) {
	service, ctx := setupTestService(t)

	// Test deleting message
	err := service.DeleteMessage(ctx, "msg1")
	assert.NoError(t, err)
}

func TestMessageService_HandleWhatsAppMessage(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	service := createTestMessageService(bridge, db, mediaCache)

	ctx := context.Background()

	tests := []struct {
		name      string
		chatID    string
		msgID     string
		sender    string
		content   string
		mediaPath string
		wantError bool
		setup     func()
	}{
		{
			name:    "new text message",
			chatID:  "chat123",
			msgID:   "msg123",
			sender:  "sender123",
			content: "Hello, World!",
			setup: func() {
				// Check if message exists
				db.On("GetMessageMapping", ctx, "msg123").Return(nil, nil).Once()
				bridge.On("HandleWhatsAppMessage", ctx, "chat123", "msg123", "sender123", "Hello, World!", "").Return(nil).Once()
			},
		},
		{
			name:      "new media message",
			chatID:    "chat124",
			msgID:     "msg124",
			sender:    "sender123",
			content:   "Check this out!",
			mediaPath: "http://example.com/image.jpg",
			setup: func() {
				// Check if message exists
				db.On("GetMessageMapping", ctx, "msg124").Return(nil, nil).Once()
				bridge.On("HandleWhatsAppMessage", ctx, "chat124", "msg124", "sender123", "Check this out!", "http://example.com/image.jpg").Return(nil).Once()
			},
		},
		{
			name:    "duplicate message",
			chatID:  "chat125",
			msgID:   "msg125",
			sender:  "sender123",
			content: "Duplicate message",
			setup: func() {
				db.On("GetMessageMapping", ctx, "msg125").Return(&models.MessageMapping{
					WhatsAppMsgID:  "msg125",
					DeliveryStatus: "delivered",
				}, nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := service.HandleWhatsAppMessage(ctx, tt.chatID, tt.msgID, tt.sender, tt.content, tt.mediaPath)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			bridge.AssertExpectations(t)
			db.AssertExpectations(t)
			mediaCache.AssertExpectations(t)
		})
	}
}

func TestMessageService_HandleSignalMessageDetailed(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	service := createTestMessageService(bridge, db, mediaCache)

	ctx := context.Background()

	tests := []struct {
		name      string
		msg       *models.Message
		wantError bool
		setup     func()
	}{
		{
			name: "text message",
			msg: &models.Message{
				ID:        "sig123",
				Sender:    "sender123",
				Content:   "Hello, Signal!",
				Timestamp: time.UnixMilli(time.Now().UnixMilli()),
				Type:      models.TextMessage,
				Platform:  "signal",
			},
			setup: func() {
				bridge.On("SendMessage", ctx, mock.MatchedBy(func(msg *models.Message) bool {
					return msg.ID == "sig123" &&
						msg.Content == "Hello, Signal!" &&
						msg.Type == models.TextMessage
				})).Return(nil).Once()
			},
		},
		{
			name: "media message",
			msg: &models.Message{
				ID:        "sig124",
				Sender:    "sender123",
				Content:   "Check this out!",
				Timestamp: time.UnixMilli(time.Now().UnixMilli()),
				Type:      models.ImageMessage,
				Platform:  "signal",
				MediaURL:  "http://example.com/image.jpg",
			},
			setup: func() {
				mediaCache.On("ProcessMedia", "http://example.com/image.jpg").Return("/cache/image.jpg", nil).Once()
				bridge.On("SendMessage", ctx, mock.MatchedBy(func(msg *models.Message) bool {
					return msg.ID == "sig124" &&
						msg.Content == "Check this out!" &&
						msg.Type == models.ImageMessage &&
						msg.MediaPath == "/cache/image.jpg"
				})).Return(nil).Once()
			},
		},
		{
			name: "group message",
			msg: &models.Message{
				ID:        "sig125",
				Sender:    "group.123",
				Content:   "Group message",
				Timestamp: time.UnixMilli(time.Now().UnixMilli()),
				Type:      models.TextMessage,
				Platform:  "signal",
			},
			wantError: true,
			setup: func() {
				// No expectations needed as group messages are not supported yet
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			err := service.HandleSignalMessage(ctx, tt.msg)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			bridge.AssertExpectations(t)
			db.AssertExpectations(t)
			mediaCache.AssertExpectations(t)
		})
	}
}

func TestMessageService_UpdateDeliveryStatusDetailed(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	service := createTestMessageService(bridge, db, mediaCache)

	ctx := context.Background()

	tests := []struct {
		name      string
		msgID     string
		status    string
		wantError bool
		setup     func()
	}{
		{
			name:   "successful update",
			msgID:  "msg123",
			status: "delivered",
			setup: func() {
				db.On("UpdateDeliveryStatus", ctx, "msg123", "delivered").Return(nil).Once()
			},
		},
		{
			name:      "non-existent message",
			msgID:     "msg124",
			status:    "delivered",
			wantError: true,
			setup: func() {
				db.On("UpdateDeliveryStatus", ctx, "msg124", "delivered").Return(assert.AnError).Once()
			},
		},
		{
			name:      "invalid status",
			msgID:     "msg125",
			status:    "invalid",
			wantError: true,
			setup: func() {
				db.On("UpdateDeliveryStatus", ctx, "msg125", "invalid").Return(assert.AnError).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			err := service.UpdateDeliveryStatus(ctx, tt.msgID, tt.status)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			db.AssertExpectations(t)
		})
	}
}

func TestProcessIncomingSignalMessage(t *testing.T) {
	bridge := new(mockBridge)
	db := new(mockDB)
	mediaCache := new(mockMediaCache)
	signalClient := &mockSignalClient{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
		PollTimeoutSec: 10,
	}
	service := NewMessageService(bridge, db, mediaCache, signalClient, signalConfig)

	ctx := context.Background()

	tests := []struct {
		name      string
		msg       *signaltypes.SignalMessage
		wantError bool
		setup     func()
	}{
		{
			name: "successful processing",
			msg: &signaltypes.SignalMessage{
				MessageID: "sig123",
				Sender:    "+1234567890",
				Message:   "Hello from Signal!",
				Timestamp: time.Now().UnixMilli(),
			},
			setup: func() {
				bridge.On("HandleSignalMessage", ctx, mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
					return msg.MessageID == "sig123" && msg.Sender == "+1234567890"
				})).Return(nil).Once()
			},
		},
		{
			name: "bridge error",
			msg: &signaltypes.SignalMessage{
				MessageID: "sig124",
				Sender:    "+1234567890",
				Message:   "Error message",
				Timestamp: time.Now().UnixMilli(),
			},
			wantError: true,
			setup: func() {
				bridge.On("HandleSignalMessage", ctx, mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
					return msg.MessageID == "sig124" && msg.Sender == "+1234567890"
				})).Return(assert.AnError).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			err := service.ProcessIncomingSignalMessage(ctx, tt.msg)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			bridge.AssertExpectations(t)
		})
	}
}

func TestPollSignalMessages(t *testing.T) {
	tests := []struct {
		name      string
		messages  []signaltypes.SignalMessage
		pollError error
		wantError bool
		setup     func(*mockBridge, *mockSignalClient)
	}{
		{
			name: "successful polling with messages",
			messages: []signaltypes.SignalMessage{
				{
					MessageID: "sig123",
					Sender:    "+1234567890",
					Message:   "Hello from Signal!",
					Timestamp: time.Now().UnixMilli(),
				},
				{
					MessageID: "sig124",
					Sender:    "+0987654321",
					Message:   "Another message",
					Timestamp: time.Now().UnixMilli(),
				},
			},
			setup: func(bridge *mockBridge, signalClient *mockSignalClient) {
				signalClient.On("ReceiveMessages", mock.AnythingOfType("context.backgroundCtx"), 10).Return([]signaltypes.SignalMessage{
					{
						MessageID: "sig123",
						Sender:    "+1234567890",
						Message:   "Hello from Signal!",
						Timestamp: time.Now().UnixMilli(),
					},
					{
						MessageID: "sig124",
						Sender:    "+0987654321",
						Message:   "Another message",
						Timestamp: time.Now().UnixMilli(),
					},
				}, nil).Once()
				bridge.On("HandleSignalMessage", mock.AnythingOfType("context.backgroundCtx"), mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
					return msg.Sender == "+1234567890" || msg.Sender == "+0987654321"
				})).Return(nil).Twice()
			},
		},
		{
			name:      "polling error",
			pollError: assert.AnError,
			wantError: true,
			setup: func(bridge *mockBridge, signalClient *mockSignalClient) {
				signalClient.On("ReceiveMessages", mock.AnythingOfType("context.backgroundCtx"), 10).Return(nil, assert.AnError).Once()
			},
		},
		{
			name: "no messages",
			messages: []signaltypes.SignalMessage{},
			setup: func(bridge *mockBridge, signalClient *mockSignalClient) {
				signalClient.On("ReceiveMessages", mock.AnythingOfType("context.backgroundCtx"), 10).Return([]signaltypes.SignalMessage{}, nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := new(mockBridge)
			db := new(mockDB)
			mediaCache := new(mockMediaCache)
			signalClient := &mockSignalClient{}
			signalConfig := models.SignalConfig{
				PollIntervalSec: 5,
				PollTimeoutSec: 10,
			}
			service := NewMessageService(bridge, db, mediaCache, signalClient, signalConfig)

			if tt.setup != nil {
				tt.setup(bridge, signalClient)
			}

			ctx := context.Background()
			err := service.PollSignalMessages(ctx)

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to poll Signal messages")
			} else {
				assert.NoError(t, err)
			}

			bridge.AssertExpectations(t)
			signalClient.AssertExpectations(t)
		})
	}
}
