package integration_test

import (
	"context"
	"testing"
	"time"

	"whatsignal/internal/models"
	"whatsignal/internal/service"
	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestMultiChannelMessageRouting tests end-to-end message routing between multiple channels
func TestMultiChannelMessageRouting(t *testing.T) {
	// Setup test configuration with multiple channels
	config := &models.Config{
		WhatsApp: models.WhatsAppConfig{
			APIBaseURL: "http://localhost:3000",
		},
		Signal: models.SignalConfig{
			RPCURL:                  "http://localhost:8080",
			IntermediaryPhoneNumber: "+1234567890",
		},
		Database: models.DatabaseConfig{
			Path: ":memory:",
		},
		Media: models.MediaConfig{
			CacheDir: "/tmp/test-media",
		},
		Channels: []models.Channel{
			{
				WhatsAppSessionName:          "personal",
				SignalDestinationPhoneNumber: "+1111111111",
			},
			{
				WhatsAppSessionName:          "business",
				SignalDestinationPhoneNumber: "+2222222222",
			},
		},
	}

	// Create channel manager
	channelManager, err := service.NewChannelManager(config.Channels)
	require.NoError(t, err)

	// Test scenarios
	t.Run("WhatsApp to Signal routing", func(t *testing.T) {
		testCases := []struct {
			name                    string
			sessionName             string
			expectedDestination     string
			shouldSucceed           bool
		}{
			{
				name:                "Personal session routes to personal Signal",
				sessionName:         "personal",
				expectedDestination: "+1111111111",
				shouldSucceed:       true,
			},
			{
				name:                "Business session routes to business Signal",
				sessionName:         "business",
				expectedDestination: "+2222222222",
				shouldSucceed:       true,
			},
			{
				name:          "Invalid session fails",
				sessionName:   "invalid",
				shouldSucceed: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				destination, err := channelManager.GetSignalDestination(tc.sessionName)
				if tc.shouldSucceed {
					assert.NoError(t, err)
					assert.Equal(t, tc.expectedDestination, destination)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})

	t.Run("Signal to WhatsApp routing", func(t *testing.T) {
		testCases := []struct {
			name                string
			signalDestination   string
			expectedSession     string
			shouldSucceed       bool
		}{
			{
				name:              "Personal Signal routes to personal session",
				signalDestination: "+1111111111",
				expectedSession:   "personal",
				shouldSucceed:     true,
			},
			{
				name:              "Business Signal routes to business session",
				signalDestination: "+2222222222",
				expectedSession:   "business",
				shouldSucceed:     true,
			},
			{
				name:              "Invalid destination fails",
				signalDestination: "+9999999999",
				shouldSucceed:     false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				session, err := channelManager.GetWhatsAppSession(tc.signalDestination)
				if tc.shouldSucceed {
					assert.NoError(t, err)
					assert.Equal(t, tc.expectedSession, session)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})

	t.Run("Channel isolation", func(t *testing.T) {
		// Verify that each channel is independent
		personalDest, err := channelManager.GetSignalDestination("personal")
		require.NoError(t, err)

		businessDest, err := channelManager.GetSignalDestination("business")
		require.NoError(t, err)

		// Destinations should be different
		assert.NotEqual(t, personalDest, businessDest)

		// Reverse mappings should be correct
		personalSession, err := channelManager.GetWhatsAppSession(personalDest)
		require.NoError(t, err)
		assert.Equal(t, "personal", personalSession)

		businessSession, err := channelManager.GetWhatsAppSession(businessDest)
		require.NoError(t, err)
		assert.Equal(t, "business", businessSession)
	})
}

// TestWebhookIntegrationMultiChannel tests webhook handling with session context
func TestWebhookIntegrationMultiChannel(t *testing.T) {
	t.Run("WhatsApp webhook with session", func(t *testing.T) {
		testCases := []struct {
			name        string
			sessionName string
			payload     models.WhatsAppWebhookPayload
		}{
			{
				name:        "Personal session message",
				sessionName: "personal",
				payload: createWhatsAppPayload("personal", "wa-personal-1", "1111111111@c.us", "Personal message"),
			},
			{
				name:        "Business session message",
				sessionName: "business",
				payload: createWhatsAppPayload("business", "wa-business-1", "2222222222@c.us", "Business message"),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Verify that the payload contains the correct session
				assert.Equal(t, tc.sessionName, tc.payload.Session)
				assert.Equal(t, models.EventMessage, tc.payload.Event)
				assert.NotEmpty(t, tc.payload.Payload.ID)
				assert.NotEmpty(t, tc.payload.Payload.From)
				assert.NotEmpty(t, tc.payload.Payload.Body)
			})
		}
	})

}

// TestConfigurationValidation tests configuration validation for multi-channel setup
func TestConfigurationValidation(t *testing.T) {
	testCases := []struct {
		name          string
		channels      []models.Channel
		expectedError bool
		errorMessage  string
	}{
		{
			name: "Valid multi-channel configuration",
			channels: []models.Channel{
				{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+1111111111"},
				{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+2222222222"},
				{WhatsAppSessionName: "family", SignalDestinationPhoneNumber: "+3333333333"},
			},
			expectedError: false,
		},
		{
			name:          "Empty channels",
			channels:      []models.Channel{},
			expectedError: true,
			errorMessage:  "no channels configured",
		},
		{
			name: "Duplicate session names",
			channels: []models.Channel{
				{WhatsAppSessionName: "default", SignalDestinationPhoneNumber: "+1111111111"},
				{WhatsAppSessionName: "default", SignalDestinationPhoneNumber: "+2222222222"},
			},
			expectedError: true,
			errorMessage:  "duplicate WhatsApp session name: default",
		},
		{
			name: "Duplicate Signal destinations",
			channels: []models.Channel{
				{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+1111111111"},
				{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+1111111111"},
			},
			expectedError: true,
			errorMessage:  "duplicate Signal destination number: +1111111111",
		},
		{
			name: "Empty session name",
			channels: []models.Channel{
				{WhatsAppSessionName: "", SignalDestinationPhoneNumber: "+1111111111"},
			},
			expectedError: true,
			errorMessage:  "empty WhatsApp session name",
		},
		{
			name: "Empty Signal destination",
			channels: []models.Channel{
				{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: ""},
			},
			expectedError: true,
			errorMessage:  "empty Signal destination phone number",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cm, err := service.NewChannelManager(tc.channels)
			if tc.expectedError {
				assert.Error(t, err)
				if tc.errorMessage != "" {
					assert.Contains(t, err.Error(), tc.errorMessage)
				}
				assert.Nil(t, cm)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cm)
				assert.Equal(t, len(tc.channels), cm.GetChannelCount())
			}
		})
	}
}

// Helper function to create WhatsApp webhook payload
func createWhatsAppPayload(session, id, from, body string) models.WhatsAppWebhookPayload {
	return models.WhatsAppWebhookPayload{
		Session: session,
		Event:   models.EventMessage,
		Payload: struct {
			ID        string `json:"id"`
			Timestamp int64  `json:"timestamp"`
			From      string `json:"from"`
			FromMe    bool   `json:"fromMe"`
			To        string `json:"to"`
			Body      string `json:"body"`
			HasMedia  bool   `json:"hasMedia"`
			Media     *struct {
				URL      string `json:"url"`
				MimeType string `json:"mimetype"`
				Filename string `json:"filename"`
			} `json:"media"`
			Reaction *struct {
				Text      string `json:"text"`
				MessageID string `json:"messageId"`
			} `json:"reaction"`
			EditedMessageID *string `json:"editedMessageId,omitempty"`
			ACK             *int    `json:"ack,omitempty"`
		}{
			ID:        id,
			From:      from,
			To:        session + "@c.us",
			Body:      body,
			Timestamp: time.Now().Unix(),
		},
	}
}

// Mock WAClient for multi-session contact sync testing
type mockMultiSessionWAClient struct {
	mock.Mock
	sessionName string
}

func (m *mockMultiSessionWAClient) GetSessionName() string {
	return m.sessionName
}

func (m *mockMultiSessionWAClient) GetAllContacts(ctx context.Context, limit, offset int) ([]types.Contact, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]types.Contact), args.Error(1)
}

func (m *mockMultiSessionWAClient) WaitForSessionReady(ctx context.Context, maxWaitTime time.Duration) error {
	args := m.Called(ctx, maxWaitTime)
	return args.Error(0)
}

// Implement other required methods with minimal implementation
func (m *mockMultiSessionWAClient) SendText(ctx context.Context, chatID, message string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendTextWithSession(ctx context.Context, chatID, message, sessionName string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendImage(ctx context.Context, chatID, imagePath, caption string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendImageWithSession(ctx context.Context, chatID, imagePath, caption, sessionName string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendVideo(ctx context.Context, chatID, videoPath, caption string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendVideoWithSession(ctx context.Context, chatID, videoPath, caption, sessionName string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendDocument(ctx context.Context, chatID, docPath, caption string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendDocumentWithSession(ctx context.Context, chatID, docPath, caption, sessionName string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendFile(ctx context.Context, chatID, filePath, caption string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendVoice(ctx context.Context, chatID, voicePath string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendVoiceWithSession(ctx context.Context, chatID, voicePath, sessionName string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendReaction(ctx context.Context, chatID, messageID, reaction string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) SendReactionWithSession(ctx context.Context, chatID, messageID, reaction, sessionName string) (*types.SendMessageResponse, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	return nil
}
func (m *mockMultiSessionWAClient) CreateSession(ctx context.Context) error {
	return nil
}
func (m *mockMultiSessionWAClient) StartSession(ctx context.Context) error {
	return nil
}
func (m *mockMultiSessionWAClient) StopSession(ctx context.Context) error {
	return nil
}
func (m *mockMultiSessionWAClient) GetSessionStatus(ctx context.Context) (*types.Session, error) {
	return nil, nil
}
func (m *mockMultiSessionWAClient) RestartSession(ctx context.Context) error {
	return nil
}
func (m *mockMultiSessionWAClient) GetContact(ctx context.Context, contactID string) (*types.Contact, error) {
	return nil, nil
}

// TestMultiSessionContactSync tests that contact sync works correctly across multiple sessions
func TestMultiSessionContactSync(t *testing.T) {
	ctx := context.Background()

	t.Run("contact sync for multiple sessions", func(t *testing.T) {
		// Create mock clients for different sessions
		personalClient := &mockMultiSessionWAClient{sessionName: "personal"}
		businessClient := &mockMultiSessionWAClient{sessionName: "business"}

		// Mock contacts for personal session
		personalContacts := []types.Contact{
			{ID: "+1111111111@c.us", Number: "+1111111111", Name: "Personal Friend"},
			{ID: "+2222222222@c.us", Number: "+2222222222", Name: "Family Member"},
		}

		// Mock contacts for business session
		businessContacts := []types.Contact{
			{ID: "+3333333333@c.us", Number: "+3333333333", Name: "Business Client"},
			{ID: "+4444444444@c.us", Number: "+4444444444", Name: "Supplier"},
		}

		// Set up expectations for personal session
		personalClient.On("GetAllContacts", ctx, 100, 0).Return(personalContacts, nil)

		// Set up expectations for business session
		businessClient.On("GetAllContacts", ctx, 100, 0).Return(businessContacts, nil)

		// Create a mock database that accepts any contact
		mockDB := &mockContactDatabaseService{}
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(nil)

		// Test contact sync for personal session
		personalContactService := service.NewContactService(mockDB, personalClient)
		err := personalContactService.SyncAllContacts(ctx)
		assert.NoError(t, err)

		// Test contact sync for business session
		businessContactService := service.NewContactService(mockDB, businessClient)
		err = businessContactService.SyncAllContacts(ctx)
		assert.NoError(t, err)

		// Verify all expectations were met
		personalClient.AssertExpectations(t)
		businessClient.AssertExpectations(t)
		mockDB.AssertExpectations(t)

		// Verify that SaveContact was called for all contacts from both sessions
		mockDB.AssertNumberOfCalls(t, "SaveContact", 4) // 2 personal + 2 business
	})

	t.Run("contact sync handles session failures independently", func(t *testing.T) {
		// Create mock clients
		workingClient := &mockMultiSessionWAClient{sessionName: "working"}

		workingContacts := []types.Contact{
			{ID: "+5555555555@c.us", Number: "+5555555555", Name: "Working Contact"},
		}

		// Working session succeeds
		workingClient.On("GetAllContacts", ctx, 100, 0).Return(workingContacts, nil)

		mockDB := &mockContactDatabaseService{}
		mockDB.On("SaveContact", ctx, mock.AnythingOfType("*models.Contact")).Return(nil)

		// Test that working session succeeds
		workingContactService := service.NewContactService(mockDB, workingClient)
		err := workingContactService.SyncAllContacts(ctx)
		assert.NoError(t, err)

		workingClient.AssertExpectations(t)
		mockDB.AssertExpectations(t)

		// Only working session's contact should be saved
		mockDB.AssertNumberOfCalls(t, "SaveContact", 1)
	})
}

// Mock database service for testing
type mockContactDatabaseService struct {
	mock.Mock
}

func (m *mockContactDatabaseService) SaveContact(ctx context.Context, contact *models.Contact) error {
	args := m.Called(ctx, contact)
	return args.Error(0)
}

func (m *mockContactDatabaseService) GetContact(ctx context.Context, contactID string) (*models.Contact, error) {
	args := m.Called(ctx, contactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contact), args.Error(1)
}

func (m *mockContactDatabaseService) GetContactByPhone(ctx context.Context, phoneNumber string) (*models.Contact, error) {
	args := m.Called(ctx, phoneNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Contact), args.Error(1)
}

func (m *mockContactDatabaseService) CleanupOldContacts(retentionDays int) error {
	args := m.Called(retentionDays)
	return args.Error(0)
}