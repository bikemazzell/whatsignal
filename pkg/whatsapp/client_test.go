package whatsapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Define constants in the test file for clarity if they are used in switch cases
// or directly in assertions. This mirrors the constants in client.go.
const (
	testAPIBase             = "/api/test-session"
	testEndpointSendText    = "/sendText"
	testEndpointSendImage   = "/sendImage"
	testEndpointSendFile    = "/sendFile"
	testEndpointSendVoice   = "/sendVoice"
	testEndpointSendVideo   = "/sendVideo"
	testEndpointSendSeen    = "/sendSeen"
	testEndpointStartTyping = "/startTyping"
	testEndpointStopTyping  = "/stopTyping"
	// Session related endpoints
	testEndpointSessions       = "/api/sessions"
	testEndpointSessionDefault = "/api/sessions/test-session"
	testEndpointSessionStart   = "/api/sessions/test-session/start"
	testEndpointSessionStop    = "/api/sessions/test-session/stop"
)

func setupTestClient(t *testing.T) (*WhatsAppClient, *httptest.Server) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check API key
		if apiKey := r.Header.Get("X-Api-Key"); apiKey != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case testAPIBase + testEndpointSendText:
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "Failed to decode request body", http.StatusBadRequest)
				return
			}
			resp := types.SendMessageResponse{
				MessageID: "msg123",
				Status:    "sent",
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointSendImage:
			resp := types.SendMessageResponse{
				MessageID: "media123",
				Status:    "sent",
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointSendFile:
			resp := types.SendMessageResponse{
				MessageID: "file123",
				Status:    "sent",
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointSendVoice:
			resp := types.SendMessageResponse{
				MessageID: "voice123",
				Status:    "sent",
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointSendVideo:
			resp := types.SendMessageResponse{
				MessageID: "video123",
				Status:    "sent",
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointSendSeen:
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointStartTyping:
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointStopTyping:
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testEndpointSessions:
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
					http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				}
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case testEndpointSessionDefault:
			if r.Method == http.MethodGet {
				if err := json.NewEncoder(w).Encode(types.Session{Name: "test-session", Status: types.SessionStatusRunning}); err != nil {
					http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				}
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case testEndpointSessionStart:
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testEndpointSessionStop:
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	config := types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-api-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	}

	client := NewClient(config).(*WhatsAppClient)
	return client, server
}

func TestClient_Session(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	ctx := context.Background()

	// Test session lifecycle
	err := client.CreateSession(ctx)
	require.NoError(t, err)

	err = client.StartSession(ctx)
	require.NoError(t, err)

	status, err := client.GetSessionStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, types.SessionStatusRunning, status.Status)

	err = client.StopSession(ctx)
	require.NoError(t, err)
}

func TestClient_SendText(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	ctx := context.Background()

	// Test successful send
	resp, err := client.SendText(ctx, "123456", "Hello, World!")
	require.NoError(t, err)
	assert.Equal(t, "msg123", resp.MessageID)
	assert.Equal(t, "sent", resp.Status)
}

func TestClient_SendImage(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-image-*.jpg")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write some test data
	_, err = tmpFile.Write([]byte("test image data"))
	require.NoError(t, err)
	tmpFile.Close()

	// Set up test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, testAPIBase+testEndpointSendImage, r.URL.Path)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)

		// Verify form fields
		assert.Equal(t, "123456", r.FormValue("chatId"))
		assert.Equal(t, "Test image", r.FormValue("caption"))

		// Send response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(types.SendMessageResponse{
			MessageID: "test-msg-id",
			Status:    "sent",
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create client
	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	})

	ctx := context.Background()

	// Test successful send
	resp, err := client.SendImage(ctx, "123456", tmpFile.Name(), "Test image")
	assert.NoError(t, err)
	assert.Equal(t, "test-msg-id", resp.MessageID)
	assert.Equal(t, "sent", resp.Status)

	// Test file not found
	_, err = client.SendImage(ctx, "123456", "/nonexistent/path.jpg", "Test image")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open media file")
}

func TestClient_SendFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-file-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write some test data
	_, err = tmpFile.Write([]byte("test file data"))
	require.NoError(t, err)
	tmpFile.Close()

	// Set up test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, testAPIBase+testEndpointSendFile, r.URL.Path)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)

		// Verify form fields
		assert.Equal(t, "123456", r.FormValue("chatId"))
		assert.Equal(t, "Test file", r.FormValue("caption"))

		// Send response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(types.SendMessageResponse{
			MessageID: "test-msg-id",
			Status:    "sent",
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create client
	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	})

	ctx := context.Background()

	// Test successful send
	resp, err := client.SendFile(ctx, "123456", tmpFile.Name(), "Test file")
	assert.NoError(t, err)
	assert.Equal(t, "test-msg-id", resp.MessageID)
	assert.Equal(t, "sent", resp.Status)
}

func TestClient_SendVoice(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-voice-*.ogg")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write some test data
	_, err = tmpFile.Write([]byte("test voice data"))
	require.NoError(t, err)
	tmpFile.Close()

	// Set up test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, testAPIBase+testEndpointSendVoice, r.URL.Path)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)

		// Verify form fields
		assert.Equal(t, "123456", r.FormValue("chatId"))

		// Send response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(types.SendMessageResponse{
			MessageID: "test-msg-id",
			Status:    "sent",
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create client
	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	})

	ctx := context.Background()

	// Test successful send
	resp, err := client.SendVoice(ctx, "123456", tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, "test-msg-id", resp.MessageID)
	assert.Equal(t, "sent", resp.Status)
}

func TestClient_SendVideo(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-video-*.mp4")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write some test data
	_, err = tmpFile.Write([]byte("test video data"))
	require.NoError(t, err)
	tmpFile.Close()

	// Set up test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, testAPIBase+testEndpointSendVideo, r.URL.Path)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)

		// Verify form fields
		assert.Equal(t, "123456", r.FormValue("chatId"))
		assert.Equal(t, "Test video", r.FormValue("caption"))

		// Send response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(types.SendMessageResponse{
			MessageID: "test-msg-id",
			Status:    "sent",
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Create client
	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	})

	ctx := context.Background()

	// Test successful send
	resp, err := client.SendVideo(ctx, "123456", tmpFile.Name(), "Test video")
	assert.NoError(t, err)
	assert.Equal(t, "test-msg-id", resp.MessageID)
	assert.Equal(t, "sent", resp.Status)
}

func TestClient_Authentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-Api-Key")
		if apiKey != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(types.SendMessageResponse{
			MessageID: "msg123",
			Status:    "sent",
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	ctx := context.Background()

	// Test with valid API key
	validConfig := types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-api-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	}
	validClient := NewClient(validConfig)
	resp, err := validClient.SendText(ctx, "123456", "test")
	require.NoError(t, err)
	assert.Equal(t, "msg123", resp.MessageID)

	// Test with invalid API key
	invalidConfig := types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "invalid-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	}
	invalidClient := NewClient(invalidConfig)
	_, err = invalidClient.SendText(ctx, "123456", "test")
	assert.Error(t, err)
}

func TestSendDocument(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-doc-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write some test data
	_, err = tmpFile.Write([]byte("test document content"))
	require.NoError(t, err)
	tmpFile.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, testAPIBase+testEndpointSendFile, r.URL.Path)

		// Check request body
		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)

		// Verify form fields
		assert.Equal(t, "chat123", r.FormValue("chatId"))
		assert.Equal(t, "Check this document", r.FormValue("caption"))

		// Send response
		resp := types.SendMessageResponse{
			MessageID: "msg123",
			Status:    "sent",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-token",
		SessionName: "test-session",
		Timeout:     10 * time.Second,
	})

	// Test successful send
	resp, err := client.SendDocument(context.Background(), "chat123", tmpFile.Name(), "Check this document")
	require.NoError(t, err)
	assert.Equal(t, "msg123", resp.MessageID)
	assert.Equal(t, "sent", resp.Status)

	// Test error case
	resp, err = client.SendDocument(context.Background(), "chat123", "", "Check this document")
	assert.Error(t, err)
	assert.Nil(t, resp)
}
