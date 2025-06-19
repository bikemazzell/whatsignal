package migrations

import (
	"os"
	"path/filepath"
	"whatsignal/internal/security"
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
		filepath.Join("..", "..", "..", MigrationsDir, "001_initial_schema.sql"),
	}

	var schemaContent []byte
	var err error

	for _, path := range searchPaths {
		// Validate file path to prevent directory traversal
		if err := security.ValidateFilePath(path); err != nil {
			continue // Skip invalid paths
		}

		schemaContent, err = os.ReadFile(path)
		if err == nil {
			return string(schemaContent), nil
		}
	}

	// Fallback to embedded schema for tests
	return getEmbeddedSchema(), nil
}

// getEmbeddedSchema returns the schema as a string for cases where the file cannot be found
func getEmbeddedSchema() string {
	return `-- Initial schema for WhatsSignal
-- Version: 1
-- Created: 2024-05-21

CREATE TABLE IF NOT EXISTS message_mappings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    whatsapp_chat_id TEXT NOT NULL,
    whatsapp_msg_id TEXT NOT NULL,
    signal_msg_id TEXT NOT NULL,
    signal_timestamp DATETIME NOT NULL,
    forwarded_at DATETIME NOT NULL,
    delivery_status TEXT NOT NULL,
    media_path TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_msg_id ON message_mappings(whatsapp_msg_id);
CREATE INDEX IF NOT EXISTS idx_signal_msg_id ON message_mappings(signal_msg_id);
CREATE INDEX IF NOT EXISTS idx_chat_time ON message_mappings(whatsapp_chat_id, forwarded_at);

CREATE TRIGGER IF NOT EXISTS message_mappings_updated_at
AFTER UPDATE ON message_mappings
BEGIN
    UPDATE message_mappings SET updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;`
}
