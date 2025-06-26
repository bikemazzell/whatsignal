package integration_test

import (
	"testing"
	"time"

	"whatsignal/internal/models"
	"whatsignal/internal/service"

	"github.com/stretchr/testify/assert"
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