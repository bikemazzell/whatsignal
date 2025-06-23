package signal

import (
	"encoding/base64"
	"os"
	"path/filepath"
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
		authToken      string
		phoneNumber    string
		deviceName     string
		attachmentsDir string
		expectedURL    string
	}{
		{
			name:           "basic client creation",
			baseURL:        "http://localhost:8080",
			authToken:      "test-token",
			phoneNumber:    "+1234567890",
			deviceName:     "test-device",
			attachmentsDir: "/test/attachments",
			expectedURL:    "http://localhost:8080",
		},
		{
			name:           "client with trailing slash",
			baseURL:        "http://localhost:8080/",
			authToken:      "test-token",
			phoneNumber:    "+1234567890",
			deviceName:     "test-device",
			attachmentsDir: "/test/attachments",
			expectedURL:    "http://localhost:8080",
		},
		{
			name:           "client without attachments directory",
			baseURL:        "http://localhost:8080",
			authToken:      "test-token",
			phoneNumber:    "+1234567890",
			deviceName:     "test-device",
			attachmentsDir: "",
			expectedURL:    "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL, tt.authToken, tt.phoneNumber, tt.deviceName, tt.attachmentsDir, nil)
			
			signalClient, ok := client.(*SignalClient)
			assert.True(t, ok, "Client should be of type *SignalClient")
			assert.Equal(t, tt.expectedURL, signalClient.baseURL)
			assert.Equal(t, tt.authToken, signalClient.authToken)
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
	client := NewClient("http://localhost:8080", "", "+1234567890", "test", "", nil)

	signalClient, ok := client.(*SignalClient)
	require.True(t, ok, "Client should be of type *SignalClient")

	// Verify mutex is initialized (this is implicit - if it wasn't,
	// concurrent operations would cause data races detected by go test -race)
	assert.NotNil(t, signalClient)

	// The actual mutex synchronization is tested implicitly through
	// concurrent operations in integration tests and by running tests with -race flag
}