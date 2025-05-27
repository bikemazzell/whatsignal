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

	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains directory traversal: %s", path)
	}

	// Check for absolute paths that might escape intended directories
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("absolute paths not allowed: %s", path)
	}

	return nil
}

// ValidateFilePathWithBase validates a file path against a base directory
func ValidateFilePathWithBase(path, baseDir string) error {
	if err := ValidateFilePath(path); err != nil {
		return err
	}

	// Resolve the full path
	fullPath := filepath.Join(baseDir, path)
	cleanPath := filepath.Clean(fullPath)
	cleanBase := filepath.Clean(baseDir)

	// Ensure the resolved path is still within the base directory
	if !strings.HasPrefix(cleanPath, cleanBase) {
		return fmt.Errorf("path escapes base directory: %s", path)
	}

	return nil
}
