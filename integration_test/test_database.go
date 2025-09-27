package integration_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"whatsignal/internal/database"
	"whatsignal/internal/migrations"
	"whatsignal/internal/models"
)

// TestDatabaseOptions configures test database creation
type TestDatabaseOptions struct {
	UseInMemory      bool
	SkipMigrations   bool
	MigrationsPath   string
	EncryptionSecret string
}

// NewTestDatabase creates a database instance for testing with proper migration handling
func NewTestDatabase(t *testing.T, opts *TestDatabaseOptions) (*database.Database, func()) {
	if opts == nil {
		opts = &TestDatabaseOptions{
			UseInMemory:    true,
			SkipMigrations: false,
		}
	}

	// Set up migration path based on test execution context
	if opts.MigrationsPath == "" {
		// Try to find migrations directory relative to test location
		searchPaths := []string{
			"../scripts/migrations",           // When running from integration_test directory
			"scripts/migrations",              // When running from project root
			"../../scripts/migrations",        // In case of nested test directories
			"/tmp/whatsignal-test-migrations", // Fallback for CI/CD
		}

		for _, path := range searchPaths {
			absPath, err := filepath.Abs(path)
			if err != nil {
				continue
			}
			if _, err := os.Stat(absPath); err == nil {
				opts.MigrationsPath = absPath
				break
			}
		}

		// If no migrations found and we're not skipping them, create a minimal schema
		if opts.MigrationsPath == "" && !opts.SkipMigrations {
			opts.MigrationsPath = createMinimalTestMigrations(t)
		}
	}

	// Set environment variable for migrations
	if opts.MigrationsPath != "" {
		oldMigrationsDir := os.Getenv("WHATSIGNAL_MIGRATIONS_DIR")
		_ = os.Setenv("WHATSIGNAL_MIGRATIONS_DIR", opts.MigrationsPath)
		defer func() {
			if oldMigrationsDir != "" {
				_ = os.Setenv("WHATSIGNAL_MIGRATIONS_DIR", oldMigrationsDir)
			} else {
				_ = os.Unsetenv("WHATSIGNAL_MIGRATIONS_DIR")
			}
		}()

		// Also update the migrations package variable
		migrations.MigrationsDir = opts.MigrationsPath
	}

	// Set encryption secret if provided
	if opts.EncryptionSecret != "" {
		oldSecret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
		_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", opts.EncryptionSecret)
		defer func() {
			if oldSecret != "" {
				_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", oldSecret)
			} else {
				_ = os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
			}
		}()
	}

	// Determine database path
	var dbPath string
	if opts.UseInMemory {
		dbPath = ":memory:"
	} else {
		tmpDir := t.TempDir()
		dbPath = filepath.Join(tmpDir, "test.db")
	}

	// Create database configuration
	config := &models.DatabaseConfig{
		Path:               dbPath,
		MaxOpenConnections: 10,
		MaxIdleConnections: 5,
		ConnMaxLifetimeSec: 300,
		ConnMaxIdleTimeSec: 60,
	}

	// Create database with migrations
	db, err := database.New(dbPath, config)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Return database and cleanup function
	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close test database: %v", err)
		}

		// Clean up test migrations if we created them
		if opts.MigrationsPath != "" && filepath.Base(opts.MigrationsPath) == "whatsignal-test-migrations" {
			_ = os.RemoveAll(filepath.Dir(opts.MigrationsPath))
		}
	}

	return db, cleanup
}

// createMinimalTestMigrations creates a minimal set of migrations for testing
func createMinimalTestMigrations(t *testing.T) string {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")

	if err := os.MkdirAll(migrationsDir, 0750); err != nil {
		t.Fatalf("Failed to create test migrations directory: %v", err)
	}

	// Create minimal schema migration
	migrationContent := `
-- Test migration for integration tests
CREATE TABLE IF NOT EXISTS contacts (
    contact_id TEXT PRIMARY KEY,
    phone_number TEXT,
    name TEXT,
    push_name TEXT,
    short_name TEXT,
    is_blocked INTEGER DEFAULT 0,
    is_group INTEGER DEFAULT 0,
    is_my_contact INTEGER DEFAULT 0,
    cached_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS message_mappings (
    whatsapp_chat_id TEXT NOT NULL,
    whatsapp_msg_id TEXT NOT NULL,
    signal_msg_id TEXT NOT NULL,
    session_name TEXT NOT NULL,
    delivery_status INTEGER DEFAULT 0,
    signal_timestamp DATETIME NOT NULL,
    forwarded_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (whatsapp_msg_id, signal_msg_id)
);

CREATE INDEX IF NOT EXISTS idx_message_mappings_whatsapp_id ON message_mappings(whatsapp_msg_id);
CREATE INDEX IF NOT EXISTS idx_message_mappings_signal_id ON message_mappings(signal_msg_id);
CREATE INDEX IF NOT EXISTS idx_message_mappings_session ON message_mappings(session_name);
CREATE INDEX IF NOT EXISTS idx_message_mappings_created_at ON message_mappings(created_at);

CREATE TABLE IF NOT EXISTS schema_migrations (
    filename TEXT PRIMARY KEY,
    applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

	migrationFile := filepath.Join(migrationsDir, "001_test_schema.sql")
	if err := os.WriteFile(migrationFile, []byte(migrationContent), 0600); err != nil {
		t.Fatalf("Failed to create test migration file: %v", err)
	}

	return migrationsDir
}

// CreateTestDatabaseWithData creates a database and populates it with test data
func CreateTestDatabaseWithData(t *testing.T) (*database.Database, func()) {
	db, cleanup := NewTestDatabase(t, &TestDatabaseOptions{
		UseInMemory:      true,
		EncryptionSecret: "test-secret-key-for-integration-tests-32bytes!!",
	})

	ctx := context.Background()

	// Add some test contacts
	testContacts := []models.Contact{
		{
			ContactID:   "alice@c.us",
			PhoneNumber: "+1234567890",
			Name:        "Alice Test",
			IsMyContact: true,
		},
		{
			ContactID:   "bob@c.us",
			PhoneNumber: "+0987654321",
			Name:        "Bob Test",
			IsMyContact: true,
		},
	}

	for _, contact := range testContacts {
		if err := db.SaveContact(ctx, &contact); err != nil {
			t.Errorf("Failed to save test contact: %v", err)
		}
	}

	return db, cleanup
}

// RunMigrationsForTest ensures migrations are properly run for a test database
func RunMigrationsForTest(t *testing.T, db *sql.DB) error {
	// Find migrations relative to test
	searchPaths := []string{
		"../scripts/migrations",
		"scripts/migrations",
		"../../scripts/migrations",
	}

	var migrationsPath string
	for _, path := range searchPaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			migrationsPath = absPath
			break
		}
	}

	if migrationsPath == "" {
		// Create minimal migrations if not found
		migrationsPath = createMinimalTestMigrations(t)
	}

	// Set migrations directory
	oldDir := migrations.MigrationsDir
	migrations.MigrationsDir = migrationsPath
	defer func() {
		migrations.MigrationsDir = oldDir
	}()

	// Run migrations
	return migrations.RunMigrations(db)
}
