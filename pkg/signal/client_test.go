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
		resp := types.SendMessageResponse{
			Jsonrpc: "2.0",
			Result: struct {
				Timestamp int64  `json:"timestamp"`
				MessageID string `json:"messageId"`
			}{
				Timestamp: time.Now().UnixMilli(),
				MessageID: "test-msg-id",
			},
			ID: 1,
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
	assert.Equal(t, "test-msg-id", resp.Result.MessageID)
}

func TestClient_SendMessageError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := types.SendMessageResponse{
			Jsonrpc: "2.0",
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{
				Code:    500,
				Message: "test error",
			},
			ID: 1,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "+1234567890", "test-device", nil)

	ctx := context.Background()
	_, err := client.SendMessage(ctx, "recipient", "test message", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test error")
}

func TestClient_ReceiveMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := types.ReceiveMessageResponse{
			Jsonrpc: "2.0",
			Result:  []types.SignalMessage{},
			ID:      1,
		}
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
		resp := types.ReceiveMessageResponse{
			Jsonrpc: "2.0",
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{
				Code:    500,
				Message: "test error",
			},
			ID: 1,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "+1234567890", "test-device", nil)

	ctx := context.Background()
	_, err := client.ReceiveMessages(ctx, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test error")
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
		var reqBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)
		assert.Equal(t, "register", reqBody["method"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", "+1234567890", "test-device", nil)

	ctx := context.Background()
	err := client.InitializeDevice(ctx)
	require.NoError(t, err)
}
