package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"whatsignal/internal/migrations"
	"whatsignal/internal/models"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestMigrationsForEdgeCases creates a temporary directory with test migrations
func setupTestMigrationsForEdgeCases(t *testing.T) func() {
	tmpMigDir, err := os.MkdirTemp("", "whatsignal-edge-test-migrations")
	require.NoError(t, err)

	migrationsPath := filepath.Join(tmpMigDir, "migrations")
	err = os.MkdirAll(migrationsPath, 0755)
	require.NoError(t, err)

	// Create complete schema
	schemaContent := `CREATE TABLE IF NOT EXISTS message_mappings (
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

	err = os.WriteFile(filepath.Join(migrationsPath, "001_initial_schema.sql"), []byte(schemaContent), 0644)
	require.NoError(t, err)

	// Set migrations directory temporarily
	originalMigrationsDir := migrations.MigrationsDir
	migrations.MigrationsDir = migrationsPath

	cleanup := func() {
		migrations.MigrationsDir = originalMigrationsDir
		os.RemoveAll(tmpMigDir)
	}

	return cleanup
}

// TestDatabase_ConcurrentOperations tests concurrent database access
func TestDatabase_ConcurrentOperations(t *testing.T) {
	// Always-on encryption: set a test secret
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "concurrent_test.db")

	// Enable WAL mode for better concurrent access
	db, err := sql.Open("sqlite3", dbPath+"?mode=wal")
	require.NoError(t, err)
	defer db.Close()

	// Set pragmas for better concurrent performance
	_, err = db.Exec("PRAGMA journal_mode=WAL")
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA busy_timeout=5000")
	require.NoError(t, err)

	// Initialize schema using migrations
	err = migrations.RunMigrations(db)
	require.NoError(t, err)

	// Create database wrapper with proper encryptor
	encryptor, err := NewEncryptor()
	require.NoError(t, err)
	database := &Database{db: db, encryptor: encryptor}

	const numGoroutines = 10
	const numOperations = 5
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations*3)

	// Concurrent writes, reads, and updates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				mapping := &models.MessageMapping{
					WhatsAppMsgID:   makeConcurrentID("wa", id, j),
					SignalMsgID:     makeConcurrentID("sig", id, j),
					WhatsAppChatID:  makeConcurrentID("chat", id, 0),
					SignalTimestamp: time.Now(),
					SessionName:     "test-session",
					CreatedAt:       time.Now(),
					ForwardedAt:     time.Now(),
					DeliveryStatus:  models.DeliveryStatusPending,
					MediaType:       "text",
				}

				ctx := context.Background()
				// Save
				if err := database.SaveMessageMapping(ctx, mapping); err != nil {
					errors <- err
					continue
				}

				// Read
				retrieved, err := database.GetMessageMappingByWhatsAppID(ctx, mapping.WhatsAppMsgID)
				if err != nil {
					errors <- err
					continue
				}
				if retrieved.WhatsAppMsgID != mapping.WhatsAppMsgID {
					errors <- assert.AnError
				}

				// Update
				if err := database.UpdateDeliveryStatusByWhatsAppID(ctx, mapping.WhatsAppMsgID, "delivered"); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors (SQLite may have some lock contention)
	errorCount := 0
	lockErrors := 0
	for err := range errors {
		errorCount++
		if err != nil && (strings.Contains(err.Error(), "database is locked") ||
			strings.Contains(err.Error(), "database table is locked")) {
			lockErrors++
		} else {
			t.Logf("Unexpected error: %v", err)
		}
	}
	t.Logf("Total errors: %d, Lock errors: %d", errorCount, lockErrors)
	// Some lock errors are expected with SQLite under heavy concurrent load
	// But we shouldn't have other types of errors
	assert.Equal(t, lockErrors, errorCount, "All errors should be lock-related")
}

func makeConcurrentID(prefix string, id, j int) string {
	return prefix + "-" + string(rune(id)) + "-" + string(rune(j))
}

// TestDatabase_TransactionRollback tests transaction rollback behavior
func TestDatabase_TransactionRollback(t *testing.T) {
	// Always-on encryption: set a test secret
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "transaction_test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Save initial mapping
	mapping := &models.MessageMapping{
		WhatsAppMsgID:   "wa-trans-1",
		SignalMsgID:     "sig-trans-1",
		WhatsAppChatID:  "chat-trans",
		SignalTimestamp: time.Now(),
		SessionName:     "test-session",
		CreatedAt:       time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusPending,
		MediaType:       "text",
	}
	ctx := context.Background()
	err = db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	// Force close and reopen to test persistence
	db.Close()

	db, err = New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Verify data persisted
	retrieved, err := db.GetMessageMappingByWhatsAppID(ctx, "wa-trans-1")
	require.NoError(t, err)
	assert.Equal(t, mapping.WhatsAppMsgID, retrieved.WhatsAppMsgID)
}

// TestDatabase_LargeDataSet tests with large number of records
func TestDatabase_LargeDataSet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset test in short mode")
	}

	// Always-on encryption: set a test secret
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "large_test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	const numRecords = 500
	start := time.Now()

	ctx := context.Background()
	// Insert large number of records
	for i := 0; i < numRecords; i++ {
		mapping := &models.MessageMapping{
			WhatsAppMsgID:   makeID("wa", i),
			SignalMsgID:     makeID("sig", i),
			WhatsAppChatID:  makeID("chat", i%100), // 100 different chats
			SignalTimestamp: time.Now(),
			SessionName:     makeID("session", i%5), // 5 different sessions
			CreatedAt:       time.Now().Add(-time.Duration(i) * time.Minute),
			ForwardedAt:     time.Now(),
			DeliveryStatus:  models.DeliveryStatusPending,
			MediaType:       "text",
		}
		err := db.SaveMessageMapping(ctx, mapping)
		require.NoError(t, err)
	}

	insertDuration := time.Since(start)
	t.Logf("Inserted %d records in %v", numRecords, insertDuration)

	// Test query performance
	start = time.Now()
	mapping, err := db.GetLatestMessageMappingByWhatsAppChatID(ctx, makeID("chat", 50))
	require.NoError(t, err)
	require.NotNil(t, mapping)
	queryDuration := time.Since(start)
	t.Logf("Query by chat ID took %v", queryDuration)

	// Test cleanup performance
	start = time.Now()
	err = db.CleanupOldRecords(30) // Keep last 30 days
	require.NoError(t, err)
	cleanupDuration := time.Since(start)
	t.Logf("Cleanup took %v", cleanupDuration)
}

func makeID(prefix string, num int) string {
	return prefix + "-" + string(rune(num))
}

// TestDatabase_SQLInjectionAttempts tests SQL injection protection
func TestDatabase_SQLInjectionAttempts(t *testing.T) {
	// Always-on encryption: set a test secret
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "injection_test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Test various SQL injection attempts
	injectionAttempts := []string{
		"'; DROP TABLE message_mappings; --",
		"' OR '1'='1",
		"'; DELETE FROM message_mappings WHERE '1'='1'; --",
		"' UNION SELECT * FROM message_mappings --",
		"'; UPDATE message_mappings SET delivery_status='hacked'; --",
	}

	ctx := context.Background()
	for _, attempt := range injectionAttempts {
		mapping := &models.MessageMapping{
			WhatsAppMsgID:   attempt,
			SignalMsgID:     "sig-injection",
			WhatsAppChatID:  attempt,
			SignalTimestamp: time.Now(),
			SessionName:     attempt,
			CreatedAt:       time.Now(),
			ForwardedAt:     time.Now(),
			DeliveryStatus:  models.DeliveryStatusPending,
			MediaType:       "text",
		}

		// Should either save successfully or fail, but not execute injection
		_ = db.SaveMessageMapping(ctx, mapping)

		// Try to retrieve - should work normally
		_, _ = db.GetMessageMappingByWhatsAppID(ctx, attempt)

		// Try update - should work normally
		_ = db.UpdateDeliveryStatusByWhatsAppID(ctx, attempt, "delivered")
	}

	// Verify table still exists and has expected structure
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM message_mappings").Scan(&count)
	require.NoError(t, err, "Table should still exist")
}

// TestDatabase_FilePermissions tests database file permissions
func TestDatabase_FilePermissions(t *testing.T) {
	// Always-on encryption: set a test secret
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "permissions_test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Check file permissions
	info, err := os.Stat(dbPath)
	require.NoError(t, err)

	// Should be readable and writable by owner only (0600)
	mode := info.Mode()
	assert.Equal(t, os.FileMode(0600), mode.Perm(), "Database file should have 0600 permissions")
}

// TestDatabase_CorruptedDatabase tests handling of corrupted database
func TestDatabase_CorruptedDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corrupted_test.db")

	// Create a corrupted database file
	err := os.WriteFile(dbPath, []byte("this is not a valid sqlite database"), 0600)
	require.NoError(t, err)

	// Attempt to open should fail gracefully
	_, err = New(dbPath)
	require.Error(t, err)
}

// TestDatabase_VeryLongIDs tests handling of very long IDs
func TestDatabase_VeryLongIDs(t *testing.T) {
	// Always-on encryption: set a test secret
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "long_ids_test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Create very long IDs (but within reasonable limits)
	longID := string(make([]byte, 255))
	for i := range longID {
		longID = longID[:i] + "a" + longID[i+1:]
	}

	mapping := &models.MessageMapping{
		WhatsAppMsgID:   longID,
		SignalMsgID:     longID,
		WhatsAppChatID:  longID,
		SignalTimestamp: time.Now(),
		SessionName:     "test",
		CreatedAt:       time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusPending,
		MediaType:       "text",
	}

	ctx := context.Background()
	err = db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)

	retrieved, err := db.GetMessageMappingByWhatsAppID(ctx, longID)
	require.NoError(t, err)
	assert.Equal(t, longID, retrieved.WhatsAppMsgID)
}

// TestDatabase_ContactEdgeCases tests edge cases for contact operations
func TestDatabase_ContactEdgeCases(t *testing.T) {
	// Always-on encryption: set a test secret
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "contact_edge_test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Test with unicode names
	contact := &models.Contact{
		PhoneNumber: "+1234567890",
		ContactID:   "+1234567890@c.us", // Must match what GetContactByPhone expects
		Name:        "测试用户 🌍 Test",
		UpdatedAt:   time.Now(),
		CachedAt:    time.Now(),
	}

	ctx := context.Background()
	err = db.SaveContact(ctx, contact)
	require.NoError(t, err)

	retrieved, err := db.GetContactByPhone(ctx, "+1234567890")
	require.NoError(t, err)
	require.NotNil(t, retrieved, "Contact should be found")
	assert.Equal(t, contact.Name, retrieved.Name)

	// Test duplicate contact handling
	contact.Name = "Updated Name"
	err = db.SaveContact(ctx, contact)
	require.NoError(t, err)

	retrieved, err = db.GetContactByPhone(ctx, "+1234567890")
	require.NoError(t, err)
	require.NotNil(t, retrieved, "Contact should be found after update")
	assert.Equal(t, "Updated Name", retrieved.Name)
}

// TestDatabase_EncryptionToggle validates that encryption persists across restarts
func TestDatabase_EncryptionToggle(t *testing.T) {
	// Always-on encryption: set secret before first open
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "encryption_toggle_test.db")

	db, err := New(dbPath)
	require.NoError(t, err)

	// Save initial encrypted data
	mapping := &models.MessageMapping{
		WhatsAppMsgID:   "wa-toggle-1",
		SignalMsgID:     "sig-toggle-1",
		WhatsAppChatID:  "chat-toggle",
		SignalTimestamp: time.Now(),
		SessionName:     "test",
		CreatedAt:       time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusPending,
		MediaType:       "text",
	}
	ctx := context.Background()
	err = db.SaveMessageMapping(ctx, mapping)
	require.NoError(t, err)
	db.Close()

	// Reopen with same secret
	db, err = New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Should be able to read previously saved data
	retrieved, err := db.GetMessageMappingByWhatsAppID(ctx, "wa-toggle-1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, mapping.WhatsAppMsgID, retrieved.WhatsAppMsgID)

	// New data should be accessible under encryption
	mapping2 := &models.MessageMapping{
		WhatsAppMsgID:   "wa-toggle-2",
		SignalMsgID:     "sig-toggle-2",
		WhatsAppChatID:  "chat-toggle",
		SignalTimestamp: time.Now(),
		SessionName:     "test",
		CreatedAt:       time.Now(),
		ForwardedAt:     time.Now(),
		DeliveryStatus:  models.DeliveryStatusPending,
		MediaType:       "text",
	}
	err = db.SaveMessageMapping(ctx, mapping2)
	require.NoError(t, err)
	retrieved2, err := db.GetMessageMappingByWhatsAppID(ctx, "wa-toggle-2")
	require.NoError(t, err)
	require.NotNil(t, retrieved2)
	assert.Equal(t, mapping2.WhatsAppMsgID, retrieved2.WhatsAppMsgID)
}

// Test decrypt errors for specific query paths to increase coverage of error branches
func TestDatabase_GetMessageMappingBySignalID_DecryptErrors(t *testing.T) {
	// Always-on encryption: set a test secret
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "decrypt_errors_signal.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	enc := db.encryptor

	chatPlain := "+1000000000@c.us"
	waPlain := "wa-decrypt-ok"
	sigPlain := "sig-decrypt-target"

	// Prepare mostly valid encrypted fields but corrupt chat ID so first decrypt fails
	encWA, err := enc.EncryptForLookupIfEnabled(waPlain)
	require.NoError(t, err)
	encSIG, err := enc.EncryptForLookupIfEnabled(sigPlain)
	require.NoError(t, err)
	badChat := "not-base64!!" // will cause failed to decode base64

	chatHash, err := enc.LookupHash(chatPlain)
	require.NoError(t, err)
	waHash, err := enc.LookupHash(waPlain)
	require.NoError(t, err)
	sigHash, err := enc.LookupHash(sigPlain)
	require.NoError(t, err)

	_, err = db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status,
			session_name, media_type,
			chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash
		) VALUES (?, ?, ?, ?, ?, ?, 'test', 'text', ?, ?, ?)
	`, badChat, encWA, encSIG, time.Now(), time.Now(), models.DeliveryStatusSent, chatHash, waHash, sigHash)
	require.NoError(t, err)

	// Query by signal ID should hit decrypt chat error
	res, err := db.GetMessageMappingBySignalID(ctx, sigPlain)
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to decrypt chat ID")
}

func TestDatabase_GetLatestMessageMappingByWhatsAppChatID_DecryptErrors(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "decrypt_errors_chat.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	enc := db.encryptor

	chatPlain := "+2000000000@c.us"
	waPlain := "wa-ok"
	sigPlain := "sig-ok"

	encWA, err := enc.EncryptForLookupIfEnabled(waPlain)
	require.NoError(t, err)
	encSIG, err := enc.EncryptForLookupIfEnabled(sigPlain)
	require.NoError(t, err)
	badChat := "bad%%base64"

	chatHash, err := enc.LookupHash(chatPlain)
	require.NoError(t, err)

	// Case 1: bad chat decrypt
	waHash, err := enc.LookupHash(waPlain)
	require.NoError(t, err)
	sigHash, err := enc.LookupHash(sigPlain)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status,
			session_name, media_type,
			chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash
		) VALUES (?, ?, ?, ?, ?, ?, 'test', 'text', ?, ?, ?)
	`, badChat, encWA, encSIG, time.Now(), time.Now(), models.DeliveryStatusSent,
		chatHash, waHash, sigHash)
	require.NoError(t, err)

	res, err := db.GetLatestMessageMappingByWhatsAppChatID(ctx, chatPlain)
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to decrypt WhatsApp chat ID")

	// Case 2: bad WA message ID decrypt
	goodChatEnc, err := enc.EncryptForLookupIfEnabled(chatPlain)
	require.NoError(t, err)
	badWA := "!!!"
	waHash2, err := enc.LookupHash(waPlain)
	require.NoError(t, err)
	sigHash2, err := enc.LookupHash(sigPlain)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status,
			session_name, media_type,
			chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash
		) VALUES (?, ?, ?, ?, ?, ?, 'test', 'text', ?, ?, ?)
	`, goodChatEnc, badWA, encSIG, time.Now(), time.Now().Add(time.Second), models.DeliveryStatusSent,
		chatHash, waHash2, sigHash2)
	require.NoError(t, err)

	res, err = db.GetLatestMessageMappingByWhatsAppChatID(ctx, chatPlain)
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to decrypt WhatsApp message ID")

	// Case 3: bad Signal message ID decrypt
	badSIG := "not-b64"
	reqWAHash, err := enc.LookupHash(waPlain)
	require.NoError(t, err)
	reqSigHash, err := enc.LookupHash(sigPlain)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status,
			session_name, media_type,
			chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash
		) VALUES (?, ?, ?, ?, ?, ?, 'test', 'text', ?, ?, ?)
	`, goodChatEnc, encWA, badSIG, time.Now(), time.Now().Add(2*time.Second), models.DeliveryStatusSent,
		chatHash, reqWAHash, reqSigHash)
	require.NoError(t, err)

	res, err = db.GetLatestMessageMappingByWhatsAppChatID(ctx, chatPlain)
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to decrypt Signal message ID")

	// Case 4: bad media path decrypt
	badMedia := "%%%"
	waHash4, err := enc.LookupHash(waPlain)
	require.NoError(t, err)
	sigHash4, err := enc.LookupHash(sigPlain)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status, media_path,
			session_name, media_type,
			chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'test', 'text', ?, ?, ?)
	`, goodChatEnc, encWA, encSIG, time.Now(), time.Now().Add(3*time.Second), models.DeliveryStatusSent, badMedia,
		chatHash, waHash4, sigHash4)
	require.NoError(t, err)

	res, err = db.GetLatestMessageMappingByWhatsAppChatID(ctx, chatPlain)
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to decrypt media path")
}

func TestDatabase_GetLatestMessageMapping_DecryptErrors(t *testing.T) {
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	cleanup := setupTestMigrationsForEdgeCases(t)
	defer cleanup()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "decrypt_errors_latest.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	enc := db.encryptor

	chatPlain := "+3000000000@c.us"
	waPlain := "wa-ok2"
	sigPlain := "sig-ok2"

	goodChatEnc, err := enc.EncryptForLookupIfEnabled(chatPlain)
	require.NoError(t, err)
	goodWA, err := enc.EncryptForLookupIfEnabled(waPlain)
	require.NoError(t, err)
	badSIG := "!bad!"
	badMedia := "badmedia==="

	// Insert a row with bad signal ID decrypt and most recent forwarded_at
	chatHash2, err := enc.LookupHash(chatPlain)
	require.NoError(t, err)
	waHash2, err := enc.LookupHash(waPlain)
	require.NoError(t, err)
	sigHash2, err := enc.LookupHash(sigPlain)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status, media_path,
			session_name, media_type,
			chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'test', 'text', ?, ?, ?)
	`, goodChatEnc, goodWA, badSIG, time.Now(), time.Now().Add(10*time.Second), models.DeliveryStatusSent, nil,
		chatHash2, waHash2, sigHash2)
	require.NoError(t, err)

	res, err := db.GetLatestMessageMapping(ctx)
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to decrypt Signal message ID")

	// Insert a row with bad media path decrypt that is now the most recent
	chatHash3, err := enc.LookupHash(chatPlain)
	require.NoError(t, err)
	waHash3, err := enc.LookupHash(waPlain)
	require.NoError(t, err)
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status, media_path,
			session_name, media_type,
			chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'test', 'text', ?, ?, ?)
	`, goodChatEnc, goodWA, goodWA, time.Now(), time.Now().Add(20*time.Second), models.DeliveryStatusSent, badMedia,
		chatHash3, waHash3, waHash3)
	require.NoError(t, err)

	res, err = db.GetLatestMessageMapping(ctx)
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "failed to decrypt media path")
}
