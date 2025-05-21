package media

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestHandler(t *testing.T) (Handler, string, func()) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
	require.NoError(t, err)

	// Create cache directory
	cacheDir := filepath.Join(tmpDir, "cache")
	handler, err := NewHandler(cacheDir)
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
	handler, err := NewHandler(cacheDir)
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
	handler, err := NewHandler(filepath.Join(tmpDir, "cache"))
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
	handler, err := NewHandler(cacheDir)
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

	// Should process without error as we only validate size for known types
	cachedPath, err := handler.ProcessMedia(sourcePath)
	assert.NoError(t, err)
	assert.NotEmpty(t, cachedPath)
}
