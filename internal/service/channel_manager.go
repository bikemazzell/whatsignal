package service

import (
	"fmt"
	"sync"

	"whatsignal/internal/models"
)

// ChannelManager manages the mapping between WhatsApp sessions and Signal destinations
type ChannelManager struct {
	channels     map[string]string // whatsappSessionName -> signalDestinationPhoneNumber
	reverse      map[string]string // signalDestinationPhoneNumber -> whatsappSessionName
	orderedNames []string          // ordered list of session names (preserves config order)
	mu           sync.RWMutex
}

// NewChannelManager creates a new channel manager from configuration
func NewChannelManager(channels []models.Channel) (*ChannelManager, error) {
	cm := &ChannelManager{
		channels:     make(map[string]string),
		reverse:      make(map[string]string),
		orderedNames: make([]string, 0, len(channels)),
	}

	// Build the mappings
	for _, channel := range channels {
		if channel.WhatsAppSessionName == "" {
			return nil, fmt.Errorf("empty WhatsApp session name in channel configuration")
		}
		if channel.SignalDestinationPhoneNumber == "" {
			return nil, fmt.Errorf("empty Signal destination phone number for session %s", channel.WhatsAppSessionName)
		}

		// Check for duplicate session names
		if _, exists := cm.channels[channel.WhatsAppSessionName]; exists {
			return nil, fmt.Errorf("duplicate WhatsApp session name: %s", channel.WhatsAppSessionName)
		}

		// Check for duplicate destination numbers
		if _, exists := cm.reverse[channel.SignalDestinationPhoneNumber]; exists {
			return nil, fmt.Errorf("duplicate Signal destination number: %s", channel.SignalDestinationPhoneNumber)
		}

		cm.channels[channel.WhatsAppSessionName] = channel.SignalDestinationPhoneNumber
		cm.reverse[channel.SignalDestinationPhoneNumber] = channel.WhatsAppSessionName
		cm.orderedNames = append(cm.orderedNames, channel.WhatsAppSessionName)
	}

	// Ensure at least one channel is configured
	if len(cm.channels) == 0 {
		return nil, fmt.Errorf("no channels configured")
	}

	return cm, nil
}

// GetSignalDestination returns the Signal destination for a WhatsApp session
func (cm *ChannelManager) GetSignalDestination(whatsappSessionName string) (string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	destination, exists := cm.channels[whatsappSessionName]
	if !exists {
		return "", fmt.Errorf("no Signal destination configured for WhatsApp session: %s", whatsappSessionName)
	}

	return destination, nil
}

// GetWhatsAppSession returns the WhatsApp session for a Signal destination
func (cm *ChannelManager) GetWhatsAppSession(signalDestination string) (string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	session, exists := cm.reverse[signalDestination]
	if !exists {
		return "", fmt.Errorf("no WhatsApp session configured for Signal destination: %s", signalDestination)
	}

	return session, nil
}

// GetAllSignalDestinations returns all configured Signal destination numbers
func (cm *ChannelManager) GetAllSignalDestinations() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	destinations := make([]string, 0, len(cm.reverse))
	for destination := range cm.reverse {
		destinations = append(destinations, destination)
	}

	return destinations
}

// GetChannelCount returns the number of configured channels
func (cm *ChannelManager) GetChannelCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return len(cm.channels)
}

// GetAllWhatsAppSessions returns all configured WhatsApp session names
func (cm *ChannelManager) GetAllWhatsAppSessions() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	sessions := make([]string, 0, len(cm.channels))
	for session := range cm.channels {
		sessions = append(sessions, session)
	}

	return sessions
}

// IsValidSession checks if a WhatsApp session is configured
func (cm *ChannelManager) IsValidSession(sessionName string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	_, exists := cm.channels[sessionName]
	return exists
}

// IsValidDestination checks if a Signal destination is configured
func (cm *ChannelManager) IsValidDestination(destination string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	_, exists := cm.reverse[destination]
	return exists
}
