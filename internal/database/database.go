package database

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	"github.com/whatsignal/internal/models"
)

type Database struct {
	db *sql.DB
}

func New(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) SaveMessageMapping(mapping *models.MessageMapping) error {
	query := `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status, media_path
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := d.db.Exec(query,
		mapping.WhatsAppChatID,
		mapping.WhatsAppMsgID,
		mapping.SignalMsgID,
		mapping.SignalTimestamp,
		mapping.ForwardedAt,
		mapping.DeliveryStatus,
		mapping.MediaPath,
	)

	return err
}

func (d *Database) GetMessageMappingByWhatsAppID(msgID string) (*models.MessageMapping, error) {
	query := `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			   signal_timestamp, forwarded_at, delivery_status, media_path,
			   created_at, updated_at
		FROM message_mappings
		WHERE whatsapp_msg_id = ?
	`

	mapping := &models.MessageMapping{}
	err := d.db.QueryRow(query, msgID).Scan(
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
	return mapping, err
}

func (d *Database) UpdateDeliveryStatus(msgID string, status models.DeliveryStatus) error {
	query := `
		UPDATE message_mappings
		SET delivery_status = ?
		WHERE whatsapp_msg_id = ? OR signal_msg_id = ?
	`

	_, err := d.db.Exec(query, status, msgID, msgID)
	return err
}

func (d *Database) CleanupOldRecords(retentionDays int) error {
	query := `
		DELETE FROM message_mappings
		WHERE created_at < datetime('now', '-? days')
	`

	_, err := d.db.Exec(query, retentionDays)
	return err
}
