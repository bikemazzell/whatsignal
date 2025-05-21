package migrations

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	// MigrationsDir can be overridden in tests or by the application
	MigrationsDir = "scripts/migrations"
)

// GetInitialSchema returns the initial database schema
func GetInitialSchema() (string, error) {
	// Try to find schema file in different locations
	searchPaths := []string{
		filepath.Join(MigrationsDir, "001_initial_schema.sql"),
		filepath.Join("..", "..", MigrationsDir, "001_initial_schema.sql"),
		filepath.Join("..", MigrationsDir, "001_initial_schema.sql"),
	}

	var schemaContent []byte
	var err error

	for _, path := range searchPaths {
		schemaContent, err = os.ReadFile(path)
		if err == nil {
			return string(schemaContent), nil
		}
	}

	return "", fmt.Errorf("could not find schema file in any location")
}
