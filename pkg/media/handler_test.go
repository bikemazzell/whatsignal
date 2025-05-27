package media

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
	"whatsignal/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestMediaConfig() models.MediaConfig {
	return models.MediaConfig{
		MaxSizeMB: models.MediaSizeLimits{
			Image:    5,
			Video:    100,
			Gif:      25,
			Document: 100,
			Voice:    16,
		},
		AllowedTypes: models.MediaAllowedTypes{
			Image:    []string{"jpg", "jpeg", "png"},
			Video:    []string{"mp4", "mov"},
			Document: []string{"pdf", "doc", "docx"},
			Voice:    []string{"ogg"},
		},
	}
}

func setupTestHandler(t *testing.T) (Handler, string, func()) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
	require.NoError(t, err)

	// Create cache directory
	cacheDir := filepath.Join(tmpDir, "cache")
	handler, err := NewHandler(cacheDir, getTestMediaConfig())
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return handler, tmpDir, cleanup
}

func createTestFile(t *testing.T, dir, name string, size int64) string {
	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	// Write random data to achieve desired size
	data := make([]byte, size)
	_, err = file.Write(data)
	require.NoError(t, err)

	return path
}

func TestNewHandler(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cacheDir := filepath.Join(tmpDir, "cache")
	handler, err := NewHandler(cacheDir, getTestMediaConfig())
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	// Verify cache directory was created
	info, err := os.Stat(cacheDir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestProcessMedia(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test file
	content := []byte("test content")
	sourcePath := filepath.Join(tmpDir, "source.jpg")
	err = os.WriteFile(sourcePath, content, 0644)
	require.NoError(t, err)

	// Calculate expected hash
	hash := sha256.New()
	hash.Write(content)
	expectedHash := hex.EncodeToString(hash.Sum(nil))

	// Initialize handler
	handler, err := NewHandler(filepath.Join(tmpDir, "cache"), getTestMediaConfig())
	require.NoError(t, err)

	// Test processing
	cachePath, err := handler.ProcessMedia(sourcePath)
	require.NoError(t, err)
	assert.Contains(t, cachePath, expectedHash)

	// Verify file exists and content matches
	cachedContent, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	assert.Equal(t, content, cachedContent)

	// Test processing same file again (should return same path)
	cachePath2, err := handler.ProcessMedia(sourcePath)
	require.NoError(t, err)
	assert.Equal(t, cachePath, cachePath2)

	// Test processing non-existent file
	_, err = handler.ProcessMedia("/nonexistent/file.jpg")
	assert.Error(t, err)
}

func TestCleanupOldFiles(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create cache directory
	cacheDir := filepath.Join(tmpDir, "cache")
	err = os.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)

	// Create test files with different timestamps
	oldContent := []byte("old content")
	newContent := []byte("new content")

	oldHash := sha256.New()
	oldHash.Write(oldContent)
	oldFileName := hex.EncodeToString(oldHash.Sum(nil)) + ".jpg"
	oldPath := filepath.Join(cacheDir, oldFileName)

	newHash := sha256.New()
	newHash.Write(newContent)
	newFileName := hex.EncodeToString(newHash.Sum(nil)) + ".jpg"
	newPath := filepath.Join(cacheDir, newFileName)

	err = os.WriteFile(oldPath, oldContent, 0644)
	require.NoError(t, err)
	err = os.WriteFile(newPath, newContent, 0644)
	require.NoError(t, err)

	// Set old file's modification time to 8 days ago
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	err = os.Chtimes(oldPath, oldTime, oldTime)
	require.NoError(t, err)

	// Initialize handler
	handler, err := NewHandler(cacheDir, getTestMediaConfig())
	require.NoError(t, err)

	// Run cleanup with 7 days retention
	err = handler.CleanupOldFiles(7 * 24 * 60 * 60)
	require.NoError(t, err)

	// Verify old file is gone and new file remains
	_, err = os.Stat(oldPath)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(newPath)
	assert.NoError(t, err)
}

func TestProcessMediaWithInvalidPath(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// Test with non-existent file
	cachedPath, err := handler.ProcessMedia("/nonexistent/path")
	assert.Error(t, err)
	assert.Empty(t, cachedPath)
}

func TestProcessMediaWithUnsupportedType(t *testing.T) {
	handler, tmpDir, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create test file with unsupported extension
	sourcePath := createTestFile(t, tmpDir, "test.xyz", 1024)

	// Should return error for unsupported file types
	cachedPath, err := handler.ProcessMedia(sourcePath)
	assert.Error(t, err)
	assert.Empty(t, cachedPath)
	assert.Contains(t, err.Error(), "file type .xyz is not allowed")
}

func TestProcessMediaSizeLimits(t *testing.T) {
	handler, tmpDir, cleanup := setupTestHandler(t)
	defer cleanup()

	config := getTestMediaConfig()
	maxImageSize := int64(config.MaxSizeMB.Image) * 1024 * 1024
	maxVideoSize := int64(config.MaxSizeMB.Video) * 1024 * 1024
	maxGifSize := int64(config.MaxSizeMB.Gif) * 1024 * 1024

	tests := []struct {
		name      string
		filename  string
		size      int64
		wantError bool
	}{
		{
			name:      "image within limit",
			filename:  "test.jpg",
			size:      maxImageSize - 1024,
			wantError: false,
		},
		{
			name:      "image exceeds limit",
			filename:  "test.png",
			size:      maxImageSize + 1024,
			wantError: true,
		},
		{
			name:      "video within limit",
			filename:  "test.mp4",
			size:      maxVideoSize - 1024,
			wantError: false,
		},
		{
			name:      "video exceeds limit",
			filename:  "test.mov",
			size:      maxVideoSize + 1024,
			wantError: true,
		},
		{
			name:      "gif within limit",
			filename:  "test.gif",
			size:      maxGifSize - 1024,
			wantError: false,
		},
		{
			name:      "gif exceeds limit",
			filename:  "test.gif",
			size:      maxGifSize + 1024,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourcePath := createTestFile(t, tmpDir, tt.filename, tt.size)
			cachedPath, err := handler.ProcessMedia(sourcePath)
			if tt.wantError {
				assert.Error(t, err)
				assert.Empty(t, cachedPath)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, cachedPath)

				// Verify file exists in cache
				_, err := os.Stat(cachedPath)
				assert.NoError(t, err)
			}
		})
	}
}

func TestCleanupOldFilesWithReadOnlyError(t *testing.T) {
	handler, tmpDir, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create test files
	files := []struct {
		name    string
		age     time.Duration
		content string
	}{
		{
			name:    "old.jpg",
			age:     -8 * 24 * time.Hour,
			content: "old content",
		},
		{
			name:    "new.jpg",
			age:     -1 * time.Hour,
			content: "new content",
		},
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	for _, f := range files {
		path := filepath.Join(cacheDir, f.name)
		err := os.WriteFile(path, []byte(f.content), 0644)
		require.NoError(t, err)

		modTime := time.Now().Add(f.age)
		err = os.Chtimes(path, modTime, modTime)
		require.NoError(t, err)
	}

	// Make the directory unwritable
	err := os.Chmod(cacheDir, 0555)
	require.NoError(t, err)
	defer func() {
		if err := os.Chmod(cacheDir, 0755); err != nil {
			t.Errorf("Failed to restore directory permissions: %v", err)
		}
	}()

	err = handler.CleanupOldFiles(7 * 24 * 60 * 60)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove old file")

	// Verify old file still exists (due to permission error)
	oldPath := filepath.Join(cacheDir, "old.jpg")
	_, err = os.Stat(oldPath)
	assert.NoError(t, err)
}

func TestCleanupOldFilesWithNonExistentDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	nonExistentDir := filepath.Join(tmpDir, "nonexistent")
	handler, err := NewHandler(nonExistentDir, getTestMediaConfig())
	require.NoError(t, err)

	// Remove the directory after creating the handler
	err = os.RemoveAll(nonExistentDir)
	require.NoError(t, err)

	err = handler.CleanupOldFiles(7 * 24 * 60 * 60)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read cache directory")
}

func TestProcessMediaCopyFallback(t *testing.T) {
	handler, tmpDir, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create source file on a different path
	sourceDir := filepath.Join(tmpDir, "source")
	err := os.MkdirAll(sourceDir, 0755)
	require.NoError(t, err)

	sourcePath := filepath.Join(sourceDir, "test.jpg")
	err = os.WriteFile(sourcePath, []byte("test content"), 0644)
	require.NoError(t, err)

	// Process the file
	cachedPath, err := handler.ProcessMedia(sourcePath)
	assert.NoError(t, err)
	assert.NotEmpty(t, cachedPath)

	// Verify content was copied correctly
	sourceContent, err := os.ReadFile(sourcePath)
	require.NoError(t, err)
	cachedContent, err := os.ReadFile(cachedPath)
	require.NoError(t, err)
	assert.Equal(t, sourceContent, cachedContent)
}

func TestCopyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Test successful copy
	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")
	content := []byte("test content")
	err = os.WriteFile(srcPath, content, 0644)
	require.NoError(t, err)

	err = copyFile(srcPath, dstPath)
	assert.NoError(t, err)

	// Verify content
	copiedContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, content, copiedContent)

	// Test source file not found
	err = copyFile("/nonexistent/source.txt", dstPath)
	assert.Error(t, err)

	// Test destination directory not found
	err = copyFile(srcPath, "/nonexistent/dir/dest.txt")
	assert.Error(t, err)

	// Test source file not readable
	unreadablePath := filepath.Join(tmpDir, "unreadable.txt")
	err = os.WriteFile(unreadablePath, content, 0644)
	require.NoError(t, err)
	err = os.Chmod(unreadablePath, 0000)
	require.NoError(t, err)
	defer func() {
		if err := os.Chmod(unreadablePath, 0644); err != nil {
			t.Errorf("Failed to restore file permissions: %v", err)
		}
	}()

	err = copyFile(unreadablePath, dstPath)
	assert.Error(t, err)
}

func TestNewHandlerErrors(t *testing.T) {
	// Test with unwritable parent directory
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Make parent directory unwritable
	err = os.Chmod(tmpDir, 0555)
	require.NoError(t, err)
	defer func() {
		if err := os.Chmod(tmpDir, 0755); err != nil {
			t.Errorf("Failed to restore directory permissions: %v", err)
		}
	}()

	cacheDir := filepath.Join(tmpDir, "cache")
	_, err = NewHandler(cacheDir, getTestMediaConfig())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create cache directory")

	// Test with invalid path characters (Windows-specific, but should be tested)
	invalidPath := filepath.Join(tmpDir, "invalid\x00path")
	_, err = NewHandler(invalidPath, getTestMediaConfig())
	assert.Error(t, err)
}
