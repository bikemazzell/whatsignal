package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"whatsignal/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*Database, string, func()) {
	// Create a temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "whatsignal-db-test")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := New(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, tmpDir, cleanup
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
	_, err := db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, datetime('now', '-2 days'), datetime('now', '-2 days'))`,
		"chat123", "msg123", "sig123",
		oldTime, oldTime, models.DeliveryStatusDelivered,
	)
	require.NoError(t, err)

	_, err = db.db.ExecContext(ctx, `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"chat124", "msg124", "sig124",
		newTime, newTime, models.DeliveryStatusDelivered,
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
