package database

import (
	"context"
	"database/sql"
	"fmt"

	"whatsignal/internal/migrations"
	"whatsignal/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

func New(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize schema
	schema, err := migrations.GetInitialSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error {
	query := `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status, media_path
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.db.ExecContext(ctx, query,
		mapping.WhatsAppChatID,
		mapping.WhatsAppMsgID,
		mapping.SignalMsgID,
		mapping.SignalTimestamp,
		mapping.ForwardedAt,
		mapping.DeliveryStatus,
		mapping.MediaPath,
	)

	if err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (d *Database) GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error) {
	query := `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			   signal_timestamp, forwarded_at, delivery_status, media_path,
			   created_at, updated_at
		FROM message_mappings
		WHERE whatsapp_msg_id = ? OR signal_msg_id = ?
	`

	mapping := &models.MessageMapping{}
	err := d.db.QueryRowContext(ctx, query, id, id).Scan(
		&mapping.ID,
		&mapping.WhatsAppChatID,
		&mapping.WhatsAppMsgID,
		&mapping.SignalMsgID,
		&mapping.SignalTimestamp,
		&mapping.ForwardedAt,
		&mapping.DeliveryStatus,
		&mapping.MediaPath,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message mapping: %w", err)
	}
	return mapping, nil
}

func (d *Database) GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error) {
	query := `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			   signal_timestamp, forwarded_at, delivery_status, media_path,
			   created_at, updated_at
		FROM message_mappings
		WHERE whatsapp_msg_id = ?
	`

	mapping := &models.MessageMapping{}
	err := d.db.QueryRowContext(ctx, query, whatsappID).Scan(
		&mapping.ID,
		&mapping.WhatsAppChatID,
		&mapping.WhatsAppMsgID,
		&mapping.SignalMsgID,
		&mapping.SignalTimestamp,
		&mapping.ForwardedAt,
		&mapping.DeliveryStatus,
		&mapping.MediaPath,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message mapping: %w", err)
	}
	return mapping, nil
}

func (d *Database) UpdateDeliveryStatus(ctx context.Context, id string, status string) error {
	query := `
		UPDATE message_mappings
		SET delivery_status = ?
		WHERE whatsapp_msg_id = ? OR signal_msg_id = ?
	`

	result, err := d.db.ExecContext(ctx, query, status, id, id)
	if err != nil {
		return fmt.Errorf("failed to update delivery status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no message found with ID: %s", id)
	}

	return nil
}

func (d *Database) CleanupOldRecords(retentionDays int) error {
	query := `
		DELETE FROM message_mappings
		WHERE created_at < datetime('now', '-' || ? || ' days')
	`

	_, err := d.db.Exec(query, retentionDays)
	if err != nil {
		return fmt.Errorf("failed to cleanup old records: %w", err)
	}

	return nil
}
