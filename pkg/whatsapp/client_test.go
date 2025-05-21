package whatsapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*WhatsAppClient, *httptest.Server) {
	server := httptest.NewServer(handler)
	client := NewClient(server.URL).(*WhatsAppClient)
	return client, server
}

func TestSendText(t *testing.T) {
	tests := []struct {
		name       string
		chatID     string
		message    string
		serverResp *SendMessageResponse
		wantError  bool
	}{
		{
			name:    "successful send",
			chatID:  "123456",
			message: "Hello, World!",
			serverResp: &SendMessageResponse{
				MessageID: "msg123",
				Status:    "sent",
			},
			wantError: false,
		},
		{
			name:    "server error",
			chatID:  "invalid",
			message: "test",
			serverResp: &SendMessageResponse{
				Status: "error",
				Error:  "invalid chat ID",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/api/sendText", r.URL.Path)

				var payload map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&payload)
				require.NoError(t, err)

				assert.Equal(t, tt.chatID, payload["chatId"])
				assert.Equal(t, tt.message, payload["text"])

				w.Header().Set("Content-Type", "application/json")
				if tt.wantError {
					w.WriteHeader(http.StatusBadRequest)
				}
				json.NewEncoder(w).Encode(tt.serverResp)
			})
			defer server.Close()

			resp, err := client.SendText(tt.chatID, tt.message)
			if tt.wantError {
				assert.Error(t, err)
				assert.NotEmpty(t, resp.Error)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.serverResp.MessageID, resp.MessageID)
				assert.Equal(t, tt.serverResp.Status, resp.Status)
			}
		})
	}
}

func TestSendMedia(t *testing.T) {
	// Create a temporary test file
	tmpFile, err := os.CreateTemp("", "whatsignal-test-*.jpg")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	content := []byte("test image content")
	_, err = tmpFile.Write(content)
	require.NoError(t, err)
	tmpFile.Close()

	tests := []struct {
		name       string
		chatID     string
		mediaPath  string
		caption    string
		serverResp *SendMessageResponse
		wantError  bool
	}{
		{
			name:      "successful media send",
			chatID:    "123456",
			mediaPath: tmpFile.Name(),
			caption:   "Test image",
			serverResp: &SendMessageResponse{
				MessageID: "media123",
				Status:    "sent",
			},
			wantError: false,
		},
		{
			name:      "invalid media path",
			chatID:    "123456",
			mediaPath: "/nonexistent/path.jpg",
			caption:   "Test image",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if tt.wantError {
					return
				}

				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/api/sendMedia", r.URL.Path)

				err := r.ParseMultipartForm(10 << 20)
				require.NoError(t, err)

				assert.Equal(t, tt.chatID, r.FormValue("chatId"))
				assert.Equal(t, tt.caption, r.FormValue("caption"))

				file, header, err := r.FormFile("file")
				require.NoError(t, err)
				defer file.Close()

				assert.Equal(t, filepath.Base(tt.mediaPath), header.Filename)

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.serverResp)
			})
			defer server.Close()

			resp, err := client.SendMedia(tt.chatID, tt.mediaPath, tt.caption)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.serverResp.MessageID, resp.MessageID)
				assert.Equal(t, tt.serverResp.Status, resp.Status)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	baseURL := "http://localhost:8080"
	client := NewClient(baseURL).(*WhatsAppClient)
	assert.Equal(t, baseURL, client.baseURL)
	assert.NotNil(t, client.client)
}
