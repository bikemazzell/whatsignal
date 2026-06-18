package signal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestSignalClient() *SignalClient {
	client := NewClientWithLogger("http://localhost:8080", "+1234567890", "test-device", "/tmp/attachments", nil, nil)
	return client.(*SignalClient)
}

func TestSignalClient_IsInitialized_DefaultFalse(t *testing.T) {
	c := newTestSignalClient()
	assert.False(t, c.IsInitialized())
}

func TestSignalClient_InitializationError_DefaultEmpty(t *testing.T) {
	c := newTestSignalClient()
	assert.Empty(t, c.InitializationError())
}

func TestSignalClient_DetectedMode_DefaultEmpty(t *testing.T) {
	c := newTestSignalClient()
	assert.Empty(t, c.DetectedMode())
}

func TestSignalClient_SetInitialized(t *testing.T) {
	c := newTestSignalClient()
	c.initialized = true
	c.initError = "some error"
	c.detectedMode = "json-rpc"

	assert.True(t, c.IsInitialized())
	assert.Equal(t, "some error", c.InitializationError())
	assert.Equal(t, "json-rpc", c.DetectedMode())
}
