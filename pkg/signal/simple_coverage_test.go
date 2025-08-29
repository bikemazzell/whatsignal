package signal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSendMessage_Coverage tests basic SendMessage functionality
func TestSendMessage_Coverage(t *testing.T) {
	tempDir := t.TempDir()
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Return the expected structure
		if _, err := w.Write([]byte(`{"timestamp": 123456789, "messageId": "msg123"}`)); err != nil {
			panic(err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "+1234567890", "test-device", tempDir, nil)
	
	resp, err := client.SendMessage(context.Background(), "+0987654321", "test", nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}