package signal

import (
	"testing"

	"whatsignal/pkg/signal/types"

	"github.com/stretchr/testify/assert"
)

func TestExtractAttachmentPaths(t *testing.T) {
	tests := []struct {
		name        string
		attachments []types.RestMessageAttachment
		expected    []string
	}{
		{
			name:        "empty attachments",
			attachments: []types.RestMessageAttachment{},
			expected:    nil,
		},
		{
			name:        "nil attachments",
			attachments: nil,
			expected:    nil,
		},
		{
			name: "single attachment",
			attachments: []types.RestMessageAttachment{
				{ID: "attachment1", Filename: "file1.jpg"},
			},
			expected: []string{"attachment1"},
		},
		{
			name: "multiple attachments",
			attachments: []types.RestMessageAttachment{
				{ID: "attachment1", Filename: "file1.jpg"},
				{ID: "attachment2", Filename: "file2.pdf"},
				{ID: "attachment3", Filename: "file3.mp3"},
			},
			expected: []string{"attachment1", "attachment2", "attachment3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAttachmentPaths(tt.attachments)
			assert.Equal(t, tt.expected, result)
		})
	}
}