package signal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	client := NewClient(server.URL, "test-token")
	return client, server
}

func TestSendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var request SendMessageRequest
		err := json.NewDecoder(r.Body).Decode(&request)
		require.NoError(t, err)

		response := SendMessageResponse{
			Jsonrpc: "2.0",
			Result: struct {
				Timestamp int64  `json:"timestamp"`
				MessageID string `json:"messageId"`
			}{
				Timestamp: 1234567890000,
				MessageID: "test-message-id",
			},
			ID: request.ID,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	resp, err := client.SendMessage("+1234567890", "test message", nil)
	require.NoError(t, err)
	assert.Equal(t, "test-message-id", resp.Result.MessageID)
	assert.Equal(t, int64(1234567890000), resp.Result.Timestamp)
}

func TestSendMessageError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		response := SendMessageResponse{
			Jsonrpc: "2.0",
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{
				Code:    400,
				Message: "invalid recipient",
			},
			ID: 1,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	resp, err := client.SendMessage("+1234567890", "test message", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid recipient")
	assert.Equal(t, 400, resp.Error.Code)
}

func TestReceiveMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var request ReceiveMessageRequest
		err := json.NewDecoder(r.Body).Decode(&request)
		require.NoError(t, err)

		response := ReceiveMessageResponse{
			Jsonrpc: "2.0",
			Result: []SignalMessage{
				{
					Timestamp:   1234567890000,
					Sender:      "+1234567890",
					MessageID:   "test-message-id",
					Message:     "test message",
					Attachments: []string{"/path/to/attachment.jpg"},
					QuotedMessage: &struct {
						ID        string `json:"id"`
						Author    string `json:"author"`
						Text      string `json:"text"`
						Timestamp int64  `json:"timestamp"`
					}{
						ID:        "quoted-message-id",
						Author:    "+0987654321",
						Text:      "quoted message",
						Timestamp: 1234567880000,
					},
				},
			},
			ID: request.ID,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	messages, err := client.ReceiveMessages(30)
	require.NoError(t, err)
	require.Len(t, messages, 1)

	msg := messages[0]
	assert.Equal(t, int64(1234567890000), msg.Timestamp)
	assert.Equal(t, "+1234567890", msg.Sender)
	assert.Equal(t, "test-message-id", msg.MessageID)
	assert.Equal(t, "test message", msg.Message)
	assert.Equal(t, []string{"/path/to/attachment.jpg"}, msg.Attachments)
	require.NotNil(t, msg.QuotedMessage)
	assert.Equal(t, "quoted-message-id", msg.QuotedMessage.ID)
	assert.Equal(t, "+0987654321", msg.QuotedMessage.Author)
	assert.Equal(t, "quoted message", msg.QuotedMessage.Text)
	assert.Equal(t, int64(1234567880000), msg.QuotedMessage.Timestamp)
}

func TestNewClient(t *testing.T) {
	rpcURL := "http://localhost:8080"
	authToken := "test-token"
	client := NewClient(rpcURL, authToken).(*SignalClient)
	assert.Equal(t, rpcURL, client.rpcURL)
	assert.Equal(t, authToken, client.authToken)
	assert.NotNil(t, client.client)
}
