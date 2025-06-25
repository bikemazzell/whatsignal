package config

import (
	"os"
	"path/filepath"
	"testing"

	"whatsignal/internal/models"
	"whatsignal/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_MultiChannel(t *testing.T) {
	tests := []struct {
		name           string
		configContent  string
		expectedErr    bool
		errorContains  string
		validateConfig func(t *testing.T, cfg *models.Config)
	}{
		{
			name: "Valid multi-channel configuration",
			configContent: `{
				"whatsapp": {
					"api_base_url": "http://localhost:3000"
				},
				"signal": {
					"rpc_url": "http://localhost:8080",
					"intermediaryPhoneNumber": "+1234567890",
					"destinationPhoneNumber": "+0987654321"
				},
				"database": {
					"path": "./test.db"
				},
				"media": {
					"cache_dir": "./media"
				},
				"channels": [
					{
						"whatsappSessionName": "default",
						"signalDestinationPhoneNumber": "+1111111111"
					},
					{
						"whatsappSessionName": "business",
						"signalDestinationPhoneNumber": "+2222222222"
					}
				]
			}`,
			expectedErr: false,
			validateConfig: func(t *testing.T, cfg *models.Config) {
				assert.Len(t, cfg.Channels, 2)
				assert.Equal(t, "default", cfg.Channels[0].WhatsAppSessionName)
				assert.Equal(t, "+1111111111", cfg.Channels[0].SignalDestinationPhoneNumber)
				assert.Equal(t, "business", cfg.Channels[1].WhatsAppSessionName)
				assert.Equal(t, "+2222222222", cfg.Channels[1].SignalDestinationPhoneNumber)
			},
		},
		{
			name: "Missing channels array",
			configContent: `{
				"whatsapp": {
					"api_base_url": "http://localhost:3000"
				},
				"signal": {
					"rpc_url": "http://localhost:8080",
					"intermediaryPhoneNumber": "+1234567890"
				},
				"database": {
					"path": "./test.db"
				},
				"media": {
					"cache_dir": "./media"
				}
			}`,
			expectedErr: true,
			errorContains: "channels array is required",
		},
		{
			name: "Duplicate WhatsApp session names",
			configContent: `{
				"whatsapp": {
					"api_base_url": "http://localhost:3000"
				},
				"signal": {
					"rpc_url": "http://localhost:8080",
					"intermediaryPhoneNumber": "+1234567890"
				},
				"database": {
					"path": "./test.db"
				},
				"media": {
					"cache_dir": "./media"
				},
				"channels": [
					{
						"whatsappSessionName": "default",
						"signalDestinationPhoneNumber": "+1111111111"
					},
					{
						"whatsappSessionName": "default",
						"signalDestinationPhoneNumber": "+2222222222"
					}
				]
			}`,
			expectedErr:   true,
			errorContains: "duplicate WhatsApp session name: default",
		},
		{
			name: "Duplicate Signal destinations",
			configContent: `{
				"whatsapp": {
					"api_base_url": "http://localhost:3000"
				},
				"signal": {
					"rpc_url": "http://localhost:8080",
					"intermediaryPhoneNumber": "+1234567890"
				},
				"database": {
					"path": "./test.db"
				},
				"media": {
					"cache_dir": "./media"
				},
				"channels": [
					{
						"whatsappSessionName": "default",
						"signalDestinationPhoneNumber": "+1111111111"
					},
					{
						"whatsappSessionName": "business",
						"signalDestinationPhoneNumber": "+1111111111"
					}
				]
			}`,
			expectedErr:   true,
			errorContains: "duplicate Signal destination: +1111111111",
		},
		{
			name: "Empty WhatsApp session name",
			configContent: `{
				"whatsapp": {
					"api_base_url": "http://localhost:3000"
				},
				"signal": {
					"rpc_url": "http://localhost:8080",
					"intermediaryPhoneNumber": "+1234567890"
				},
				"database": {
					"path": "./test.db"
				},
				"media": {
					"cache_dir": "./media"
				},
				"channels": [
					{
						"whatsappSessionName": "",
						"signalDestinationPhoneNumber": "+1111111111"
					}
				]
			}`,
			expectedErr:   true,
			errorContains: "empty WhatsApp session name in channel 0",
		},
		{
			name: "Empty Signal destination",
			configContent: `{
				"whatsapp": {
					"api_base_url": "http://localhost:3000"
				},
				"signal": {
					"rpc_url": "http://localhost:8080",
					"intermediaryPhoneNumber": "+1234567890"
				},
				"database": {
					"path": "./test.db"
				},
				"media": {
					"cache_dir": "./media"
				},
				"channels": [
					{
						"whatsappSessionName": "default",
						"signalDestinationPhoneNumber": ""
					}
				]
			}`,
			expectedErr:   true,
			errorContains: "empty Signal destination in channel 0",
		},
		{
			name: "No channels and no legacy config",
			configContent: `{
				"whatsapp": {
					"api_base_url": "http://localhost:3000"
				},
				"signal": {
					"rpc_url": "http://localhost:8080",
					"intermediaryPhoneNumber": "+1234567890"
				},
				"database": {
					"path": "./test.db"
				},
				"media": {
					"cache_dir": "./media"
				}
			}`,
			expectedErr:   true,
			errorContains: "channels array is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.json")
			
			err := os.WriteFile(configPath, []byte(tt.configContent), 0600)
			require.NoError(t, err)

			// Load config
			cfg, err := LoadConfig(configPath)

			if tt.expectedErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, cfg)
				if tt.validateConfig != nil {
					tt.validateConfig(t, cfg)
				}
			}
		})
	}
}

func TestChannelManager(t *testing.T) {
	tests := []struct {
		name          string
		channels      []models.Channel
		expectedErr   bool
		errorContains string
	}{
		{
			name: "Valid channel configuration",
			channels: []models.Channel{
				{
					WhatsAppSessionName:          "default",
					SignalDestinationPhoneNumber: "+1111111111",
				},
				{
					WhatsAppSessionName:          "business",
					SignalDestinationPhoneNumber: "+2222222222",
				},
			},
			expectedErr: false,
		},
		{
			name:          "Empty channels",
			channels:      []models.Channel{},
			expectedErr:   true,
			errorContains: "no channels configured",
		},
		{
			name: "Duplicate session names",
			channels: []models.Channel{
				{
					WhatsAppSessionName:          "default",
					SignalDestinationPhoneNumber: "+1111111111",
				},
				{
					WhatsAppSessionName:          "default",
					SignalDestinationPhoneNumber: "+2222222222",
				},
			},
			expectedErr:   true,
			errorContains: "duplicate WhatsApp session name: default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := service.NewChannelManager(tt.channels)

			if tt.expectedErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, cm)

				// Test channel manager functionality
				for _, channel := range tt.channels {
					// Test GetSignalDestination
					dest, err := cm.GetSignalDestination(channel.WhatsAppSessionName)
					assert.NoError(t, err)
					assert.Equal(t, channel.SignalDestinationPhoneNumber, dest)

					// Test GetWhatsAppSession
					session, err := cm.GetWhatsAppSession(channel.SignalDestinationPhoneNumber)
					assert.NoError(t, err)
					assert.Equal(t, channel.WhatsAppSessionName, session)

					// Test IsValidSession
					assert.True(t, cm.IsValidSession(channel.WhatsAppSessionName))

					// Test IsValidDestination
					assert.True(t, cm.IsValidDestination(channel.SignalDestinationPhoneNumber))
				}

				// Test with invalid values
				_, err = cm.GetSignalDestination("invalid-session")
				assert.Error(t, err)

				_, err = cm.GetWhatsAppSession("invalid-destination")
				assert.Error(t, err)

				assert.False(t, cm.IsValidSession("invalid-session"))
				assert.False(t, cm.IsValidDestination("invalid-destination"))

				// Test GetChannelCount
				assert.Equal(t, len(tt.channels), cm.GetChannelCount())
			}
		})
	}
}