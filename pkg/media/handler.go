package media

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"whatsignal/internal/models"
	"whatsignal/internal/security"
)

type Handler interface {
	ProcessMedia(path string) (string, error)
	CleanupOldFiles(maxAge int64) error
}

type handler struct {
	cacheDir   string
	config     models.MediaConfig
	httpClient *http.Client
	wahaBaseURL string // For URL rewriting
	wahaAPIKey  string // For WAHA authentication
}

func NewHandler(cacheDir string, config models.MediaConfig) (Handler, error) {
	return NewHandlerWithWAHA(cacheDir, config, "", "")
}

func NewHandlerWithWAHA(cacheDir string, config models.MediaConfig, wahaBaseURL, wahaAPIKey string) (Handler, error) {
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &handler{
		cacheDir:    cacheDir,
		config:      config,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		wahaBaseURL: wahaBaseURL,
		wahaAPIKey:  wahaAPIKey,
	}, nil
}

func (h *handler) ProcessMedia(pathOrURL string) (string, error) {
	// Check if input is a URL
	if isURL(pathOrURL) {
		return h.processMediaFromURL(pathOrURL)
	}

	// Process as local file path
	return h.processMediaFromFile(pathOrURL)
}

func (h *handler) processMediaFromURL(mediaURL string) (string, error) {
	// Rewrite localhost URLs to use the correct WAHA host
	rewrittenURL := h.rewriteMediaURL(mediaURL)

	// Download the file from URL
	tempPath, ext, err := h.downloadFromURL(rewrittenURL)
	if err != nil {
		return "", fmt.Errorf("failed to download media from URL: %w", err)
	}
	defer os.Remove(tempPath) // Clean up temp file

	// Get file info for validation
	info, err := os.Stat(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to get downloaded file info: %w", err)
	}

	// Validate media type and size
	if err := h.validateMedia(ext, info.Size()); err != nil {
		return "", err
	}

	// Process the downloaded file
	return h.processDownloadedFile(tempPath, ext)
}

func (h *handler) processMediaFromFile(path string) (string, error) {
	// Validate file path to prevent directory traversal
	if err := security.ValidateFilePath(path); err != nil {
		return "", fmt.Errorf("invalid media path: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))

	// Check if file type is allowed and validate size
	if err := h.validateMedia(ext, info.Size()); err != nil {
		return "", err
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	hashStr := fmt.Sprintf("%x", hash.Sum(nil))
	cachedPath := filepath.Join(h.cacheDir, hashStr+"."+ext)

	if _, err := os.Stat(cachedPath); err == nil {
		return cachedPath, nil
	}

	if err := os.Link(path, cachedPath); err != nil {
		if err := copyFile(path, cachedPath); err != nil {
			return "", fmt.Errorf("failed to copy file to cache: %w", err)
		}
	}

	return cachedPath, nil
}

func (h *handler) validateMedia(ext string, size int64) error {
	// Check if extension is allowed for any media type
	var maxSizeMB int
	var mediaType string

	for _, allowedExt := range h.config.AllowedTypes.Image {
		if ext == allowedExt {
			maxSizeMB = h.config.MaxSizeMB.Image
			mediaType = "image"
			break
		}
	}

	if maxSizeMB == 0 {
		for _, allowedExt := range h.config.AllowedTypes.Video {
			if ext == allowedExt {
				maxSizeMB = h.config.MaxSizeMB.Video
				mediaType = "video"
				break
			}
		}
	}

	if maxSizeMB == 0 && ext == "gif" {
		maxSizeMB = h.config.MaxSizeMB.Gif
		mediaType = "gif"
	}

	if maxSizeMB == 0 {
		for _, allowedExt := range h.config.AllowedTypes.Document {
			if ext == allowedExt {
				maxSizeMB = h.config.MaxSizeMB.Document
				mediaType = "document"
				break
			}
		}
	}

	if maxSizeMB == 0 {
		for _, allowedExt := range h.config.AllowedTypes.Voice {
			if ext == allowedExt {
				maxSizeMB = h.config.MaxSizeMB.Voice
				mediaType = "voice"
				break
			}
		}
	}

	if maxSizeMB == 0 {
		return fmt.Errorf("file type .%s is not allowed", ext)
	}

	maxSizeBytes := int64(maxSizeMB) * 1024 * 1024
	if size > maxSizeBytes {
		return fmt.Errorf("%s too large: %d > %d bytes", mediaType, size, maxSizeBytes)
	}

	return nil
}

func (h *handler) CleanupOldFiles(maxAge int64) error {
	entries, err := os.ReadDir(h.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	now := time.Now()
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info: %w", err)
		}

		age := now.Sub(info.ModTime())
		if age.Seconds() > float64(maxAge) {
			path := filepath.Join(h.cacheDir, info.Name())
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove old file: %w", err)
			}
		}
	}

	return nil
}

func (h *handler) downloadFromURL(mediaURL string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", mediaURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add WAHA API key authentication if available
	if h.wahaAPIKey != "" {
		req.Header.Set("X-Api-Key", h.wahaAPIKey)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Determine file extension from Content-Type or URL
	ext := h.getFileExtensionFromResponse(resp, mediaURL)

	// Create temporary file
	tempFile, err := os.CreateTemp(h.cacheDir, "download_*"+ext)
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Copy response body to temp file
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", "", fmt.Errorf("failed to save downloaded file: %w", err)
	}

	return tempFile.Name(), strings.TrimPrefix(ext, "."), nil
}

func (h *handler) processDownloadedFile(tempPath, ext string) (string, error) {
	file, err := os.Open(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to open downloaded file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	hashStr := fmt.Sprintf("%x", hash.Sum(nil))
	cachedPath := filepath.Join(h.cacheDir, hashStr+"."+ext)

	// Check if file already exists in cache
	if _, err := os.Stat(cachedPath); err == nil {
		return cachedPath, nil
	}

	// Copy temp file to cache
	if err := copyFile(tempPath, cachedPath); err != nil {
		return "", fmt.Errorf("failed to copy file to cache: %w", err)
	}

	return cachedPath, nil
}

func (h *handler) rewriteMediaURL(mediaURL string) string {
	// If no WAHA base URL is configured, return original URL
	if h.wahaBaseURL == "" {
		return mediaURL
	}

	// Parse the media URL
	u, err := url.Parse(mediaURL)
	if err != nil {
		return mediaURL // Return original if parsing fails
	}

	// Check if this is a localhost URL that needs rewriting
	if u.Host == "localhost:3000" || u.Host == "127.0.0.1:3000" || u.Host == "[::1]:3000" {
		// Parse the WAHA base URL to get the correct host
		wahaURL, err := url.Parse(h.wahaBaseURL)
		if err != nil {
			return mediaURL // Return original if WAHA URL parsing fails
		}

		// Replace the host with the WAHA host
		u.Scheme = wahaURL.Scheme
		u.Host = wahaURL.Host

		return u.String()
	}

	return mediaURL
}

func isURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func (h *handler) getFileExtensionFromResponse(resp *http.Response, mediaURL string) string {
	// Try to get extension from Content-Type header
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
			return exts[0]
		}
	}

	// Fallback to URL path extension
	if ext := filepath.Ext(mediaURL); ext != "" {
		return ext
	}

	// Default extension for unknown types
	return ".bin"
}

func copyFile(src, dst string) error {
	// Validate source path to prevent directory traversal
	if err := security.ValidateFilePath(src); err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
