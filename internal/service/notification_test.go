package service

import (
	"context"
	"testing"

	"whatsignal/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSendSignalNotification(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		message     string
		setupMocks  func(*mockBridge)
		expectError bool
	}{
		{
			name:        "successful notification",
			sessionName: "default",
			message:     "Test notification",
			setupMocks: func(bridge *mockBridge) {
				bridge.On("SendSignalNotificationForSession", mock.Anything, "default", "Test notification").Return(nil)
			},
			expectError: false,
		},
		{
			name:        "bridge error",
			sessionName: "business",
			message:     "Failed notification",
			setupMocks: func(bridge *mockBridge) {
				bridge.On("SendSignalNotificationForSession", mock.Anything, "business", "Failed notification").Return(assert.AnError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := new(mockBridge)
			db := new(mockDB)
			mediaCache := new(mockMediaCache)
			
			if tt.setupMocks != nil {
				tt.setupMocks(bridge)
			}

			service := createTestMessageService(bridge, db, mediaCache)
			ctx := context.Background()

			err := service.SendSignalNotification(ctx, tt.sessionName, tt.message)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			bridge.AssertExpectations(t)
		})
	}
}

func TestGetMessageMappingByWhatsAppID(t *testing.T) {
	tests := []struct {
		name         string
		whatsappID   string
		setupMocks   func(*mockDB)
		expectedData *models.MessageMapping
		expectError  bool
	}{
		{
			name:       "successful mapping retrieval",
			whatsappID: "wa-12345",
			setupMocks: func(db *mockDB) {
				mapping := &models.MessageMapping{
					WhatsAppMsgID:   "wa-12345",
					SignalMsgID:     "sig-67890",
					WhatsAppChatID:  "chat123",
					SessionName:     "default",
					DeliveryStatus:  models.DeliveryStatusSent,
				}
				db.On("GetMessageMappingByWhatsAppID", mock.Anything, "wa-12345").Return(mapping, nil)
			},
			expectedData: &models.MessageMapping{
				WhatsAppMsgID:   "wa-12345",
				SignalMsgID:     "sig-67890",
				WhatsAppChatID:  "chat123",
				SessionName:     "default",
				DeliveryStatus:  models.DeliveryStatusSent,
			},
			expectError: false,
		},
		{
			name:       "database error",
			whatsappID: "wa-invalid",
			setupMocks: func(db *mockDB) {
				db.On("GetMessageMappingByWhatsAppID", mock.Anything, "wa-invalid").Return(nil, assert.AnError)
			},
			expectedData: nil,
			expectError:  true,
		},
		{
			name:       "mapping not found",
			whatsappID: "wa-notfound",
			setupMocks: func(db *mockDB) {
				db.On("GetMessageMappingByWhatsAppID", mock.Anything, "wa-notfound").Return(nil, nil)
			},
			expectedData: nil,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := new(mockBridge)
			db := new(mockDB)
			mediaCache := new(mockMediaCache)
			
			if tt.setupMocks != nil {
				tt.setupMocks(db)
			}

			service := createTestMessageService(bridge, db, mediaCache)
			ctx := context.Background()

			result, err := service.GetMessageMappingByWhatsAppID(ctx, tt.whatsappID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				if tt.expectedData != nil {
					assert.Equal(t, tt.expectedData, result)
				} else {
					assert.Nil(t, result)
				}
			}

			db.AssertExpectations(t)
		})
	}
}

func TestChannelManagerIntegration(t *testing.T) {
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

	channelManager, err := NewChannelManager(channels)
	assert.NoError(t, err)
	assert.NotNil(t, channelManager)

	t.Run("GetSignalDestination", func(t *testing.T) {
		dest, err := channelManager.GetSignalDestination("personal")
		assert.NoError(t, err)
		assert.Equal(t, "+1111111111", dest)

		dest, err = channelManager.GetSignalDestination("business")
		assert.NoError(t, err)
		assert.Equal(t, "+2222222222", dest)

		_, err = channelManager.GetSignalDestination("invalid")
		assert.Error(t, err)
	})

	t.Run("GetWhatsAppSession", func(t *testing.T) {
		session, err := channelManager.GetWhatsAppSession("+1111111111")
		assert.NoError(t, err)
		assert.Equal(t, "personal", session)

		session, err = channelManager.GetWhatsAppSession("+2222222222")
		assert.NoError(t, err)
		assert.Equal(t, "business", session)

		_, err = channelManager.GetWhatsAppSession("+9999999999")
		assert.Error(t, err)
	})

	t.Run("GetChannelCount", func(t *testing.T) {
		count := channelManager.GetChannelCount()
		assert.Equal(t, 2, count)
	})

	t.Run("GetAllSignalDestinations", func(t *testing.T) {
		destinations := channelManager.GetAllSignalDestinations()
		assert.Len(t, destinations, 2)
		assert.Contains(t, destinations, "+1111111111")
		assert.Contains(t, destinations, "+2222222222")
	})
}