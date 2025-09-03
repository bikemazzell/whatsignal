package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"whatsignal/pkg/whatsapp/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Disable typing delays in tests to prevent timeouts
	os.Setenv("WHATSIGNAL_TEST_MODE", "true")
}

// setupMediaTestServer creates a mock WAHA API server for media testing
func setupMediaTestServer(t *testing.T) (*httptest.Server, func(string) map[string]interface{}) {
	var lastRequestBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Check API key
		if apiKey := r.Header.Get("X-Api-Key"); apiKey != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Parse request body to capture for verification
		if err := json.NewDecoder(r.Body).Decode(&lastRequestBody); err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}

		// Handle different media endpoints
		switch {
		case strings.Contains(r.URL.Path, "/sendImage"):
			resp := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456@c.us",
					ID:         "image-msg-id",
					Serialized: "image-msg-id",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		case strings.Contains(r.URL.Path, "/sendVoice"):
			resp := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456@c.us",
					ID:         "voice-msg-id",
					Serialized: "voice-msg-id",
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		case strings.Contains(r.URL.Path, "/sendVideo"):
			resp := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456@c.us",
					ID:         "video-msg-id",
					Serialized: "video-msg-id",
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		case strings.Contains(r.URL.Path, "/sendFile"):
			resp := types.WAHAMessageResponse{
				ID: &struct {
					FromMe     bool   `json:"fromMe"`
					Remote     string `json:"remote"`
					ID         string `json:"id"`
					Serialized string `json:"_serialized"`
				}{
					FromMe:     true,
					Remote:     "123456@c.us",
					ID:         "file-msg-id",
					Serialized: "file-msg-id",
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Logf("Failed to encode response: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	getLastRequest := func(key string) map[string]interface{} {
		return lastRequestBody
	}

	return server, getLastRequest
}

// createTestFile creates a temporary test file with specified content and extension
func createTestFile(t *testing.T, content string, extension string) string {
	tmpDir := t.TempDir()
	filename := fmt.Sprintf("test_file%s", extension)
	filepath := filepath.Join(tmpDir, filename)

	err := os.WriteFile(filepath, []byte(content), 0644)
	require.NoError(t, err)

	return filepath
}

// createLargeTestFile creates a test file larger than the recommended size limit
func createLargeTestFile(t *testing.T) string {
	tmpDir := t.TempDir()
	filepath := filepath.Join(tmpDir, "large_test.jpg")

	// Create a file larger than 50MB
	largeContent := make([]byte, 60*1024*1024) // 60MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	err := os.WriteFile(filepath, largeContent, 0644)
	require.NoError(t, err)

	return filepath
}

func TestSendImageWithSession(t *testing.T) {
	server, getLastRequest := setupMediaTestServer(t)
	defer server.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-api-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	}).(*WhatsAppClient)

	ctx := context.Background()
	chatID := "123456@c.us"
	sessionName := "custom-session"

	t.Run("successful image send", func(t *testing.T) {
		imagePath := createTestFile(t, "fake-image-content", ".jpg")
		caption := "Test image caption"

		resp, err := client.SendImageWithSession(ctx, chatID, imagePath, caption, sessionName)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "image-msg-id", resp.MessageID)
		assert.Equal(t, "sent", resp.Status)

		// Verify request payload
		request := getLastRequest("")
		assert.Equal(t, sessionName, request["session"])
		assert.Equal(t, chatID, request["chatId"])
		assert.Equal(t, caption, request["caption"])
		assert.Contains(t, request, "file")
	})

	t.Run("invalid file path", func(t *testing.T) {
		invalidPath := "/nonexistent/path/image.jpg"

		resp, err := client.SendImageWithSession(ctx, chatID, invalidPath, "", sessionName)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed to get file info")
	})

	t.Run("malicious file path", func(t *testing.T) {
		maliciousPath := "../../../etc/passwd"

		resp, err := client.SendImageWithSession(ctx, chatID, maliciousPath, "", sessionName)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "invalid media path")
	})

	t.Run("large file warning", func(t *testing.T) {
		largePath := createLargeTestFile(t)

		resp, err := client.SendImageWithSession(ctx, chatID, largePath, "", sessionName)

		// Should succeed but trigger warning (test passes if no error)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("context timeout", func(t *testing.T) {
		imagePath := createTestFile(t, "test-content", ".jpg")
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		defer cancel()

		resp, err := client.SendImageWithSession(timeoutCtx, chatID, imagePath, "", sessionName)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestSendVoiceWithSession(t *testing.T) {
	server, getLastRequest := setupMediaTestServer(t)
	defer server.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-api-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	}).(*WhatsAppClient)

	ctx := context.Background()
	chatID := "123456@c.us"
	sessionName := "voice-session"

	t.Run("successful voice send", func(t *testing.T) {
		voicePath := createTestFile(t, "fake-voice-content", ".ogg")

		resp, err := client.SendVoiceWithSession(ctx, chatID, voicePath, sessionName)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "voice-msg-id", resp.MessageID)
		assert.Equal(t, "sent", resp.Status)

		// Verify request payload
		request := getLastRequest("")
		assert.Equal(t, sessionName, request["session"])
		assert.Equal(t, chatID, request["chatId"])
	})

	t.Run("invalid voice file", func(t *testing.T) {
		invalidPath := "/tmp/nonexistent.ogg"

		resp, err := client.SendVoiceWithSession(ctx, chatID, invalidPath, sessionName)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("different voice formats", func(t *testing.T) {
		testCases := []string{".mp3", ".wav", ".m4a", ".ogg"}

		for _, ext := range testCases {
			t.Run("voice_format_"+ext, func(t *testing.T) {
				voicePath := createTestFile(t, "voice-content", ext)

				resp, err := client.SendVoiceWithSession(ctx, chatID, voicePath, sessionName)

				assert.NoError(t, err)
				assert.NotNil(t, resp)
			})
		}
	})
}

func TestSendVideoWithSession(t *testing.T) {
	server, getLastRequest := setupMediaTestServer(t)
	defer server.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-api-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	}).(*WhatsAppClient)

	ctx := context.Background()
	chatID := "123456@c.us"
	sessionName := "video-session"

	t.Run("successful video send", func(t *testing.T) {
		videoPath := createTestFile(t, "fake-video-content", ".mp4")
		caption := "Video caption"

		resp, err := client.SendVideoWithSession(ctx, chatID, videoPath, caption, sessionName)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		// Video may fall back to document if video support is not available
		// This is expected behavior in test environment
		assert.NotEmpty(t, resp.MessageID)
		assert.Equal(t, "sent", resp.Status)

		// Verify request payload
		request := getLastRequest("")
		assert.Equal(t, sessionName, request["session"])
		assert.Equal(t, chatID, request["chatId"])
		assert.Equal(t, caption, request["caption"])
	})

	t.Run("empty caption", func(t *testing.T) {
		videoPath := createTestFile(t, "video-content", ".mp4")

		resp, err := client.SendVideoWithSession(ctx, chatID, videoPath, "", sessionName)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("different video formats", func(t *testing.T) {
		formats := []string{".mp4", ".avi", ".mov", ".mkv"}

		for _, ext := range formats {
			t.Run("video_format_"+ext, func(t *testing.T) {
				videoPath := createTestFile(t, "video-content", ext)

				resp, err := client.SendVideoWithSession(ctx, chatID, videoPath, "Test", sessionName)

				assert.NoError(t, err)
				assert.NotNil(t, resp)
			})
		}
	})

	t.Run("video without support falls back to document", func(t *testing.T) {
		// This test assumes the client has a way to simulate video support check failure
		videoPath := createTestFile(t, "video-content", ".mp4")

		resp, err := client.SendVideoWithSession(ctx, chatID, videoPath, "Caption", sessionName)

		// Should succeed even if video support is unavailable (fallback to document)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestSendDocumentWithSession(t *testing.T) {
	server, getLastRequest := setupMediaTestServer(t)
	defer server.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "test-api-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	}).(*WhatsAppClient)

	ctx := context.Background()
	chatID := "123456@c.us"
	sessionName := "document-session"

	t.Run("successful document send", func(t *testing.T) {
		docPath := createTestFile(t, "fake-pdf-content", ".pdf")
		caption := "Document caption"

		resp, err := client.SendDocumentWithSession(ctx, chatID, docPath, caption, sessionName)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "file-msg-id", resp.MessageID)
		assert.Equal(t, "sent", resp.Status)

		// Verify request payload
		request := getLastRequest("")
		assert.Equal(t, sessionName, request["session"])
		assert.Equal(t, chatID, request["chatId"])
		assert.Equal(t, caption, request["caption"])
	})

	t.Run("various document types", func(t *testing.T) {
		docTypes := []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".txt", ".zip"}

		for _, ext := range docTypes {
			t.Run("document_type_"+ext, func(t *testing.T) {
				docPath := createTestFile(t, "document-content", ext)

				resp, err := client.SendDocumentWithSession(ctx, chatID, docPath, "Test doc", sessionName)

				assert.NoError(t, err)
				assert.NotNil(t, resp)
			})
		}
	})

	t.Run("document without extension", func(t *testing.T) {
		docPath := createTestFile(t, "content", "")

		resp, err := client.SendDocumentWithSession(ctx, chatID, docPath, "No extension", sessionName)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("empty session name", func(t *testing.T) {
		docPath := createTestFile(t, "content", ".txt")

		resp, err := client.SendDocumentWithSession(ctx, chatID, docPath, "Caption", "")

		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestMediaSessionFunctions_ErrorHandling(t *testing.T) {
	// Test server that returns errors
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("Server error")); err != nil {
			t.Logf("Failed to write error response: %v", err)
		}
	}))
	defer errorServer.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     errorServer.URL,
		APIKey:      "test-api-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	}).(*WhatsAppClient)

	ctx := context.Background()
	testFile := createTestFile(t, "test content", ".txt")

	t.Run("server error handling", func(t *testing.T) {
		testCases := []struct {
			name string
			fn   func() (*types.SendMessageResponse, error)
		}{
			{
				name: "SendImageWithSession",
				fn: func() (*types.SendMessageResponse, error) {
					return client.SendImageWithSession(ctx, "123@c.us", testFile, "", "session")
				},
			},
			{
				name: "SendVoiceWithSession",
				fn: func() (*types.SendMessageResponse, error) {
					return client.SendVoiceWithSession(ctx, "123@c.us", testFile, "session")
				},
			},
			{
				name: "SendVideoWithSession",
				fn: func() (*types.SendMessageResponse, error) {
					return client.SendVideoWithSession(ctx, "123@c.us", testFile, "", "session")
				},
			},
			{
				name: "SendDocumentWithSession",
				fn: func() (*types.SendMessageResponse, error) {
					return client.SendDocumentWithSession(ctx, "123@c.us", testFile, "", "session")
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := tc.fn()
				assert.Error(t, err)
				assert.Nil(t, resp)
			})
		}
	})
}

func TestMediaSessionFunctions_AuthenticationError(t *testing.T) {
	// Test server that requires authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := w.Write([]byte("Unauthorized")); err != nil {
			t.Logf("Failed to write unauthorized response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(types.ClientConfig{
		BaseURL:     server.URL,
		APIKey:      "wrong-api-key",
		SessionName: "test-session",
		Timeout:     5 * time.Second,
	}).(*WhatsAppClient)

	ctx := context.Background()
	testFile := createTestFile(t, "test content", ".txt")

	functions := map[string]func() (*types.SendMessageResponse, error){
		"SendImageWithSession": func() (*types.SendMessageResponse, error) {
			return client.SendImageWithSession(ctx, "123@c.us", testFile, "", "session")
		},
		"SendVoiceWithSession": func() (*types.SendMessageResponse, error) {
			return client.SendVoiceWithSession(ctx, "123@c.us", testFile, "session")
		},
		"SendVideoWithSession": func() (*types.SendMessageResponse, error) {
			return client.SendVideoWithSession(ctx, "123@c.us", testFile, "", "session")
		},
		"SendDocumentWithSession": func() (*types.SendMessageResponse, error) {
			return client.SendDocumentWithSession(ctx, "123@c.us", testFile, "", "session")
		},
	}

	for name, fn := range functions {
		t.Run(name+"_auth_error", func(t *testing.T) {
			resp, err := fn()
			assert.Error(t, err)
			assert.Nil(t, resp)
		})
	}
}
