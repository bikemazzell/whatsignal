package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"whatsignal/internal/migrations"
	"whatsignal/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db        *sql.DB
	encryptor *encryptor
}

func New(dbPath string) (*Database, error) {
	if len(dbPath) == 0 || dbPath[0] == '\x00' {
		return nil, fmt.Errorf("invalid database path")
	}

	file, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create database file: %w", err)
	}
	file.Close()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	schema, err := migrations.GetInitialSchema()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	encryptor, err := NewEncryptor()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize encryptor: %w", err)
	}

	return &Database{db: db, encryptor: encryptor}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error {
	encryptedChatID, err := d.encryptor.EncryptIfEnabled(mapping.WhatsAppChatID)
	if err != nil {
		return fmt.Errorf("failed to encrypt chat ID: %w", err)
	}

	encryptedWhatsAppMsgID, err := d.encryptor.EncryptIfEnabled(mapping.WhatsAppMsgID)
	if err != nil {
		return fmt.Errorf("failed to encrypt WhatsApp message ID: %w", err)
	}

	encryptedSignalMsgID, err := d.encryptor.EncryptIfEnabled(mapping.SignalMsgID)
	if err != nil {
		return fmt.Errorf("failed to encrypt Signal message ID: %w", err)
	}

	var encryptedMediaPath *string
	if mapping.MediaPath != nil {
		encrypted, err := d.encryptor.EncryptIfEnabled(*mapping.MediaPath)
		if err != nil {
			return fmt.Errorf("failed to encrypt media path: %w", err)
		}
		encryptedMediaPath = &encrypted
	}

	query := `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status, media_path
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = d.db.ExecContext(ctx, query,
		encryptedChatID,
		encryptedWhatsAppMsgID,
		encryptedSignalMsgID,
		mapping.SignalTimestamp,
		mapping.ForwardedAt,
		mapping.DeliveryStatus,
		encryptedMediaPath,
	)

	if err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (d *Database) GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error) {
	encryptedID, err := d.encryptor.EncryptIfEnabled(id)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt search ID: %w", err)
	}

	query := `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			   signal_timestamp, forwarded_at, delivery_status, media_path,
			   created_at, updated_at
		FROM message_mappings
		WHERE whatsapp_msg_id = ? OR signal_msg_id = ?
	`

	var encryptedChatID, encryptedWhatsAppMsgID, encryptedSignalMsgID string
	var encryptedMediaPath *string
	mapping := &models.MessageMapping{}

	err = d.db.QueryRowContext(ctx, query, encryptedID, encryptedID).Scan(
		&mapping.ID,
		&encryptedChatID,
		&encryptedWhatsAppMsgID,
		&encryptedSignalMsgID,
		&mapping.SignalTimestamp,
		&mapping.ForwardedAt,
		&mapping.DeliveryStatus,
		&encryptedMediaPath,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message mapping: %w", err)
	}

	mapping.WhatsAppChatID, err = d.encryptor.DecryptIfEnabled(encryptedChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt chat ID: %w", err)
	}

	mapping.WhatsAppMsgID, err = d.encryptor.DecryptIfEnabled(encryptedWhatsAppMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt WhatsApp message ID: %w", err)
	}

	mapping.SignalMsgID, err = d.encryptor.DecryptIfEnabled(encryptedSignalMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt Signal message ID: %w", err)
	}

	if encryptedMediaPath != nil {
		decryptedMediaPath, err := d.encryptor.DecryptIfEnabled(*encryptedMediaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt media path: %w", err)
		}
		mapping.MediaPath = &decryptedMediaPath
	}

	return mapping, nil
}

func (d *Database) GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error) {
	encryptedWhatsAppID, err := d.encryptor.EncryptIfEnabled(whatsappID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt WhatsApp ID: %w", err)
	}

	query := `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			   signal_timestamp, forwarded_at, delivery_status, media_path,
			   created_at, updated_at
		FROM message_mappings
		WHERE whatsapp_msg_id = ?
	`

	var encryptedChatID, encryptedWhatsAppMsgID, encryptedSignalMsgID string
	var encryptedMediaPath *string
	mapping := &models.MessageMapping{}

	err = d.db.QueryRowContext(ctx, query, encryptedWhatsAppID).Scan(
		&mapping.ID,
		&encryptedChatID,
		&encryptedWhatsAppMsgID,
		&encryptedSignalMsgID,
		&mapping.SignalTimestamp,
		&mapping.ForwardedAt,
		&mapping.DeliveryStatus,
		&encryptedMediaPath,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message mapping: %w", err)
	}

	mapping.WhatsAppChatID, err = d.encryptor.DecryptIfEnabled(encryptedChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt chat ID: %w", err)
	}

	mapping.WhatsAppMsgID, err = d.encryptor.DecryptIfEnabled(encryptedWhatsAppMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt WhatsApp message ID: %w", err)
	}

	mapping.SignalMsgID, err = d.encryptor.DecryptIfEnabled(encryptedSignalMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt Signal message ID: %w", err)
	}

	if encryptedMediaPath != nil {
		decryptedMediaPath, err := d.encryptor.DecryptIfEnabled(*encryptedMediaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt media path: %w", err)
		}
		mapping.MediaPath = &decryptedMediaPath
	}

	return mapping, nil
}

func (d *Database) UpdateDeliveryStatus(ctx context.Context, id string, status string) error {
	encryptedID, err := d.encryptor.EncryptIfEnabled(id)
	if err != nil {
		return fmt.Errorf("failed to encrypt message ID: %w", err)
	}

	query := `
		UPDATE message_mappings
		SET delivery_status = ?
		WHERE whatsapp_msg_id = ? OR signal_msg_id = ?
	`

	result, err := d.db.ExecContext(ctx, query, status, encryptedID, encryptedID)
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
