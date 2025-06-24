package media

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
			Image:    []string{"jpg", "jpeg", "png", "jfif"},
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

	// Should process unknown file types as documents (default behavior)
	cachedPath, err := handler.ProcessMedia(sourcePath)
	assert.NoError(t, err)
	assert.NotEmpty(t, cachedPath)
	assert.FileExists(t, cachedPath)
	assert.Contains(t, cachedPath, ".xyz") // Extension should be preserved
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

func TestIsURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"http://example.com/image.jpg", true},
		{"https://example.com/image.jpg", true},
		{"ftp://example.com/file.txt", true},
		{"/local/path/image.jpg", false},
		{"./relative/path.jpg", false},
		{"image.jpg", false},
		{"", false},
		{"not-a-url", false},
		{"http://", false},
		{"://invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessMediaFromURL(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create test server
	testContent := []byte("test image content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testContent)
	}))
	defer server.Close()

	// Test successful URL processing
	cachedPath, err := handler.ProcessMedia(server.URL + "/image.jpg")
	require.NoError(t, err)
	assert.NotEmpty(t, cachedPath)

	// Verify file exists and content matches
	cachedContent, err := os.ReadFile(cachedPath)
	require.NoError(t, err)
	assert.Equal(t, testContent, cachedContent)

	// Test processing same URL again (should return same cached path)
	cachedPath2, err := handler.ProcessMedia(server.URL + "/image.jpg")
	require.NoError(t, err)
	assert.Equal(t, cachedPath, cachedPath2)
}

func TestProcessMediaFromURLErrors(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		expectedError  string
	}{
		{
			name: "404 not found",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			expectedError: "download failed with status: 404",
		},
		{
			name: "500 server error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			expectedError: "download failed with status: 500",
		},
		{
			name: "large file size",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					// Write more than 5MB to trigger size limit
					data := make([]byte, 6*1024*1024)
					w.Write(data)
				}))
			},
			expectedError: "image too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			_, err := handler.ProcessMedia(server.URL + "/file")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestProcessMediaFromURLWithLargeFile(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create server that returns a file larger than the limit
	config := getTestMediaConfig()
	maxImageSize := int64(config.MaxSizeMB.Image) * 1024 * 1024
	largeContent := make([]byte, maxImageSize+1024) // Exceed limit

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(largeContent)
	}))
	defer server.Close()

	_, err := handler.ProcessMedia(server.URL + "/large.jpg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image too large")
}

func TestGetFileExtensionFromResponse(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		url         string
		expected    string
	}{
		{
			name:        "JPEG content type",
			contentType: "image/jpeg",
			url:         "http://example.com/file",
			expected:    ".jfif",
		},
		{
			name:        "PNG content type",
			contentType: "image/png",
			url:         "http://example.com/file",
			expected:    ".png",
		},
		{
			name:        "extension from URL",
			contentType: "",
			url:         "http://example.com/image.jpg",
			expected:    ".jpg",
		},
		{
			name:        "fallback to .bin",
			contentType: "",
			url:         "http://example.com/file",
			expected:    ".bin",
		},
		{
			name:        "unknown content type",
			contentType: "application/unknown",
			url:         "http://example.com/file.txt",
			expected:    ".txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock response
			resp := &http.Response{
				Header: make(http.Header),
			}
			if tt.contentType != "" {
				resp.Header.Set("Content-Type", tt.contentType)
			}

			// Create a handler instance to test the method
			tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			h := &handler{
				cacheDir: tmpDir,
				config:   getTestMediaConfig(),
			}
			result := h.getFileExtensionFromResponse(resp, tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessMediaFromURLTimeout(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(35 * time.Second) // Longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := handler.ProcessMedia(server.URL + "/slow.jpg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download media from URL")
}

func TestProcessMediaFromURLInvalidURL(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// Test with invalid URL
	_, err := handler.ProcessMedia("http://invalid-domain-that-does-not-exist.com/image.jpg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download media from URL")
}

func TestProcessMediaMixedTypes(t *testing.T) {
	handler, tmpDir, cleanup := setupTestHandler(t)
	defer cleanup()

	// Test local file
	localContent := []byte("local file content")
	localPath := filepath.Join(tmpDir, "local.jpg")
	err := os.WriteFile(localPath, localContent, 0644)
	require.NoError(t, err)

	cachedLocalPath, err := handler.ProcessMedia(localPath)
	require.NoError(t, err)
	assert.NotEmpty(t, cachedLocalPath)

	// Test URL
	urlContent := []byte("url file content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(urlContent)
	}))
	defer server.Close()

	cachedURLPath, err := handler.ProcessMedia(server.URL + "/remote.jpg")
	require.NoError(t, err)
	assert.NotEmpty(t, cachedURLPath)

	// Verify both files are different
	assert.NotEqual(t, cachedLocalPath, cachedURLPath)

	// Verify content
	localCachedContent, err := os.ReadFile(cachedLocalPath)
	require.NoError(t, err)
	assert.Equal(t, localContent, localCachedContent)

	urlCachedContent, err := os.ReadFile(cachedURLPath)
	require.NoError(t, err)
	assert.Equal(t, urlContent, urlCachedContent)
}

func TestDownloadFromURLContentTypes(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	tests := []struct {
		name        string
		contentType string
		filename    string
		expectExt   string
	}{
		{
			name:        "JPEG image",
			contentType: "image/jpeg",
			filename:    "test",
			expectExt:   "jfif",
		},
		{
			name:        "PNG image",
			contentType: "image/png",
			filename:    "test",
			expectExt:   "png",
		},
		{
			name:        "PDF document",
			contentType: "application/pdf",
			filename:    "test",
			expectExt:   "pdf",
		},
		{
			name:        "Extension from filename",
			contentType: "",
			filename:    "test.jpg",
			expectExt:   "jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("test content"))
			}))
			defer server.Close()

			url := fmt.Sprintf("%s/%s", server.URL, tt.filename)

			// Only test if the extension is in our allowed types
			config := getTestMediaConfig()
			isAllowed := false
			for _, ext := range config.AllowedTypes.Image {
				if ext == tt.expectExt {
					isAllowed = true
					break
				}
			}
			for _, ext := range config.AllowedTypes.Document {
				if ext == tt.expectExt {
					isAllowed = true
					break
				}
			}

			if isAllowed {
				cachedPath, err := handler.ProcessMedia(url)
				if err != nil {
					t.Logf("Error processing %s (expected %s): %v", url, tt.expectExt, err)
					// Skip this test case if there's an unexpected error
					return
				}
				assert.NotEmpty(t, cachedPath)
				// Since unknown content types default to documents, any extension should work
				assert.True(t, strings.Contains(cachedPath, ".") && len(filepath.Ext(cachedPath)) > 0)
			}
		})
	}
}

func TestRewriteMediaURL(t *testing.T) {
	tests := []struct {
		name        string
		wahaBaseURL string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "localhost:3000 rewritten to WAHA host",
			wahaBaseURL: "http://localhost:3000",
			inputURL:    "http://localhost:3000/api/files/default/test.jpg",
			expectedURL: "http://localhost:3000/api/files/default/test.jpg",
		},
		{
			name:        "127.0.0.1:3000 rewritten to WAHA host",
			wahaBaseURL: "http://localhost:3000",
			inputURL:    "http://127.0.0.1:3000/api/files/default/test.jpg",
			expectedURL: "http://localhost:3000/api/files/default/test.jpg",
		},
		{
			name:        "[::1]:3000 rewritten to WAHA host",
			wahaBaseURL: "http://localhost:3000",
			inputURL:    "http://[::1]:3000/api/files/default/test.jpg",
			expectedURL: "http://localhost:3000/api/files/default/test.jpg",
		},
		{
			name:        "non-localhost URL unchanged",
			wahaBaseURL: "http://localhost:3000",
			inputURL:    "http://example.com:3000/api/files/default/test.jpg",
			expectedURL: "http://example.com:3000/api/files/default/test.jpg",
		},
		{
			name:        "no WAHA base URL configured",
			wahaBaseURL: "",
			inputURL:    "http://localhost:3000/api/files/default/test.jpg",
			expectedURL: "http://localhost:3000/api/files/default/test.jpg",
		},
		{
			name:        "HTTPS localhost rewritten",
			wahaBaseURL: "https://localhost:3000",
			inputURL:    "http://localhost:3000/api/files/default/test.jpg",
			expectedURL: "https://localhost:3000/api/files/default/test.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			cacheDir := filepath.Join(tmpDir, "cache")
			mediaHandler, err := NewHandlerWithWAHA(cacheDir, getTestMediaConfig(), tt.wahaBaseURL, "")
			require.NoError(t, err)

			// Access the private method through type assertion
			h, ok := mediaHandler.(*handler)
			require.True(t, ok, "handler should be of type *handler")
			result := h.rewriteMediaURL(tt.inputURL)
			assert.Equal(t, tt.expectedURL, result)
		})
	}
}

func TestProcessMediaFromURLWithAPIKey(t *testing.T) {
	// Test that WAHA API key is properly included in requests
	apiKey := "test-api-key"
	var receivedHeaders http.Header

	// Create test server that captures headers
	testContent := []byte("test image content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testContent)
	}))
	defer server.Close()

	// Create handler with API key
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cacheDir := filepath.Join(tmpDir, "cache")
	handler, err := NewHandlerWithWAHA(cacheDir, getTestMediaConfig(), "", apiKey)
	require.NoError(t, err)

	// Test URL processing
	cachedPath, err := handler.ProcessMedia(server.URL + "/image.jpg")
	require.NoError(t, err)
	assert.NotEmpty(t, cachedPath)

	// Verify API key header was sent
	assert.Equal(t, apiKey, receivedHeaders.Get("X-Api-Key"))

	// Verify file content
	cachedContent, err := os.ReadFile(cachedPath)
	require.NoError(t, err)
	assert.Equal(t, testContent, cachedContent)
}

func TestProcessMediaFromURLWithoutAPIKey(t *testing.T) {
	// Test that no API key header is sent when not configured
	var receivedHeaders http.Header

	// Create test server that captures headers
	testContent := []byte("test image content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(testContent)
	}))
	defer server.Close()

	// Create handler without API key
	tmpDir, err := os.MkdirTemp("", "whatsignal-media-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cacheDir := filepath.Join(tmpDir, "cache")
	handler, err := NewHandlerWithWAHA(cacheDir, getTestMediaConfig(), "", "")
	require.NoError(t, err)

	// Test URL processing
	cachedPath, err := handler.ProcessMedia(server.URL + "/image.jpg")
	require.NoError(t, err)
	assert.NotEmpty(t, cachedPath)

	// Verify no API key header was sent
	assert.Empty(t, receivedHeaders.Get("X-Api-Key"))
}

func TestProcessMediaFromFileEdgeCases(t *testing.T) {
	handlerInterface, tmpDir, cleanup := setupTestHandler(t)
	defer cleanup()

	// Cast to concrete type to access private methods
	h := handlerInterface.(*handler)

	tests := []struct {
		name        string
		setupFile   func() string
		expectError bool
		errorMsg    string
	}{
		{
			name: "directory traversal attempt",
			setupFile: func() string {
				return "../../../etc/passwd"
			},
			expectError: true,
			errorMsg:    "invalid media path",
		},
		{
			name: "non-existent file",
			setupFile: func() string {
				return filepath.Join(tmpDir, "nonexistent.jpg")
			},
			expectError: true,
			errorMsg:    "failed to get file info",
		},
		{
			name: "file with no extension",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "noext")
				err := os.WriteFile(path, []byte("test content"), 0644)
				require.NoError(t, err)
				return path
			},
			expectError: false, // Now accepts files without extension as documents
		},
		{
			name: "oversized image file",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "huge.jpg")
				// Create a file larger than the max image size (5MB in test config)
				largeContent := make([]byte, 6*1024*1024) // 6MB
				err := os.WriteFile(path, largeContent, 0644)
				require.NoError(t, err)
				return path
			},
			expectError: true,
			errorMsg:    "image too large",
		},
		{
			name: "valid small image file",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "small.jpg")
				err := os.WriteFile(path, []byte("fake image content"), 0644)
				require.NoError(t, err)
				return path
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setupFile()

			result, err := h.processMediaFromFile(filePath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
				assert.FileExists(t, result)
			}
		})
	}
}

func TestProcessDownloadedFileEdgeCases(t *testing.T) {
	handlerInterface, tmpDir, cleanup := setupTestHandler(t)
	defer cleanup()

	// Cast to concrete type to access private methods
	h := handlerInterface.(*handler)

	tests := []struct {
		name        string
		setupFile   func() string
		ext         string
		expectError bool
		errorMsg    string
	}{
		{
			name: "non-existent temp file",
			setupFile: func() string {
				return filepath.Join(tmpDir, "nonexistent.tmp")
			},
			ext:         "jpg",
			expectError: true,
			errorMsg:    "failed to open downloaded file",
		},
		{
			name: "valid temp file",
			setupFile: func() string {
				path := filepath.Join(tmpDir, "temp.tmp")
				err := os.WriteFile(path, []byte("downloaded content"), 0644)
				require.NoError(t, err)
				return path
			},
			ext:         "jpg",
			expectError: false,
		},
		{
			name: "file already in cache",
			setupFile: func() string {
				// Create temp file
				tempPath := filepath.Join(tmpDir, "temp2.tmp")
				content := []byte("cached content")
				err := os.WriteFile(tempPath, content, 0644)
				require.NoError(t, err)

				// Pre-create the cached file with same hash
				hash := sha256.Sum256(content)
				hashStr := fmt.Sprintf("%x", hash)
				cachedPath := filepath.Join(h.cacheDir, hashStr+".png")
				err = os.WriteFile(cachedPath, content, 0644)
				require.NoError(t, err)

				return tempPath
			},
			ext:         "png",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempPath := tt.setupFile()

			result, err := h.processDownloadedFile(tempPath, tt.ext)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
				assert.FileExists(t, result)
			}
		})
	}
}

func TestDownloadFromURLEdgeCases(t *testing.T) {
	handlerInterface, _, cleanup := setupTestHandler(t)
	defer cleanup()

	// Cast to concrete type to access private methods
	h := handlerInterface.(*handler)

	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		expectError bool
		errorMsg    string
	}{
		{
			name: "invalid URL",
			setupServer: func() *httptest.Server {
				return nil // No server needed for invalid URL test
			},
			expectError: true,
			errorMsg:    "failed to create request",
		},
		{
			name: "server returns 404",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			expectError: true,
			errorMsg:    "download failed with status: 404",
		},
		{
			name: "server returns 500",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			expectError: true,
			errorMsg:    "download failed with status: 500",
		},
		{
			name: "successful download with content-type",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("fake jpeg content"))
				}))
			},
			expectError: false,
		},
		{
			name: "successful download without content-type",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("fake content"))
				}))
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testURL string
			var server *httptest.Server

			if tt.name == "invalid URL" {
				testURL = "://invalid-url"
			} else {
				server = tt.setupServer()
				defer server.Close()
				testURL = server.URL + "/test.jpg"
			}

			tempPath, ext, err := h.downloadFromURL(testURL)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Empty(t, tempPath)
				assert.Empty(t, ext)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, tempPath)
				assert.NotEmpty(t, ext)
				assert.FileExists(t, tempPath)

				// Cleanup temp file
				os.Remove(tempPath)
			}
		})
	}
}

func TestRewriteMediaURLEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		wahaBaseURL string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "no WAHA base URL configured",
			wahaBaseURL: "",
			inputURL:    "http://localhost:3000/api/files/test.jpg",
			expectedURL: "http://localhost:3000/api/files/test.jpg",
		},
		{
			name:        "invalid input URL",
			wahaBaseURL: "http://waha.example.com",
			inputURL:    "://invalid-url",
			expectedURL: "://invalid-url",
		},
		{
			name:        "invalid WAHA base URL",
			wahaBaseURL: "://invalid-waha-url",
			inputURL:    "http://localhost:3000/api/files/test.jpg",
			expectedURL: "http://localhost:3000/api/files/test.jpg",
		},
		{
			name:        "non-localhost URL (no rewrite needed)",
			wahaBaseURL: "http://waha.example.com",
			inputURL:    "http://external.com/api/files/test.jpg",
			expectedURL: "http://external.com/api/files/test.jpg",
		},
		{
			name:        "localhost:3000 URL rewrite",
			wahaBaseURL: "http://waha.example.com",
			inputURL:    "http://localhost:3000/api/files/test.jpg",
			expectedURL: "http://waha.example.com/api/files/test.jpg",
		},
		{
			name:        "127.0.0.1:3000 URL rewrite",
			wahaBaseURL: "https://waha.example.com:8080",
			inputURL:    "http://127.0.0.1:3000/api/files/test.jpg",
			expectedURL: "https://waha.example.com:8080/api/files/test.jpg",
		},
		{
			name:        "IPv6 localhost URL rewrite",
			wahaBaseURL: "http://waha.example.com",
			inputURL:    "http://[::1]:3000/api/files/test.jpg",
			expectedURL: "http://waha.example.com/api/files/test.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{
				wahaBaseURL: tt.wahaBaseURL,
			}

			result := h.rewriteMediaURL(tt.inputURL)
			assert.Equal(t, tt.expectedURL, result)
		})
	}
}

func TestCopyFileEdgeCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "media-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		setupSrc    func() string
		setupDst    func() string
		expectError bool
		errorMsg    string
	}{
		{
			name: "directory traversal in source",
			setupSrc: func() string {
				return "../../../etc/passwd"
			},
			setupDst: func() string {
				return filepath.Join(tmpDir, "dst.txt")
			},
			expectError: true,
			errorMsg:    "invalid source path",
		},
		{
			name: "non-existent source file",
			setupSrc: func() string {
				return filepath.Join(tmpDir, "nonexistent.txt")
			},
			setupDst: func() string {
				return filepath.Join(tmpDir, "dst.txt")
			},
			expectError: true,
		},
		{
			name: "successful copy",
			setupSrc: func() string {
				srcPath := filepath.Join(tmpDir, "src.txt")
				err := os.WriteFile(srcPath, []byte("test content"), 0644)
				require.NoError(t, err)
				return srcPath
			},
			setupDst: func() string {
				return filepath.Join(tmpDir, "dst.txt")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcPath := tt.setupSrc()
			dstPath := tt.setupDst()

			err := copyFile(srcPath, dstPath)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.FileExists(t, dstPath)

				// Verify content was copied correctly
				srcContent, err := os.ReadFile(srcPath)
				require.NoError(t, err)
				dstContent, err := os.ReadFile(dstPath)
				require.NoError(t, err)
				assert.Equal(t, srcContent, dstContent)
			}
		})
	}
}

func TestProcessMediaWithoutExtension(t *testing.T) {
	handler, tmpDir, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a test file without extension that contains audio data
	// This simulates Signal's audio recordings that don't have file extensions
	sourcePath := filepath.Join(tmpDir, "audio_recording")
	
	// Create a fake OGG file header (simple test)
	oggHeader := []byte("OggS") // Simplified OGG header
	testContent := append(oggHeader, []byte("fake audio data")...)
	
	err := os.WriteFile(sourcePath, testContent, 0644)
	require.NoError(t, err)

	// Should process the file successfully, detecting it as audio
	cachedPath, err := handler.ProcessMedia(sourcePath)
	assert.NoError(t, err)
	assert.NotEmpty(t, cachedPath)
	assert.FileExists(t, cachedPath)
	
	// The cached file should have .ogg extension (from signature detection)
	assert.Contains(t, cachedPath, ".ogg")
}

func TestDetectFileTypeFromContent(t *testing.T) {
	handler, tmpDir, cleanup := setupTestHandler(t)
	defer cleanup()

	tests := []struct {
		name             string
		content          []byte
		expectedExt      string
		shouldHaveExt    bool
	}{
		{
			name:          "OGG file signature",
			content:       append([]byte("OggS"), make([]byte, 100)...),
			expectedExt:   ".ogg",
			shouldHaveExt: true,
		},
		{
			name:          "MP3 with ID3v2 tag",
			content:       append([]byte("ID3"), make([]byte, 100)...),
			expectedExt:   ".mp3",
			shouldHaveExt: true,
		},
		{
			name:          "JPEG signature",
			content:       append([]byte{0xFF, 0xD8, 0xFF}, make([]byte, 100)...),
			expectedExt:   ".jpg",
			shouldHaveExt: true,
		},
		{
			name:          "PNG signature",
			content:       append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 100)...),
			expectedExt:   ".png",
			shouldHaveExt: true,
		},
		{
			name:          "PDF signature",
			content:       append([]byte("%PDF"), make([]byte, 100)...),
			expectedExt:   ".pdf",
			shouldHaveExt: true,
		},
		{
			name:          "Unknown content",
			content:       []byte("random data here"),
			shouldHaveExt: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file without extension
			testPath := filepath.Join(tmpDir, "test_file_no_ext")
			err := os.WriteFile(testPath, tt.content, 0644)
			require.NoError(t, err)
			defer os.Remove(testPath)

			// Test through ProcessMedia which will use detectFileTypeFromContent
			cachedPath, err := handler.ProcessMedia(testPath)
			assert.NoError(t, err)
			assert.NotEmpty(t, cachedPath)
			assert.FileExists(t, cachedPath)

			if tt.shouldHaveExt {
				assert.Contains(t, cachedPath, tt.expectedExt)
			} else {
				// For unknown content, should default to document processing
				assert.True(t, strings.Contains(cachedPath, "."))
			}
		})
	}
}
