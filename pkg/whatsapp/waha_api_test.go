package whatsapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhatsAppClient_WAHA_API_Format(t *testing.T) {
	tests := []struct {
		name           string
		sessionName    string
		expectedURL    string
		expectedPayload map[string]interface{}
	}{
		{
			name:        "correct API endpoint format",
			sessionName: "default",
			expectedURL: "/api/sendText",
			expectedPayload: map[string]interface{}{
				"chatId":  "123456789@c.us",
				"text":    "test message",
				"session": "default",
			},
		},
		{
			name:        "custom session name",
			sessionName: "test-session",
			expectedURL: "/api/sendText",
			expectedPayload: map[string]interface{}{
				"chatId":  "987654321@c.us",
				"text":    "hello world",
				"session": "test-session",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedPayload map[string]interface{}
			var receivedURL string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedURL = r.URL.Path
				
				err := json.NewDecoder(r.Body).Decode(&receivedPayload)
				require.NoError(t, err)

				response := types.SendMessageResponse{
					MessageID: "msg123",
					Status:    "sent",
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			client := NewClient(types.ClientConfig{
				BaseURL:     server.URL,
				APIKey:      "test-key",
				SessionName: tt.sessionName,
			})

			ctx := context.Background()
			chatID := tt.expectedPayload["chatId"].(string)
			text := tt.expectedPayload["text"].(string)

			_, err := client.SendText(ctx, chatID, text)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedURL, receivedURL)
			assert.Equal(t, tt.expectedPayload, receivedPayload)
		})
	}
}

func TestWhatsAppClient_StatusCodeHandling(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		expectError    bool
		errorContains  string
	}{
		{
			name:        "HTTP 200 OK",
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:        "HTTP 201 Created",
			statusCode:  http.StatusCreated,
			expectError: false,
		},
		{
			name:          "HTTP 400 Bad Request",
			statusCode:    http.StatusBadRequest,
			expectError:   true,
			errorContains: "request failed with status 400",
		},
		{
			name:          "HTTP 404 Not Found",
			statusCode:    http.StatusNotFound,
			expectError:   true,
			errorContains: "request failed with status 404",
		},
		{
			name:          "HTTP 500 Internal Server Error",
			statusCode:    http.StatusInternalServerError,
			expectError:   true,
			errorContains: "request failed with status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := types.SendMessageResponse{
					MessageID: "msg123",
					Status:    "sent",
				}
				if tt.expectError {
					response.Error = "test error"
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			client := NewClient(types.ClientConfig{
				BaseURL:     server.URL,
				APIKey:      "test-key",
				SessionName: "default",
			})

			ctx := context.Background()
			_, err := client.SendText(ctx, "123456789@c.us", "test message")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWhatsAppClient_SessionInPayload(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		require.NoError(t, err)

		response := types.SendMessageResponse{
			MessageID: "msg123",
			Status:    "sent",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-key",
		SessionName: "custom-session",
	})

	ctx := context.Background()
	_, err := client.SendText(ctx, "123456789@c.us", "test message")
	require.NoError(t, err)

	assert.Equal(t, "custom-session", receivedPayload["session"])
	assert.Equal(t, "123456789@c.us", receivedPayload["chatId"])
	assert.Equal(t, "test message", receivedPayload["text"])
	assert.Len(t, receivedPayload, 3, "Should only have chatId, text, and session fields")
}

func TestWhatsAppClient_OptionalEndpoints(t *testing.T) {
	endpointsCalled := make(map[string]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpointsCalled[r.URL.Path] = true

		switch r.URL.Path {
		case "/api/sendText":
			response := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456789@c.us",
					ID:         "msg123",
					Serialized: "msg123",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
		case "/api/sendSeen", "/api/startTyping", "/api/stopTyping":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-key",
		SessionName: "default",
	})

	ctx := context.Background()
	resp, err := client.SendText(ctx, "123456789@c.us", "test message")

	require.NoError(t, err, "SendText should succeed even if optional endpoints fail")
	assert.NotNil(t, resp)
	assert.Equal(t, "msg123", resp.MessageID)

	assert.True(t, endpointsCalled["/api/sendText"], "Should call sendText endpoint")
	assert.True(t, endpointsCalled["/api/sendSeen"], "Should attempt sendSeen endpoint")
	assert.True(t, endpointsCalled["/api/startTyping"], "Should attempt startTyping endpoint")
	assert.True(t, endpointsCalled["/api/stopTyping"], "Should attempt stopTyping endpoint")
}