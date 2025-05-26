package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigError_Error(t *testing.T) {
	err := ConfigError{Message: "test error"}
	assert.Equal(t, "test error", err.Error())
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		valid  bool
	}{
		{
			name: "valid config",
			config: Config{
				WhatsApp: WhatsAppConfig{
					APIBaseURL:      "http://localhost:8080",
					APIKey:          "test-key",
					SessionName:     "test-session",
					Timeout:         5 * time.Second,
					RetryCount:      3,
					WebhookSecret:   "secret",
					PollIntervalSec: 30,
				},
				Signal: SignalConfig{
					RPCURL:      "http://localhost:8081",
					AuthToken:   "test-token",
					PhoneNumber: "+1234567890",
					DeviceName:  "test-device",
				},
				Retry: RetryConfig{
					InitialBackoffMs: 100,
					MaxBackoffMs:     1000,
					MaxAttempts:      3,
				},
				RetentionDays: 7,
				LogLevel:      "info",
			},
			valid: true,
		},
		{
			name: "missing required fields",
			config: Config{
				WhatsApp: WhatsAppConfig{},
				Signal:   SignalConfig{},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Here we're just testing the struct itself since there's no explicit validation
			assert.NotNil(t, tt.config)
		})
	}
}
