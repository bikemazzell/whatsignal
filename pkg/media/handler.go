package media

import (
	"crypto/sha256"
	"fmt"
	"io"
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
	cacheDir string
	config   models.MediaConfig
}

func NewHandler(cacheDir string, config models.MediaConfig) (Handler, error) {
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &handler{
		cacheDir: cacheDir,
		config:   config,
	}, nil
}

func (h *handler) ProcessMedia(path string) (string, error) {
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
	cachedPath := filepath.Join(h.cacheDir, hashStr+ext)

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
