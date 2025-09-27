package validation

import (
	"net/http/httptest"
	"strings"
	"testing"

	"whatsignal/internal/errors"

	"github.com/stretchr/testify/assert"
)

func TestValidatePhoneNumber(t *testing.T) {
	tests := []struct {
		name        string
		phone       string
		expectError bool
		errorCode   errors.ErrorCode
	}{
		// Valid cases
		{
			name:        "valid US number",
			phone:       "+1234567890",
			expectError: false,
		},
		{
			name:        "valid international number",
			phone:       "+447911123456",
			expectError: false,
		},
		{
			name:        "valid with WhatsApp suffix",
			phone:       "+1234567890@c.us",
			expectError: false,
		},
		{
			name:        "valid with group suffix",
			phone:       "+1234567890@g.us",
			expectError: false,
		},
		{
			name:        "valid without prefix",
			phone:       "1234567890",
			expectError: false,
		},

		// Invalid cases
		{
			name:        "empty phone",
			phone:       "",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "too short",
			phone:       "+123",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "too long",
			phone:       "+123456789012345678901",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "contains letters",
			phone:       "+123456789a",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "contains special chars",
			phone:       "+1234-567-890",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "contains spaces",
			phone:       "+123 456 7890",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePhoneNumber(tt.phone)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					assert.Equal(t, string(tt.errorCode), string(errors.GetCode(err)))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMessageID(t *testing.T) {
	tests := []struct {
		name        string
		messageID   string
		expectError bool
		errorCode   errors.ErrorCode
	}{
		// Valid cases
		{
			name:        "valid short ID",
			messageID:   "msg123",
			expectError: false,
		},
		{
			name:        "valid long ID",
			messageID:   "very-long-message-id-with-dashes-and-numbers-123456789",
			expectError: false,
		},
		{
			name:        "valid with special chars",
			messageID:   "msg_123-456.789",
			expectError: false,
		},

		// Invalid cases
		{
			name:        "empty ID",
			messageID:   "",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "too long",
			messageID:   strings.Repeat("a", 1001), // Assume max is 1000
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "contains null byte",
			messageID:   "msg\x00123",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "contains newline",
			messageID:   "msg\n123",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "contains carriage return",
			messageID:   "msg\r123",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "contains tab",
			messageID:   "msg\t123",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessageID(tt.messageID)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					assert.Equal(t, string(tt.errorCode), string(errors.GetCode(err)))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSessionName(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		expectError bool
		errorCode   errors.ErrorCode
	}{
		// Valid cases
		{
			name:        "valid alphanumeric",
			sessionName: "session123",
			expectError: false,
		},
		{
			name:        "valid with underscores",
			sessionName: "my_session",
			expectError: false,
		},
		{
			name:        "valid with dashes",
			sessionName: "my-session",
			expectError: false,
		},
		{
			name:        "valid mixed",
			sessionName: "my_session-123",
			expectError: false,
		},

		// Invalid cases
		{
			name:        "empty name",
			sessionName: "",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "too long",
			sessionName: strings.Repeat("a", 101), // Assume max is 100
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "contains space",
			sessionName: "my session",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "contains special chars",
			sessionName: "my@session",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "contains dot",
			sessionName: "my.session",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSessionName(tt.sessionName)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					assert.Equal(t, string(tt.errorCode), string(errors.GetCode(err)))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMediaSize(t *testing.T) {
	limits := map[string]int{
		"image": 10,  // 10MB
		"video": 100, // 100MB
		"audio": 50,  // 50MB
	}

	tests := []struct {
		name        string
		sizeBytes   int64
		mediaType   string
		expectError bool
		errorCode   errors.ErrorCode
	}{
		// Valid cases
		{
			name:        "valid image size",
			sizeBytes:   5 * 1024 * 1024, // 5MB
			mediaType:   "image",
			expectError: false,
		},
		{
			name:        "valid video size",
			sizeBytes:   50 * 1024 * 1024, // 50MB
			mediaType:   "video",
			expectError: false,
		},
		{
			name:        "maximum allowed size",
			sizeBytes:   10 * 1024 * 1024, // 10MB
			mediaType:   "image",
			expectError: false,
		},

		// Invalid cases
		{
			name:        "negative size",
			sizeBytes:   -1,
			mediaType:   "image",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "zero size",
			sizeBytes:   0,
			mediaType:   "image",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "unsupported media type",
			sizeBytes:   1024,
			mediaType:   "document",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "image too large",
			sizeBytes:   15 * 1024 * 1024, // 15MB
			mediaType:   "image",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "video too large",
			sizeBytes:   150 * 1024 * 1024, // 150MB
			mediaType:   "video",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMediaSize(tt.sizeBytes, tt.mediaType, limits)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					assert.Equal(t, string(tt.errorCode), string(errors.GetCode(err)))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateHTTPRequestSize(t *testing.T) {
	maxSize := int64(1024 * 1024) // 1MB

	tests := []struct {
		name          string
		contentLength int64
		expectError   bool
		errorCode     errors.ErrorCode
	}{
		// Valid cases
		{
			name:          "valid small request",
			contentLength: 1024, // 1KB
			expectError:   false,
		},
		{
			name:          "valid max size request",
			contentLength: maxSize,
			expectError:   false,
		},
		{
			name:          "zero size request",
			contentLength: 0,
			expectError:   false,
		},

		// Invalid cases
		{
			name:          "negative content length",
			contentLength: -1,
			expectError:   true,
			errorCode:     errors.ErrCodeInvalidInput,
		},
		{
			name:          "too large request",
			contentLength: maxSize + 1,
			expectError:   true,
			errorCode:     errors.ErrCodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			req.ContentLength = tt.contentLength

			err := ValidateHTTPRequestSize(req, maxSize)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					assert.Equal(t, string(tt.errorCode), string(errors.GetCode(err)))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateStringLength(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		fieldName   string
		minLength   int
		maxLength   int
		expectError bool
		errorCode   errors.ErrorCode
	}{
		// Valid cases
		{
			name:        "valid string within bounds",
			value:       "hello",
			fieldName:   "message",
			minLength:   1,
			maxLength:   10,
			expectError: false,
		},
		{
			name:        "minimum length string",
			value:       "h",
			fieldName:   "message",
			minLength:   1,
			maxLength:   10,
			expectError: false,
		},
		{
			name:        "maximum length string",
			value:       "1234567890",
			fieldName:   "message",
			minLength:   1,
			maxLength:   10,
			expectError: false,
		},

		// Invalid cases
		{
			name:        "string too short",
			value:       "",
			fieldName:   "message",
			minLength:   1,
			maxLength:   10,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "string too long",
			value:       "12345678901",
			fieldName:   "message",
			minLength:   1,
			maxLength:   10,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStringLength(tt.value, tt.fieldName, tt.minLength, tt.maxLength)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					assert.Equal(t, string(tt.errorCode), string(errors.GetCode(err)))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNumericRange(t *testing.T) {
	tests := []struct {
		name        string
		value       int
		fieldName   string
		min         int
		max         int
		expectError bool
		errorCode   errors.ErrorCode
	}{
		// Valid cases
		{
			name:        "valid value within range",
			value:       5,
			fieldName:   "port",
			min:         1,
			max:         10,
			expectError: false,
		},
		{
			name:        "minimum value",
			value:       1,
			fieldName:   "port",
			min:         1,
			max:         10,
			expectError: false,
		},
		{
			name:        "maximum value",
			value:       10,
			fieldName:   "port",
			min:         1,
			max:         10,
			expectError: false,
		},

		// Invalid cases
		{
			name:        "value too small",
			value:       0,
			fieldName:   "port",
			min:         1,
			max:         10,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "value too large",
			value:       11,
			fieldName:   "port",
			min:         1,
			max:         10,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNumericRange(tt.value, tt.fieldName, tt.min, tt.max)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					assert.Equal(t, string(tt.errorCode), string(errors.GetCode(err)))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTimeout(t *testing.T) {
	tests := []struct {
		name        string
		timeoutSec  int
		fieldName   string
		expectError bool
		errorCode   errors.ErrorCode
	}{
		// Valid cases
		{
			name:        "valid short timeout",
			timeoutSec:  1,
			fieldName:   "connect timeout",
			expectError: false,
		},
		{
			name:        "valid medium timeout",
			timeoutSec:  60,
			fieldName:   "connect timeout",
			expectError: false,
		},
		{
			name:        "valid maximum timeout",
			timeoutSec:  3600,
			fieldName:   "connect timeout",
			expectError: false,
		},

		// Invalid cases
		{
			name:        "zero timeout",
			timeoutSec:  0,
			fieldName:   "connect timeout",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "negative timeout",
			timeoutSec:  -1,
			fieldName:   "connect timeout",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "timeout too large",
			timeoutSec:  3601,
			fieldName:   "connect timeout",
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimeout(tt.timeoutSec, tt.fieldName)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					assert.Equal(t, string(tt.errorCode), string(errors.GetCode(err)))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateConnectionPool(t *testing.T) {
	tests := []struct {
		name        string
		maxOpen     int
		maxIdle     int
		expectError bool
		errorCode   errors.ErrorCode
	}{
		// Valid cases
		{
			name:        "valid pool settings",
			maxOpen:     10,
			maxIdle:     5,
			expectError: false,
		},
		{
			name:        "minimum settings",
			maxOpen:     1,
			maxIdle:     0,
			expectError: false,
		},
		{
			name:        "maxIdle equals maxOpen",
			maxOpen:     10,
			maxIdle:     10,
			expectError: false,
		},
		{
			name:        "maximum settings",
			maxOpen:     1000,
			maxIdle:     1000,
			expectError: false,
		},

		// Invalid cases
		{
			name:        "maxOpen too small",
			maxOpen:     0,
			maxIdle:     0,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "maxOpen too large",
			maxOpen:     1001,
			maxIdle:     10,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "maxIdle negative",
			maxOpen:     10,
			maxIdle:     -1,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "maxIdle greater than maxOpen",
			maxOpen:     10,
			maxIdle:     15,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConnectionPool(tt.maxOpen, tt.maxIdle)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					assert.Equal(t, string(tt.errorCode), string(errors.GetCode(err)))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRetentionDays(t *testing.T) {
	tests := []struct {
		name        string
		days        int
		expectError bool
		errorCode   errors.ErrorCode
	}{
		// Valid cases
		{
			name:        "valid short retention",
			days:        1,
			expectError: false,
		},
		{
			name:        "valid medium retention",
			days:        30,
			expectError: false,
		},
		{
			name:        "valid long retention",
			days:        365,
			expectError: false,
		},
		{
			name:        "maximum retention",
			days:        3650,
			expectError: false,
		},

		// Invalid cases
		{
			name:        "zero days",
			days:        0,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "negative days",
			days:        -1,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
		{
			name:        "days too large",
			days:        3651,
			expectError: true,
			errorCode:   errors.ErrCodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRetentionDays(tt.days)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					assert.Equal(t, string(tt.errorCode), string(errors.GetCode(err)))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
