package media

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	MaxSignalImageSize = 5 * 1024 * 1024   // 5MB
	MaxSignalVideoSize = 100 * 1024 * 1024 // 100MB
	MaxSignalGifSize   = 25 * 1024 * 1024  // 25MB
)

type Handler interface {
	ProcessMedia(path string) (string, error)
	CleanupOldFiles(maxAge int64) error
}

type handler struct {
	cacheDir string
}

func NewHandler(cacheDir string) (Handler, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &handler{
		cacheDir: cacheDir,
	}, nil
}

func (h *handler) ProcessMedia(path string) (string, error) {
	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	// Check file size based on extension
	ext := filepath.Ext(path)
	switch ext {
	case ".jpg", ".jpeg", ".png":
		if info.Size() > MaxSignalImageSize {
			return "", fmt.Errorf("image too large: %d > %d", info.Size(), MaxSignalImageSize)
		}
	case ".mp4", ".mov":
		if info.Size() > MaxSignalVideoSize {
			return "", fmt.Errorf("video too large: %d > %d", info.Size(), MaxSignalVideoSize)
		}
	case ".gif":
		if info.Size() > MaxSignalGifSize {
			return "", fmt.Errorf("gif too large: %d > %d", info.Size(), MaxSignalGifSize)
		}
	}

	// Calculate file hash
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	// Create cache file path
	hashStr := fmt.Sprintf("%x", hash.Sum(nil))
	cachedPath := filepath.Join(h.cacheDir, hashStr+ext)

	// Check if file already exists in cache
	if _, err := os.Stat(cachedPath); err == nil {
		return cachedPath, nil
	}

	// Copy file to cache
	if err := os.Link(path, cachedPath); err != nil {
		// If hard linking fails (e.g., across devices), fall back to copying
		if err := copyFile(path, cachedPath); err != nil {
			return "", fmt.Errorf("failed to copy file to cache: %w", err)
		}
	}

	return cachedPath, nil
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
			continue
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
