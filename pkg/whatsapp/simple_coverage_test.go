package whatsapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetSessionName tests GetSessionName method
func TestGetSessionName_Coverage(t *testing.T) {
	client := &WhatsAppClient{
		sessionName: "test-session",
	}
	
	assert.Equal(t, "test-session", client.GetSessionName())
}