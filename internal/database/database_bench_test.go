package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"whatsignal/internal/migrations"
	"whatsignal/internal/models"
)

func BenchmarkDatabase_SaveMessageMapping(b *testing.B) {
	db, cleanup := setupInMemoryDB(b)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mapping := &models.MessageMapping{
			WhatsAppMsgID:   generateRandomID(),
			SignalMsgID:     generateRandomID(),
			WhatsAppChatID:  "test-chat",
			SessionName:     "default",
			MediaType:       "text",
			DeliveryStatus:  models.DeliveryStatusPending,
			SignalTimestamp: time.Now(),
			ForwardedAt:     time.Now(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		_ = db.SaveMessageMapping(ctx, mapping)
	}
}

func BenchmarkDatabase_GetMessageMapping(b *testing.B) {
	db, cleanup := setupInMemoryDB(b)
	defer cleanup()

	ctx := context.Background()

	// Pre-populate with test data
	testMappings := make([]*models.MessageMapping, 100)
	for i := 0; i < 100; i++ {
		mapping := &models.MessageMapping{
			WhatsAppMsgID:   generateRandomID(),
			SignalMsgID:     generateRandomID(),
			WhatsAppChatID:  "test-chat",
			SessionName:     "default",
			MediaType:       "text",
			DeliveryStatus:  models.DeliveryStatusPending,
			SignalTimestamp: time.Now(),
			ForwardedAt:     time.Now(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		_ = db.SaveMessageMapping(ctx, mapping)
		testMappings[i] = mapping
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mapping := testMappings[i%len(testMappings)]
		_, _ = db.GetMessageMapping(ctx, mapping.WhatsAppMsgID)
	}
}

func BenchmarkDatabase_GetMessageMappingByWhatsAppID(b *testing.B) {
	db, cleanup := setupInMemoryDB(b)
	defer cleanup()

	ctx := context.Background()

	// Pre-populate with test data
	testMappings := make([]*models.MessageMapping, 100)
	for i := 0; i < 100; i++ {
		mapping := &models.MessageMapping{
			WhatsAppMsgID:   generateRandomID(),
			SignalMsgID:     generateRandomID(),
			WhatsAppChatID:  "test-chat",
			SessionName:     "default",
			MediaType:       "text",
			DeliveryStatus:  models.DeliveryStatusPending,
			SignalTimestamp: time.Now(),
			ForwardedAt:     time.Now(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		_ = db.SaveMessageMapping(ctx, mapping)
		testMappings[i] = mapping
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mapping := testMappings[i%len(testMappings)]
		_, _ = db.GetMessageMappingByWhatsAppID(ctx, mapping.WhatsAppMsgID)
	}
}

func BenchmarkDatabase_GetMessageMappingBySignalID(b *testing.B) {
	db, cleanup := setupInMemoryDB(b)
	defer cleanup()

	ctx := context.Background()

	// Pre-populate with test data
	testMappings := make([]*models.MessageMapping, 100)
	for i := 0; i < 100; i++ {
		mapping := &models.MessageMapping{
			WhatsAppMsgID:   generateRandomID(),
			SignalMsgID:     generateRandomID(),
			WhatsAppChatID:  "test-chat",
			SessionName:     "default",
			MediaType:       "text",
			DeliveryStatus:  models.DeliveryStatusPending,
			SignalTimestamp: time.Now(),
			ForwardedAt:     time.Now(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		_ = db.SaveMessageMapping(ctx, mapping)
		testMappings[i] = mapping
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mapping := testMappings[i%len(testMappings)]
		_, _ = db.GetMessageMappingBySignalID(ctx, mapping.SignalMsgID)
	}
}

func BenchmarkDatabase_UpdateDeliveryStatus(b *testing.B) {
	db, cleanup := setupInMemoryDB(b)
	defer cleanup()

	ctx := context.Background()

	// Pre-populate with test data
	testMappings := make([]*models.MessageMapping, 100)
	for i := 0; i < 100; i++ {
		mapping := &models.MessageMapping{
			WhatsAppMsgID:   generateRandomID(),
			SignalMsgID:     generateRandomID(),
			WhatsAppChatID:  "test-chat",
			SessionName:     "default",
			MediaType:       "text",
			DeliveryStatus:  models.DeliveryStatusPending,
			SignalTimestamp: time.Now(),
			ForwardedAt:     time.Now(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		_ = db.SaveMessageMapping(ctx, mapping)
		testMappings[i] = mapping
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mapping := testMappings[i%len(testMappings)]
		_ = db.UpdateDeliveryStatus(ctx, mapping.WhatsAppMsgID, "delivered")
	}
}

func BenchmarkDatabase_GetLatestMessageMapping(b *testing.B) {
	db, cleanup := setupInMemoryDB(b)
	defer cleanup()

	ctx := context.Background()

	// Pre-populate with test data
	for i := 0; i < 100; i++ {
		mapping := &models.MessageMapping{
			WhatsAppMsgID:   generateRandomID(),
			SignalMsgID:     generateRandomID(),
			WhatsAppChatID:  "test-chat",
			SessionName:     "default",
			MediaType:       "text",
			DeliveryStatus:  models.DeliveryStatusPending,
			SignalTimestamp: time.Now().Add(time.Duration(i) * time.Second),
			ForwardedAt:     time.Now(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		_ = db.SaveMessageMapping(ctx, mapping)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = db.GetLatestMessageMapping(ctx)
	}
}

func BenchmarkDatabase_ConcurrentSave(b *testing.B) {
	db, cleanup := setupInMemoryDB(b)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mapping := &models.MessageMapping{
				WhatsAppMsgID:   generateRandomID(),
				SignalMsgID:     generateRandomID(),
				WhatsAppChatID:  "test-chat",
				SessionName:     "default",
				MediaType:       "text",
				DeliveryStatus:  models.DeliveryStatusPending,
				SignalTimestamp: time.Now(),
				ForwardedAt:     time.Now(),
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}

			_ = db.SaveMessageMapping(ctx, mapping)
		}
	})
}

func BenchmarkDatabase_ConcurrentRead(b *testing.B) {
	db, cleanup := setupInMemoryDB(b)
	defer cleanup()

	ctx := context.Background()

	// Pre-populate with test data
	testMappings := make([]*models.MessageMapping, 1000)
	for i := 0; i < 1000; i++ {
		mapping := &models.MessageMapping{
			WhatsAppMsgID:   generateRandomID(),
			SignalMsgID:     generateRandomID(),
			WhatsAppChatID:  "test-chat",
			SessionName:     "default",
			MediaType:       "text",
			DeliveryStatus:  models.DeliveryStatusPending,
			SignalTimestamp: time.Now(),
			ForwardedAt:     time.Now(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		_ = db.SaveMessageMapping(ctx, mapping)
		testMappings[i] = mapping
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			mapping := testMappings[i%len(testMappings)]
			_, _ = db.GetMessageMapping(ctx, mapping.WhatsAppMsgID)
			i++
		}
	})
}

// Helper function to set up an in-memory database for benchmarking
func setupInMemoryDB(b *testing.B) (*Database, func()) {
	// Set up encryption secret for benchmarks
	originalSecret := os.Getenv("WHATSIGNAL_ENCRYPTION_SECRET")
	os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", "this-is-a-very-long-benchmark-secret-key-for-database-testing")

	// Create a temporary directory for migrations
	tmpDir, err := os.MkdirTemp("", "whatsignal-bench-test")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}

	// Set up test migrations
	migrationsPath := setupBenchmarkMigrations(b, tmpDir)

	// Set migrations directory for the test
	originalMigrationsDir := migrations.MigrationsDir
	migrations.MigrationsDir = migrationsPath

	db, err := New(":memory:", nil)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}

	cleanup := func() {
		if db != nil {
			db.Close()
		}
		// Restore original values
		migrations.MigrationsDir = originalMigrationsDir
		if originalSecret == "" {
			os.Unsetenv("WHATSIGNAL_ENCRYPTION_SECRET")
		} else {
			os.Setenv("WHATSIGNAL_ENCRYPTION_SECRET", originalSecret)
		}
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

// setupBenchmarkMigrations creates test migration files for benchmarking
func setupBenchmarkMigrations(b *testing.B, tmpDir string) string {
	// Create migrations directory
	migrationsPath := filepath.Join(tmpDir, "migrations")
	err := os.MkdirAll(migrationsPath, 0755)
	if err != nil {
		b.Fatalf("Failed to create migrations dir: %v", err)
	}

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
    cached_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_contact_id ON contacts(contact_id);
CREATE INDEX IF NOT EXISTS idx_phone_number ON contacts(phone_number);`

	migrationFile := filepath.Join(migrationsPath, "001_initial_schema.sql")
	err = os.WriteFile(migrationFile, []byte(schemaContent), 0644)
	if err != nil {
		b.Fatalf("Failed to write migration file: %v", err)
	}

	return migrationsPath
}

// Helper function to generate random IDs for benchmarking
func generateRandomID() string {
	return "test-id-" + time.Now().Format("20060102150405.000000")
}
