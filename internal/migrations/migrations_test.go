package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestMigrations(t *testing.T) (string, func()) {
	// Create a temporary directory for test migrations
	tmpDir, err := os.MkdirTemp("", "whatsignal-migrations-test")
	require.NoError(t, err)

	// Create migrations directory
	migrationsPath := filepath.Join(tmpDir, "migrations")
	err = os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	// Create a test schema file
	schemaContent := `CREATE TABLE IF NOT EXISTS message_mappings (
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

	// Write the schema to the test directory
	err = os.WriteFile(filepath.Join(migrationsPath, "001_initial_schema.sql"), []byte(schemaContent), 0644)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestGetInitialSchema(t *testing.T) {
	tmpDir, cleanup := setupTestMigrations(t)
	defer cleanup()

	// Test with direct path
	originalDir := MigrationsDir
	MigrationsDir = filepath.Join(tmpDir, "migrations")
	defer func() { MigrationsDir = originalDir }()

	schema, err := GetInitialSchema()
	require.NoError(t, err)
	assert.Contains(t, schema, "CREATE TABLE IF NOT EXISTS message_mappings")
	assert.Contains(t, schema, "whatsapp_chat_id TEXT NOT NULL")
	assert.Contains(t, schema, "CREATE INDEX IF NOT EXISTS idx_whatsapp_msg_id")
	assert.Contains(t, schema, "CREATE TRIGGER IF NOT EXISTS message_mappings_updated_at")

	// Test with non-existent directory - should return embedded schema
	MigrationsDir = "nonexistent/path"
	schema, err = GetInitialSchema()
	assert.NoError(t, err) // Should not error, returns embedded schema
	assert.Contains(t, schema, "CREATE TABLE IF NOT EXISTS message_mappings")
}

func TestGetInitialSchemaWithExecutablePath(t *testing.T) {
	tmpDir, cleanup := setupTestMigrations(t)
	defer cleanup()

	// Save current working directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	// Change to temp directory to simulate running from a different location
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Set migrations dir relative to current directory
	originalDir := MigrationsDir
	MigrationsDir = "migrations"
	defer func() { MigrationsDir = originalDir }()

	schema, err := GetInitialSchema()
	require.NoError(t, err)
	assert.Contains(t, schema, "CREATE TABLE IF NOT EXISTS message_mappings")
}

func TestGetInitialSchemaWithParentDirectorySearch(t *testing.T) {
	tmpDir, cleanup := setupTestMigrations(t)
	defer cleanup()

	// Create a deeper directory structure
	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	err := os.MkdirAll(deepDir, 0755)
	require.NoError(t, err)

	// Save current working directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	// Change to the deep directory
	err = os.Chdir(deepDir)
	require.NoError(t, err)

	// Set migrations dir to look for
	originalDir := MigrationsDir
	MigrationsDir = filepath.Join(tmpDir, "migrations")
	defer func() { MigrationsDir = originalDir }()

	schema, err := GetInitialSchema()
	require.NoError(t, err)
	assert.Contains(t, schema, "CREATE TABLE IF NOT EXISTS message_mappings")
}

func TestSchemaContent(t *testing.T) {
	tmpDir, cleanup := setupTestMigrations(t)
	defer cleanup()

	originalDir := MigrationsDir
	MigrationsDir = filepath.Join(tmpDir, "migrations")
	defer func() { MigrationsDir = originalDir }()

	schema, err := GetInitialSchema()
	require.NoError(t, err)

	// Test schema structure
	assert.True(t, strings.Contains(schema, "id INTEGER PRIMARY KEY AUTOINCREMENT"))
	assert.True(t, strings.Contains(schema, "whatsapp_chat_id TEXT NOT NULL"))
	assert.True(t, strings.Contains(schema, "whatsapp_msg_id TEXT NOT NULL"))
	assert.True(t, strings.Contains(schema, "signal_msg_id TEXT NOT NULL"))
	assert.True(t, strings.Contains(schema, "signal_timestamp DATETIME NOT NULL"))
	assert.True(t, strings.Contains(schema, "forwarded_at DATETIME NOT NULL"))
	assert.True(t, strings.Contains(schema, "delivery_status TEXT NOT NULL"))
	assert.True(t, strings.Contains(schema, "media_path TEXT"))
	assert.True(t, strings.Contains(schema, "created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP"))
	assert.True(t, strings.Contains(schema, "updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP"))

	// Test indexes
	assert.True(t, strings.Contains(schema, "CREATE INDEX IF NOT EXISTS idx_whatsapp_msg_id"))
	assert.True(t, strings.Contains(schema, "CREATE INDEX IF NOT EXISTS idx_signal_msg_id"))
	assert.True(t, strings.Contains(schema, "CREATE INDEX IF NOT EXISTS idx_chat_time"))

	// Test trigger
	assert.True(t, strings.Contains(schema, "CREATE TRIGGER IF NOT EXISTS message_mappings_updated_at"))
	assert.True(t, strings.Contains(schema, "UPDATE message_mappings SET updated_at = CURRENT_TIMESTAMP"))
}
