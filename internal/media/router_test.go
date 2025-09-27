package media

import (
	"testing"

	"whatsignal/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestNewRouter(t *testing.T) {
	config := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Image:    []string{"jpg", "png"},
			Video:    []string{"mp4", "avi"},
			Voice:    []string{"mp3", "wav"},
			Document: []string{"pdf", "doc"},
		},
	}

	router := NewRouter(config)
	assert.NotNil(t, router)

	// Verify it implements the Router interface
	var _ Router = router
}

func TestGetMediaType(t *testing.T) {
	config := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Image:    []string{"jpg", "png", "jpeg", "gif"},
			Video:    []string{"mp4", "avi", "mov", "mkv"},
			Voice:    []string{"mp3", "wav", "ogg", "m4a"},
			Document: []string{"pdf", "doc", "docx", "txt"},
		},
	}

	router := NewRouter(config)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		// Image files
		{
			name:     "JPEG image",
			path:     "/path/to/image.jpg",
			expected: "image",
		},
		{
			name:     "PNG image",
			path:     "/path/to/image.png",
			expected: "image",
		},
		{
			name:     "GIF image",
			path:     "/path/to/animation.gif",
			expected: "image",
		},
		{
			name:     "JPEG with uppercase extension",
			path:     "/path/to/image.JPG",
			expected: "image",
		},

		// Video files
		{
			name:     "MP4 video",
			path:     "/path/to/video.mp4",
			expected: "video",
		},
		{
			name:     "AVI video",
			path:     "/path/to/video.avi",
			expected: "video",
		},
		{
			name:     "MOV video",
			path:     "/path/to/video.mov",
			expected: "video",
		},

		// Voice/Audio files
		{
			name:     "MP3 audio",
			path:     "/path/to/audio.mp3",
			expected: "voice",
		},
		{
			name:     "WAV audio",
			path:     "/path/to/audio.wav",
			expected: "voice",
		},
		{
			name:     "OGG audio",
			path:     "/path/to/audio.ogg",
			expected: "voice",
		},

		// Document files (explicit)
		{
			name:     "PDF document",
			path:     "/path/to/document.pdf",
			expected: "document",
		},
		{
			name:     "Word document",
			path:     "/path/to/document.doc",
			expected: "document",
		},

		// Default to document for unknown types
		{
			name:     "Unknown extension",
			path:     "/path/to/file.xyz",
			expected: "document",
		},
		{
			name:     "No extension",
			path:     "/path/to/file",
			expected: "document",
		},
		{
			name:     "Text file not in allowed types",
			path:     "/path/to/readme.md",
			expected: "document",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.GetMediaType(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsImageAttachment(t *testing.T) {
	config := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Image: []string{"jpg", "png", "jpeg", "gif", "bmp"},
		},
	}

	router := NewRouter(config)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "JPEG image",
			path:     "/path/image.jpg",
			expected: true,
		},
		{
			name:     "PNG image",
			path:     "/path/image.png",
			expected: true,
		},
		{
			name:     "Uppercase extension",
			path:     "/path/image.JPG",
			expected: true,
		},
		{
			name:     "Mixed case extension",
			path:     "/path/image.Png",
			expected: true,
		},
		{
			name:     "Non-image file",
			path:     "/path/video.mp4",
			expected: false,
		},
		{
			name:     "No extension",
			path:     "/path/file",
			expected: false,
		},
		{
			name:     "Unknown image type",
			path:     "/path/image.tiff",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.IsImageAttachment(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsVideoAttachment(t *testing.T) {
	config := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Video: []string{"mp4", "avi", "mov", "mkv", "webm"},
		},
	}

	router := NewRouter(config)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "MP4 video",
			path:     "/path/video.mp4",
			expected: true,
		},
		{
			name:     "AVI video",
			path:     "/path/video.avi",
			expected: true,
		},
		{
			name:     "Uppercase extension",
			path:     "/path/video.MP4",
			expected: true,
		},
		{
			name:     "Non-video file",
			path:     "/path/image.jpg",
			expected: false,
		},
		{
			name:     "Audio file",
			path:     "/path/audio.mp3",
			expected: false,
		},
		{
			name:     "Unknown video type",
			path:     "/path/video.wmv",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.IsVideoAttachment(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsVoiceAttachment(t *testing.T) {
	config := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Voice: []string{"mp3", "wav", "ogg", "m4a", "aac"},
		},
	}

	router := NewRouter(config)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "MP3 audio",
			path:     "/path/audio.mp3",
			expected: true,
		},
		{
			name:     "WAV audio",
			path:     "/path/audio.wav",
			expected: true,
		},
		{
			name:     "Uppercase extension",
			path:     "/path/audio.MP3",
			expected: true,
		},
		{
			name:     "Non-audio file",
			path:     "/path/image.jpg",
			expected: false,
		},
		{
			name:     "Video file",
			path:     "/path/video.mp4",
			expected: false,
		},
		{
			name:     "Unknown audio type",
			path:     "/path/audio.flac",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.IsVoiceAttachment(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDocumentAttachment(t *testing.T) {
	config := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Document: []string{"pdf", "doc", "docx", "txt", "rtf"},
		},
	}

	router := NewRouter(config)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "PDF document",
			path:     "/path/document.pdf",
			expected: true,
		},
		{
			name:     "Word document",
			path:     "/path/document.doc",
			expected: true,
		},
		{
			name:     "Uppercase extension",
			path:     "/path/document.PDF",
			expected: true,
		},
		{
			name:     "Text file",
			path:     "/path/readme.txt",
			expected: true,
		},
		{
			name:     "Non-document file",
			path:     "/path/image.jpg",
			expected: false,
		},
		{
			name:     "Unknown document type",
			path:     "/path/document.odt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.IsDocumentAttachment(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMaxSizeForMediaType(t *testing.T) {
	config := models.MediaConfig{
		MaxSizeMB: models.MediaSizeLimits{
			Image:    10,  // 10MB
			Video:    100, // 100MB
			Voice:    50,  // 50MB
			Document: 25,  // 25MB
		},
	}

	router := NewRouter(config)

	tests := []struct {
		name      string
		mediaType string
		expected  int64
	}{
		{
			name:      "Image max size",
			mediaType: "image",
			expected:  10 * 1024 * 1024, // 10MB in bytes
		},
		{
			name:      "Video max size",
			mediaType: "video",
			expected:  100 * 1024 * 1024, // 100MB in bytes
		},
		{
			name:      "Voice max size",
			mediaType: "voice",
			expected:  50 * 1024 * 1024, // 50MB in bytes
		},
		{
			name:      "Document max size",
			mediaType: "document",
			expected:  25 * 1024 * 1024, // 25MB in bytes
		},
		{
			name:      "Unknown media type defaults to document",
			mediaType: "unknown",
			expected:  25 * 1024 * 1024, // 25MB in bytes (document default)
		},
		{
			name:      "Empty media type defaults to document",
			mediaType: "",
			expected:  25 * 1024 * 1024, // 25MB in bytes (document default)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.GetMaxSizeForMediaType(tt.mediaType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasAllowedExtension(t *testing.T) {
	config := models.MediaConfig{
		AllowedTypes: models.MediaAllowedTypes{
			Image: []string{"jpg", "png", "gif"},
		},
	}

	router := NewRouter(config).(*router)

	tests := []struct {
		name              string
		path              string
		allowedExtensions []string
		expected          bool
	}{
		{
			name:              "Matching extension",
			path:              "/path/file.jpg",
			allowedExtensions: []string{"jpg", "png"},
			expected:          true,
		},
		{
			name:              "Case insensitive matching",
			path:              "/path/file.JPG",
			allowedExtensions: []string{"jpg", "png"},
			expected:          true,
		},
		{
			name:              "Mixed case allowed extension",
			path:              "/path/file.jpg",
			allowedExtensions: []string{"JPG", "PNG"},
			expected:          true,
		},
		{
			name:              "No matching extension",
			path:              "/path/file.pdf",
			allowedExtensions: []string{"jpg", "png"},
			expected:          false,
		},
		{
			name:              "No extension in path",
			path:              "/path/file",
			allowedExtensions: []string{"jpg", "png"},
			expected:          false,
		},
		{
			name:              "Empty allowed extensions",
			path:              "/path/file.jpg",
			allowedExtensions: []string{},
			expected:          false,
		},
		{
			name:              "Dot only extension",
			path:              "/path/file.",
			allowedExtensions: []string{"jpg", "png"},
			expected:          false,
		},
		{
			name:              "Multiple dots in filename",
			path:              "/path/file.backup.jpg",
			allowedExtensions: []string{"jpg", "png"},
			expected:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.hasAllowedExtension(tt.path, tt.allowedExtensions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRouterWithEmptyConfig(t *testing.T) {
	// Test router behavior with empty config
	config := models.MediaConfig{}
	router := NewRouter(config)

	// All file types should default to document when no allowed types are configured
	assert.Equal(t, "document", router.GetMediaType("/path/image.jpg"))
	assert.Equal(t, "document", router.GetMediaType("/path/video.mp4"))
	assert.Equal(t, "document", router.GetMediaType("/path/audio.mp3"))

	// All specific type checks should return false with empty config
	assert.False(t, router.IsImageAttachment("/path/image.jpg"))
	assert.False(t, router.IsVideoAttachment("/path/video.mp4"))
	assert.False(t, router.IsVoiceAttachment("/path/audio.mp3"))
	assert.False(t, router.IsDocumentAttachment("/path/document.pdf"))

	// Max sizes should be 0 with empty config
	assert.Equal(t, int64(0), router.GetMaxSizeForMediaType("image"))
	assert.Equal(t, int64(0), router.GetMaxSizeForMediaType("video"))
	assert.Equal(t, int64(0), router.GetMaxSizeForMediaType("voice"))
	assert.Equal(t, int64(0), router.GetMaxSizeForMediaType("document"))
}
