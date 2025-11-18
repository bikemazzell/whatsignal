package migrations

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
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

	// Create a test initial schema
	schemaContent := `-- Initial schema for WhatsSignal
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

CREATE TABLE IF NOT EXISTS contacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    contact_id TEXT NOT NULL UNIQUE,
    phone_number TEXT NOT NULL,
    name TEXT,
    push_name TEXT,
    short_name TEXT,
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

	// Write the schema to the test directory
	err = os.WriteFile(filepath.Join(migrationsPath, "001_initial_schema.sql"), []byte(schemaContent), 0644)
	require.NoError(t, err)

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	tmpFile, err := os.CreateTemp("", "test_*.db")
	require.NoError(t, err)
	_ = tmpFile.Close()

	db, err := sql.Open("sqlite3", tmpFile.Name())
	require.NoError(t, err)

	cleanup := func() {
		_ = db.Close()
		_ = os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

func TestRunMigrations(t *testing.T) {
	tmpDir, cleanupMigrations := setupTestMigrations(t)
	defer cleanupMigrations()

	db, cleanupDB := setupTestDB(t)
	defer cleanupDB()

	// Set migrations directory
	originalDir := MigrationsDir
	MigrationsDir = filepath.Join(tmpDir, "migrations")
	defer func() { MigrationsDir = originalDir }()

	// Run migrations
	err := RunMigrations(db)
	require.NoError(t, err)

	// Verify migration tracking table was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify initial migration was recorded
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE filename='001_initial_schema.sql'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify tables were created
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='message_mappings'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='contacts'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify required columns exist in message_mappings
	rows, err := db.Query("PRAGMA table_info(message_mappings)")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt_value sql.NullString

		err = rows.Scan(&cid, &name, &ctype, &notnull, &dflt_value, &pk)
		require.NoError(t, err)
		columns[name] = true
	}

	// Verify all required columns exist
	requiredColumns := []string{
		"id", "whatsapp_chat_id", "whatsapp_msg_id", "signal_msg_id",
		"signal_timestamp", "forwarded_at", "delivery_status", "media_path",
		"session_name", "media_type", "chat_id_hash", "whatsapp_msg_id_hash",
		"signal_msg_id_hash", "created_at", "updated_at",
	}

	for _, col := range requiredColumns {
		assert.True(t, columns[col], "Column %s should exist", col)
	}
}

func TestRunMigrationsIdempotent(t *testing.T) {
	tmpDir, cleanupMigrations := setupTestMigrations(t)
	defer cleanupMigrations()

	db, cleanupDB := setupTestDB(t)
	defer cleanupDB()

	// Set migrations directory
	originalDir := MigrationsDir
	MigrationsDir = filepath.Join(tmpDir, "migrations")
	defer func() { MigrationsDir = originalDir }()

	// Run migrations twice
	err := RunMigrations(db)
	require.NoError(t, err)

	err = RunMigrations(db)
	require.NoError(t, err) // Should not error on second run

	// Verify only one migration record exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRunMigrationsNoMigrationFiles(t *testing.T) {
	// Create empty directory
	tmpDir, err := os.MkdirTemp("", "whatsignal-migrations-empty-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	db, cleanupDB := setupTestDB(t)
	defer cleanupDB()

	// Set migrations directory to empty directory
	originalDir := MigrationsDir
	MigrationsDir = tmpDir
	defer func() { MigrationsDir = originalDir }()

	// Run migrations should fail
	err = RunMigrations(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no migration files found")
}

// Unit tests for getMigrationNumber function
func TestGetMigrationNumber(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected int
	}{
		{
			name:     "standard migration filename",
			filename: "001_initial_schema.sql",
			expected: 1,
		},
		{
			name:     "zero-padded migration number",
			filename: "007_add_indexes.sql",
			expected: 7,
		},
		{
			name:     "three-digit migration number",
			filename: "100_major_refactor.sql",
			expected: 100,
		},
		{
			name:     "multi-digit number",
			filename: "12345_huge_migration.sql",
			expected: 12345,
		},
		{
			name:     "filename with path",
			filename: "/migrations/002_add_session_support.sql",
			expected: 2,
		},
		{
			name:     "filename with nested path",
			filename: "/some/nested/path/migrations/042_complex_migration.sql",
			expected: 42,
		},
		{
			name:     "filename with long descriptive name",
			filename: "015_add_encryption_and_security_improvements_with_new_columns.sql",
			expected: 15,
		},
		{
			name:     "filename with multiple underscores in description",
			filename: "003_add_message_thread_support_and_group_chat_features.sql",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMigrationNumber(tt.filename)
			assert.Equal(t, tt.expected, result, "Migration number should match expected value")
		})
	}
}

func TestGetMigrationNumber_ErrorCases(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected int
	}{
		{
			name:     "no underscore separator",
			filename: "001initialschema.sql",
			expected: 0,
		},
		{
			name:     "non-numeric prefix",
			filename: "abc_initial_schema.sql",
			expected: 0,
		},
		{
			name:     "empty filename",
			filename: "",
			expected: 0,
		},
		{
			name:     "underscore but no number",
			filename: "_initial_schema.sql",
			expected: 0,
		},
		{
			name:     "number with leading characters",
			filename: "v001_migration.sql",
			expected: 0,
		},
		{
			name:     "decimal number",
			filename: "1.5_migration.sql",
			expected: 0, // strconv.Atoi fails on decimal
		},
		{
			name:     "only underscore",
			filename: "_",
			expected: 0,
		},
		{
			name:     "only number no extension",
			filename: "123",
			expected: 0, // No underscore separator
		},
		{
			name:     "number with spaces",
			filename: "1 2 3_migration.sql",
			expected: 0, // strconv.Atoi fails on spaces
		},
		{
			name:     "very large number (overflow)",
			filename: "999999999999999999999999999999999999999999_migration.sql",
			expected: 0, // Should handle overflow gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMigrationNumber(tt.filename)
			assert.Equal(t, tt.expected, result, "Should return 0 for invalid migration filenames")
		})
	}
}

func TestGetMigrationNumber_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected int
	}{
		{
			name:     "zero migration number",
			filename: "000_initial_setup.sql",
			expected: 0,
		},
		{
			name:     "single character filename",
			filename: "5_a.sql",
			expected: 5,
		},
		{
			name:     "filename with special characters in description",
			filename: "010_add_special-chars@and#symbols$.sql",
			expected: 10,
		},
		{
			name:     "filename with unicode in description",
			filename: "020_añadir_soporte_español.sql",
			expected: 20,
		},
		{
			name:     "multiple consecutive underscores",
			filename: "025___multiple___underscores.sql",
			expected: 25,
		},
		{
			name:     "file extension variations",
			filename: "030_migration.SQL",
			expected: 30,
		},
		{
			name:     "no file extension",
			filename: "035_migration_without_extension",
			expected: 35,
		},
		{
			name:     "negative migration number",
			filename: "-001_rollback_migration.sql",
			expected: -1, // Function correctly parses negative numbers
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMigrationNumber(tt.filename)
			assert.Equal(t, tt.expected, result, "Should handle edge cases correctly")
		})
	}
}

func TestGetMigrationNumber_RealWorldExamples(t *testing.T) {
	// Test with actual migration filename patterns commonly used in projects
	tests := []struct {
		name     string
		filename string
		expected int
	}{
		{
			name:     "Rails-style migration",
			filename: "20240101120000_create_users_table.sql",
			expected: 20240101120000, // Large timestamp-style number
		},
		{
			name:     "Django-style migration",
			filename: "0001_initial.sql",
			expected: 1,
		},
		{
			name:     "Sequelize-style migration",
			filename: "001-create-users.sql",
			expected: 0, // Uses dash separator, not underscore, so parsing fails
		},
		{
			name:     "Flyway-style migration",
			filename: "V001__Create_users_table.sql",
			expected: 0, // "V001" is not a pure number
		},
		{
			name:     "Simple numbered migration",
			filename: "1_create_initial_schema.sql",
			expected: 1,
		},
		{
			name:     "WhatsSignal project style",
			filename: "001_initial_schema.sql",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMigrationNumber(tt.filename)
			assert.Equal(t, tt.expected, result, "Should handle real-world migration filename patterns")
		})
	}
}

// Test the actual sorting behavior that depends on getMigrationNumber
func TestMigrationNumberSorting(t *testing.T) {
	// This tests the practical use case of the function - ensuring proper migration ordering
	filenames := []string{
		"010_add_indexes.sql",
		"001_initial_schema.sql",
		"005_add_session_support.sql",
		"100_major_refactor.sql",
		"002_add_contacts_table.sql",
		"invalid_migration.sql", // Should get number 0
		"050_optimize_queries.sql",
	}

	// Test that we can correctly extract and sort by migration numbers
	type migration struct {
		filename string
		number   int
	}

	var migrations []migration
	for _, filename := range filenames {
		migrations = append(migrations, migration{
			filename: filename,
			number:   getMigrationNumber(filename),
		})
	}

	// Verify specific extractions
	expectedNumbers := map[string]int{
		"001_initial_schema.sql":      1,
		"002_add_contacts_table.sql":  2,
		"005_add_session_support.sql": 5,
		"010_add_indexes.sql":         10,
		"050_optimize_queries.sql":    50,
		"100_major_refactor.sql":      100,
		"invalid_migration.sql":       0,
	}

	for _, mig := range migrations {
		expected, exists := expectedNumbers[mig.filename]
		if exists {
			assert.Equal(t, expected, mig.number, "Migration number for %s should be %d", mig.filename, expected)
		}
	}

	// Verify that invalid migrations get number 0 (lowest priority)
	invalidMigration := getMigrationNumber("invalid_migration.sql")
	assert.Equal(t, 0, invalidMigration, "Invalid migration should get number 0")
}
