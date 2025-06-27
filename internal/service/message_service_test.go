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

func (m *mockBridge) HandleWhatsAppMessageWithSession(ctx context.Context, sessionName, chatID, msgID, sender, content string, mediaPath string) error {
	args := m.Called(ctx, sessionName, chatID, msgID, sender, content, mediaPath)
	return args.Error(0)
}

func (m *mockBridge) HandleSignalMessage(ctx context.Context, msg *signaltypes.SignalMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *mockBridge) HandleSignalMessageWithDestination(ctx context.Context, msg *signaltypes.SignalMessage, destination string) error {
	args := m.Called(ctx, msg, destination)
	return args.Error(0)
}

func (m *mockBridge) SendSignalNotificationForSession(ctx context.Context, sessionName, message string) error {
	args := m.Called(ctx, sessionName, message)
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

func (m *mockDB) HasMessageHistoryBetween(ctx context.Context, sessionName, signalSender string) (bool, error) {
	args := m.Called(ctx, sessionName, signalSender)
	return args.Bool(0), args.Error(1)
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
	channelManager, _ := NewChannelManager([]models.Channel{
		{
			WhatsAppSessionName:          "default",
			SignalDestinationPhoneNumber: "+1234567890",
		},
	})
	service := NewMessageService(bridge, db, mediaCache, signalClient, signalConfig, channelManager)
	require.NotNil(t, service)
	return service, ctx
}

func createTestMessageService(bridge *mockBridge, db *mockDB, mediaCache *mockMediaCache) MessageService {
	signalClient := &mockSignalClient{}
	signalConfig := models.SignalConfig{
		PollIntervalSec: 5,
		PollTimeoutSec: 10,
	}
	channelManager, _ := NewChannelManager([]models.Channel{
		{
			WhatsAppSessionName:          "default",
			SignalDestinationPhoneNumber: "+1234567890",
		},
	})
	return NewMessageService(bridge, db, mediaCache, signalClient, signalConfig, channelManager)
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

func TestMessageService_HandleWhatsAppMessageWithSession(t *testing.T) {
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
				bridge.On("HandleWhatsAppMessageWithSession", ctx, "default", "chat123", "msg123", "sender123", "Hello, World!", "").Return(nil).Once()
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
				bridge.On("HandleWhatsAppMessageWithSession", ctx, "default", "chat124", "msg124", "sender123", "Check this out!", "http://example.com/image.jpg").Return(nil).Once()
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
			err := service.HandleWhatsAppMessageWithSession(ctx, "default", tt.chatID, tt.msgID, tt.sender, tt.content, tt.mediaPath)
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
	channelManager, _ := NewChannelManager([]models.Channel{
		{
			WhatsAppSessionName:          "default",
			SignalDestinationPhoneNumber: "+1234567890",
		},
	})
	service := NewMessageService(bridge, db, mediaCache, signalClient, signalConfig, channelManager)

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
				bridge.On("HandleSignalMessageWithDestination", mock.AnythingOfType("context.backgroundCtx"), mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
					return msg.Sender == "+1234567890" || msg.Sender == "+0987654321"
				}), mock.AnythingOfType("string")).Return(nil).Twice()
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
			channelManager, _ := NewChannelManager([]models.Channel{
				{
					WhatsAppSessionName:          "default",
					SignalDestinationPhoneNumber: "+1234567890",
				},
			})
			service := NewMessageService(bridge, db, mediaCache, signalClient, signalConfig, channelManager)

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

func TestPollSignalMessages_MultiChannel(t *testing.T) {
	tests := []struct {
		name         string
		messages     []signaltypes.SignalMessage
		setupHistory func(*mockDB)
		expectations func(*mockBridge)
		description  string
	}{
		{
			name: "multi-channel polling with message history routing",
			messages: []signaltypes.SignalMessage{
				{
					MessageID: "sig1",
					Sender:    "+9999999999", // External sender with personal session history
					Message:   "Hello from personal contact",
					Timestamp: time.Now().UnixMilli(),
				},
				{
					MessageID: "sig2", 
					Sender:    "+8888888888", // External sender with business session history
					Message:   "Hello from business contact",
					Timestamp: time.Now().UnixMilli(),
				},
			},
			setupHistory: func(db *mockDB) {
				ctx := context.Background()
				// Set up history expectations. Due to non-deterministic map iteration,
				// we need to handle both possible session orderings:
				
				// +9999999999 has history with personal only
				db.On("HasMessageHistoryBetween", ctx, "personal", "+9999999999").Return(true, nil).Maybe()
				db.On("HasMessageHistoryBetween", ctx, "business", "+9999999999").Return(false, nil).Maybe()
				
				// +8888888888 has history with business only  
				db.On("HasMessageHistoryBetween", ctx, "personal", "+8888888888").Return(false, nil).Maybe()
				db.On("HasMessageHistoryBetween", ctx, "business", "+8888888888").Return(true, nil).Maybe()
			},
			expectations: func(bridge *mockBridge) {
				ctx := context.Background()
				// First message should route to personal destination
				bridge.On("HandleSignalMessageWithDestination", ctx, mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
					return msg.MessageID == "sig1" && msg.Sender == "+9999999999"
				}), "+1111111111").Return(nil)
				
				// Second message should route to business destination
				bridge.On("HandleSignalMessageWithDestination", ctx, mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
					return msg.MessageID == "sig2" && msg.Sender == "+8888888888"
				}), "+2222222222").Return(nil)
			},
			description: "Messages should route to correct destinations based on message history",
		},
		{
			name: "multi-channel polling with unknown sender",
			messages: []signaltypes.SignalMessage{
				{
					MessageID: "sig3",
					Sender:    "+3333333333", // No history with any session
					Message:   "Hello from unknown contact",
					Timestamp: time.Now().UnixMilli(),
				},
			},
			setupHistory: func(db *mockDB) {
				ctx := context.Background()
				// Unknown contact has no history with any session
				db.On("HasMessageHistoryBetween", ctx, "personal", "+3333333333").Return(false, nil)
				db.On("HasMessageHistoryBetween", ctx, "business", "+3333333333").Return(false, nil)
			},
			expectations: func(bridge *mockBridge) {
				// No bridge calls should be made for unknown senders
			},
			description: "Messages from unknown senders should be skipped",
		},
		{
			name: "single channel polling (legacy behavior)", 
			messages: []signaltypes.SignalMessage{
				{
					MessageID: "sig4",
					Sender:    "+4444444444",
					Message:   "Single channel message",
					Timestamp: time.Now().UnixMilli(),
				},
			},
			setupHistory: func(db *mockDB) {
				// No history calls needed for single channel
			},
			expectations: func(bridge *mockBridge) {
				ctx := context.Background()
				bridge.On("HandleSignalMessageWithDestination", ctx, mock.MatchedBy(func(msg *signaltypes.SignalMessage) bool {
					return msg.MessageID == "sig4" && msg.Sender == "+4444444444"
				}), "+1234567890").Return(nil)
			},
			description: "Single channel should use first destination directly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := new(mockBridge)
			db := new(mockDB)
			mediaCache := new(mockMediaCache)
			signalClient := &mockSignalClient{}

			// Setup channels based on test case
			var channels []models.Channel
			if tt.name == "single channel polling (legacy behavior)" {
				channels = []models.Channel{
					{
						WhatsAppSessionName:          "default",
						SignalDestinationPhoneNumber: "+1234567890",
					},
				}
			} else {
				channels = []models.Channel{
					{
						WhatsAppSessionName:          "personal",
						SignalDestinationPhoneNumber: "+1111111111",
					},
					{
						WhatsAppSessionName:          "business", 
						SignalDestinationPhoneNumber: "+2222222222",
					},
				}
			}

			signalConfig := models.SignalConfig{
				PollIntervalSec: 5,
				PollTimeoutSec:  10,
			}
			channelManager, _ := NewChannelManager(channels)
			service := NewMessageService(bridge, db, mediaCache, signalClient, signalConfig, channelManager)

			// Setup mocks
			signalClient.On("ReceiveMessages", mock.Anything, 10).Return(tt.messages, nil)
			tt.setupHistory(db)
			tt.expectations(bridge)

			ctx := context.Background()
			err := service.PollSignalMessages(ctx)

			assert.NoError(t, err, tt.description)
			bridge.AssertExpectations(t)
			signalClient.AssertExpectations(t)
			db.AssertExpectations(t)
		})
	}
}

func TestDetermineDestinationForSender(t *testing.T) {
	channels := []models.Channel{
		{
			WhatsAppSessionName:          "personal",
			SignalDestinationPhoneNumber: "+1111111111",
		},
		{
			WhatsAppSessionName:          "business",
			SignalDestinationPhoneNumber: "+2222222222",
		},
	}

	tests := []struct {
		name                   string
		sender                 string
		availableDestinations  []string
		setupHistory          func(*mockDB)
		expectedDestination   string
		description           string
	}{
		{
			name:                  "sender matches configured destination",
			sender:                "+1111111111",
			availableDestinations: []string{"+1111111111", "+2222222222"},
			setupHistory: func(db *mockDB) {
				// No history setup needed - sender matches destination directly
			},
			expectedDestination: "+1111111111",
			description:         "Should return matching destination when sender is a configured destination",
		},
		{
			name:                  "sender with personal history",
			sender:                "+9999999999", // Different from destinations
			availableDestinations: []string{"+1111111111", "+2222222222"},
			setupHistory: func(db *mockDB) {
				ctx := context.Background()
				// Due to non-deterministic map iteration, either session could be checked first
				db.On("HasMessageHistoryBetween", ctx, "personal", "+9999999999").Return(true, nil).Maybe()
				db.On("HasMessageHistoryBetween", ctx, "business", "+9999999999").Return(false, nil).Maybe()
			},
			expectedDestination: "+1111111111",
			description:         "Should return personal destination for sender with personal history",
		},
		{
			name:                  "sender with business history",
			sender:                "+8888888888", // Different from destinations
			availableDestinations: []string{"+1111111111", "+2222222222"},
			setupHistory: func(db *mockDB) {
				ctx := context.Background()
				// Due to non-deterministic map iteration, either session could be checked first
				db.On("HasMessageHistoryBetween", ctx, "personal", "+8888888888").Return(false, nil).Maybe()
				db.On("HasMessageHistoryBetween", ctx, "business", "+8888888888").Return(true, nil).Maybe()
			},
			expectedDestination: "+2222222222",
			description:         "Should return business destination for sender with business history",
		},
		{
			name:                  "sender with no history",
			sender:                "+3333333333",
			availableDestinations: []string{"+1111111111", "+2222222222"}, 
			setupHistory: func(db *mockDB) {
				ctx := context.Background()
				// Due to non-deterministic map iteration, either session could be checked first
				db.On("HasMessageHistoryBetween", ctx, "personal", "+3333333333").Return(false, nil).Maybe()
				db.On("HasMessageHistoryBetween", ctx, "business", "+3333333333").Return(false, nil).Maybe()
			},
			expectedDestination: "",
			description:         "Should return empty string for sender with no history",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh mocks for each test case
			bridge := new(mockBridge)
			db := new(mockDB)
			mediaCache := new(mockMediaCache)
			signalClient := &mockSignalClient{}
			
			signalConfig := models.SignalConfig{}
			channelManager, _ := NewChannelManager(channels)
			service := NewMessageService(bridge, db, mediaCache, signalClient, signalConfig, channelManager).(*messageService)
			
			tt.setupHistory(db)

			ctx := context.Background()
			result := service.determineDestinationForSender(ctx, tt.sender, tt.availableDestinations)

			assert.Equal(t, tt.expectedDestination, result, tt.description)
			db.AssertExpectations(t)
		})
	}
}
