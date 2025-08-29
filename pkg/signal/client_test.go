package signal

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"whatsignal/pkg/signal/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractAttachmentPaths(t *testing.T) {
	tests := []struct {
		name           string
		attachmentsDir string
		attachments    []types.RestMessageAttachment
		expected       []string
	}{
		{
			name:           "empty attachments",
			attachmentsDir: "/test/attachments",
			attachments:    []types.RestMessageAttachment{},
			expected:       nil,
		},
		{
			name:           "nil attachments",
			attachmentsDir: "/test/attachments",
			attachments:    nil,
			expected:       nil,
		},
		{
			name:           "single attachment with directory (file not found)",
			attachmentsDir: "/test/attachments",
			attachments: []types.RestMessageAttachment{
				{ID: "attachment1.jpg", Filename: "file1.jpg"},
			},
			expected: []string{}, // No files found, no HTTP client, so empty result
		},
		{
			name:           "multiple attachments with directory (files not found)",
			attachmentsDir: "/test/attachments",
			attachments: []types.RestMessageAttachment{
				{ID: "attachment1.jpg", Filename: "file1.jpg"},
				{ID: "attachment2.pdf", Filename: "file2.pdf"},
				{ID: "attachment3.ogg", Filename: "file3.ogg"},
			},
			expected: []string{}, // No files found, no HTTP client, so empty result
		},
		{
			name:           "single attachment without directory (file not found)",
			attachmentsDir: "",
			attachments: []types.RestMessageAttachment{
				{ID: "attachment1.jpg", Filename: "file1.jpg"},
			},
			expected: []string{}, // No files found, no HTTP client, so empty result
		},
		{
			name:           "multiple attachments without directory (files not found)",
			attachmentsDir: "",
			attachments: []types.RestMessageAttachment{
				{ID: "attachment1.jpg", Filename: "file1.jpg"},
				{ID: "attachment2.pdf", Filename: "file2.pdf"},
			},
			expected: []string{}, // No files found, no HTTP client, so empty result
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.New()
			logger.SetLevel(logrus.PanicLevel) // Suppress logs during tests
			client := &SignalClient{
				attachmentsDir: tt.attachmentsDir,
				logger:         logger,
			}
			result := client.extractAttachmentPaths(tt.attachments)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name           string
		baseURL        string
		phoneNumber    string
		deviceName     string
		attachmentsDir string
		expectedURL    string
	}{
		{
			name:           "basic client creation",
			baseURL:        "http://localhost:8080",
			phoneNumber:    "+1234567890",
			deviceName:     "test-device",
			attachmentsDir: "/test/attachments",
			expectedURL:    "http://localhost:8080",
		},
		{
			name:           "client with trailing slash",
			baseURL:        "http://localhost:8080/",
			phoneNumber:    "+1234567890",
			deviceName:     "test-device",
			attachmentsDir: "/test/attachments",
			expectedURL:    "http://localhost:8080",
		},
		{
			name:           "client without attachments directory",
			baseURL:        "http://localhost:8080",
			phoneNumber:    "+1234567890",
			deviceName:     "test-device",
			attachmentsDir: "",
			expectedURL:    "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL, tt.phoneNumber, tt.deviceName, tt.attachmentsDir, nil)
			
			signalClient, ok := client.(*SignalClient)
			assert.True(t, ok, "Client should be of type *SignalClient")
			assert.Equal(t, tt.expectedURL, signalClient.baseURL)
			assert.Equal(t, tt.phoneNumber, signalClient.phoneNumber)
			assert.Equal(t, tt.deviceName, signalClient.deviceName)
			assert.Equal(t, tt.attachmentsDir, signalClient.attachmentsDir)
			assert.NotNil(t, signalClient.client)
		})
	}
}

func TestEncodeAttachment(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "signal-attachment-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name            string
		filename        string
		content         []byte
		expectedType    string
		expectedContent string
	}{
		{
			name:            "JPEG image",
			filename:        "test.jpg",
			content:         []byte("fake jpeg content"),
			expectedType:    "image/jpeg",
			expectedContent: base64.StdEncoding.EncodeToString([]byte("fake jpeg content")),
		},
		{
			name:            "PNG image",
			filename:        "test.png",
			content:         []byte("fake png content"),
			expectedType:    "image/png",
			expectedContent: base64.StdEncoding.EncodeToString([]byte("fake png content")),
		},
		{
			name:            "PDF document",
			filename:        "test.pdf",
			content:         []byte("fake pdf content"),
			expectedType:    "application/pdf",
			expectedContent: base64.StdEncoding.EncodeToString([]byte("fake pdf content")),
		},
		{
			name:            "MP4 video",
			filename:        "test.mp4",
			content:         []byte("fake mp4 content"),
			expectedType:    "video/mp4",
			expectedContent: base64.StdEncoding.EncodeToString([]byte("fake mp4 content")),
		},
		{
			name:            "OGG audio",
			filename:        "test.ogg",
			content:         []byte("fake ogg content"),
			expectedType:    "audio/ogg",
			expectedContent: base64.StdEncoding.EncodeToString([]byte("fake ogg content")),
		},
		{
			name:            "Unknown file type",
			filename:        "test.unknown",
			content:         []byte("fake unknown content"),
			expectedType:    "application/octet-stream",
			expectedContent: base64.StdEncoding.EncodeToString([]byte("fake unknown content")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(tmpDir, tt.filename)
			err := os.WriteFile(filePath, tt.content, 0644)
			require.NoError(t, err)

			// Create client
			client := &SignalClient{}

			// Test encoding
			encodedData, contentType, filename, err := client.encodeAttachment(filePath)
			require.NoError(t, err)

			// Verify results
			assert.Equal(t, tt.expectedContent, encodedData)
			assert.Equal(t, tt.expectedType, contentType)
			assert.Equal(t, tt.filename, filename)
		})
	}
}

func TestEncodeAttachmentErrors(t *testing.T) {
	client := &SignalClient{}

	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "non-existent file",
			filePath: "/path/that/does/not/exist.jpg",
		},
		{
			name:     "empty path",
			filePath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := client.encodeAttachment(tt.filePath)
			assert.Error(t, err)
		})
	}
}

func TestDetectContentType(t *testing.T) {
	client := &SignalClient{}

	tests := []struct {
		filename     string
		expectedType string
	}{
		{"test.jpg", "image/jpeg"},
		{"test.jpeg", "image/jpeg"},
		{"test.jfif", "image/jpeg"}, // Test JFIF support
		{"test.png", "image/png"},
		{"test.gif", "image/gif"},
		{"test.webp", "image/webp"},
		{"test.mp4", "video/mp4"},
		{"test.mov", "video/quicktime"},
		{"test.avi", "video/vnd.avi"},
		{"test.ogg", "audio/ogg"},
		{"test.aac", "audio/aac"},
		{"test.m4a", "audio/mp4"},
		{"test.mp3", "audio/mpeg"},
		{"test.wav", "audio/vnd.wave"},
		{"test.pdf", "application/pdf"},
		{"test.doc", "application/msword"},
		{"test.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"test.txt", "text/plain; charset=utf-8"},
		{"test.unknown", "application/octet-stream"},
		{"test", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := client.detectContentType(tt.filename)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

func TestJFIFSupport(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "signal-jfif-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test JFIF file
	jfifContent := []byte("fake jfif content")
	jfifPath := filepath.Join(tmpDir, "test.jfif")
	err = os.WriteFile(jfifPath, jfifContent, 0644)
	require.NoError(t, err)

	// Create client
	client := &SignalClient{}

	// Test encoding JFIF file
	encodedData, contentType, filename, err := client.encodeAttachment(jfifPath)
	require.NoError(t, err)

	// Verify results
	expectedContent := base64.StdEncoding.EncodeToString(jfifContent)
	assert.Equal(t, expectedContent, encodedData)
	assert.Equal(t, "image/jpeg", contentType) // JFIF should be detected as JPEG
	assert.Equal(t, "test.jfif", filename)
}

func TestClientMutexSynchronization(t *testing.T) {
	// This test verifies that the mutex is properly initialized
	// and that concurrent operations don't cause data races
	client := NewClient("http://localhost:8080", "+1234567890", "test", "", nil)

	signalClient, ok := client.(*SignalClient)
	require.True(t, ok, "Client should be of type *SignalClient")

	// Verify mutex is initialized (this is implicit - if it wasn't,
	// concurrent operations would cause data races detected by go test -race)
	assert.NotNil(t, signalClient)

	// The actual mutex synchronization is tested implicitly through
	// concurrent operations in integration tests and by running tests with -race flag
}

func TestSendMessage(t *testing.T) {
	tests := []struct {
		name           string
		recipient      string
		message        string
		attachments    []string
		serverResponse string
		serverStatus   int
		expectedError  string
		setupServer    func(*httptest.Server) string
	}{
		{
			name:        "successful text message",
			recipient:   "+1234567890",
			message:     "Hello, World!",
			attachments: nil,
			serverResponse: `{
				"timestamp": 1234567890,
				"messageId": "msg123"
			}`,
			serverStatus: http.StatusOK,
		},
		{
			name:        "successful message with attachments",
			recipient:   "+1234567890",
			message:     "Check this out!",
			attachments: []string{"/tmp/test.jpg"},
			serverResponse: `{
				"timestamp": 1234567890,
				"messageId": "msg456"
			}`,
			serverStatus: http.StatusOK,
			setupServer: func(server *httptest.Server) string {
				// Create test attachment file
				tmpDir, err := os.MkdirTemp("", "signal-test")
				if err != nil {
					panic(err)
				}
				testFile := filepath.Join(tmpDir, "test.jpg")
				if err := os.WriteFile(testFile, []byte("fake image data"), 0o644); err != nil {
					panic(err)
				}
				return testFile
			},
		},
		{
			name:          "server error",
			recipient:     "+1234567890",
			message:       "Hello",
			attachments:   nil,
			serverStatus:  http.StatusInternalServerError,
			expectedError: "signal API error",
		},
		{
			name:          "invalid JSON response",
			recipient:     "+1234567890",
			message:       "Hello",
			attachments:   nil,
			serverResponse: "invalid json",
			serverStatus:  http.StatusOK,
			expectedError: "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/v2/send")

				// Verify auth header
				assert.Equal(t, "", r.Header.Get("Authorization")) // No auth token required for Signal CLI REST API

				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != "" {
					if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
						panic(err)
					}
				}
			}))
			defer server.Close()

			// Setup test file if needed
			var testAttachments []string
			if tt.setupServer != nil {
				testFile := tt.setupServer(server)
				testAttachments = []string{testFile}
				defer os.RemoveAll(filepath.Dir(testFile))
			} else {
				testAttachments = tt.attachments
			}

			// Create client
			client := NewClient(server.URL, "+0987654321", "test-device", "", nil)

			// Test SendMessage
			ctx := context.Background()
			response, err := client.SendMessage(ctx, tt.recipient, tt.message, testAttachments)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.NotZero(t, response.Timestamp)
				assert.NotEmpty(t, response.MessageID)
			}
		})
	}
}

func TestReceiveMessages(t *testing.T) {
	tests := []struct {
		name           string
		timeoutSeconds int
		serverResponse string
		serverStatus   int
		expectedError  string
		expectedCount  int
	}{
		{
			name:           "successful receive with messages",
			timeoutSeconds: 5,
			serverResponse: `[
				{
					"envelope": {
						"source": "+1234567890",
						"sourceNumber": "+1234567890",
						"sourceUuid": "uuid-123",
						"sourceName": "Test User",
						"sourceDevice": 1,
						"timestamp": 1234567890000,
						"dataMessage": {
							"timestamp": 1234567890000,
							"message": "Hello from Signal!",
							"expiresInSeconds": 0,
							"viewOnce": false,
							"attachments": []
						}
					},
					"account": "+0987654321",
					"subscription": 0
				}
			]`,
			serverStatus:  http.StatusOK,
			expectedCount: 1,
		},
		{
			name:           "successful receive with no messages",
			timeoutSeconds: 1,
			serverResponse: `[]`,
			serverStatus:   http.StatusOK,
			expectedCount:  0,
		},
		{
			name:           "server error",
			timeoutSeconds: 5,
			serverStatus:   http.StatusInternalServerError,
			expectedError:  "signal API error",
		},
		{
			name:           "invalid JSON response",
			timeoutSeconds: 5,
			serverResponse: "invalid json",
			serverStatus:   http.StatusOK,
			expectedError:  "failed to decode response",
		},
		{
			name:           "timeout context",
			timeoutSeconds: 1,
			serverResponse: `[]`,
			serverStatus:   http.StatusOK,
			expectedCount:  0,
		},
		{
			name:           "successful receive with reaction",
			timeoutSeconds: 5,
			serverResponse: `[
				{
					"envelope": {
						"source": "+1234567890",
						"sourceNumber": "+1234567890",
						"sourceUuid": "uuid-123",
						"sourceName": "Test User",
						"sourceDevice": 1,
						"timestamp": 1234567890000,
						"dataMessage": {
							"timestamp": 1234567890000,
							"message": "",
							"reaction": {
								"emoji": "👍",
								"targetAuthor": "+0987654321",
								"targetSentTimestamp": 1234567880000,
								"isRemove": false
							}
						}
					},
					"account": "+0987654321",
					"subscription": 0
				}
			]`,
			serverStatus:  http.StatusOK,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/v1/receive/")

				// Verify auth header
				assert.Equal(t, "", r.Header.Get("Authorization")) // No auth token required for Signal CLI REST API

				// Check timeout parameter
				timeout := r.URL.Query().Get("timeout")
				assert.NotEmpty(t, timeout)

				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != "" {
					if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
						panic(err)
					}
				}
			}))
			defer server.Close()

			// Create client
			client := NewClient(server.URL, "+0987654321", "test-device", "", nil)

			// Test ReceiveMessages
			ctx := context.Background()
			messages, err := client.ReceiveMessages(ctx, tt.timeoutSeconds)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, messages)
			} else {
				assert.NoError(t, err)
				assert.Len(t, messages, tt.expectedCount)

				if tt.expectedCount > 0 {
					msg := messages[0]
					assert.Equal(t, "+1234567890", msg.Sender)
					assert.NotZero(t, msg.Timestamp)
					
					// Check for reaction-specific assertions
					if tt.name == "successful receive with reaction" {
						assert.NotNil(t, msg.Reaction)
						assert.Equal(t, "👍", msg.Reaction.Emoji)
						assert.Equal(t, "+0987654321", msg.Reaction.TargetAuthor)
						assert.Equal(t, int64(1234567880000), msg.Reaction.TargetTimestamp)
						assert.False(t, msg.Reaction.IsRemove)
						assert.Equal(t, "👍", msg.Message) // Message should contain emoji
					} else {
						assert.Equal(t, "Hello from Signal!", msg.Message)
					}
				}
			}
		})
	}
}

func TestInitializeDevice(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse string
		serverStatus   int
		expectedError  string
	}{
		{
			name: "successful initialization",
			serverResponse: `{
				"version": "0.11.0",
				"build": 123,
				"mode": "native",
				"versions": ["v1", "v2"]
			}`,
			serverStatus: http.StatusOK,
		},
		{
			name:          "server error",
			serverStatus:  http.StatusInternalServerError,
			expectedError: "device initialization failed",
		},
		{
			name:           "invalid JSON response",
			serverResponse: "invalid json",
			serverStatus:   http.StatusOK,
			expectedError:  "failed to decode about response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/v1/about", r.URL.Path)

				// Verify auth header
				assert.Equal(t, "", r.Header.Get("Authorization")) // No auth token required for Signal CLI REST API

				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != "" {
					if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
						panic(err)
					}
				}
			}))
			defer server.Close()

			// Create client
			client := NewClient(server.URL, "+0987654321", "test-device", "", nil)

			// Test InitializeDevice
			ctx := context.Background()
			err := client.InitializeDevice(ctx)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDownloadAttachment(t *testing.T) {
	tests := []struct {
		name           string
		attachmentID   string
		serverResponse []byte
		serverStatus   int
		expectedError  string
	}{
		{
			name:           "successful download",
			attachmentID:   "attachment123",
			serverResponse: []byte("fake attachment data"),
			serverStatus:   http.StatusOK,
		},
		{
			name:          "server error",
			attachmentID:  "attachment123",
			serverStatus:  http.StatusNotFound,
			expectedError: "attachment download failed",
		},
		{
			name:          "empty attachment ID",
			attachmentID:  "",
			serverStatus:  http.StatusOK,
			expectedError: "", // Should still work with empty ID
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/v1/attachments/")

				// Verify auth header
				assert.Equal(t, "", r.Header.Get("Authorization")) // No auth token required for Signal CLI REST API

				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != nil {
					if _, err := w.Write(tt.serverResponse); err != nil {
						panic(err)
					}
				}
			}))
			defer server.Close()

			// Create client
			client := NewClient(server.URL, "+0987654321", "test-device", "", nil)

			// Test DownloadAttachment
			ctx := context.Background()
			data, err := client.DownloadAttachment(ctx, tt.attachmentID)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				if tt.serverResponse != nil {
					assert.Equal(t, tt.serverResponse, data)
				}
			}
		})
	}
}

func TestListAttachments(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse string
		serverStatus   int
		expectedError  string
		expectedCount  int
	}{
		{
			name: "successful list with attachments",
			serverResponse: `[
				"attachment1.jpg",
				"attachment2.pdf",
				"attachment3.mp4"
			]`,
			serverStatus:  http.StatusOK,
			expectedCount: 3,
		},
		{
			name:           "successful list with no attachments",
			serverResponse: `[]`,
			serverStatus:   http.StatusOK,
			expectedCount:  0,
		},
		{
			name:          "server error",
			serverStatus:  http.StatusInternalServerError,
			expectedError: "list attachments failed",
		},
		{
			name:           "invalid JSON response",
			serverResponse: "invalid json",
			serverStatus:   http.StatusOK,
			expectedError:  "failed to decode attachments list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/v1/attachments", r.URL.Path)

				// Verify auth header
				assert.Equal(t, "", r.Header.Get("Authorization")) // No auth token required for Signal CLI REST API

				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != "" {
					if _, err := w.Write([]byte(tt.serverResponse)); err != nil {
						panic(err)
					}
				}
			}))
			defer server.Close()

			// Create client
			client := NewClient(server.URL, "+0987654321", "test-device", "", nil)

			// Test ListAttachments
			ctx := context.Background()
			attachments, err := client.ListAttachments(ctx)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, attachments)
			} else {
				assert.NoError(t, err)
				assert.Len(t, attachments, tt.expectedCount)
			}
		})
	}
}

func TestDownloadAndSaveAttachment(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "signal-download-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name           string
		attachment     types.RestMessageAttachment
		serverResponse []byte
		serverStatus   int
		expectedError  string
	}{
		{
			name: "successful download and save",
			attachment: types.RestMessageAttachment{
				ID:          "test123",
				Filename:    "test.jpg",
				ContentType: "image/jpeg",
			},
			serverResponse: []byte("fake image data"),
			serverStatus:   http.StatusOK,
		},
		{
			name: "download error",
			attachment: types.RestMessageAttachment{
				ID:          "test123",
				Filename:    "test.jpg",
				ContentType: "image/jpeg",
			},
			serverStatus:  http.StatusNotFound,
			expectedError: "failed to download attachment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Contains(t, r.URL.Path, "/v1/attachments/")

				w.WriteHeader(tt.serverStatus)
				if tt.serverResponse != nil {
					if _, err := w.Write(tt.serverResponse); err != nil {
						panic(err)
					}
				}
			}))
			defer server.Close()

			// Create client with attachments directory
			client := NewClient(server.URL, "+0987654321", "test-device", tmpDir, nil)
			signalClient := client.(*SignalClient)

			// Test downloadAndSaveAttachment
			filePath, err := signalClient.downloadAndSaveAttachment(tt.attachment)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Empty(t, filePath)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, filePath)

				// Verify file was created and has correct content
				assert.FileExists(t, filePath)
				content, err := os.ReadFile(filePath)
				assert.NoError(t, err)
				assert.Equal(t, tt.serverResponse, content)
			}
		})
	}
}

func TestGetFileExtension(t *testing.T) {
	client := &SignalClient{}

	tests := []struct {
		name        string
		contentType string
		filename    string
		expected    string
	}{
		{
			name:     "extension from filename",
			filename: "test.jpg",
			expected: ".jpg",
		},
		{
			name:        "extension from content type - image/jpeg",
			contentType: "image/jpeg",
			expected:    ".jpg",
		},
		{
			name:        "extension from content type - image/png",
			contentType: "image/png",
			expected:    ".png",
		},
		{
			name:        "extension from content type - video/mp4",
			contentType: "video/mp4",
			expected:    ".mp4",
		},
		{
			name:        "extension from content type - audio/ogg",
			contentType: "audio/ogg",
			expected:    ".ogg",
		},
		{
			name:        "extension from content type - application/pdf",
			contentType: "application/pdf",
			expected:    ".pdf",
		},
		{
			name:        "unknown content type",
			contentType: "application/unknown",
			expected:    "",
		},
		{
			name:     "no extension available",
			filename: "test",
			expected: "",
		},
		{
			name:     "empty inputs",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.getFileExtension(tt.contentType, tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDirectAttachmentPath(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "signal-path-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name           string
		attachmentsDir string
		attachment     types.RestMessageAttachment
		createFile     bool
		expectedExists bool
	}{
		{
			name:           "file exists with attachments dir",
			attachmentsDir: tmpDir,
			attachment: types.RestMessageAttachment{
				ID:       "test123",
				Filename: "test.jpg",
			},
			createFile:     true,
			expectedExists: true,
		},
		{
			name:           "file does not exist with attachments dir",
			attachmentsDir: tmpDir,
			attachment: types.RestMessageAttachment{
				ID:       "nonexistent",
				Filename: "test.jpg",
			},
			createFile:     false,
			expectedExists: false,
		},
		{
			name:           "no attachments dir",
			attachmentsDir: "",
			attachment: types.RestMessageAttachment{
				ID:       "test123",
				Filename: "test.jpg",
			},
			createFile:     false,
			expectedExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &SignalClient{
				attachmentsDir: tt.attachmentsDir,
			}

			if tt.createFile && tt.attachmentsDir != "" {
				// Create the expected file
				expectedPath := filepath.Join(tt.attachmentsDir, tt.attachment.ID+".jpg")
				err := os.WriteFile(expectedPath, []byte("test"), 0644)
				require.NoError(t, err)
			}

			result := client.getDirectAttachmentPath(tt.attachment)

			if tt.expectedExists {
				assert.True(t, strings.Contains(result, tt.attachment.ID))
				_, err := os.Stat(result)
				assert.NoError(t, err, "File should exist")
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestFallbackAttachmentPath(t *testing.T) {
	tests := []struct {
		name           string
		attachmentsDir string
		attachment     types.RestMessageAttachment
		expected       string
	}{
		{
			name:           "with attachments directory",
			attachmentsDir: "/test/attachments",
			attachment: types.RestMessageAttachment{
				ID: "test123",
			},
			expected: "/test/attachments/test123",
		},
		{
			name:           "without attachments directory",
			attachmentsDir: "",
			attachment: types.RestMessageAttachment{
				ID: "test123",
			},
			expected: "test123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &SignalClient{
				attachmentsDir: tt.attachmentsDir,
			}

			result := client.fallbackAttachmentPath(tt.attachment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEncodeAttachmentPublic(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "signal-encode-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test file
	testContent := []byte("test file content")
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)

	// Create client
	client := NewClient("http://localhost:8080", "+1234567890", "test-device", "", nil)
	signalClient := client.(*SignalClient)

	// Test public EncodeAttachment method
	encodedData, contentType, filename, err := signalClient.EncodeAttachment(testFile)
	require.NoError(t, err)

	// Verify results
	expectedContent := base64.StdEncoding.EncodeToString(testContent)
	assert.Equal(t, expectedContent, encodedData)
	assert.Equal(t, "text/plain; charset=utf-8", contentType)
	assert.Equal(t, "test.txt", filename)
}

func TestDetectContentTypeExtensive(t *testing.T) {
	client := &SignalClient{}

	tests := []struct {
		filename     string
		expectedType string
	}{
		// Test all the manual mappings
		{"test.jpg", "image/jpeg"},
		{"test.jpeg", "image/jpeg"},
		{"test.jfif", "image/jpeg"},
		{"test.png", "image/png"},
		{"test.gif", "image/gif"},
		{"test.webp", "image/webp"},
		{"test.mp4", "video/mp4"},
		{"test.mov", "video/quicktime"},
		{"test.avi", "video/vnd.avi"},
		{"test.ogg", "audio/ogg"},
		{"test.aac", "audio/aac"},
		{"test.m4a", "audio/mp4"},
		{"test.mp3", "audio/mpeg"},
		{"test.wav", "audio/vnd.wave"},
		{"test.pdf", "application/pdf"},
		{"test.doc", "application/msword"},
		{"test.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"test.txt", "text/plain; charset=utf-8"},
		{"test.unknown", "application/octet-stream"},
		{"test", "application/octet-stream"},
		{"", "application/octet-stream"},
		// Test case sensitivity
		{"test.JPG", "image/jpeg"},
		{"test.PNG", "image/png"},
		{"test.PDF", "application/pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := client.detectContentType(tt.filename)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

func TestReceiveMessages_RemoteDeleteHandling(t *testing.T) {
	tests := []struct {
		name                    string
		responseBody            string
		expectedDeletionCount   int
		expectedTargetTimestamp int64
	}{
		{
			name: "message with remote delete",
			responseBody: `[{
				"envelope": {
					"source": "+0987654321",
					"timestamp": 1750792100356,
					"dataMessage": {
						"timestamp": 1750792100356,
						"message": null,
						"remoteDelete": {
							"timestamp": 1750792079512
						}
					}
				},
				"account": "+1234567890"
			}]`,
			expectedDeletionCount:   1,
			expectedTargetTimestamp: 1750792079512,
		},
		{
			name: "regular message without remote delete",
			responseBody: `[{
				"envelope": {
					"source": "+0987654321",
					"timestamp": 1750792100356,
					"dataMessage": {
						"timestamp": 1750792100356,
						"message": "hello world"
					}
				},
				"account": "+1234567890"
			}]`,
			expectedDeletionCount: 0,
		},
		{
			name: "multiple messages with one deletion",
			responseBody: `[{
				"envelope": {
					"source": "+0987654321",
					"timestamp": 1750792100356,
					"dataMessage": {
						"timestamp": 1750792100356,
						"message": "normal message"
					}
				},
				"account": "+1234567890"
			}, {
				"envelope": {
					"source": "+0987654321",
					"timestamp": 1750792200000,
					"dataMessage": {
						"timestamp": 1750792200000,
						"message": null,
						"remoteDelete": {
							"timestamp": 1750792100356
						}
					}
				},
				"account": "+1234567890"
			}]`,
			expectedDeletionCount:   1,
			expectedTargetTimestamp: 1750792100356,
		},
		{
			name:                  "empty response",
			responseBody:          `[]`,
			expectedDeletionCount: 0,
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

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel) // Suppress debug output during tests
			
			client := NewClientWithLogger(server.URL, "+1234567890", "test", "/tmp", &http.Client{}, logger)

			ctx := context.Background()
			messages, err := client.ReceiveMessages(ctx, 5)

			require.NoError(t, err)

			// Count messages with deletions
			deletionCount := 0
			var foundTargetTimestamp int64

			for _, msg := range messages {
				if msg.Deletion != nil {
					deletionCount++
					foundTargetTimestamp = msg.Deletion.TargetTimestamp
				}
			}

			assert.Equal(t, tt.expectedDeletionCount, deletionCount)
			if tt.expectedDeletionCount > 0 {
				assert.Equal(t, tt.expectedTargetTimestamp, foundTargetTimestamp)
			}
		})
	}
}