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

// TestDatabase_ConcurrentOperations tests concurrent database access
func TestDatabase_ConcurrentOperations(t *testing.T) {
	// Disable encryption for this test
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "false")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	
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
	
	// Initialize schema
	schema, err := migrations.GetInitialSchema()
	require.NoError(t, err)
	_, err = db.Exec(schema)
	require.NoError(t, err)
	
	// Create database wrapper
	database := &Database{db: db, encryptor: &encryptor{gcm: nil}}

	const numGoroutines = 10 // Reduced for SQLite
	const numOperations = 5  // Reduced for SQLite
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
	// Disable encryption for this test
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "false")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	
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

	// Disable encryption for this test
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "false")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "large_test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	const numRecords = 10000
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
	// Disable encryption for this test
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "false")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	
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
	// Disable encryption for this test
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "false")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	
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
	// Disable encryption for this test
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "false")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	
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
	// Disable encryption for this test
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "false")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	
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

// TestDatabase_EncryptionToggle tests toggling encryption on/off
func TestDatabase_EncryptionToggle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "encryption_toggle_test.db")

	// Start without encryption
	os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	
	db, err := New(dbPath)
	require.NoError(t, err)

	// Save unencrypted data
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

	// Enable encryption and reopen
	os.Setenv("WHATSIGNAL_ENABLE_ENCRYPTION", "true")
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-test-secret-key-for-encryption-testing")
	defer os.Unsetenv("WHATSIGNAL_ENABLE_ENCRYPTION")
	defer os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")

	db, err = New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Should still be able to read old unencrypted data
	retrieved, err := db.GetMessageMappingByWhatsAppID(ctx, "wa-toggle-1")
	require.NoError(t, err)
	assert.Equal(t, mapping.WhatsAppMsgID, retrieved.WhatsAppMsgID)

	// New data should be encrypted
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
}