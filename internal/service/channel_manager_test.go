package service

import (
	"testing"

	"whatsignal/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelManager_GetDefaultSessionName(t *testing.T) {
	tests := []struct {
		name     string
		channels []models.Channel
		expected string
	}{
		{
			name: "single channel",
			channels: []models.Channel{
				{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+1111111111"},
			},
			expected: "personal",
		},
		{
			name: "multiple channels - returns first",
			channels: []models.Channel{
				{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+1111111111"},
				{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+2222222222"},
				{WhatsAppSessionName: "family", SignalDestinationPhoneNumber: "+3333333333"},
			},
			expected: "business",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := NewChannelManager(tt.channels)
			require.NoError(t, err)

			result := cm.GetDefaultSessionName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChannelManager_GetAllWhatsAppSessions(t *testing.T) {
	tests := []struct {
		name     string
		channels []models.Channel
		expected []string
	}{
		{
			name: "single channel",
			channels: []models.Channel{
				{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+1111111111"},
			},
			expected: []string{"personal"},
		},
		{
			name: "multiple channels",
			channels: []models.Channel{
				{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+1111111111"},
				{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+2222222222"},
				{WhatsAppSessionName: "family", SignalDestinationPhoneNumber: "+3333333333"},
			},
			expected: []string{"business", "personal", "family"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := NewChannelManager(tt.channels)
			require.NoError(t, err)

			result := cm.GetAllWhatsAppSessions()
			assert.Len(t, result, len(tt.expected))
			
			// Check that all expected sessions are present (order may vary due to map iteration)
			for _, expected := range tt.expected {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestChannelManager_IsValidSession(t *testing.T) {
	channels := []models.Channel{
		{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+1111111111"},
		{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+2222222222"},
	}

	cm, err := NewChannelManager(channels)
	require.NoError(t, err)

	tests := []struct {
		name        string
		sessionName string
		expected    bool
	}{
		{
			name:        "valid session - business",
			sessionName: "business",
			expected:    true,
		},
		{
			name:        "valid session - personal",
			sessionName: "personal",
			expected:    true,
		},
		{
			name:        "invalid session",
			sessionName: "nonexistent",
			expected:    false,
		},
		{
			name:        "empty session name",
			sessionName: "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cm.IsValidSession(tt.sessionName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChannelManager_IsValidDestination(t *testing.T) {
	channels := []models.Channel{
		{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+1111111111"},
		{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+2222222222"},
	}

	cm, err := NewChannelManager(channels)
	require.NoError(t, err)

	tests := []struct {
		name        string
		destination string
		expected    bool
	}{
		{
			name:        "valid destination - business",
			destination: "+1111111111",
			expected:    true,
		},
		{
			name:        "valid destination - personal",
			destination: "+2222222222",
			expected:    true,
		},
		{
			name:        "invalid destination",
			destination: "+9999999999",
			expected:    false,
		},
		{
			name:        "empty destination",
			destination: "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cm.IsValidDestination(tt.destination)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChannelManager_GetChannelCount(t *testing.T) {
	tests := []struct {
		name     string
		channels []models.Channel
		expected int
	}{
		{
			name:     "empty channels",
			channels: []models.Channel{},
			expected: 0,
		},
		{
			name: "single channel",
			channels: []models.Channel{
				{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+1111111111"},
			},
			expected: 1,
		},
		{
			name: "multiple channels",
			channels: []models.Channel{
				{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+1111111111"},
				{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+2222222222"},
				{WhatsAppSessionName: "family", SignalDestinationPhoneNumber: "+3333333333"},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expected == 0 {
				// Empty channels should fail during creation
				_, err := NewChannelManager(tt.channels)
				assert.Error(t, err)
				return
			}

			cm, err := NewChannelManager(tt.channels)
			require.NoError(t, err)

			result := cm.GetChannelCount()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChannelManager_ConcurrentAccess(t *testing.T) {
	// Test that the channel manager is thread-safe
	channels := []models.Channel{
		{WhatsAppSessionName: "business", SignalDestinationPhoneNumber: "+1111111111"},
		{WhatsAppSessionName: "personal", SignalDestinationPhoneNumber: "+2222222222"},
	}

	cm, err := NewChannelManager(channels)
	require.NoError(t, err)

	// Start multiple goroutines that access the channel manager methods
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Test various methods concurrently
			_ = cm.GetDefaultSessionName()
			_ = cm.GetAllWhatsAppSessions()
			_ = cm.IsValidSession("business")
			_ = cm.IsValidDestination("+1111111111")
			_ = cm.GetChannelCount()

			// Test the main methods as well
			_, _ = cm.GetSignalDestination("business")
			_, _ = cm.GetWhatsAppSession("+1111111111")
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify the manager is still working correctly
	assert.Equal(t, "business", cm.GetDefaultSessionName())
	assert.Equal(t, 2, cm.GetChannelCount())
	assert.True(t, cm.IsValidSession("personal"))
	assert.True(t, cm.IsValidDestination("+2222222222"))
}