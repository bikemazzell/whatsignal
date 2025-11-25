package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateFilePath validates that a file path is safe and doesn't contain directory traversal attempts
func ValidateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Check for null bytes
	if len(path) != len(strings.ReplaceAll(path, "\x00", "")) {
		return fmt.Errorf("path contains null bytes")
	}

	// Clean path to resolve any .. or . components
	cleanPath := filepath.Clean(path)

	// Check for directory traversal attempts by looking for ".." in cleaned path
	// This catches attempts to escape the intended directory structure
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains directory traversal: %s", path)
	}

	// Check for path length to prevent DoS attacks
	if len(cleanPath) > 4096 {
		return fmt.Errorf("path too long (max 4096 characters)")
	}

	return nil
}

// ValidateFilePathWithBase validates a file path against a base directory
func ValidateFilePathWithBase(path, baseDir string) error {
	if err := ValidateFilePath(path); err != nil {
		return err
	}

	// If path is absolute, check if it's within the base directory
	if filepath.IsAbs(path) {
		cleanPath := filepath.Clean(path)
		cleanBase := filepath.Clean(baseDir)

		// Ensure absolute path is within the base directory
		if !strings.HasPrefix(cleanPath, cleanBase) {
			return fmt.Errorf("path escapes base directory: %s", path)
		}
	} else {
		// For relative paths, resolve against the base directory
		fullPath := filepath.Join(baseDir, path)
		cleanPath := filepath.Clean(fullPath)
		cleanBase := filepath.Clean(baseDir)

		// Ensure resolved path is still within the base directory
		if !strings.HasPrefix(cleanPath, cleanBase) {
			return fmt.Errorf("path escapes base directory: %s", path)
		}
	}

	return nil
}

// ValidateFilePathStrict validates that a file path is relative and safe for production use
func ValidateFilePathStrict(path string) error {
	if err := ValidateFilePath(path); err != nil {
		return err
	}

	// In production, only allow relative paths
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths not allowed in production: %s", path)
	}

	return nil
}
