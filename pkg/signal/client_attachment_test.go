package signal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"whatsignal/pkg/signal/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractAttachmentPaths_WithRealFiles(t *testing.T) {
	// Create temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "signal-extract-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test files with different extensions
	testFiles := map[string][]byte{
		"att1.jpg": []byte("fake jpeg content"),
		"att2":     []byte("no extension file"),
		"att3.pdf": []byte("fake pdf content"),
	}

	for filename, content := range testFiles {
		err := os.WriteFile(filepath.Join(tmpDir, filename), content, 0644)
		require.NoError(t, err)
	}

	tests := []struct {
		name        string
		attachments []types.RestMessageAttachment
		expected    []string
	}{
		{
			name: "files exist with extensions",
			attachments: []types.RestMessageAttachment{
				{ID: "att1", Filename: "test1.jpg", ContentType: "image/jpeg"},
				{ID: "att3", Filename: "test3.pdf", ContentType: "application/pdf"},
			},
			expected: []string{
				filepath.Join(tmpDir, "att1.jpg"),
				filepath.Join(tmpDir, "att3.pdf"),
			},
		},
		{
			name: "file exists without extension",
			attachments: []types.RestMessageAttachment{
				{ID: "att2", Filename: "test2", ContentType: "text/plain"},
			},
			expected: []string{filepath.Join(tmpDir, "att2")},
		},
		{
			name: "mixed existing and non-existing files",
			attachments: []types.RestMessageAttachment{
				{ID: "att1", Filename: "test1.jpg", ContentType: "image/jpeg"},         // exists
				{ID: "nonexistent", Filename: "missing.png", ContentType: "image/png"}, // doesn't exist
			},
			expected: []string{filepath.Join(tmpDir, "att1.jpg")}, // only existing file
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			logger.SetLevel(logrus.PanicLevel)

			client := &SignalClient{
				attachmentsDir: tmpDir,
				logger:         logger,
			}

			result := client.extractAttachmentPaths(context.Background(), tt.attachments)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractAttachmentPaths_WithHTTPDownloads(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "signal-http-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tests := []struct {
		name            string
		attachments     []types.RestMessageAttachment
		serverResponses map[string][]byte // attachment ID -> response data
		serverStatus    int
		expectedCount   int
		expectDownload  bool
	}{
		{
			name: "successful HTTP download",
			attachments: []types.RestMessageAttachment{
				{ID: "download1", Filename: "test.jpg", ContentType: "image/jpeg"},
			},
			serverResponses: map[string][]byte{
				"download1": []byte("downloaded image data"),
			},
			serverStatus:   http.StatusOK,
			expectedCount:  1,
			expectDownload: true,
		},
		{
			name: "failed HTTP download",
			attachments: []types.RestMessageAttachment{
				{ID: "download2", Filename: "test.pdf", ContentType: "application/pdf"},
			},
			serverStatus:   http.StatusNotFound,
			expectedCount:  0, // Should be empty on failed download
			expectDownload: false,
		},
		{
			name: "mixed downloads - some succeed, some fail",
			attachments: []types.RestMessageAttachment{
				{ID: "good", Filename: "good.jpg", ContentType: "image/jpeg"},
				{ID: "bad", Filename: "bad.jpg", ContentType: "image/jpeg"},
			},
			serverResponses: map[string][]byte{
				"good": []byte("good data"),
				// "bad" will get 404
			},
			serverStatus:   http.StatusOK, // Only for "good"
			expectedCount:  1,             // Only successful download
			expectDownload: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/v1/attachments/")

				// Extract attachment ID from URL
				pathParts := strings.Split(r.URL.Path, "/")
				attachmentID := pathParts[len(pathParts)-1]

				if data, exists := tt.serverResponses[attachmentID]; exists {
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write(data); err != nil {
						t.Logf("Failed to write response: %v", err)
					}
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			logger := logrus.New()
			logger.SetLevel(logrus.PanicLevel) // Suppress logs during test

			client := &SignalClient{
				baseURL:        server.URL,
				attachmentsDir: tmpDir,
				client:         &http.Client{Timeout: 30 * time.Second},
				logger:         logger,
			}

			result := client.extractAttachmentPaths(context.Background(), tt.attachments)

			assert.Len(t, result, tt.expectedCount)

			if tt.expectDownload && tt.expectedCount > 0 {
				// Verify downloaded file exists and has correct content
				for _, path := range result {
					assert.FileExists(t, path)

					// Find corresponding attachment ID from path
					filename := filepath.Base(path)
					var attachmentID string
					for id := range tt.serverResponses {
						if strings.Contains(filename, id) {
							attachmentID = id
							break
						}
					}

					if attachmentID != "" {
						content, err := os.ReadFile(path)
						assert.NoError(t, err)
						assert.Equal(t, tt.serverResponses[attachmentID], content)
					}
				}
			}
		})
	}
}

func TestExtractAttachmentPaths_TimeoutHandling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "signal-timeout-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create server that delays response to trigger timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the 15-second timeout in extractAttachmentPaths
		// But use a select to allow the test to finish when connection closes
		select {
		case <-time.After(16 * time.Second):
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("too late")); err != nil {
				t.Logf("Failed to write response: %v", err)
			}
		case <-r.Context().Done():
			// Request cancelled, exit immediately
			return
		}
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &SignalClient{
		baseURL:        server.URL,
		attachmentsDir: tmpDir,
		client:         &http.Client{Timeout: 30 * time.Second},
		logger:         logger,
	}

	attachments := []types.RestMessageAttachment{
		{ID: "timeout", Filename: "slow.jpg", ContentType: "image/jpeg"},
	}

	// This should timeout and return empty slice
	result := client.extractAttachmentPaths(context.Background(), attachments)
	assert.Empty(t, result, "Should return empty slice when download times out")
}

func TestExtractAttachmentPaths_NoHTTPClient(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "signal-no-client-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &SignalClient{
		attachmentsDir: tmpDir,
		client:         nil, // No HTTP client
		logger:         logger,
	}

	attachments := []types.RestMessageAttachment{
		{ID: "missing", Filename: "missing.jpg", ContentType: "image/jpeg"},
	}

	result := client.extractAttachmentPaths(context.Background(), attachments)
	assert.Empty(t, result, "Should return empty slice when no HTTP client and files don't exist")
}

func TestExtractAttachmentPaths_ContextCancellation(t *testing.T) {
	// This test verifies that context cancellation is handled properly
	// and doesn't cause race conditions or goroutine leaks.
	tmpDir, err := os.MkdirTemp("", "signal-context-cancel-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("delayed response"))
		case <-r.Context().Done():
			return
		}
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &SignalClient{
		baseURL:        server.URL,
		attachmentsDir: tmpDir,
		client:         &http.Client{Timeout: 30 * time.Second},
		logger:         logger,
	}

	attachments := []types.RestMessageAttachment{
		{ID: "cancel1", Filename: "test.jpg", ContentType: "image/jpeg"},
	}

	// Create cancellable context and cancel it after a short delay
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// This should return quickly after context is cancelled
	start := time.Now()
	result := client.extractAttachmentPaths(ctx, attachments)
	elapsed := time.Since(start)

	// Should return empty and not take 5 seconds (the server delay)
	assert.Empty(t, result)
	assert.Less(t, elapsed, 2*time.Second, "Should return quickly when context is cancelled")
}

func TestExtractAttachmentPaths_ConcurrentDownloads(t *testing.T) {
	// This test verifies that concurrent attachment downloads don't cause
	// race conditions. Run with -race flag.
	tmpDir, err := os.MkdirTemp("", "signal-concurrent-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// Small delay to increase chance of concurrency
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		pathParts := strings.Split(r.URL.Path, "/")
		attachmentID := pathParts[len(pathParts)-1]
		_, _ = w.Write([]byte("content for " + attachmentID))
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	client := &SignalClient{
		baseURL:        server.URL,
		attachmentsDir: tmpDir,
		client:         &http.Client{Timeout: 30 * time.Second},
		logger:         logger,
	}

	// Multiple attachments to download
	attachments := []types.RestMessageAttachment{
		{ID: "att1", Filename: "file1.jpg", ContentType: "image/jpeg"},
		{ID: "att2", Filename: "file2.pdf", ContentType: "application/pdf"},
		{ID: "att3", Filename: "file3.png", ContentType: "image/png"},
	}

	// Run multiple times to increase chance of detecting race
	for i := 0; i < 5; i++ {
		// Clean up files from previous iteration
		files, _ := os.ReadDir(tmpDir)
		for _, f := range files {
			_ = os.Remove(filepath.Join(tmpDir, f.Name()))
		}

		result := client.extractAttachmentPaths(context.Background(), attachments)

		// Should get all 3 attachments
		assert.Len(t, result, 3, "iteration %d: should download all attachments", i)

		// Verify each file exists and has content
		for _, path := range result {
			assert.FileExists(t, path)
			content, err := os.ReadFile(path)
			assert.NoError(t, err)
			assert.NotEmpty(t, content)
		}
	}
}

func TestDownloadAndSaveAttachment_Comprehensive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "signal-save-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tests := []struct {
		name               string
		attachment         types.RestMessageAttachment
		serverResponse     []byte
		serverStatus       int
		attachmentsDir     string
		expectedError      string
		validateFile       func(t *testing.T, filePath string)
		setupPermissions   func() error
		cleanupPermissions func()
	}{
		{
			name: "successful download with content type extension",
			attachment: types.RestMessageAttachment{
				ID:          "test1",
				ContentType: "image/jpeg",
			},
			serverResponse: []byte("jpeg image data"),
			serverStatus:   http.StatusOK,
			attachmentsDir: tmpDir,
			validateFile: func(t *testing.T, filePath string) {
				assert.True(t, strings.HasSuffix(filePath, "test1.jpg"))
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, []byte("jpeg image data"), content)
			},
		},
		{
			name: "successful download with filename extension",
			attachment: types.RestMessageAttachment{
				ID:          "test2",
				Filename:    "document.pdf",
				ContentType: "application/octet-stream",
			},
			serverResponse: []byte("pdf document data"),
			serverStatus:   http.StatusOK,
			attachmentsDir: tmpDir,
			validateFile: func(t *testing.T, filePath string) {
				assert.True(t, strings.HasSuffix(filePath, "test2.pdf"))
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, []byte("pdf document data"), content)
			},
		},
		{
			name: "download without attachments directory",
			attachment: types.RestMessageAttachment{
				ID:          "test3",
				ContentType: "text/plain",
			},
			serverResponse: []byte("text content"),
			serverStatus:   http.StatusOK,
			attachmentsDir: "", // No directory
			validateFile: func(t *testing.T, filePath string) {
				assert.Equal(t, "test3.txt", filePath) // ID + extension from content type
				// File should be created in current directory
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, []byte("text content"), content)
				// Clean up file in current directory
				_ = os.Remove(filePath)
			},
		},
		{
			name: "download failure - server error",
			attachment: types.RestMessageAttachment{
				ID:          "test4",
				ContentType: "image/png",
			},
			serverStatus:   http.StatusInternalServerError,
			attachmentsDir: tmpDir,
			expectedError:  "failed to download attachment",
		},
		{
			name: "directory creation failure",
			attachment: types.RestMessageAttachment{
				ID:          "test5",
				ContentType: "image/gif",
			},
			serverResponse: []byte("gif data"),
			serverStatus:   http.StatusOK,
			attachmentsDir: "/root/cannot-create", // Directory we can't create
			expectedError:  "failed to create attachments directory",
		},
		{
			name: "unknown content type and no filename",
			attachment: types.RestMessageAttachment{
				ID:          "test6",
				ContentType: "application/unknown",
			},
			serverResponse: []byte("unknown data"),
			serverStatus:   http.StatusOK,
			attachmentsDir: tmpDir,
			validateFile: func(t *testing.T, filePath string) {
				assert.Equal(t, filepath.Join(tmpDir, "test6"), filePath) // No extension
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, []byte("unknown data"), content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != nil {
					if _, err := w.Write(tt.serverResponse); err != nil {
						t.Logf("Failed to write response: %v", err)
					}
				}
			}))
			defer server.Close()

			if tt.setupPermissions != nil {
				err := tt.setupPermissions()
				require.NoError(t, err)
			}
			if tt.cleanupPermissions != nil {
				defer tt.cleanupPermissions()
			}

			client := &SignalClient{
				baseURL:        server.URL,
				attachmentsDir: tt.attachmentsDir,
				client:         &http.Client{Timeout: 30 * time.Second},
			}

			filePath, err := client.downloadAndSaveAttachment(context.Background(), tt.attachment)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Empty(t, filePath)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, filePath)
				if tt.validateFile != nil {
					tt.validateFile(t, filePath)
				}
			}
		})
	}
}

func TestSendMessage_AttachmentIntegration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "signal-send-integration-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test attachment files with various types
	testFiles := map[string]struct {
		content     []byte
		contentType string
	}{
		"image.jpg":    {[]byte("fake jpeg content"), "image/jpeg"},
		"document.pdf": {[]byte("fake pdf content"), "application/pdf"},
		"audio.ogg":    {[]byte("fake ogg content"), "audio/ogg"},
		"noext":        {[]byte("no extension content"), "application/octet-stream"},
	}

	var attachmentPaths []string
	for filename, data := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		err := os.WriteFile(filePath, data.content, 0644)
		require.NoError(t, err)
		attachmentPaths = append(attachmentPaths, filePath)
	}

	tests := []struct {
		name              string
		attachments       []string
		serverResponse    string
		serverStatus      int
		expectedError     string
		validateRequest   func(t *testing.T, requestBody []byte)
		invalidAttachment bool
	}{
		{
			name:        "successful send with multiple attachments",
			attachments: attachmentPaths,
			serverResponse: `{
				"timestamp": 1234567890,
				"messageId": "msg123"
			}`,
			serverStatus: http.StatusOK,
			validateRequest: func(t *testing.T, requestBody []byte) {
				var req types.SendMessageRequest
				err := json.Unmarshal(requestBody, &req)
				require.NoError(t, err)

				// Should have 4 base64 attachments
				assert.Len(t, req.Base64Attachments, 4)

				// Verify each attachment is properly base64 encoded
				for _, attachment := range req.Base64Attachments {
					assert.NotEmpty(t, attachment)
					// Should be valid base64
					assert.NotContains(t, attachment, "\n") // No newlines in base64
				}
			},
		},
		{
			name:        "single attachment send",
			attachments: attachmentPaths[:1], // Just one attachment
			serverResponse: `{
				"timestamp": 1234567890,
				"messageId": "msg456"
			}`,
			serverStatus: http.StatusOK,
			validateRequest: func(t *testing.T, requestBody []byte) {
				var req types.SendMessageRequest
				err := json.Unmarshal(requestBody, &req)
				require.NoError(t, err)

				assert.Len(t, req.Base64Attachments, 1)
				assert.NotEmpty(t, req.Base64Attachments[0])
			},
		},
		{
			name:              "send with invalid attachment path",
			attachments:       []string{"/nonexistent/file.jpg"},
			invalidAttachment: true,
			expectedError:     "failed to encode attachment",
		},
		{
			name:          "send with mixed valid and invalid attachments",
			attachments:   []string{attachmentPaths[0], "/nonexistent/file.jpg"},
			expectedError: "failed to encode attachment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequestBody []byte

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/v2/send")

				// Capture request body for validation
				body, err := io.ReadAll(r.Body)
				if err == nil {
					capturedRequestBody = body
				}

				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != "" {
					if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
						t.Logf("Failed to write response: %v", err)
					}
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, "+1234567890", "test-device", tmpDir, nil)

			ctx := context.Background()
			response, err := client.SendMessage(ctx, "+0987654321", "Test message with attachments", tt.attachments)

			if tt.expectedError != "" || tt.invalidAttachment {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.NotZero(t, response.Timestamp)

				if tt.validateRequest != nil && len(capturedRequestBody) > 0 {
					tt.validateRequest(t, capturedRequestBody)
				}
			}
		})
	}
}

func TestReceiveMessages_AttachmentExtraction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "signal-receive-integration-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create some existing attachment files
	existingFiles := map[string][]byte{
		"existing1.jpg": []byte("existing jpeg"),
		"existing2":     []byte("existing no extension"),
	}

	for filename, content := range existingFiles {
		err := os.WriteFile(filepath.Join(tmpDir, filename), content, 0644)
		require.NoError(t, err)
	}

	tests := []struct {
		name                string
		serverResponse      string
		downloadResponses   map[string][]byte // For download server
		expectedMessages    int
		expectedAttachments map[int]int // message index -> attachment count
		validateMessage     func(t *testing.T, msg types.SignalMessage)
	}{
		{
			name: "message with existing attachments",
			serverResponse: `[{
				"envelope": {
					"source": "+1234567890",
					"timestamp": 1234567890000,
					"dataMessage": {
						"timestamp": 1234567890000,
						"message": "Check these files",
						"attachments": [
							{
								"id": "existing1",
								"filename": "photo.jpg",
								"contentType": "image/jpeg"
							}
						]
					}
				},
				"account": "+0987654321"
			}]`,
			expectedMessages:    1,
			expectedAttachments: map[int]int{0: 1},
			validateMessage: func(t *testing.T, msg types.SignalMessage) {
				assert.Equal(t, "Check these files", msg.Message)
				assert.Len(t, msg.Attachments, 1)
				assert.Contains(t, msg.Attachments[0], "existing1.jpg")
				assert.FileExists(t, msg.Attachments[0])
			},
		},
		{
			name: "message with downloadable attachments",
			serverResponse: `[{
				"envelope": {
					"source": "+1234567890",
					"timestamp": 1234567890000,
					"dataMessage": {
						"timestamp": 1234567890000,
						"message": "Downloaded files",
						"attachments": [
							{
								"id": "download1",
								"filename": "downloaded.pdf",
								"contentType": "application/pdf"
							}
						]
					}
				},
				"account": "+0987654321"
			}]`,
			downloadResponses: map[string][]byte{
				"download1": []byte("downloaded pdf content"),
			},
			expectedMessages:    1,
			expectedAttachments: map[int]int{0: 1},
			validateMessage: func(t *testing.T, msg types.SignalMessage) {
				assert.Equal(t, "Downloaded files", msg.Message)
				assert.Len(t, msg.Attachments, 1)
				assert.Contains(t, msg.Attachments[0], "download1")
				assert.FileExists(t, msg.Attachments[0])

				// Verify downloaded content
				content, err := os.ReadFile(msg.Attachments[0])
				assert.NoError(t, err)
				assert.Equal(t, []byte("downloaded pdf content"), content)
			},
		},
		{
			name: "message with mixed attachments",
			serverResponse: `[{
				"envelope": {
					"source": "+1234567890",
					"timestamp": 1234567890000,
					"dataMessage": {
						"timestamp": 1234567890000,
						"message": "Mixed attachments",
						"attachments": [
							{
								"id": "existing2",
								"filename": "local.bin",
								"contentType": "application/octet-stream"
							},
							{
								"id": "download2",
								"filename": "remote.txt",
								"contentType": "text/plain"
							}
						]
					}
				},
				"account": "+0987654321"
			}]`,
			downloadResponses: map[string][]byte{
				"download2": []byte("remote text content"),
			},
			expectedMessages:    1,
			expectedAttachments: map[int]int{0: 2},
			validateMessage: func(t *testing.T, msg types.SignalMessage) {
				assert.Equal(t, "Mixed attachments", msg.Message)
				assert.Len(t, msg.Attachments, 2)

				// Should have one existing file and one downloaded file
				existingFound := false
				downloadedFound := false

				for _, path := range msg.Attachments {
					if strings.Contains(path, "existing2") {
						existingFound = true
						assert.FileExists(t, path)
					} else if strings.Contains(path, "download2") {
						downloadedFound = true
						assert.FileExists(t, path)
						content, err := os.ReadFile(path)
						assert.NoError(t, err)
						assert.Equal(t, []byte("remote text content"), content)
					}
				}

				assert.True(t, existingFound, "Should find existing attachment")
				assert.True(t, downloadedFound, "Should find downloaded attachment")
			},
		},
		{
			name: "message with failed attachment downloads",
			serverResponse: `[{
				"envelope": {
					"source": "+1234567890",
					"timestamp": 1234567890000,
					"dataMessage": {
						"timestamp": 1234567890000,
						"message": "Failed downloads",
						"attachments": [
							{
								"id": "missing1",
								"filename": "missing.jpg",
								"contentType": "image/jpeg"
							},
							{
								"id": "missing2", 
								"filename": "missing.pdf",
								"contentType": "application/pdf"
							}
						]
					}
				},
				"account": "+0987654321"
			}]`,
			downloadResponses: map[string][]byte{
				// Returning 404s for these downloads by not including them
			},
			expectedMessages:    1,
			expectedAttachments: map[int]int{0: 0}, // No attachments when downloads fail
			validateMessage: func(t *testing.T, msg types.SignalMessage) {
				assert.Equal(t, "Failed downloads", msg.Message)
				assert.Empty(t, msg.Attachments, "Should have no attachments when downloads fail")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup main receive server
			mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/v1/receive/") {
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
						t.Logf("Failed to write response: %v", err)
					}
				}
			}))
			defer mainServer.Close()

			logger := logrus.New()
			logger.SetLevel(logrus.PanicLevel)

			// For tests with downloads, use a unified server that handles both operations
			var unifiedServer *httptest.Server
			if tt.downloadResponses != nil { // Create unified server if downloadResponses is specified (even if empty)
				unifiedServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "/v1/receive/") {
						w.WriteHeader(http.StatusOK)
						if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
							t.Logf("Failed to write response: %v", err)
						}
					} else if strings.Contains(r.URL.Path, "/v1/attachments/") {
						pathParts := strings.Split(r.URL.Path, "/")
						attachmentID := pathParts[len(pathParts)-1]

						if data, exists := tt.downloadResponses[attachmentID]; exists {
							w.WriteHeader(http.StatusOK)
							if _, err := w.Write(data); err != nil {
								t.Logf("Failed to write response: %v", err)
							}
						} else {
							w.WriteHeader(http.StatusNotFound)
						}
					}
				}))
				defer unifiedServer.Close()
			}

			// Use unified server if available, otherwise main server
			baseURL := mainServer.URL
			if unifiedServer != nil {
				baseURL = unifiedServer.URL
			}

			client := NewClientWithLogger(baseURL, "+0987654321", "test-device", tmpDir,
				&http.Client{Timeout: 30 * time.Second}, logger)

			ctx := context.Background()
			messages, err := client.ReceiveMessages(ctx, 5)

			require.NoError(t, err)
			assert.Len(t, messages, tt.expectedMessages)

			for i, msg := range messages {
				if expectedCount, exists := tt.expectedAttachments[i]; exists {
					assert.Len(t, msg.Attachments, expectedCount,
						"Message %d should have %d attachments", i, expectedCount)
				}

				if tt.validateMessage != nil {
					tt.validateMessage(t, msg)
				}
			}
		})
	}
}
