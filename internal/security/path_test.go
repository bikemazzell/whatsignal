package security

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid relative path",
			path:    "config/test.json",
			wantErr: false,
		},
		{
			name:    "valid absolute path",
			path:    "/etc/config/test.json",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errMsg:  "path cannot be empty",
		},
		{
			name:    "path with directory traversal",
			path:    "../../../etc/passwd",
			wantErr: true,
			errMsg:  "path contains directory traversal",
		},
		{
			name:    "path with embedded traversal",
			path:    "config/../../../etc/passwd",
			wantErr: true,
			errMsg:  "path contains directory traversal",
		},
		{
			name:    "path with dot in filename",
			path:    "config/test.config",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFilePathWithBase(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		basePath string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid path within base",
			path:     filepath.Join(tmpDir, "test.txt"),
			basePath: tmpDir,
			wantErr:  false,
		},
		{
			name:     "valid path in subdirectory",
			path:     filepath.Join(subDir, "test.txt"),
			basePath: tmpDir,
			wantErr:  false,
		},
		{
			name:     "path outside base",
			path:     "/etc/passwd",
			basePath: tmpDir,
			wantErr:  true,
			errMsg:   "path escapes base directory",
		},
		{
			name:     "empty path",
			path:     "",
			basePath: tmpDir,
			wantErr:  true,
			errMsg:   "path cannot be empty",
		},
		{
			name:     "relative path within base",
			path:     "test.txt",
			basePath: tmpDir,
			wantErr:  false,
		},
		{
			name:     "path with traversal trying to escape",
			path:     filepath.Join(tmpDir, "..", "..", "etc", "passwd"),
			basePath: tmpDir,
			wantErr:  true,
			errMsg:   "path escapes base directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePathWithBase(tt.path, tt.basePath)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFilePathStrict(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple path",
			path:    "config.json",
			wantErr: false,
		},
		{
			name:    "valid path with directory",
			path:    "config/app.json",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errMsg:  "path cannot be empty",
		},
		{
			name:    "absolute path",
			path:    "/etc/config.json",
			wantErr: true,
			errMsg:  "absolute paths not allowed in production",
		},
		{
			name:    "path with dot segments",
			path:    "./config.json",
			wantErr: false,
		},
		{
			name:    "path with parent traversal",
			path:    "../config.json",
			wantErr: true,
			errMsg:  "path contains directory traversal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePathStrict(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
