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

		schemaContent, err = os.ReadFile(path) // #nosec G304 - Path validated by security.ValidateFilePath above
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
-- Version: 2
-- Created: 2024-05-21
-- Updated: 2025-06-25 (added session_name and media_type)

CREATE TABLE IF NOT EXISTS message_mappings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    whatsapp_chat_id TEXT NOT NULL,
    whatsapp_msg_id TEXT NOT NULL,
    signal_msg_id TEXT NOT NULL,
    signal_timestamp DATETIME NOT NULL,
    forwarded_at DATETIME NOT NULL,
    delivery_status TEXT NOT NULL,
    media_path TEXT,
    session_name TEXT NOT NULL DEFAULT 'default',
    media_type TEXT,
    chat_id_hash TEXT,
    whatsapp_msg_id_hash TEXT,
    signal_msg_id_hash TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_msg_id ON message_mappings(whatsapp_msg_id);
CREATE INDEX IF NOT EXISTS idx_signal_msg_id ON message_mappings(signal_msg_id);
CREATE INDEX IF NOT EXISTS idx_chat_time ON message_mappings(whatsapp_chat_id, forwarded_at);
CREATE INDEX IF NOT EXISTS idx_session_name ON message_mappings(session_name);
CREATE INDEX IF NOT EXISTS idx_session_chat ON message_mappings(session_name, whatsapp_chat_id);
CREATE INDEX IF NOT EXISTS idx_whatsapp_msg_id_hash ON message_mappings(whatsapp_msg_id_hash);
CREATE INDEX IF NOT EXISTS idx_signal_msg_id_hash ON message_mappings(signal_msg_id_hash);
CREATE INDEX IF NOT EXISTS idx_chat_id_hash ON message_mappings(chat_id_hash);

CREATE TRIGGER IF NOT EXISTS message_mappings_updated_at
AFTER UPDATE ON message_mappings
BEGIN
    UPDATE message_mappings SET updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;

-- Contacts table for caching WhatsApp contact information
CREATE TABLE IF NOT EXISTS contacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    contact_id TEXT NOT NULL UNIQUE,  -- WhatsApp ID like "1234567890@c.us"
    phone_number TEXT NOT NULL,       -- Just the phone number "1234567890"
    name TEXT,                        -- Contact book name (highest priority)
    push_name TEXT,                   -- User's display name (fallback)
    short_name TEXT,                  -- Shortened name
    is_blocked BOOLEAN DEFAULT FALSE,
    is_group BOOLEAN DEFAULT FALSE,
    is_my_contact BOOLEAN DEFAULT FALSE,
    cached_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_contact_id ON contacts(contact_id);
CREATE INDEX IF NOT EXISTS idx_phone_number ON contacts(phone_number);
CREATE INDEX IF NOT EXISTS idx_cached_at ON contacts(cached_at);

CREATE TRIGGER IF NOT EXISTS contacts_updated_at
AFTER UPDATE ON contacts
BEGIN
    UPDATE contacts SET updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;`
}
