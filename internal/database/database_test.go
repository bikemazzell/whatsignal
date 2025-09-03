package database

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"whatsignal/internal/migrations"
	"whatsignal/internal/models"

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

	return migrationsPath
}

func setupTestDB(t *testing.T) (*Database, string, func()) {
	// Set up encryption secret for tests
	originalSecret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-database-testing")

	// Create a temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "whatsignal-db-test")
	require.NoError(t, err)

	// Set up test migrations
	migrationsPath := setupTestMigrations(t, tmpDir)

	// Set migrations directory for the test
	originalMigrationsDir := migrations.MigrationsDir
	migrations.MigrationsDir = migrationsPath

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := New(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
		// Restore original environment
		migrations.MigrationsDir = originalMigrationsDir
		if originalSecret != "" {
			os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", originalSecret)
		} else {
			os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
		}
	}

	return db, tmpDir, cleanup
}

func TestNewDatabase(t *testing.T) {
	// Set up encryption secret for tests
	originalSecret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-database-testing")
	defer func() {
		if originalSecret != "" {
			os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", originalSecret)
		} else {
			os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
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
				t.Cleanup(func() { os.RemoveAll(tmpDir) })

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
					os.RemoveAll(tmpDir)
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

			db, err := New(dbPath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, db)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, db)
				if db != nil {
					db.Close()
				}
			}
		})
	}
}

func TestDatabaseEncryptionErrors(t *testing.T) {
	// Test with encryption enabled but invalid secret
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "true")
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "short") // Too short secret
	defer func() {
		os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
		os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
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
	err = db.CleanupOldRecords(7)
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
	err = db.CleanupOldRecords(1)
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
	os.RemoveAll(tmpDir)
}

func TestNewDatabaseErrors(t *testing.T) {
	// Test with invalid path
	db, err := New("\x00invalid")
	assert.Error(t, err, "Expected error with invalid path")
	assert.Nil(t, db)

	// Test with unwritable directory
	tmpDir, err := os.MkdirTemp("", "whatsignal-db-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Make parent directory read-only
	err = os.Chmod(tmpDir, 0444)
	require.NoError(t, err)
	defer func() {
		if err := os.Chmod(tmpDir, 0755); err != nil {
			t.Errorf("Failed to restore directory permissions: %v", err)
		}
	}()

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err = New(dbPath)
	assert.Error(t, err, "Expected error with unwritable directory")
	assert.Nil(t, db)
}

func TestSaveMessageMappingEncryptionErrors(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test with encryption enabled but corrupted encryptor
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "true")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")

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
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "true")
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

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
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a database file with invalid schema
	file, err := os.Create(dbPath)
	require.NoError(t, err)
	if _, err := file.WriteString("invalid sql content"); err != nil {
		t.Errorf("Failed to write to file: %v", err)
	}
	file.Close()

	// This should fail when trying to initialize schema
	db, err := New(dbPath)
	if err != nil {
		// Expected case - schema initialization failed
		assert.Contains(t, err.Error(), "failed to")
		assert.Nil(t, db)
	} else {
		// If it somehow succeeded, clean up
		db.Close()
	}
}

func TestEncryptorEdgeCases(t *testing.T) {
	// Always-on encryption
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

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
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "true")
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

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
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-custom-secret-key-for-testing-purposes")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

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
	err = db.CleanupOldContacts(30)
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
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-database-testing")
	defer func() {
		if originalSecret != "" {
			os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", originalSecret)
		} else {
			os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
		}
	}()

	// Test with invalid database path
	_, err := New("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid database path")

	// Test with null byte in path
	_, err = New("/invalid\x00path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create database file")

	// Test with directory that doesn't exist
	_, err = New("/nonexistent/dir/test.db")
	assert.Error(t, err)
}
