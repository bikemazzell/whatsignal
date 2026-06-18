package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMimeTypes_HasExpectedExtensions(t *testing.T) {
	tests := map[string]string{
		".jpg": "image/jpeg",
		".png": "image/png",
		".mp4": "video/mp4",
		".pdf": "application/pdf",
		".ogg": "audio/ogg",
	}
	for ext, expected := range tests {
		assert.Equal(t, expected, MimeTypes[ext])
	}
}

func TestDefaultMimeType(t *testing.T) {
	assert.Equal(t, "application/octet-stream", DefaultMimeType)
}

func TestContentTypeToExtension_HasExpectedMappings(t *testing.T) {
	tests := map[string]string{
		"audio/ogg":  "ogg",
		"image/jpeg": "jpg",
		"video/mp4":  "mp4",
	}
	for ct, expected := range tests {
		assert.Equal(t, expected, ContentTypeToExtension[ct])
	}
}

func TestFileSignatures_HasExpectedSignatures(t *testing.T) {
	tests := map[string]string{
		"OggS": "ogg",
		"%PDF": "pdf",
		"ID3":  "mp3",
	}
	for sig, expected := range tests {
		assert.Equal(t, expected, FileSignatures[sig])
	}
}

func TestMimeTypeToExtension_HasExpectedMappings(t *testing.T) {
	tests := map[string]string{
		"image/jpeg": ".jpg",
		"video/mp4":  ".mp4",
		"audio/ogg":  ".ogg",
	}
	for mime, expected := range tests {
		assert.Equal(t, expected, MimeTypeToExtension[mime])
	}
}

func TestDefaultConstants_Values(t *testing.T) {
	assert.Equal(t, 5, DefaultSignalPollIntervalSec)
	assert.Equal(t, 1000, DefaultRetryBackoffMs)
	assert.Equal(t, 5, DefaultMaxAttempts)
	assert.Equal(t, 30, DefaultRetentionDays)
	assert.Equal(t, 8082, DefaultServerPort)
	assert.Equal(t, 0600, DefaultFilePermissions)
	assert.Equal(t, 0750, DefaultDirectoryPermissions)
	assert.Equal(t, 600000, PBKDF2Iterations)
	assert.Equal(t, 1000, MaxChatLocks)
	assert.Equal(t, 100, DefaultPendingMessageBatchSize)
	assert.Equal(t, 1024*1024, BytesPerMegabyte)
	assert.Equal(t, 10, MinPhoneNumberLength)
	assert.Equal(t, 16, MessageIDRandomBytesLength)
}

func TestPollFailureAlertThresholds_Ordered(t *testing.T) {
	for i := 1; i < len(PollFailureAlertThresholds); i++ {
		assert.Greater(t, PollFailureAlertThresholds[i], PollFailureAlertThresholds[i-1])
	}
}
