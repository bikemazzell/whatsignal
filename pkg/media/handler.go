package media

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MaxSignalVideoSize = 100 * 1024 * 1024 // 100MB
	MaxSignalGifSize   = 25 * 1024 * 1024  // 25MB
	MaxSignalImageSize = 8 * 1024 * 1024   // 8MB
)

type Handler struct {
	cacheDir string
}

func NewHandler(cacheDir string) (*Handler, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	return &Handler{cacheDir: cacheDir}, nil
}

func (h *Handler) ProcessMedia(sourcePath string) (string, error) {
	hash, err := h.calculateFileHash(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate file hash: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(sourcePath))
	cachedPath := filepath.Join(h.cacheDir, hash+ext)

	if _, err := os.Stat(cachedPath); err == nil {
		return cachedPath, nil
	}

	if err := h.copyAndValidateFile(sourcePath, cachedPath); err != nil {
		return "", err
	}

	return cachedPath, nil
}

func (h *Handler) calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (h *Handler) copyAndValidateFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	fileInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if err := h.validateFileSize(src, fileInfo.Size()); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		os.Remove(dst)
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func (h *Handler) validateFileSize(path string, size int64) error {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".mp4", ".mov", ".avi":
		if size > MaxSignalVideoSize {
			return fmt.Errorf("video file too large: %d bytes (max: %d)", size, MaxSignalVideoSize)
		}
	case ".gif":
		if size > MaxSignalGifSize {
			return fmt.Errorf("GIF file too large: %d bytes (max: %d)", size, MaxSignalGifSize)
		}
	case ".jpg", ".jpeg", ".png":
		if size > MaxSignalImageSize {
			return fmt.Errorf("image file too large: %d bytes (max: %d)", size, MaxSignalImageSize)
		}
	}

	return nil
}

func (h *Handler) CleanupOldFiles(maxAge int64) error {
	entries, err := os.ReadDir(h.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	now := time.Now().Unix()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(h.cacheDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now-info.ModTime().Unix() > maxAge {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove old file %s: %w", path, err)
			}
		}
	}

	return nil
}
