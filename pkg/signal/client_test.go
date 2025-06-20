package signal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"whatsignal/pkg/signal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_SendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request is to the correct endpoint
		assert.Equal(t, "/v2/send", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		resp := types.SendResponse{
			Timestamp: types.FlexibleInt64(time.Now().UnixMilli()),
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "+1234567890", "test-device", nil)

	ctx := context.Background()
	resp, err := client.SendMessage(ctx, "recipient", "test message", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.MessageID)
	assert.Greater(t, resp.Timestamp, int64(0))
}

func TestClient_SendMessageError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Failed to send message"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "+1234567890", "test-device", nil)

	ctx := context.Background()
	_, err := client.SendMessage(ctx, "recipient", "test message", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signal API error")
}

func TestClient_ReceiveMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request is to the correct endpoint
		assert.Contains(t, r.URL.Path, "/v1/receive/")
		assert.Equal(t, "GET", r.Method)

		// Return empty array of messages
		resp := []types.RestMessage{}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "+1234567890", "test-device", nil)

	ctx := context.Background()
	msgs, err := client.ReceiveMessages(ctx, 1)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestClient_ReceiveMessagesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "+1234567890", "test-device", nil)

	ctx := context.Background()
	_, err := client.ReceiveMessages(ctx, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signal API error")
}

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8080", "test-token", "+1234567890", "test-device", nil)
	assert.NotNil(t, client)
	assert.NotNil(t, client.(*SignalClient).client)
	sConcreteClient := client.(*SignalClient)
	assert.Equal(t, 30*time.Second, sConcreteClient.client.Timeout)
}

func TestClient_InitializeDevice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request is to the correct endpoint
		assert.Equal(t, "/v1/about", r.URL.Path)
		assert.Equal(t, "GET", r.Method)

		// Return a valid about response
		resp := types.AboutResponse{
			Versions: []string{"v1", "v2"},
			Build:    1,
			Mode:     "native",
			Version:  "0.92",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "+1234567890", "test-device", nil)

	ctx := context.Background()
	err := client.InitializeDevice(ctx)
	require.NoError(t, err)
}
