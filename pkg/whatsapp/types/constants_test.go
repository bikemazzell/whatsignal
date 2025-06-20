package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMediaTypeConstants(t *testing.T) {
	tests := []struct {
		name      string
		mediaType MediaType
		expected  string
	}{
		{
			name:      "image media type",
			mediaType: MediaTypeImage,
			expected:  "Image",
		},
		{
			name:      "file media type",
			mediaType: MediaTypeFile,
			expected:  "File",
		},
		{
			name:      "voice media type",
			mediaType: MediaTypeVoice,
			expected:  "Voice",
		},
		{
			name:      "video media type",
			mediaType: MediaTypeVideo,
			expected:  "Video",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.mediaType))
		})
	}
}

func TestMediaTypeString(t *testing.T) {
	// Test that MediaType can be converted to string
	assert.Equal(t, "Image", string(MediaTypeImage))
	assert.Equal(t, "File", string(MediaTypeFile))
	assert.Equal(t, "Voice", string(MediaTypeVoice))
	assert.Equal(t, "Video", string(MediaTypeVideo))
}

func TestAPIEndpointConstants(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		expected string
	}{
		{
			name:     "send text endpoint",
			endpoint: EndpointSendText,
			expected: "/sendText",
		},
		{
			name:     "send seen endpoint",
			endpoint: EndpointSendSeen,
			expected: "/sendSeen",
		},
		{
			name:     "start typing endpoint",
			endpoint: EndpointStartTyping,
			expected: "/startTyping",
		},
		{
			name:     "stop typing endpoint",
			endpoint: EndpointStopTyping,
			expected: "/stopTyping",
		},
		{
			name:     "send image endpoint",
			endpoint: EndpointSendImage,
			expected: "/sendImage",
		},
		{
			name:     "send file endpoint",
			endpoint: EndpointSendFile,
			expected: "/sendFile",
		},
		{
			name:     "send voice endpoint",
			endpoint: EndpointSendVoice,
			expected: "/sendVoice",
		},
		{
			name:     "send video endpoint",
			endpoint: EndpointSendVideo,
			expected: "/sendVideo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.endpoint)
		})
	}
}

func TestAPIBaseConstant(t *testing.T) {
	// Test the API base constant
	assert.Equal(t, "/api", APIBase)
}

func TestAPIEndpointFormatting(t *testing.T) {
	// Test building complete API paths
	baseURL := APIBase

	tests := []struct {
		name     string
		endpoint string
		expected string
	}{
		{
			name:     "complete send text URL",
			endpoint: EndpointSendText,
			expected: "/api/sendText",
		},
		{
			name:     "complete send image URL",
			endpoint: EndpointSendImage,
			expected: "/api/sendImage",
		},
		{
			name:     "complete send file URL",
			endpoint: EndpointSendFile,
			expected: "/api/sendFile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullURL := baseURL + tt.endpoint
			assert.Equal(t, tt.expected, fullURL)
		})
	}
}

func TestMediaTypeComparison(t *testing.T) {
	// Test that media types can be compared
	assert.Equal(t, MediaTypeImage, MediaTypeImage)
	assert.NotEqual(t, MediaTypeImage, MediaTypeFile)
	assert.NotEqual(t, MediaTypeVideo, MediaTypeVoice)
}

func TestMediaTypeInSlice(t *testing.T) {
	// Test using media types in slices
	supportedTypes := []MediaType{
		MediaTypeImage,
		MediaTypeFile,
		MediaTypeVoice,
		MediaTypeVideo,
	}

	assert.Len(t, supportedTypes, 4)
	assert.Contains(t, supportedTypes, MediaTypeImage)
	assert.Contains(t, supportedTypes, MediaTypeFile)
	assert.Contains(t, supportedTypes, MediaTypeVoice)
	assert.Contains(t, supportedTypes, MediaTypeVideo)
}

func TestEndpointConstants(t *testing.T) {
	// Test that all endpoints start with "/"
	endpoints := []string{
		EndpointSendText,
		EndpointSendSeen,
		EndpointStartTyping,
		EndpointStopTyping,
		EndpointSendImage,
		EndpointSendFile,
		EndpointSendVoice,
		EndpointSendVideo,
	}

	for _, endpoint := range endpoints {
		assert.True(t, len(endpoint) > 0, "Endpoint should not be empty")
		assert.True(t, endpoint[0] == '/', "Endpoint should start with '/'")
	}
}

func TestConstantUniqueness(t *testing.T) {
	// Test that all media type constants are unique
	mediaTypes := []MediaType{
		MediaTypeImage,
		MediaTypeFile,
		MediaTypeVoice,
		MediaTypeVideo,
	}

	seen := make(map[MediaType]bool)
	for _, mt := range mediaTypes {
		assert.False(t, seen[mt], "Media type %s should be unique", mt)
		seen[mt] = true
	}

	// Test that all endpoint constants are unique
	endpoints := []string{
		EndpointSendText,
		EndpointSendSeen,
		EndpointStartTyping,
		EndpointStopTyping,
		EndpointSendImage,
		EndpointSendFile,
		EndpointSendVoice,
		EndpointSendVideo,
	}

	seenEndpoints := make(map[string]bool)
	for _, endpoint := range endpoints {
		assert.False(t, seenEndpoints[endpoint], "Endpoint %s should be unique", endpoint)
		seenEndpoints[endpoint] = true
	}
}
