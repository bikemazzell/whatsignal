package service

import (
	"context"
	"testing"
	"time"

	"whatsignal/internal/models"
	signaltypes "whatsignal/pkg/signal/types"
	watypes "whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBridge_MultiChannel_HandleWhatsAppMessage(t *testing.T) {
	tests := []struct {
		name                string
		sessionName         string
		channels            []models.Channel
		expectedDestination string
		expectedError       bool
		errorContains       string
	}{
		{
			name:        "Route to correct Signal destination - default session",
			sessionName: "default",
			channels: []models.Channel{
				{WhatsAppSessionName: "default", SignalDestinationPhoneNumber: "+1111111111"},
				{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+2222222222"},
			},
			expectedDestination: "+1111111111",
			expectedError:       false,
		},
		{
			name:        "Route to correct Signal destination - business session",
			sessionName: "business",
			channels: []models.Channel{
				{WhatsAppSessionName: "default", SignalDestinationPhoneNumber: "+1111111111"},
				{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+2222222222"},
			},
			expectedDestination: "+2222222222",
			expectedError:       false,
		},
		{
			name:        "Invalid session name",
			sessionName: "invalid",
			channels: []models.Channel{
				{WhatsAppSessionName: "default", SignalDestinationPhoneNumber: "+1111111111"},
			},
			expectedError: true,
			errorContains: "no Signal destination configured for WhatsApp session: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create channel manager
			channelManager, err := NewChannelManager(tt.channels)
			require.NoError(t, err)

			// Setup mocks
			mockWAClient := new(mockWhatsAppClient)
			mockSigClient := new(mockSignalClient)
			mockDB := new(mockDatabaseService)
			mockMedia := new(mockMediaHandler)
			mockContacts := new(mockContactService)

			// Create bridge with channel manager
			bridge := NewBridge(
				mockWAClient,
				mockSigClient,
				mockDB,
				mockMedia,
				models.RetryConfig{
					InitialBackoffMs: 1,
					MaxBackoffMs:     5,
					MaxAttempts:      3,
				},
				models.MediaConfig{},
				channelManager,
				mockContacts,
				nil, // No group service for this test
			)

			ctx := context.Background()

			// Setup contact service mock for all cases (since it's called before session validation)
			mockContacts.On("GetContactDisplayName", ctx, "1234567890").Return("Test User")

			if !tt.expectedError {
				// Setup expectations for successful cases
				mockSigClient.On("SendMessage", ctx, tt.expectedDestination, "Test User: Hello", mock.AnythingOfType("[]string")).
					Return(&signaltypes.SendMessageResponse{
						MessageID: "sig-123",
						Timestamp: time.Now().Unix() * 1000,
					}, nil)

				mockDB.On("SaveMessageMapping", ctx, mock.MatchedBy(func(mapping *models.MessageMapping) bool {
					return mapping.SessionName == tt.sessionName &&
						mapping.WhatsAppChatID == "1234567890@c.us" &&
						mapping.SignalMsgID == "sig-123"
				})).Return(nil)
			}

			// Execute
			err = bridge.HandleWhatsAppMessageWithSession(
				ctx,
				tt.sessionName,
				"1234567890@c.us",
				"wa-123",
				"1234567890@c.us",
				"Hello",
				"",
			)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				mockSigClient.AssertExpectations(t)
				mockDB.AssertExpectations(t)
			}
		})
	}
}

func TestBridge_MultiChannel_HandleSignalMessage(t *testing.T) {
	tests := []struct {
		name            string
		destination     string
		channels        []models.Channel
		previousMapping *models.MessageMapping
		expectedSession string
		expectedError   bool
		errorContains   string
	}{
		{
			name:        "Route Signal message to correct WhatsApp session - destination 1",
			destination: "+1111111111",
			channels: []models.Channel{
				{WhatsAppSessionName: "default", SignalDestinationPhoneNumber: "+1111111111"},
				{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+2222222222"},
			},
			previousMapping: &models.MessageMapping{
				WhatsAppChatID: "1234567890@c.us",
				SessionName:    "default",
			},
			expectedSession: "default",
			expectedError:   false,
		},
		{
			name:        "Route Signal message to correct WhatsApp session - destination 2",
			destination: "+2222222222",
			channels: []models.Channel{
				{WhatsAppSessionName: "default", SignalDestinationPhoneNumber: "+1111111111"},
				{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+2222222222"},
			},
			previousMapping: &models.MessageMapping{
				WhatsAppChatID: "9876543210@c.us",
				SessionName:    "business",
			},
			expectedSession: "business",
			expectedError:   false,
		},
		{
			name:        "Invalid Signal destination",
			destination: "+9999999999",
			channels: []models.Channel{
				{WhatsAppSessionName: "default", SignalDestinationPhoneNumber: "+1111111111"},
			},
			expectedError: true,
			errorContains: "no WhatsApp session configured for Signal destination: +9999999999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create channel manager
			channelManager, err := NewChannelManager(tt.channels)
			require.NoError(t, err)

			// Setup mocks
			mockWAClient := &mockWhatsAppClient{
				sendTextResp: &watypes.SendMessageResponse{
					MessageID: "wa-456",
					Status:    "sent",
				},
			}
			mockSigClient := new(mockSignalClient)
			mockDB := new(mockDatabaseService)
			mockMedia := new(mockMediaHandler)
			mockContacts := new(mockContactService)

			// Create bridge with channel manager
			bridge := NewBridge(
				mockWAClient,
				mockSigClient,
				mockDB,
				mockMedia,
				models.RetryConfig{
					InitialBackoffMs: 1,
					MaxBackoffMs:     5,
					MaxAttempts:      3,
				},
				models.MediaConfig{},
				channelManager,
				mockContacts,
				nil, // No group service for this test
			)

			ctx := context.Background()

			// Create test Signal message
			signalMsg := &signaltypes.SignalMessage{
				MessageID: "sig-456",
				Sender:    "+9999999999",
				Message:   "Reply message",
				Timestamp: time.Now().Unix() * 1000,
			}

			if !tt.expectedError {
				// Setup expectations
				mockDB.On("GetLatestMessageMappingBySession", ctx, tt.expectedSession).
					Return(tt.previousMapping, nil)

				mockDB.On("SaveMessageMapping", ctx, mock.MatchedBy(func(mapping *models.MessageMapping) bool {
					return mapping.SessionName == tt.expectedSession &&
						mapping.WhatsAppChatID == tt.previousMapping.WhatsAppChatID &&
						mapping.SignalMsgID == "sig-456"
				})).Return(nil)
			}

			// Execute
			err = bridge.HandleSignalMessageWithDestination(ctx, signalMsg, tt.destination)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				mockDB.AssertExpectations(t)
				mockWAClient.AssertExpectations(t)
			}
		})
	}
}

func TestBridge_MultiChannel_MessageIsolation(t *testing.T) {
	// Create two channels
	channels := []models.Channel{
		{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+1111111111"},
		{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+2222222222"},
	}

	channelManager, err := NewChannelManager(channels)
	require.NoError(t, err)

	// Setup mocks
	mockWAClient := new(mockWhatsAppClient)
	mockSigClient := new(mockSignalClient)
	mockDB := new(mockDatabaseService)
	mockMedia := new(mockMediaHandler)
	mockContacts := new(mockContactService)

	// Create bridge
	bridge := NewBridge(
		mockWAClient,
		mockSigClient,
		mockDB,
		mockMedia,
		models.RetryConfig{
			InitialBackoffMs: 1,
			MaxBackoffMs:     5,
			MaxAttempts:      3,
		},
		models.MediaConfig{},
		channelManager,
		mockContacts,
		nil, // No group service for this test
	)

	ctx := context.Background()

	// Test 1: Message from personal WhatsApp should go to personal Signal
	t.Run("Personal to Personal", func(t *testing.T) {
		mockContacts.On("GetContactDisplayName", ctx, "1111111111").Return("Personal Contact")

		mockSigClient.On("SendMessage", ctx, "+1111111111", "Personal Contact: Personal message", mock.AnythingOfType("[]string")).
			Return(&signaltypes.SendMessageResponse{
				MessageID: "sig-personal-1",
				Timestamp: time.Now().Unix() * 1000,
			}, nil).Once()

		mockDB.On("SaveMessageMapping", ctx, mock.MatchedBy(func(mapping *models.MessageMapping) bool {
			return mapping.SessionName == "personal"
		})).Return(nil).Once()

		err := bridge.HandleWhatsAppMessageWithSession(
			ctx,
			"personal",
			"1111111111@c.us",
			"wa-personal-1",
			"1111111111@c.us",
			"Personal message",
			"",
		)
		assert.NoError(t, err)
	})

	// Test 2: Message from business WhatsApp should go to business Signal
	t.Run("Business to Business", func(t *testing.T) {
		mockContacts.On("GetContactDisplayName", ctx, "2222222222").Return("Business Contact")

		mockSigClient.On("SendMessage", ctx, "+2222222222", "Business Contact: Business message", mock.AnythingOfType("[]string")).
			Return(&signaltypes.SendMessageResponse{
				MessageID: "sig-business-1",
				Timestamp: time.Now().Unix() * 1000,
			}, nil).Once()

		mockDB.On("SaveMessageMapping", ctx, mock.MatchedBy(func(mapping *models.MessageMapping) bool {
			return mapping.SessionName == "business"
		})).Return(nil).Once()

		err := bridge.HandleWhatsAppMessageWithSession(
			ctx,
			"business",
			"2222222222@c.us",
			"wa-business-1",
			"2222222222@c.us",
			"Business message",
			"",
		)
		assert.NoError(t, err)
	})

	// Verify all expectations were met
	mockSigClient.AssertExpectations(t)
	mockDB.AssertExpectations(t)
	mockContacts.AssertExpectations(t)

	// Verify that personal message didn't go to business Signal and vice versa
	mockSigClient.AssertNotCalled(t, "SendMessage", ctx, "+2222222222", "Personal Contact: Personal message", mock.AnythingOfType("[]string"))
	mockSigClient.AssertNotCalled(t, "SendMessage", ctx, "+1111111111", "Business Contact: Business message", mock.AnythingOfType("[]string"))
}
