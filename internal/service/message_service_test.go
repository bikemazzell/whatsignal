package service

import (
	"context"
	"testing"
	"time"

	"whatsignal/internal/models"

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
	service := NewMessageService(bridge, db, mediaCache)
	require.NotNil(t, service)
	return service, ctx
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
			service := NewMessageService(bridge, db, mediaCache)

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
	service := NewMessageService(bridge, db, mediaCache)

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
				db.On("GetMessageMappingByWhatsAppID", ctx, "msg1").Return(nil, nil)
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
				db.On("GetMessageMappingByWhatsAppID", ctx, "msg2").Return(&models.MessageMapping{
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
	service := NewMessageService(bridge, db, mediaCache)

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
	service := NewMessageService(bridge, db, mediaCache)
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
	service := NewMessageService(bridge, db, mediaCache)

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
