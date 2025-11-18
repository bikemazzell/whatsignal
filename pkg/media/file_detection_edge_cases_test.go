package media

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectFileTypeFromContent_EdgeCases(t *testing.T) {
	// Create temporary files with different content types for testing
	tmpDir, err := os.MkdirTemp("", "media-content-detection-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tests := []struct {
		name            string
		content         []byte
		filename        string
		expectExtension bool
	}{
		{
			name:            "JPEG signature",
			content:         []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46},
			filename:        "test.jpg",
			expectExtension: true,
		},
		{
			name:            "PNG signature",
			content:         []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			filename:        "test.png",
			expectExtension: true,
		},
		{
			name:            "PDF signature",
			content:         []byte{0x25, 0x50, 0x44, 0x46, 0x2D},
			filename:        "test.pdf",
			expectExtension: true,
		},
		{
			name:            "Empty content",
			content:         []byte{},
			filename:        "empty.txt",
			expectExtension: false,
		},
		{
			name:            "Unknown signature",
			content:         []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22},
			filename:        "unknown.bin",
			expectExtension: false,
		},
	}

	h := &handler{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tt.filename)
			err := os.WriteFile(filePath, tt.content, 0644)
			require.NoError(t, err)

			detectedExt, err := h.detectFileTypeFromContent(filePath)

			if tt.expectExtension {
				assert.NoError(t, err)
				assert.NotEmpty(t, detectedExt, "expected to detect extension")
			} else {
				// May return error or empty string for unknown/empty files
				if err == nil {
					assert.Empty(t, detectedExt, "expected no extension detection")
				}
			}
		})
	}
}

func TestDetectByFileSignature_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		content        []byte
		expectedFormat string
	}{
		{
			name:           "JPEG with full header",
			content:        []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01},
			expectedFormat: "jpg",
		},
		{
			name:           "PNG with full signature",
			content:        []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52},
			expectedFormat: "png",
		},
		{
			name:           "Content too short for any signature",
			content:        []byte{0x01, 0x02},
			expectedFormat: "",
		},
		{
			name:           "PDF with version info",
			content:        []byte{0x25, 0x50, 0x44, 0x46, 0x2D, 0x31, 0x2E, 0x34},
			expectedFormat: "pdf",
		},
		{
			name:           "GIF87a signature",
			content:        []byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61},
			expectedFormat: "gif",
		},
		{
			name:           "GIF89a signature",
			content:        []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61},
			expectedFormat: "gif",
		},
		{
			name:           "WebP signature",
			content:        append([]byte("RIFF"), append([]byte{0x10, 0x00, 0x00, 0x00}, []byte("WEBP")...)...),
			expectedFormat: "webp",
		},
		{
			name:           "False positive prevention - similar but wrong signature",
			content:        []byte{0xFF, 0xD8, 0xAA}, // Starts like JPEG but wrong
			expectedFormat: "",
		},
		{
			name:           "Unknown signature",
			content:        []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22},
			expectedFormat: "",
		},
		{
			name:           "Empty content",
			content:        []byte{},
			expectedFormat: "",
		},
		{
			name:           "MP4 with ftyp signature",
			content:        append([]byte{0x00, 0x00, 0x00, 0x20}, []byte("ftypisom")...),
			expectedFormat: "mp4",
		},
		{
			name:           "OGG signature",
			content:        []byte("OggS\x00\x02\x00\x00"),
			expectedFormat: "ogg",
		},
		{
			name:           "MP3 frame sync pattern",
			content:        []byte{0xFF, 0xFB, 0x90, 0x00}, // Valid MP3 frame header
			expectedFormat: "mp3",
		},
		{
			name:           "Non-audio binary pattern",
			content:        []byte{0xFF, 0x01, 0x50, 0x80}, // Starts with 0xFF but doesn't match MP3 or AAC
			expectedFormat: "",
		},
		{
			name:           "M4A with brand M4A",
			content:        append([]byte{0x00, 0x00, 0x00, 0x20}, []byte("ftypM4A ")...),
			expectedFormat: "m4a",
		},
		{
			name:           "ID3 MP3 signature",
			content:        []byte("ID3\x03\x00\x00\x00"),
			expectedFormat: "mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{}
			format := h.detectByFileSignature(tt.content)
			assert.Equal(t, tt.expectedFormat, format, "format mismatch for %s", tt.name)
		})
	}
}

func TestFileDetection_ErrorCases(t *testing.T) {
	h := &handler{}

	t.Run("invalid file path", func(t *testing.T) {
		// Test with non-existent file
		ext, err := h.detectFileTypeFromContent("/nonexistent/file.txt")
		assert.Error(t, err)
		assert.Empty(t, ext)
	})

	t.Run("directory instead of file", func(t *testing.T) {
		// Create temporary directory
		tmpDir, err := os.MkdirTemp("", "media-detection-test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		// Try to detect file type of directory
		ext, err := h.detectFileTypeFromContent(tmpDir)
		assert.Error(t, err)
		assert.Empty(t, ext)
	})

	t.Run("very large file signature detection", func(t *testing.T) {
		// Create temporary file with large content but valid signature
		tmpDir, err := os.MkdirTemp("", "media-detection-test")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		// Create content with JPEG signature followed by large data
		content := make([]byte, 1024*1024) // 1MB
		copy(content[:10], []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46})

		filePath := filepath.Join(tmpDir, "large.jpg")
		err = os.WriteFile(filePath, content, 0644)
		require.NoError(t, err)

		ext, err := h.detectFileTypeFromContent(filePath)
		assert.NoError(t, err)
		assert.NotEmpty(t, ext, "should detect JPEG extension even for large files")
	})
}
