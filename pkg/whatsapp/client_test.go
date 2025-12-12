package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Disable typing delays in tests to prevent timeouts
	_ = os.Setenv("WHATSIGNAL_TEST_MODE", "true")
}

// Define constants in the test file for clarity if they are used in switch cases
// or directly in assertions. This mirrors the constants in client.go.
const (
	testAPIBase             = "/api"
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
			resp := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456@c.us",
					ID:         "msg123",
					Serialized: "msg123",
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointSendImage:
			resp := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456@c.us",
					ID:         "test-msg-id",
					Serialized: "test-msg-id",
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointSendFile:
			resp := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456@c.us",
					ID:         "test-msg-id",
					Serialized: "test-msg-id",
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointSendVoice:
			resp := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456@c.us",
					ID:         "test-msg-id",
					Serialized: "test-msg-id",
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case testAPIBase + testEndpointSendVideo:
			resp := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456@c.us",
					ID:         "test-msg-id",
					Serialized: "test-msg-id",
				},
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
			switch r.Method {
			case http.MethodPost:
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
					http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				}
			case http.MethodGet:
				// Return a list of sessions
				sessions := []types.Session{
					{
						Name:   "test-session",
						Status: "WORKING",
					},
				}
				if err := json.NewEncoder(w).Encode(sessions); err != nil {
					http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				}
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case testEndpointSessionDefault:
			if r.Method == http.MethodGet {
				if err := json.NewEncoder(w).Encode(types.Session{Name: "test-session", Status: "WORKING"}); err != nil {
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
		case "/api/sessions/test-session/restart":
			if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case "/api/contacts":
			if contactID := r.URL.Query().Get("contactId"); contactID != "" {
				// Single contact
				contact := types.Contact{
					ID:   contactID,
					Name: "Test Contact",
				}
				if err := json.NewEncoder(w).Encode(contact); err != nil {
					http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				}
			}
		case "/api/contacts/all":
			// All contacts
			contacts := []types.Contact{
				{ID: "contact1", Name: "Contact 1"},
				{ID: "contact2", Name: "Contact 2"},
			}
			if err := json.NewEncoder(w).Encode(contacts); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		case "/api/test-session/groups":
			// All groups - new WAHA endpoint format: /api/{session}/groups
			groups := []types.Group{
				{ID: "group1@g.us", Subject: "Group 1"},
				{ID: "group2@g.us", Subject: "Group 2"},
			}
			if err := json.NewEncoder(w).Encode(groups); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
		default:
			// Check if it's a single group request: /api/{session}/groups/{groupId}
			if strings.HasPrefix(r.URL.Path, "/api/test-session/groups/") {
				groupID := strings.TrimPrefix(r.URL.Path, "/api/test-session/groups/")
				group := types.Group{
					ID:          types.WAHAGroupID(groupID),
					Subject:     "Test Group",
					Description: "Test Description",
					Participants: []types.GroupParticipant{
						{ID: "user1@c.us", Role: "admin", IsAdmin: true},
						{ID: "user2@c.us", Role: "member", IsAdmin: false},
					},
				}
				if err := json.NewEncoder(w).Encode(group); err != nil {
					http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				}
				return
			}
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
	assert.Equal(t, types.SessionStatus("WORKING"), status.Status)

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

func TestClient_SendText_EmptyResponse(t *testing.T) {
	// Test server that returns 201 with empty body (simulates WAHA behavior)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check API key
		if apiKey := r.Header.Get("X-Api-Key"); apiKey != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == testAPIBase+testEndpointSendText {
			// Return 201 with empty body (like WAHA sometimes does)
			w.WriteHeader(http.StatusCreated)
			// No body written
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-api-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	})

	ctx := context.Background()

	// Test that empty response is handled gracefully
	resp, err := client.SendText(ctx, "123456", "Hello, World!")
	require.NoError(t, err)
	assert.Equal(t, "sent", resp.Status)
	assert.Equal(t, "", resp.MessageID) // Empty message ID is acceptable
}

func TestClient_SendImage(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-image-*.jpg")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write some test data
	_, err = tmpFile.Write([]byte("test image data"))
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Set up test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, testAPIBase+testEndpointSendImage, r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse JSON body
		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Verify JSON fields
		assert.Equal(t, "123456", payload["chatId"])
		assert.Equal(t, "test-session", payload["session"])
		assert.Equal(t, "Test image", payload["caption"])

		// Verify file structure
		file, ok := payload["file"].(map[string]interface{})
		require.True(t, ok, "file field should be an object")
		assert.Equal(t, "image/jpeg", file["mimetype"])
		assert.NotEmpty(t, file["data"], "file data should not be empty")
		assert.Contains(t, file["filename"], ".jpg")

		// Send response in WAHA format
		w.Header().Set("Content-Type", "application/json")
		resp := types.WAHAMessageResponse{
			ID: &struct {
				FromMe     bool   `json:"fromMe"`
				Remote     string `json:"remote"`
				ID         string `json:"id"`
				Serialized string `json:"_serialized"`
			}{
				FromMe:     true,
				Remote:     "123456@c.us",
				ID:         "test-msg-id",
				Serialized: "test-msg-id",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
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
	assert.Contains(t, err.Error(), "failed to get file info")
}

func TestClient_SendFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-file-*.pdf")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write some test data
	_, err = tmpFile.Write([]byte("test file data"))
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Set up test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, testAPIBase+testEndpointSendFile, r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse JSON body
		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Verify JSON fields
		assert.Equal(t, "123456", payload["chatId"])
		assert.Equal(t, "test-session", payload["session"])
		assert.Equal(t, "Test file", payload["caption"])

		// Verify file structure
		file, ok := payload["file"].(map[string]interface{})
		require.True(t, ok, "file field should be an object")
		assert.Equal(t, "application/pdf", file["mimetype"])
		assert.NotEmpty(t, file["data"], "file data should not be empty")
		assert.Contains(t, file["filename"], ".pdf")

		// Send response in WAHA format
		w.Header().Set("Content-Type", "application/json")
		resp := types.WAHAMessageResponse{
			ID: &struct {
				FromMe     bool   `json:"fromMe"`
				Remote     string `json:"remote"`
				ID         string `json:"id"`
				Serialized string `json:"_serialized"`
			}{
				FromMe:     true,
				Remote:     "123456@c.us",
				ID:         "test-msg-id",
				Serialized: "test-msg-id",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write some test data
	_, err = tmpFile.Write([]byte("test voice data"))
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Set up test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, testAPIBase+testEndpointSendVoice, r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse JSON body
		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Verify JSON fields
		assert.Equal(t, "123456", payload["chatId"])
		assert.Equal(t, "test-session", payload["session"])

		// Verify file structure
		file, ok := payload["file"].(map[string]interface{})
		require.True(t, ok, "file field should be an object")
		assert.Equal(t, "audio/ogg", file["mimetype"])
		assert.NotEmpty(t, file["data"], "file data should not be empty")
		assert.Contains(t, file["filename"], ".ogg")

		// Send response in WAHA format
		w.Header().Set("Content-Type", "application/json")
		resp := types.WAHAMessageResponse{
			ID: &struct {
				FromMe     bool   `json:"fromMe"`
				Remote     string `json:"remote"`
				ID         string `json:"id"`
				Serialized string `json:"_serialized"`
			}{
				FromMe:     true,
				Remote:     "123456@c.us",
				ID:         "test-msg-id",
				Serialized: "test-msg-id",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write some test data
	_, err = tmpFile.Write([]byte("test video data"))
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Set up test server that handles version check and defaults to no video support
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/server/version":
			// Return version without video support (simulating WAHA Core)
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(types.ServerVersion{
				Version: "2024.2.3",
				Engine:  "WEBJS",
				Tier:    "CORE",
				Browser: "",
			}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				return
			}

		case testAPIBase + testEndpointSendFile:
			// Video will be sent as file due to lack of video support
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Parse JSON body
			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)

			// Verify JSON fields
			assert.Equal(t, "123456", payload["chatId"])
			assert.Equal(t, "test-session", payload["session"])
			assert.Equal(t, "Test video", payload["caption"])

			// Verify file structure
			file, ok := payload["file"].(map[string]interface{})
			require.True(t, ok, "file field should be an object")
			assert.Equal(t, "video/mp4", file["mimetype"])
			assert.NotEmpty(t, file["data"], "file data should not be empty")
			assert.Contains(t, file["filename"], ".mp4")

			// Send response in WAHA format
			w.Header().Set("Content-Type", "application/json")
			resp := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456@c.us",
					ID:         "test-msg-id",
					Serialized: "test-msg-id",
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
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

	// Test successful send (will be sent as document due to no video support)
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
		resp := types.WAHAMessageResponse{
			ID: &struct {
				FromMe     bool   `json:"fromMe"`
				Remote     string `json:"remote"`
				ID         string `json:"id"`
				Serialized string `json:"_serialized"`
			}{
				FromMe:     true,
				Remote:     "123456@c.us",
				ID:         "msg123",
				Serialized: "msg123",
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write some test data
	_, err = tmpFile.Write([]byte("test document content"))
	require.NoError(t, err)
	_ = tmpFile.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, testAPIBase+testEndpointSendFile, r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse JSON body
		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Verify JSON fields
		assert.Equal(t, "chat123", payload["chatId"])
		assert.Equal(t, "Check this document", payload["caption"])

		// Verify file structure
		file, ok := payload["file"].(map[string]interface{})
		require.True(t, ok, "file field should be an object")
		assert.Equal(t, "application/pdf", file["mimetype"])
		assert.NotEmpty(t, file["data"], "file data should not be empty")
		assert.Contains(t, file["filename"], ".pdf")

		// Send response in WAHA format
		resp := types.WAHAMessageResponse{
			ID: &struct {
				FromMe     bool   `json:"fromMe"`
				Remote     string `json:"remote"`
				ID         string `json:"id"`
				Serialized string `json:"_serialized"`
			}{
				FromMe:     true,
				Remote:     "chat123@c.us",
				ID:         "msg123",
				Serialized: "msg123",
			},
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

func TestSendReaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/reaction", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse JSON body
		var payload types.ReactionRequest
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		// Verify JSON fields
		assert.Equal(t, "test-session", payload.Session)
		assert.Equal(t, "msg456", payload.MessageID)
		assert.Equal(t, "👍", payload.Reaction)

		// Send response
		resp := types.SendMessageResponse{
			MessageID: "reaction123",
			Status:    "sent",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-token",
		SessionName: "test-session",
		Timeout:     10 * time.Second,
	})

	// Test successful reaction
	resp, err := client.SendReaction(context.Background(), "chat123@c.us", "msg456", "👍")
	require.NoError(t, err)
	assert.Equal(t, "reaction123", resp.MessageID)
	assert.Equal(t, "sent", resp.Status)

	// Test remove reaction (empty string)
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		var payload types.ReactionRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Failed to decode payload", http.StatusBadRequest)
			return
		}
		assert.Equal(t, "", payload.Reaction) // Empty string removes reaction

		resp := types.SendMessageResponse{
			MessageID: "reaction124",
			Status:    "sent",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}))
	defer server2.Close()

	client2 := NewClient(types.ClientConfig{
		BaseURL:     server2.URL,
		APIKey:      "test-token",
		SessionName: "test-session",
		Timeout:     10 * time.Second,
	})

	resp, err = client2.SendReaction(context.Background(), "chat123@c.us", "msg456", "")
	require.NoError(t, err)
	assert.Equal(t, "reaction124", resp.MessageID)
}

func TestDeleteMessage(t *testing.T) {
	tests := []struct {
		name           string
		chatID         string
		messageID      string
		responseStatus int
		responseBody   string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "successful deletion",
			chatID:         "1234567890@c.us",
			messageID:      "msg123",
			responseStatus: http.StatusOK,
			responseBody:   "",
			expectError:    false,
		},
		{
			name:           "successful deletion with 204",
			chatID:         "1234567890@c.us",
			messageID:      "msg456",
			responseStatus: http.StatusNoContent,
			responseBody:   "",
			expectError:    false,
		},
		{
			name:           "deletion failed - not found",
			chatID:         "1234567890@c.us",
			messageID:      "nonexistent",
			responseStatus: http.StatusNotFound,
			responseBody:   `{"error": "Message not found"}`,
			expectError:    true,
			errorContains:  "delete failed with status 404",
		},
		{
			name:           "deletion failed - unauthorized",
			chatID:         "1234567890@c.us",
			messageID:      "msg789",
			responseStatus: http.StatusUnauthorized,
			responseBody:   `{"error": "Unauthorized"}`,
			expectError:    true,
			errorContains:  "delete failed with status 401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the request method and path
				assert.Equal(t, "DELETE", r.Method)
				expectedPath := fmt.Sprintf("/api/default/chats/%s/messages/%s", tt.chatID, tt.messageID)
				assert.Equal(t, expectedPath, r.URL.Path)

				// Verify API key header
				assert.Equal(t, "test-api-key", r.Header.Get("X-Api-Key"))

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != "" {
					w.Header().Set("Content-Type", "application/json")
					if _, err := w.Write([]byte(tt.responseBody)); err != nil {
						panic(err)
					}
				}
			}))
			defer server.Close()

			config := types.ClientConfig{
				BaseURL:     server.URL,
				APIKey:      "test-api-key",
				SessionName: "default",
				Timeout:     30 * time.Second,
			}

			client := NewClient(config)
			ctx := context.Background()

			err := client.DeleteMessage(ctx, tt.chatID, tt.messageID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_RestartSession(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	ctx := context.Background()

	// Test successful restart
	err := client.RestartSession(ctx)
	require.NoError(t, err)
}

func TestClient_WaitForSessionReady(t *testing.T) {
	tests := []struct {
		name          string
		maxWaitTime   time.Duration
		sessionStatus string
		expectError   bool
	}{
		{
			name:          "session ready quickly",
			maxWaitTime:   5 * time.Second,
			sessionStatus: "WORKING",
			expectError:   false,
		},
		{
			name:          "timeout waiting for session",
			maxWaitTime:   100 * time.Millisecond,
			sessionStatus: "starting",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create custom server for this test
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/sessions" && r.Method == "GET" {
					sessions := []map[string]interface{}{
						{
							"name":   "test-session",
							"status": tt.sessionStatus,
						},
					}
					if err := json.NewEncoder(w).Encode(sessions); err != nil {
						http.Error(w, "Failed to encode response", http.StatusInternalServerError)
					}
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			config := types.ClientConfig{
				BaseURL:     server.URL,
				APIKey:      "test-api-key",
				SessionName: "test-session",
				Timeout:     5 * time.Second,
			}

			client := NewClient(config).(*WhatsAppClient)
			ctx := context.Background()
			err := client.WaitForSessionReady(ctx, tt.maxWaitTime)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_GetContact(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	ctx := context.Background()

	// Test successful contact retrieval
	contact, err := client.GetContact(ctx, "contact123")
	require.NoError(t, err)
	assert.Equal(t, "contact123", contact.ID)
	assert.Equal(t, "Test Contact", contact.Name)
}

func TestClient_GetAllContacts(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	ctx := context.Background()

	// Test successful contacts retrieval
	contacts, err := client.GetAllContacts(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, contacts, 2)
	assert.Equal(t, "contact1", contacts[0].ID)
	assert.Equal(t, "Contact 1", contacts[0].Name)
	assert.Equal(t, "contact2", contacts[1].ID)
	assert.Equal(t, "Contact 2", contacts[1].Name)
}

func TestClient_DeleteMessage(t *testing.T) {
	tests := []struct {
		name           string
		chatID         string
		messageID      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "successful deletion",
			chatID:         "123456789@c.us",
			messageID:      "true_123456789@c.us_ABCD1234",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "deletion with 204 status",
			chatID:         "123456789@c.us",
			messageID:      "true_123456789@c.us_EFGH5678",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:          "empty chatID",
			chatID:        "",
			messageID:     "true_123456789@c.us_ABCD1234",
			expectedError: "chatID cannot be empty",
		},
		{
			name:          "empty messageID",
			chatID:        "123456789@c.us",
			messageID:     "",
			expectedError: "messageID cannot be empty",
		},
		{
			name:           "server error",
			chatID:         "123456789@c.us",
			messageID:      "true_123456789@c.us_ERROR",
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "delete failed with status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodDelete, r.Method)

				if tt.expectedStatus != 0 {
					expectedURL := fmt.Sprintf("/api/test-session/chats/%s/messages/%s", tt.chatID, tt.messageID)
					assert.Equal(t, expectedURL, r.URL.Path)

					if tt.messageID == "true_123456789@c.us_ERROR" {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					w.WriteHeader(tt.expectedStatus)
				}
			}))
			defer server.Close()

			client := NewClient(types.ClientConfig{
				BaseURL:     server.URL,
				SessionName: "test-session",
				APIKey:      "test-key",
			})

			ctx := context.Background()
			err := client.DeleteMessage(ctx, tt.chatID, tt.messageID)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_ServerVersionDetection(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse types.ServerVersion
		expectSupport  bool
		expectError    bool
	}{
		{
			name: "WAHA Plus with Chrome - supports video",
			serverResponse: types.ServerVersion{
				Version: "2024.2.3",
				Engine:  "NOWEB",
				Tier:    "PLUS",
				Browser: "/usr/bin/google-chrome-stable",
			},
			expectSupport: true,
			expectError:   false,
		},
		{
			name: "WAHA Core - no video support",
			serverResponse: types.ServerVersion{
				Version: "2024.2.3",
				Engine:  "WEBJS",
				Tier:    "CORE",
				Browser: "",
			},
			expectSupport: false,
			expectError:   false,
		},
		{
			name: "WAHA Plus without Chrome - no video support",
			serverResponse: types.ServerVersion{
				Version: "2024.2.3",
				Engine:  "NOWEB",
				Tier:    "PLUS",
				Browser: "/usr/bin/firefox",
			},
			expectSupport: false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/server/version" {
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(tt.serverResponse); err != nil {
						http.Error(w, "Failed to encode response", http.StatusInternalServerError)
						return
					}
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			client := &WhatsAppClient{
				baseURL: server.URL,
				client:  &http.Client{Timeout: 5 * time.Second},
			}

			ctx := context.Background()
			version, err := client.getServerVersion(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.serverResponse.Tier, version.Tier)
				assert.Equal(t, tt.serverResponse.Browser, version.Browser)
			}

			// Test video support detection
			supportsVideo := client.checkVideoSupport(ctx)
			assert.Equal(t, tt.expectSupport, supportsVideo)

			// Verify caching works
			cachedSupport := client.checkVideoSupport(ctx)
			assert.Equal(t, supportsVideo, cachedSupport)
		})
	}
}

func TestClient_VideoWithVersionDetection(t *testing.T) {
	// Create a temporary video file
	tmpFile, err := os.CreateTemp("", "test-video-*.mp4")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.Write([]byte("test video data"))
	require.NoError(t, err)
	_ = tmpFile.Close()

	tests := []struct {
		name            string
		serverVersion   types.ServerVersion
		expectVideoCall bool // true if we expect /sendVideo, false if we expect /sendFile
		expectEndpoint  string
	}{
		{
			name: "WAHA Plus - sends as video",
			serverVersion: types.ServerVersion{
				Version: "2024.2.3",
				Engine:  "NOWEB",
				Tier:    "PLUS",
				Browser: "/usr/bin/google-chrome-stable",
			},
			expectVideoCall: true,
			expectEndpoint:  testAPIBase + testEndpointSendVideo,
		},
		{
			name: "WAHA Core - sends as document",
			serverVersion: types.ServerVersion{
				Version: "2024.2.3",
				Engine:  "WEBJS",
				Tier:    "CORE",
				Browser: "",
			},
			expectVideoCall: false,
			expectEndpoint:  testAPIBase + testEndpointSendFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actualEndpoint string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/server/version":
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(tt.serverVersion); err != nil {
						http.Error(w, "Failed to encode response", http.StatusInternalServerError)
						return
					}

				case testAPIBase + testEndpointSendVideo, testAPIBase + testEndpointSendFile:
					actualEndpoint = r.URL.Path

					// Verify request payload
					var payload map[string]interface{}
					err := json.NewDecoder(r.Body).Decode(&payload)
					require.NoError(t, err)

					// Check common fields
					assert.Equal(t, "123456", payload["chatId"])
					assert.Equal(t, "test-session", payload["session"])
					assert.Equal(t, "Test video", payload["caption"])

					// Check video-specific fields only for video endpoint
					if r.URL.Path == testAPIBase+testEndpointSendVideo {
						assert.Equal(t, false, payload["convert"])
						assert.Equal(t, false, payload["asNote"])
					}

					// Send success response
					resp := types.WAHAMessageResponse{
						ID: &struct {
							FromMe     bool   `json:"fromMe"`
							Remote     string `json:"remote"`
							ID         string `json:"id"`
							Serialized string `json:"_serialized"`
						}{
							FromMe:     true,
							Remote:     "123456@c.us",
							ID:         "msg123",
							Serialized: "msg123",
						},
					}
					if err := json.NewEncoder(w).Encode(resp); err != nil {
						http.Error(w, "Failed to encode response", http.StatusInternalServerError)
					}

				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			client := NewClient(types.ClientConfig{
				BaseURL:     server.URL,
				SessionName: "test-session",
				Timeout:     5 * time.Second,
			})

			ctx := context.Background()
			resp, err := client.SendVideo(ctx, "123456", tmpFile.Name(), "Test video")

			assert.NoError(t, err)
			assert.Equal(t, "msg123", resp.MessageID)
			assert.Equal(t, "sent", resp.Status)

			// Verify the correct endpoint was called
			assert.Equal(t, tt.expectEndpoint, actualEndpoint)
		})
	}
}

func TestClient_VideoSupportCaching(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/server/version" {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(types.ServerVersion{
				Version: "2024.2.3",
				Engine:  "NOWEB",
				Tier:    "PLUS",
				Browser: "/usr/bin/google-chrome-stable",
			}); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				return
			}
		}
	}))
	defer server.Close()

	client := &WhatsAppClient{
		baseURL: server.URL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	ctx := context.Background()

	// First call should fetch from server
	support1 := client.checkVideoSupport(ctx)
	assert.True(t, support1)
	assert.Equal(t, 1, callCount)

	// Second call should use cache
	support2 := client.checkVideoSupport(ctx)
	assert.True(t, support2)
	assert.Equal(t, 1, callCount) // No additional calls

	// Third call should still use cache
	support3 := client.checkVideoSupport(ctx)
	assert.True(t, support3)
	assert.Equal(t, 1, callCount) // Still no additional calls
}

func TestClient_ServerVersionError(t *testing.T) {
	tests := []struct {
		name           string
		serverBehavior func(w http.ResponseWriter, r *http.Request)
		expectSupport  bool
	}{
		{
			name: "server returns 404",
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectSupport: false,
		},
		{
			name: "server returns invalid JSON",
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				if _, err := w.Write([]byte("invalid json")); err != nil {
					panic(err)
				}
			},
			expectSupport: false,
		},
		{
			name: "server times out",
			serverBehavior: func(w http.ResponseWriter, r *http.Request) {
				// Simulate timeout by not responding
				time.Sleep(100 * time.Millisecond)
			},
			expectSupport: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverBehavior))
			defer server.Close()

			client := &WhatsAppClient{
				baseURL: server.URL,
				client:  &http.Client{Timeout: 50 * time.Millisecond},
			}

			ctx := context.Background()
			support := client.checkVideoSupport(ctx)

			// Should default to no video support on error
			assert.Equal(t, tt.expectSupport, support)

			// Should be cached even on error
			assert.NotNil(t, client.supportsVideo)
			assert.Equal(t, tt.expectSupport, *client.supportsVideo)
		})
	}
}

func TestWAHAResponseParsing(t *testing.T) {
	tests := []struct {
		name               string
		responseBody       string
		expectedMessageID  string
		expectedParseError bool
		expectedWarning    bool
	}{
		{
			name: "valid WAHA response with ID field",
			responseBody: `{
				"id": {
					"fromMe": true,
					"remote": "1234567890123@c.us",
					"id": "ABCD1234567890ABCDEF",
					"_serialized": "true_1234567890123@c.us_ABCD1234567890ABCDEF"
				}
			}`,
			expectedMessageID: "true_1234567890123@c.us_ABCD1234567890ABCDEF",
		},
		{
			name: "valid WAHA response with _data field",
			responseBody: `{
				"_data": {
					"id": {
						"fromMe": true,
						"remote": "1234567890123@c.us",
						"id": "ABCD1234567890ABCDEF",
						"_serialized": "true_1234567890123@c.us_ABCD1234567890ABCDEF"
					}
				}
			}`,
			expectedMessageID: "true_1234567890123@c.us_ABCD1234567890ABCDEF",
		},
		{
			name: "WAHA response with both fields (ID field takes precedence)",
			responseBody: `{
				"id": {
					"_serialized": "true_1234567890123@c.us_PRIMARY"
				},
				"_data": {
					"id": {
						"_serialized": "true_1234567890123@c.us_SECONDARY"
					}
				}
			}`,
			expectedMessageID: "true_1234567890123@c.us_PRIMARY",
		},
		{
			name: "WAHA response with no message ID",
			responseBody: `{
				"result": true
			}`,
			expectedMessageID: "",
		},
		{
			name: "invalid JSON",
			responseBody: `{
				"invalid": json
			}`,
			expectedMessageID:  "",
			expectedParseError: false, // Changed: invalid JSON now returns success with warning
			expectedWarning:    true,  // New field to check for warning message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write([]byte(tt.responseBody)); err != nil {
					panic(err)
				}
			}))
			defer server.Close()

			client := NewClient(types.ClientConfig{
				BaseURL:     server.URL,
				SessionName: "test-session",
				APIKey:      "test-key",
			})

			ctx := context.Background()
			resp, err := client.SendText(ctx, "test@c.us", "test message")

			if tt.expectedParseError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to decode WAHA response")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMessageID, resp.MessageID)
				assert.Equal(t, "sent", resp.Status)

				if tt.expectedWarning {
					assert.Contains(t, resp.Error, "warning: could not parse response")
				} else {
					assert.Empty(t, resp.Error)
				}
			}
		})
	}
}

func TestClient_GetGroup(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	ctx := context.Background()

	// Test successful group retrieval
	group, err := client.GetGroup(ctx, "group123@g.us")
	require.NoError(t, err)
	require.NotNil(t, group)
	assert.Equal(t, "group123@g.us", group.ID.String())
	assert.Equal(t, "Test Group", group.Subject)
	assert.Equal(t, "Test Description", group.Description)
	assert.Len(t, group.Participants, 2)
	assert.Equal(t, "user1@c.us", group.Participants[0].ID)
	assert.True(t, group.Participants[0].IsAdmin)
	assert.Equal(t, "user2@c.us", group.Participants[1].ID)
	assert.False(t, group.Participants[1].IsAdmin)
}

func TestClient_GetAllGroups(t *testing.T) {
	client, server := setupTestClient(t)
	defer server.Close()

	ctx := context.Background()

	// Test successful groups retrieval
	groups, err := client.GetAllGroups(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Equal(t, "group1@g.us", groups[0].ID.String())
	assert.Equal(t, "Group 1", groups[0].Subject)
	assert.Equal(t, "group2@g.us", groups[1].ID.String())
	assert.Equal(t, "Group 2", groups[1].Subject)
}
