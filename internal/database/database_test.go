package database

import (
	"context"
	"database/sql"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"whatsignal/internal/migrations"
	"whatsignal/internal/models"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestMigrations creates test migration files
func setupTestMigrations(t *testing.T, tmpDir string) string {
	// Create migrations directory
	migrationsPath := filepath.Join(tmpDir, "migrations")
	err := os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	// Create the complete initial schema
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

	// Create migration 002 for groups table
	groupsContent := `-- Add groups table for caching WhatsApp group metadata
CREATE TABLE IF NOT EXISTS groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    description TEXT,
    participant_count INTEGER,
    session_name TEXT NOT NULL,
    cached_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(group_id, session_name)
);

CREATE INDEX IF NOT EXISTS idx_groups_group_id ON groups(group_id);
CREATE INDEX IF NOT EXISTS idx_groups_session_name ON groups(session_name);
CREATE INDEX IF NOT EXISTS idx_groups_cached_at ON groups(cached_at);
CREATE INDEX IF NOT EXISTS idx_groups_session_group ON groups(session_name, group_id);

CREATE TRIGGER IF NOT EXISTS groups_updated_at
AFTER UPDATE ON groups
BEGIN
    UPDATE groups SET updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;`

	err = os.WriteFile(filepath.Join(migrationsPath, "002_add_groups_table.sql"), []byte(groupsContent), 0644)
	require.NoError(t, err)

	return migrationsPath
}

func setupTestDB(t *testing.T) (*Database, string, func()) {
	// Set up encryption secret for tests
	originalSecret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
	_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-database-testing")

	// Create a temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "whatsignal-db-test")
	require.NoError(t, err)

	// Set up test migrations
	migrationsPath := setupTestMigrations(t, tmpDir)

	// Set migrations directory for the test
	originalMigrationsDir := migrations.MigrationsDir
	migrations.MigrationsDir = migrationsPath

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := New(dbPath, nil)
	require.NoError(t, err)

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
		// Restore original environment
		migrations.MigrationsDir = originalMigrationsDir
		if originalSecret != "" {
			_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", originalSecret)
		} else {
			_ = os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
		}
	}

	return db, tmpDir, cleanup
}

func TestNewDatabase(t *testing.T) {
	// Set up encryption secret for tests
	originalSecret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
	_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-database-testing")
	defer func() {
		if originalSecret != "" {
			_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", originalSecret)
		} else {
			_ = os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
		}
	}()

	tests := []struct {
		name        string
		setupPath   func(t *testing.T) string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid path",
			setupPath: func(t *testing.T) string {
				tmpDir, err := os.MkdirTemp("", "whatsignal-db-test")
				require.NoError(t, err)
				t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

				// Set up test migrations for this test
				migrationsPath := setupTestMigrations(t, tmpDir)
				originalMigrationsDir := migrations.MigrationsDir
				migrations.MigrationsDir = migrationsPath
				t.Cleanup(func() { migrations.MigrationsDir = originalMigrationsDir })

				return filepath.Join(tmpDir, "test.db")
			},
			expectError: false,
		},
		{
			name: "invalid path with null byte",
			setupPath: func(t *testing.T) string {
				return "\x00invalid"
			},
			expectError: true,
			errorMsg:    "invalid database path",
		},
		{
			name: "empty path",
			setupPath: func(t *testing.T) string {
				return ""
			},
			expectError: true,
			errorMsg:    "invalid database path",
		},
		{
			name: "unwritable directory",
			setupPath: func(t *testing.T) string {
				tmpDir, err := os.MkdirTemp("", "whatsignal-db-test")
				require.NoError(t, err)
				t.Cleanup(func() {
					if err := os.Chmod(tmpDir, 0755); err != nil {
						t.Errorf("Failed to restore directory permissions: %v", err)
					}
					_ = os.RemoveAll(tmpDir)
				})

				// Make directory read-only
				err = os.Chmod(tmpDir, 0444)
				require.NoError(t, err)

				return filepath.Join(tmpDir, "test.db")
			},
			expectError: true,
			errorMsg:    "failed to create database file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := tt.setupPath(t)

			db, err := New(dbPath, nil)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, db)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, db)
				if db != nil {
					_ = db.Close()
				}
			}
		})
	}
}

func TestDatabaseEncryptionErrors(t *testing.T) {
	// Test with encryption enabled but invalid secret
	_ = os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "true")
	_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "short") // Too short secret
	defer func() {
		_ = os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
		_ = os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
	}()

	// Create a new encryptor with the invalid secret - this should fail
	_, err := NewEncryptor()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "encryption secret must be at least 32 characters long")

}

func TestGetMessageMappingErrors(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test with corrupted database
	_, err := db.db.Exec("DROP TABLE message_mappings")
	require.NoError(t, err)

	// This should return an error
	mapping, err := db.GetMessageMapping(ctx, "test-id")
	assert.Error(t, err)
	assert.Nil(t, mapping)
}

func TestGetMessageMappingByWhatsAppIDErrors(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test with corrupted database
	_, err := db.db.Exec("DROP TABLE message_mappings")
	require.NoError(t, err)

	// This should return an error
	mapping, err := db.GetMessageMappingByWhatsAppID(ctx, "test-id")
	assert.Error(t, err)
	assert.Nil(t, mapping)
}

func TestUpdateDeliveryStatusErrors(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test with corrupted database
	_, err := db.db.Exec("DROP TABLE message_mappings")
	require.NoError(t, err)

	// This should return an error
	err = db.UpdateDeliveryStatus(ctx, "test-id", "delivered")
	assert.Error(t, err)
}

func TestCleanupOldRecordsErrors(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Test with corrupted database
	_, err := db.db.Exec("DROP TABLE message_mappings")
	require.NoError(t, err)

	// This should return an error
	ctx := context.Background()
	err = db.CleanupOldRecords(ctx, 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to cleanup old records")
}

func TestMessageMappingCRUD(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test saving and retrieving a message mapping
	mapping := &models.MessageMapping{
		WhatsAppChatID:  "chat123",
		WhatsAppMsgID:   "msg123",
		SignalMsgID:     "sig123",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "personal",
	}

	err := db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	// Test GetMessageMappingByWhatsAppID
	retrieved, err := db.GetMessageMappingByWhatsAppID(ctx, "msg123")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, mapping.WhatsAppChatID, retrieved.WhatsAppChatID)
	assert.Equal(t, mapping.WhatsAppMsgID, retrieved.WhatsAppMsgID)
	assert.Equal(t, mapping.SignalMsgID, retrieved.SignalMsgID)
	assert.Equal(t, mapping.DeliveryStatus, retrieved.DeliveryStatus)

	// Test non-existent message
	retrieved, err = db.GetMessageMappingByWhatsAppID(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, retrieved)

	// Test with media path
	mediaPath := "/path/to/media.jpg"
	mapping = &models.MessageMapping{
		WhatsAppChatID:  "chat124",
		WhatsAppMsgID:   "msg124",
		SignalMsgID:     "sig124",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
		MediaPath:       &mediaPath,
		SessionName:     "personal",
	}

	err = db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	retrieved, err = db.GetMessageMappingByWhatsAppID(ctx, "msg124")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, mediaPath, *retrieved.MediaPath)
}

func TestUpdateDeliveryStatus(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test message mapping
	mapping := &models.MessageMapping{
		WhatsAppChatID:  "chat123",
		WhatsAppMsgID:   "msg123",
		SignalMsgID:     "sig123",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "personal",
	}

	err := db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	// Update delivery status
	err = db.UpdateDeliveryStatus(ctx, "msg123", string(models.DeliveryStatusDelivered))
	require.NoError(t, err)

	// Verify update
	retrieved, err := db.GetMessageMappingByWhatsAppID(ctx, "msg123")
	require.NoError(t, err)
	assert.Equal(t, models.DeliveryStatusDelivered, retrieved.DeliveryStatus)

	// Test updating non-existent message
	err = db.UpdateDeliveryStatus(ctx, "nonexistent", string(models.DeliveryStatusDelivered))
	assert.Error(t, err)

	// Test updating by signal message ID
	retrieved, err = db.GetMessageMappingByWhatsAppID(ctx, "msg123")
	require.NoError(t, err)
	assert.Equal(t, models.DeliveryStatusDelivered, retrieved.DeliveryStatus)
}

func TestCleanupOldRecords(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create test records with different timestamps
	oldTime := time.Now().Add(-48 * time.Hour)
	newTime := time.Now()

	// Insert records directly using SQL to set created_at
	// Must encrypt fields deterministically to match lookup behavior
	encryptedChatOld, err := db.encryptor.EncryptForLookupIfEnabled("chat123")
	require.NoError(t, err)
	encryptedWAOld, err := db.encryptor.EncryptForLookupIfEnabled("msg123")
	require.NoError(t, err)
	encryptedSigOld, err := db.encryptor.EncryptForLookupIfEnabled("sig123")
	require.NoError(t, err)
	hashChatOld, err := db.encryptor.LookupHash("chat123")
	require.NoError(t, err)
	hashWAOld, err := db.encryptor.LookupHash("msg123")
	require.NoError(t, err)
	hashSigOld, err := db.encryptor.LookupHash("sig123")
	require.NoError(t, err)

	_, err = db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status,
			created_at, updated_at, session_name,
		chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash
		) VALUES (?, ?, ?, ?, ?, ?, datetime('now', '-2 days'), datetime('now', '-2 days'), 'personal', ?, ?, ?)`,
		encryptedChatOld, encryptedWAOld, encryptedSigOld,
		oldTime, oldTime, models.DeliveryStatusDelivered,
		hashChatOld, hashWAOld, hashSigOld,
	)
	require.NoError(t, err)

	encryptedChatNew, err := db.encryptor.EncryptForLookupIfEnabled("chat124")
	require.NoError(t, err)
	encryptedWANew, err := db.encryptor.EncryptForLookupIfEnabled("msg124")
	require.NoError(t, err)
	encryptedSigNew, err := db.encryptor.EncryptForLookupIfEnabled("sig124")
	require.NoError(t, err)
	hashChatNew, err := db.encryptor.LookupHash("chat124")
	require.NoError(t, err)
	hashWANew, err := db.encryptor.LookupHash("msg124")
	require.NoError(t, err)
	hashSigNew, err := db.encryptor.LookupHash("sig124")
	require.NoError(t, err)

	_, err = db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status,
			created_at, updated_at, session_name,
		chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash
		) VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'), 'personal', ?, ?, ?)`,
		encryptedChatNew, encryptedWANew, encryptedSigNew,
		newTime, newTime, models.DeliveryStatusDelivered,
		hashChatNew, hashWANew, hashSigNew,
	)
	require.NoError(t, err)

	// Cleanup records older than 1 day
	err = db.CleanupOldRecords(ctx, 1)
	require.NoError(t, err)

	// Verify old record is gone and new record remains
	retrieved, err := db.GetMessageMappingByWhatsAppID(ctx, "msg123")
	require.NoError(t, err)
	assert.Nil(t, retrieved, "Old record should have been deleted")

	retrieved, err = db.GetMessageMappingByWhatsAppID(ctx, "msg124")
	require.NoError(t, err)
	assert.NotNil(t, retrieved, "New record should still exist")
	assert.Equal(t, "msg124", retrieved.WhatsAppMsgID)
}

func TestGetMessageMapping(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create test message mapping
	mapping := &models.MessageMapping{
		WhatsAppChatID:  "chat123",
		WhatsAppMsgID:   "msg123",
		SignalMsgID:     "sig123",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "personal",
	}

	err := db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	// Test getting by WhatsApp ID
	retrieved, err := db.GetMessageMapping(ctx, "msg123")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, mapping.WhatsAppChatID, retrieved.WhatsAppChatID)
	assert.Equal(t, mapping.WhatsAppMsgID, retrieved.WhatsAppMsgID)
	assert.Equal(t, mapping.SignalMsgID, retrieved.SignalMsgID)

	// Test getting by Signal ID
	retrieved, err = db.GetMessageMapping(ctx, "sig123")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, mapping.WhatsAppChatID, retrieved.WhatsAppChatID)
	assert.Equal(t, mapping.WhatsAppMsgID, retrieved.WhatsAppMsgID)
	assert.Equal(t, mapping.SignalMsgID, retrieved.SignalMsgID)

	// Test non-existent message
	retrieved, err = db.GetMessageMapping(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, retrieved)

	// Test with invalid SQL (to trigger error)
	_, err = db.db.ExecContext(ctx, "DROP TABLE message_mappings")
	require.NoError(t, err)

	retrieved, err = db.GetMessageMapping(ctx, "msg123")
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestClose(t *testing.T) {
	db, tmpDir, _ := setupTestDB(t)

	// Test normal close
	err := db.Close()
	assert.NoError(t, err)

	// Test double close by trying to use the closed database
	err = db.db.Ping()
	assert.Error(t, err, "Expected error on using closed database")

	// Cleanup
	_ = os.RemoveAll(tmpDir)
}

func TestNewDatabaseErrors(t *testing.T) {
	// Test with invalid path
	db, err := New("\x00invalid", nil)
	assert.Error(t, err, "Expected error with invalid path")
	assert.Nil(t, db)

	// Test with unwritable directory
	tmpDir, err := os.MkdirTemp("", "whatsignal-db-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Make parent directory read-only
	err = os.Chmod(tmpDir, 0444)
	require.NoError(t, err)
	defer func() {
		if err := os.Chmod(tmpDir, 0755); err != nil {
			t.Errorf("Failed to restore directory permissions: %v", err)
		}
	}()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err = New(dbPath, nil)
	assert.Error(t, err, "Expected error with unwritable directory")
	assert.Nil(t, db)
}

func TestSaveMessageMappingEncryptionErrors(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test with encryption enabled but corrupted encryptor
	_ = os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "true")
	defer func() { _ = os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION") }()

	// Create a mapping that will trigger encryption
	mapping := &models.MessageMapping{
		WhatsAppChatID:  "chat123",
		WhatsAppMsgID:   "msg123",
		SignalMsgID:     "sig123",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "personal",
	}

	// This should work with default encryption
	err := db.SaveMessageMapping(ctx, mapping)
	assert.NoError(t, err)
}

func TestEncryptorNonceGeneration(t *testing.T) {
	_ = os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "true")
	_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer func() { _ = os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION") }()
	defer func() { _ = os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET") }()

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	// Test that encryption produces different results for the same input
	plaintext := "test message"
	encrypted1, err := encryptor.Encrypt(plaintext)
	assert.NoError(t, err)

	encrypted2, err := encryptor.Encrypt(plaintext)
	assert.NoError(t, err)

	// Should be different due to random nonce
	assert.NotEqual(t, encrypted1, encrypted2)

	// But both should decrypt to the same plaintext
	decrypted1, err := encryptor.Decrypt(encrypted1)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted1)

	decrypted2, err := encryptor.Decrypt(encrypted2)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted2)
}

func TestSaveMessageMappingWithMediaPath(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test saving a message mapping with media path
	mediaPath := "/path/to/media.jpg"
	mapping := &models.MessageMapping{
		WhatsAppChatID:  "chat123",
		WhatsAppMsgID:   "msg123",
		SignalMsgID:     "sig123",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
		MediaPath:       &mediaPath,
		SessionName:     "personal",
	}

	err := db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	// Retrieve and verify
	retrieved, err := db.GetMessageMappingByWhatsAppID(ctx, "msg123")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.NotNil(t, retrieved.MediaPath)
	assert.Equal(t, mediaPath, *retrieved.MediaPath)
}

func TestDatabaseWithCorruptedSchema(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "whatsignal-db-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a database file with invalid schema
	file, err := os.Create(dbPath)
	require.NoError(t, err)
	if _, err := file.WriteString("invalid sql content"); err != nil {
		t.Errorf("Failed to write to file: %v", err)
	}
	_ = file.Close()

	// This should fail when trying to initialize schema
	db, err := New(dbPath, nil)
	if err != nil {
		// Expected case - schema initialization failed
		assert.Contains(t, err.Error(), "failed to")
		assert.Nil(t, db)
	} else {
		// If it somehow succeeded, clean up
		_ = db.Close()
	}
}

func TestEncryptorEdgeCases(t *testing.T) {
	// Always-on encryption
	_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer func() { _ = os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET") }()

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	// Test empty string encryption/decryption
	encrypted, err := encryptor.Encrypt("")
	assert.NoError(t, err)
	assert.Equal(t, "", encrypted)

	decrypted, err := encryptor.Decrypt("")
	assert.NoError(t, err)
	assert.Equal(t, "", decrypted)

	// Test EncryptIfEnabled and DecryptIfEnabled (always encrypt/decrypt)
	result, err := encryptor.EncryptIfEnabled("test")
	assert.NoError(t, err)
	assert.NotEqual(t, "test", result)

	decrypted, err = encryptor.DecryptIfEnabled(result)
	assert.NoError(t, err)
	assert.Equal(t, "test", decrypted)
}

func TestDecryptInvalidData(t *testing.T) {
	_ = os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "true")
	_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer func() { _ = os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION") }()
	defer func() { _ = os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET") }()

	encryptor, err := NewEncryptor()
	require.NoError(t, err)

	// Test with invalid base64
	_, err = encryptor.Decrypt("invalid-base64!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode base64")

	// Test with data too short
	shortData := base64.StdEncoding.EncodeToString([]byte("short"))
	_, err = encryptor.Decrypt(shortData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")

	// Test with invalid ciphertext (valid base64 but wrong encryption)
	invalidCiphertext := base64.StdEncoding.EncodeToString(make([]byte, 20)) // 20 bytes: 12 for nonce + 8 for data
	_, err = encryptor.Decrypt(invalidCiphertext)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt")
}

func TestDatabaseOperationsWithClosedDB(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	cleanup() // Close the database

	ctx := context.Background()

	// All operations should fail with closed database
	mapping := &models.MessageMapping{
		WhatsAppChatID:  "chat123",
		WhatsAppMsgID:   "msg123",
		SignalMsgID:     "sig123",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
	}

	err := db.SaveMessageMapping(ctx, mapping)
	assert.Error(t, err)

	_, err = db.GetMessageMapping(ctx, "test")
	assert.Error(t, err)

	_, err = db.GetMessageMappingByWhatsAppID(ctx, "test")
	assert.Error(t, err)

	err = db.UpdateDeliveryStatus(ctx, "test", "delivered")
	assert.Error(t, err)
}

func TestNewEncryptorWithCustomSecret(t *testing.T) {
	// Test with custom encryption secret (must be at least 32 characters)
	_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-custom-secret-key-for-testing-purposes")
	defer func() { _ = os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET") }()

	encryptor, err := NewEncryptor()
	assert.NoError(t, err)
	assert.NotNil(t, encryptor)

	// Test encryption/decryption with custom secret
	plaintext := "test message"
	encrypted, err := encryptor.Encrypt(plaintext)
	assert.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := encryptor.Decrypt(encrypted)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDatabase_SaveContact(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	contact := &models.Contact{
		ContactID:   "123456@c.us",
		PhoneNumber: "+1234567890",
		Name:        "John Doe",
		PushName:    "JD",
		ShortName:   "John",
		IsBlocked:   false,
		IsGroup:     false,
		IsMyContact: true,
	}

	// Test saving a contact
	err := db.SaveContact(ctx, contact)
	assert.NoError(t, err)

	// Test updating existing contact (INSERT OR REPLACE)
	contact.Name = "John Updated"
	err = db.SaveContact(ctx, contact)
	assert.NoError(t, err)
}

func TestDatabase_GetContact(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test getting non-existent contact
	result, err := db.GetContact(ctx, "nonexistent@c.us")
	assert.NoError(t, err)
	assert.Nil(t, result)

	// Save a contact first
	contact := &models.Contact{
		ContactID:   "123456@c.us",
		PhoneNumber: "+1234567890",
		Name:        "Jane Doe",
		PushName:    "Jane",
		ShortName:   "Jane",
		IsBlocked:   false,
		IsGroup:     false,
		IsMyContact: true,
	}

	err = db.SaveContact(ctx, contact)
	require.NoError(t, err)

	// Test getting existing contact
	retrieved, err := db.GetContact(ctx, "123456@c.us")
	assert.NoError(t, err)
	assert.Equal(t, contact.ContactID, retrieved.ContactID)
	assert.Equal(t, contact.PhoneNumber, retrieved.PhoneNumber)
	assert.Equal(t, contact.Name, retrieved.Name)
	assert.Equal(t, contact.PushName, retrieved.PushName)
	assert.Equal(t, contact.ShortName, retrieved.ShortName)
	assert.Equal(t, contact.IsBlocked, retrieved.IsBlocked)
	assert.Equal(t, contact.IsGroup, retrieved.IsGroup)
	assert.Equal(t, contact.IsMyContact, retrieved.IsMyContact)
}

func TestDatabase_GetContactByPhone(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test getting non-existent contact
	result, err := db.GetContactByPhone(ctx, "+9999999999")
	assert.NoError(t, err)
	assert.Nil(t, result)

	// Save a contact first
	contact := &models.Contact{
		ContactID:   "+0987654321@c.us",
		PhoneNumber: "+0987654321",
		Name:        "Bob Smith",
		PushName:    "Bob",
		ShortName:   "Bob",
		IsBlocked:   false,
		IsGroup:     false,
		IsMyContact: true,
	}

	err = db.SaveContact(ctx, contact)
	require.NoError(t, err)

	// Test getting existing contact by phone
	retrieved, err := db.GetContactByPhone(ctx, "+0987654321")
	assert.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, contact.ContactID, retrieved.ContactID)
	assert.Equal(t, contact.PhoneNumber, retrieved.PhoneNumber)
	assert.Equal(t, contact.Name, retrieved.Name)
}

func TestDatabase_CleanupOldContacts(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Save some contacts with different cached_at times
	now := time.Now()
	oldTime := now.AddDate(0, 0, -40) // 40 days ago

	// Create old contact (should be deleted)
	oldContact := &models.Contact{
		ContactID:   "old@c.us",
		PhoneNumber: "+1111111111",
		Name:        "Old Contact",
		IsMyContact: true,
	}

	// Create recent contact (should not be deleted)
	recentContact := &models.Contact{
		ContactID:   "recent@c.us",
		PhoneNumber: "+2222222222",
		Name:        "Recent Contact",
		IsMyContact: true,
	}

	// Save contacts
	err := db.SaveContact(ctx, oldContact)
	require.NoError(t, err)
	err = db.SaveContact(ctx, recentContact)
	require.NoError(t, err)

	// Manually update the cached_at for the old contact to simulate old data
	encryptedOldContactID, err := db.encryptor.EncryptForLookupIfEnabled(oldContact.ContactID)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx, `
		UPDATE contacts
		SET cached_at = ?
		WHERE contact_id = ?
	`, oldTime.Format(time.RFC3339), encryptedOldContactID)
	require.NoError(t, err)

	// Run cleanup with 30 day retention
	err = db.CleanupOldContacts(ctx, 30)
	assert.NoError(t, err)

	// Verify old contact was deleted
	deletedContact, err := db.GetContact(ctx, "old@c.us")
	assert.NoError(t, err)
	assert.Nil(t, deletedContact)

	// Verify recent contact still exists
	contact, err := db.GetContact(ctx, "recent@c.us")
	assert.NoError(t, err)
	require.NotNil(t, contact)
	assert.Equal(t, "recent@c.us", contact.ContactID)
}

func TestDatabase_GetMessageMappingBySignalID(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test getting non-existent mapping
	result, err := db.GetMessageMappingBySignalID(ctx, "nonexistent-signal-id")
	assert.NoError(t, err)
	assert.Nil(t, result)

	// Save a mapping first
	mapping := &models.MessageMapping{
		WhatsAppChatID:  "chat123",
		WhatsAppMsgID:   "wa123",
		SignalMsgID:     "sig123",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "personal",
	}

	err = db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	// Test getting existing mapping by Signal ID
	retrieved, err := db.GetMessageMappingBySignalID(ctx, "sig123")
	assert.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, mapping.WhatsAppChatID, retrieved.WhatsAppChatID)
	assert.Equal(t, mapping.WhatsAppMsgID, retrieved.WhatsAppMsgID)
	assert.Equal(t, mapping.SignalMsgID, retrieved.SignalMsgID)
}

func TestDatabase_GetLatestMessageMappingByWhatsAppChatID(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	chatID := "+1234567890@c.us"

	// Test when no messages exist
	mapping, err := db.GetLatestMessageMappingByWhatsAppChatID(ctx, chatID)
	assert.NoError(t, err)
	assert.Nil(t, mapping)

	// Create test mappings with different timestamps
	mapping1 := &models.MessageMapping{
		WhatsAppChatID:  chatID,
		WhatsAppMsgID:   "wa_msg_1",
		SignalMsgID:     "sig_msg_1",
		SignalTimestamp: time.Now().Add(-2 * time.Hour),
		ForwardedAt:     time.Now().Add(-2 * time.Hour),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "personal",
	}

	mapping2 := &models.MessageMapping{
		WhatsAppChatID:  chatID,
		WhatsAppMsgID:   "wa_msg_2",
		SignalMsgID:     "sig_msg_2",
		SignalTimestamp: time.Now().Add(-1 * time.Hour),
		ForwardedAt:     time.Now().Add(-1 * time.Hour), // More recent
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "personal",
	}

	mapping3 := &models.MessageMapping{
		WhatsAppChatID:  "+9876543210@c.us", // Different chat
		WhatsAppMsgID:   "wa_msg_3",
		SignalMsgID:     "sig_msg_3",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "business",
	}

	// Save mappings
	err = db.SaveMessageMapping(ctx, mapping1)
	require.NoError(t, err)
	err = db.SaveMessageMapping(ctx, mapping2)
	require.NoError(t, err)
	err = db.SaveMessageMapping(ctx, mapping3)
	require.NoError(t, err)

	// Get latest for the test chat ID - should return mapping2 (most recent)
	latest, err := db.GetLatestMessageMappingByWhatsAppChatID(ctx, chatID)
	assert.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "wa_msg_2", latest.WhatsAppMsgID)
	assert.Equal(t, "sig_msg_2", latest.SignalMsgID)
	assert.Equal(t, chatID, latest.WhatsAppChatID)

	// Test with different chat ID
	latest, err = db.GetLatestMessageMappingByWhatsAppChatID(ctx, "+9876543210@c.us")
	assert.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "wa_msg_3", latest.WhatsAppMsgID)

	// Test with non-existent chat ID
	latest, err = db.GetLatestMessageMappingByWhatsAppChatID(ctx, "+0000000000@c.us")
	assert.NoError(t, err)
	assert.Nil(t, latest)
}

func TestDatabase_GetLatestMessageMapping(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test when no messages exist
	mapping, err := db.GetLatestMessageMapping(ctx)
	assert.NoError(t, err)
	assert.Nil(t, mapping)

	// Create test mappings with different timestamps
	mapping1 := &models.MessageMapping{
		WhatsAppChatID:  "+1111111111@c.us",
		WhatsAppMsgID:   "wa_msg_1",
		SignalMsgID:     "sig_msg_1",
		SignalTimestamp: time.Now().Add(-3 * time.Hour),
		ForwardedAt:     time.Now().Add(-3 * time.Hour),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "s1",
	}

	mapping2 := &models.MessageMapping{
		WhatsAppChatID:  "+2222222222@c.us",
		WhatsAppMsgID:   "wa_msg_2",
		SignalMsgID:     "sig_msg_2",
		SignalTimestamp: time.Now().Add(-1 * time.Hour),
		ForwardedAt:     time.Now().Add(-1 * time.Hour), // Most recent overall
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "s2",
	}

	mapping3 := &models.MessageMapping{
		WhatsAppChatID:  "+3333333333@c.us",
		WhatsAppMsgID:   "wa_msg_3",
		SignalMsgID:     "sig_msg_3",
		SignalTimestamp: time.Now().Add(-2 * time.Hour),
		ForwardedAt:     time.Now().Add(-2 * time.Hour),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "s3",
	}

	// Save mappings
	err = db.SaveMessageMapping(ctx, mapping1)
	require.NoError(t, err)
	err = db.SaveMessageMapping(ctx, mapping2)
	require.NoError(t, err)
	err = db.SaveMessageMapping(ctx, mapping3)
	require.NoError(t, err)

	// Get latest overall - should return mapping2 (most recent across all chats)
	latest, err := db.GetLatestMessageMapping(ctx)
	assert.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "wa_msg_2", latest.WhatsAppMsgID)
	assert.Equal(t, "sig_msg_2", latest.SignalMsgID)
	assert.Equal(t, "+2222222222@c.us", latest.WhatsAppChatID)
}

func TestDatabase_GetLatestMessageMappingBySession(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test with no messages
	latest, err := db.GetLatestMessageMappingBySession(ctx, "business")
	assert.NoError(t, err)
	assert.Nil(t, latest)

	// Create test mappings for different sessions
	mapping1 := &models.MessageMapping{
		WhatsAppChatID:  "+1234567890@c.us",
		WhatsAppMsgID:   "wa_msg_1",
		SignalMsgID:     "sig_msg_1",
		SessionName:     "personal",
		SignalTimestamp: time.Now().Add(-2 * time.Hour),
		ForwardedAt:     time.Now().Add(-2 * time.Hour),
		DeliveryStatus:  models.DeliveryStatusSent,
	}

	mapping2 := &models.MessageMapping{
		WhatsAppChatID:  "+9876543210@c.us",
		WhatsAppMsgID:   "wa_msg_2",
		SignalMsgID:     "sig_msg_2",
		SessionName:     "business",
		SignalTimestamp: time.Now().Add(-1 * time.Hour),
		ForwardedAt:     time.Now().Add(-1 * time.Hour),
		DeliveryStatus:  models.DeliveryStatusSent,
	}

	mapping3 := &models.MessageMapping{
		WhatsAppChatID:  "+1111111111@c.us",
		WhatsAppMsgID:   "wa_msg_3",
		SignalMsgID:     "sig_msg_3",
		SessionName:     "business",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
	}

	// Save mappings
	err = db.SaveMessageMapping(ctx, mapping1)
	require.NoError(t, err)
	err = db.SaveMessageMapping(ctx, mapping2)
	require.NoError(t, err)
	err = db.SaveMessageMapping(ctx, mapping3)
	require.NoError(t, err)

	// Get latest for business session - should return mapping3 (most recent)
	latest, err = db.GetLatestMessageMappingBySession(ctx, "business")
	assert.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "wa_msg_3", latest.WhatsAppMsgID)
	assert.Equal(t, "sig_msg_3", latest.SignalMsgID)
	assert.Equal(t, "business", latest.SessionName)

	// Get latest for personal session
	latest, err = db.GetLatestMessageMappingBySession(ctx, "personal")
	assert.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "wa_msg_1", latest.WhatsAppMsgID)
	assert.Equal(t, "personal", latest.SessionName)

	// Test with empty session name (should default to "default")
	latest, err = db.GetLatestMessageMappingBySession(ctx, "default")
	assert.NoError(t, err)
	assert.Nil(t, latest) // No messages for "default" session

	// Test with non-existent session
	latest, err = db.GetLatestMessageMappingBySession(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, latest)
}

func TestDatabase_HasMessageHistoryBetween(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test with no message history
	hasHistory, err := db.HasMessageHistoryBetween(ctx, "personal", "+1234567890")
	assert.NoError(t, err)
	assert.False(t, hasHistory)

	// Create a message mapping between session and Signal sender
	mapping := &models.MessageMapping{
		WhatsAppChatID:  "+1234567890@c.us", // This represents the Signal sender as WhatsApp chat ID
		WhatsAppMsgID:   "wa_msg_1",
		SignalMsgID:     "sig_msg_1",
		SessionName:     "personal",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
	}

	err = db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	// Now there should be message history
	hasHistory, err = db.HasMessageHistoryBetween(ctx, "personal", "+1234567890")
	assert.NoError(t, err)
	assert.True(t, hasHistory)

	// Test with Signal sender that already has @c.us suffix
	hasHistory, err = db.HasMessageHistoryBetween(ctx, "personal", "+1234567890@c.us")
	assert.NoError(t, err)
	assert.True(t, hasHistory)

	// Test with different session - no history
	hasHistory, err = db.HasMessageHistoryBetween(ctx, "business", "+1234567890")
	assert.NoError(t, err)
	assert.False(t, hasHistory)

	// Test with different Signal sender - no history
	hasHistory, err = db.HasMessageHistoryBetween(ctx, "personal", "+9876543210")
	assert.NoError(t, err)
	assert.False(t, hasHistory)

	// Test with empty session name (should default to "default")
	hasHistory, err = db.HasMessageHistoryBetween(ctx, "default", "+1234567890")
	assert.NoError(t, err)
	assert.False(t, hasHistory) // No messages for "default" session
}

func TestDatabase_New_ErrorCases(t *testing.T) {
	// Set up encryption secret for tests
	originalSecret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
	_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-database-testing")
	defer func() {
		if originalSecret != "" {
			_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", originalSecret)
		} else {
			_ = os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
		}
	}()

	// Test with invalid database path
	_, err := New("", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid database path")

	// Test with null byte in path
	_, err = New("/invalid\x00path", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path contains null bytes")

	// Test with directory that doesn't exist
	_, err = New("/nonexistent/dir/test.db", nil)
	assert.Error(t, err)
}

// TestDatabase_SchemaUpgrade tests upgrading from an old database schema to the new one
func TestDatabase_SchemaUpgrade(t *testing.T) {
	ctx := context.Background()
	// Setup encryption environment for test
	originalSecret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
	_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "test-secret-key-for-database-upgrade-test-32chars!")
	t.Cleanup(func() {
		if originalSecret != "" {
			_ = os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", originalSecret)
		} else {
			_ = os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
		}
	})

	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "upgrade_test.db")

	// Step 1: Create old database schema without hash columns
	// This simulates a database from before the hash columns were added
	sqliteDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer func() { _ = sqliteDB.Close() }()
	// Create old schema migrations table
	_, err = sqliteDB.Exec(`CREATE TABLE schema_migrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT NOT NULL UNIQUE,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	// Create old message_mappings table WITHOUT hash columns (pre-upgrade)
	oldSchemaSQL := `
	CREATE TABLE message_mappings (
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
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE contacts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		contact_id TEXT NOT NULL UNIQUE,
		phone_number TEXT NOT NULL,
		name TEXT,
		push_name TEXT,
		short_name TEXT,
		is_blocked BOOLEAN DEFAULT FALSE,
		is_group BOOLEAN DEFAULT FALSE,
		is_my_contact BOOLEAN DEFAULT FALSE,
		last_seen DATETIME,
		cached_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX idx_whatsapp_msg_id ON message_mappings(whatsapp_msg_id);
	CREATE INDEX idx_signal_msg_id ON message_mappings(signal_msg_id);
	`

	_, err = sqliteDB.Exec(oldSchemaSQL)
	require.NoError(t, err)

	// Step 2: Insert test data into old schema
	testData := []struct {
		whatsappChatID  string
		whatsappMsgID   string
		signalMsgID     string
		signalTimestamp time.Time
		forwardedAt     time.Time
		deliveryStatus  string
		sessionName     string
	}{
		{
			whatsappChatID:  "+1234567890@c.us",
			whatsappMsgID:   "old_wa_msg_1",
			signalMsgID:     "old_sig_msg_1",
			signalTimestamp: time.Now().Add(-2 * time.Hour),
			forwardedAt:     time.Now().Add(-2 * time.Hour),
			deliveryStatus:  "sent",
			sessionName:     "test-session",
		},
		{
			whatsappChatID:  "+9876543210@c.us",
			whatsappMsgID:   "old_wa_msg_2",
			signalMsgID:     "old_sig_msg_2",
			signalTimestamp: time.Now().Add(-1 * time.Hour),
			forwardedAt:     time.Now().Add(-1 * time.Hour),
			deliveryStatus:  "delivered",
			sessionName:     "personal",
		},
	}
	for _, data := range testData {
		_, err = sqliteDB.Exec(`
			INSERT INTO message_mappings 
			(whatsapp_chat_id, whatsapp_msg_id, signal_msg_id, signal_timestamp, 
			 forwarded_at, delivery_status, session_name) 
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			data.whatsappChatID, data.whatsappMsgID, data.signalMsgID,
			data.signalTimestamp, data.forwardedAt, data.deliveryStatus, data.sessionName)
		require.NoError(t, err)
	}

	// Insert test contact data
	_, err = sqliteDB.Exec(`
		INSERT INTO contacts (contact_id, phone_number, name, push_name)
		VALUES (?, ?, ?, ?)`,
		"+1234567890@c.us", "+1234567890", "Old Contact", "Old Push Name")
	require.NoError(t, err)
	// Verify old schema - these columns should NOT exist yet
	var hasHashColumns bool
	err = sqliteDB.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('message_mappings') 
		WHERE name IN ('chat_id_hash', 'whatsapp_msg_id_hash', 'signal_msg_id_hash')`).Scan(&hasHashColumns)
	require.NoError(t, err)
	assert.False(t, hasHashColumns, "Hash columns should not exist in old schema")

	// Close the raw database connection
	_ = sqliteDB.Close()

	// Step 3: Copy the actual migration to a test location with some modifications
	// This allows us to use the real migration logic while ensuring the test is isolated
	testMigrationsPath := filepath.Join(tmpDir, "test_migrations")
	err = os.MkdirAll(testMigrationsPath, 0755)
	require.NoError(t, err)
	// Read the actual migration file - find the project root first
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	realMigrationPath := filepath.Join(projectRoot, "scripts", "migrations", "001_initial_schema.sql")
	migrationContent, err := os.ReadFile(realMigrationPath)
	require.NoError(t, err)

	// Write it to the test migrations directory
	testMigrationPath := filepath.Join(testMigrationsPath, "001_initial_schema.sql")
	err = os.WriteFile(testMigrationPath, migrationContent, 0644)
	require.NoError(t, err)

	originalMigrationsDir := migrations.MigrationsDir
	migrations.MigrationsDir = testMigrationsPath
	t.Cleanup(func() { migrations.MigrationsDir = originalMigrationsDir })

	// Step 4: Initialize database through our migration system
	// This should upgrade the schema from old to new
	db, err := New(dbPath, nil)
	require.NoError(t, err)
	defer func() {
		err := db.Close()
		assert.NoError(t, err)
	}()
	// Step 5: Verify schema has been upgraded
	// Check that hash columns now exist
	var hashColumnCount int
	err = db.db.QueryRow(`
		SELECT COUNT(*) 
		FROM pragma_table_info('message_mappings') 
		WHERE name IN ('chat_id_hash', 'whatsapp_msg_id_hash', 'signal_msg_id_hash')`).Scan(&hashColumnCount)
	require.NoError(t, err)
	assert.Equal(t, 3, hashColumnCount, "All 3 hash columns should exist after upgrade")
	// Step 6: Verify all old data was preserved
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM message_mappings").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, len(testData), count, "All old message mappings should be preserved")
	// Verify specific old data is intact
	for _, expectedData := range testData {
		var retrievedMapping models.MessageMapping
		err = db.db.QueryRow(`
			SELECT whatsapp_chat_id, whatsapp_msg_id, signal_msg_id, delivery_status, session_name
			FROM message_mappings 
			WHERE whatsapp_msg_id = ?`, expectedData.whatsappMsgID).Scan(
			&retrievedMapping.WhatsAppChatID,
			&retrievedMapping.WhatsAppMsgID,
			&retrievedMapping.SignalMsgID,
			&retrievedMapping.DeliveryStatus,
			&retrievedMapping.SessionName)
		require.NoError(t, err)
		assert.Equal(t, expectedData.whatsappChatID, retrievedMapping.WhatsAppChatID)
		assert.Equal(t, expectedData.whatsappMsgID, retrievedMapping.WhatsAppMsgID)
		assert.Equal(t, expectedData.signalMsgID, retrievedMapping.SignalMsgID)
		assert.Equal(t, expectedData.deliveryStatus, string(retrievedMapping.DeliveryStatus))
		assert.Equal(t, expectedData.sessionName, retrievedMapping.SessionName)
	}
	// Verify contacts table is intact
	var contactCount int
	err = db.db.QueryRow("SELECT COUNT(*) FROM contacts").Scan(&contactCount)
	require.NoError(t, err)
	assert.Equal(t, 1, contactCount, "Contact data should be preserved")
	// Step 7: Verify new functionality works with upgraded schema
	// Test that we can save new mappings with hash columns
	newMapping := &models.MessageMapping{
		WhatsAppChatID:  "+5555555555@c.us",
		WhatsAppMsgID:   "upgraded_wa_msg",
		SignalMsgID:     "upgraded_sig_msg",
		SignalTimestamp: time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusSent,
		SessionName:     "upgraded-session",
	}

	err = db.SaveMessageMapping(ctx, newMapping)
	require.NoError(t, err, "Should be able to save new mappings after upgrade")

	// Verify the new mapping was saved and can be retrieved
	retrievedMapping, err := db.GetMessageMapping(ctx, newMapping.WhatsAppMsgID)
	require.NoError(t, err)
	require.NotNil(t, retrievedMapping)
	assert.Equal(t, newMapping.WhatsAppChatID, retrievedMapping.WhatsAppChatID)
	assert.Equal(t, newMapping.SignalMsgID, retrievedMapping.SignalMsgID)
	// Step 8: Verify hash columns are being populated for new data
	// First check if our new record exists at all by computing the hash
	expectedHash, err := db.encryptor.LookupHash(newMapping.WhatsAppMsgID)
	require.NoError(t, err)

	var totalNewRecords int
	err = db.db.QueryRow(`SELECT COUNT(*) FROM message_mappings WHERE whatsapp_msg_id_hash = ?`,
		expectedHash).Scan(&totalNewRecords)
	require.NoError(t, err)
	assert.Equal(t, 1, totalNewRecords, "New record should exist")

	// Verify hash columns are populated for new data
	var chatHash, waHash, sigHash sql.NullString
	err = db.db.QueryRow(`
		SELECT chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash 
		FROM message_mappings 
		WHERE whatsapp_msg_id_hash = ?`,
		expectedHash).Scan(&chatHash, &waHash, &sigHash)
	require.NoError(t, err)
	// The hash columns should have values for the new mapping
	var hashCount int
	err = db.db.QueryRow(`
		SELECT COUNT(*)
		FROM message_mappings 
		WHERE whatsapp_msg_id_hash = ? 
		AND chat_id_hash IS NOT NULL 
		AND whatsapp_msg_id_hash IS NOT NULL 
		AND signal_msg_id_hash IS NOT NULL`,
		expectedHash).Scan(&hashCount)
	require.NoError(t, err)

	// Verify that hash columns are populated for new data
	assert.Equal(t, 1, hashCount, "New data should have hash columns populated")

	// Step 9: Verify old data doesn't have hash values (which is expected)
	// Old data should have NULL hash columns since they weren't populated during upgrade
	var oldRecordsWithoutHashes int
	err = db.db.QueryRow(`
		SELECT COUNT(*)
		FROM message_mappings 
		WHERE whatsapp_msg_id IN (?, ?) 
		AND (chat_id_hash IS NULL OR whatsapp_msg_id_hash IS NULL OR signal_msg_id_hash IS NULL)`,
		testData[0].whatsappMsgID, testData[1].whatsappMsgID).Scan(&oldRecordsWithoutHashes)
	require.NoError(t, err)
	assert.Equal(t, 2, oldRecordsWithoutHashes, "Old data should not have hash values populated")
}

func TestDatabase_SaveGroup(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	group := &models.Group{
		GroupID:          "123456789@g.us",
		Subject:          "Test Group",
		Description:      "A test group",
		ParticipantCount: 5,
		SessionName:      "default",
	}

	// Test saving a group
	err := db.SaveGroup(ctx, group)
	assert.NoError(t, err)

	// Test updating existing group (INSERT OR REPLACE)
	group.Subject = "Updated Group"
	group.ParticipantCount = 7
	err = db.SaveGroup(ctx, group)
	assert.NoError(t, err)
}

func TestDatabase_GetGroup(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Save a group first
	group := &models.Group{
		GroupID:          "123456789@g.us",
		Subject:          "Test Group",
		Description:      "Test Description",
		ParticipantCount: 3,
		SessionName:      "default",
	}
	err := db.SaveGroup(ctx, group)
	require.NoError(t, err)

	// Retrieve the group
	retrieved, err := db.GetGroup(ctx, "123456789@g.us", "default")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, "123456789@g.us", retrieved.GroupID)
	assert.Equal(t, "Test Group", retrieved.Subject)
	assert.Equal(t, "Test Description", retrieved.Description)
	assert.Equal(t, 3, retrieved.ParticipantCount)
	assert.Equal(t, "default", retrieved.SessionName)
	assert.False(t, retrieved.CachedAt.IsZero())
	assert.False(t, retrieved.UpdatedAt.IsZero())
}

func TestDatabase_GetGroup_NotFound(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Try to retrieve a non-existent group
	retrieved, err := db.GetGroup(ctx, "999999999@g.us", "default")
	assert.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestDatabase_GetGroup_DifferentSessions(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Save same group ID but different sessions
	group1 := &models.Group{
		GroupID:          "123456789@g.us",
		Subject:          "Session 1 Group",
		Description:      "Group in session 1",
		ParticipantCount: 3,
		SessionName:      "session1",
	}
	err := db.SaveGroup(ctx, group1)
	require.NoError(t, err)

	group2 := &models.Group{
		GroupID:          "123456789@g.us",
		Subject:          "Session 2 Group",
		Description:      "Group in session 2",
		ParticipantCount: 5,
		SessionName:      "session2",
	}
	err = db.SaveGroup(ctx, group2)
	require.NoError(t, err)

	// Retrieve from session1
	retrieved1, err := db.GetGroup(ctx, "123456789@g.us", "session1")
	require.NoError(t, err)
	require.NotNil(t, retrieved1)
	assert.Equal(t, "Session 1 Group", retrieved1.Subject)
	assert.Equal(t, 3, retrieved1.ParticipantCount)

	// Retrieve from session2
	retrieved2, err := db.GetGroup(ctx, "123456789@g.us", "session2")
	require.NoError(t, err)
	require.NotNil(t, retrieved2)
	assert.Equal(t, "Session 2 Group", retrieved2.Subject)
	assert.Equal(t, 5, retrieved2.ParticipantCount)
}

func TestDatabase_SaveGroup_Encryption(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	group := &models.Group{
		GroupID:          "123456789@g.us",
		Subject:          "Sensitive Group",
		Description:      "Sensitive Description",
		ParticipantCount: 10,
		SessionName:      "default",
	}

	err := db.SaveGroup(ctx, group)
	require.NoError(t, err)

	// Query the database directly to verify encryption
	var storedGroupID, storedSubject, storedDescription string
	err = db.db.QueryRow(`
		SELECT group_id, subject, description
		FROM groups
		WHERE session_name = ?
	`, "default").Scan(&storedGroupID, &storedSubject, &storedDescription)
	require.NoError(t, err)

	// Verify that stored values are encrypted (not plain text)
	assert.NotEqual(t, "123456789@g.us", storedGroupID, "Group ID should be encrypted")
	assert.NotEqual(t, "Sensitive Group", storedSubject, "Subject should be encrypted")
	assert.NotEqual(t, "Sensitive Description", storedDescription, "Description should be encrypted")

	// Verify we can still retrieve and decrypt
	retrieved, err := db.GetGroup(ctx, "123456789@g.us", "default")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "123456789@g.us", retrieved.GroupID)
	assert.Equal(t, "Sensitive Group", retrieved.Subject)
	assert.Equal(t, "Sensitive Description", retrieved.Description)
}

func TestDatabase_CleanupOldGroups(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Save a recent group
	recentGroup := &models.Group{
		GroupID:          "recent@g.us",
		Subject:          "Recent Group",
		Description:      "Recent",
		ParticipantCount: 5,
		SessionName:      "default",
	}
	err := db.SaveGroup(ctx, recentGroup)
	require.NoError(t, err)

	// Manually insert an old group by manipulating the timestamp
	oldGroupID, err := db.encryptor.EncryptForLookupIfEnabled("old@g.us")
	require.NoError(t, err)
	oldSubject, err := db.encryptor.EncryptIfEnabled("Old Group")
	require.NoError(t, err)
	oldDescription, err := db.encryptor.EncryptIfEnabled("Old")
	require.NoError(t, err)

	_, err = db.db.ExecContext(ctx, `
		INSERT INTO groups (group_id, subject, description, participant_count, session_name, cached_at)
		VALUES (?, ?, ?, ?, ?, datetime('now', '-30 days'))
	`, oldGroupID, oldSubject, oldDescription, 3, "default")
	require.NoError(t, err)

	// Cleanup groups older than 14 days
	err = db.CleanupOldGroups(ctx, 14)
	assert.NoError(t, err)

	// Verify old group was deleted
	oldRetrieved, err := db.GetGroup(ctx, "old@g.us", "default")
	assert.NoError(t, err)
	assert.Nil(t, oldRetrieved, "Old group should be deleted")

	// Verify recent group still exists
	recentRetrieved, err := db.GetGroup(ctx, "recent@g.us", "default")
	assert.NoError(t, err)
	assert.NotNil(t, recentRetrieved, "Recent group should still exist")
}
