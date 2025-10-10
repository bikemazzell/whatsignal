package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"whatsignal/internal/constants"
	"whatsignal/internal/migrations"
	"whatsignal/internal/models"
	"whatsignal/internal/security"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db        *sql.DB
	encryptor *encryptor
}

func New(dbPath string, cfg *models.DatabaseConfig) (*Database, error) {
	if len(dbPath) == 0 || dbPath[0] == '\x00' {
		return nil, fmt.Errorf("invalid database path")
	}

	// Validate database path to prevent directory traversal
	if err := security.ValidateFilePath(dbPath); err != nil {
		return nil, fmt.Errorf("invalid database path: %w", err)
	}

	file, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, constants.DefaultFilePermissions) // #nosec G304 - Path validated by security.ValidateFilePath above
	if err != nil {
		return nil, fmt.Errorf("failed to create database file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("failed to close database file: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pooling
	if cfg != nil {
		maxOpen := cfg.MaxOpenConnections
		if maxOpen <= 0 {
			maxOpen = constants.DefaultDBMaxOpenConnections
		}
		db.SetMaxOpenConns(maxOpen)

		maxIdle := cfg.MaxIdleConnections
		if maxIdle <= 0 {
			maxIdle = constants.DefaultDBMaxIdleConnections
		}
		db.SetMaxIdleConns(maxIdle)

		if cfg.ConnMaxLifetimeSec > 0 {
			db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSec) * time.Second)
		} else {
			db.SetConnMaxLifetime(time.Duration(constants.DefaultDBConnMaxLifetimeSec) * time.Second)
		}

		if cfg.ConnMaxIdleTimeSec > 0 {
			db.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTimeSec) * time.Second)
		} else {
			db.SetConnMaxIdleTime(time.Duration(constants.DefaultDBConnMaxIdleTimeSec) * time.Second)
		}
	} else {
		// Use defaults if no config provided
		db.SetMaxOpenConns(constants.DefaultDBMaxOpenConnections)
		db.SetMaxIdleConns(constants.DefaultDBMaxIdleConnections)
		db.SetConnMaxLifetime(time.Duration(constants.DefaultDBConnMaxLifetimeSec) * time.Second)
		db.SetConnMaxIdleTime(time.Duration(constants.DefaultDBConnMaxIdleTimeSec) * time.Second)
	}

	if err := db.Ping(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to ping database: %w (close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run all database migrations
	if err := migrations.RunMigrations(db); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to run migrations: %w (close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	encryptor, err := NewEncryptor()
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to initialize encryptor: %w (close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize encryptor: %w", err)
	}

	return &Database{db: db, encryptor: encryptor}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) SaveMessageMapping(ctx context.Context, mapping *models.MessageMapping) error {
	return retryableDBOperationNoReturn(ctx, func() error {
		return d.saveMessageMappingInternal(ctx, mapping)
	}, "SaveMessageMapping")
}

func (d *Database) saveMessageMappingInternal(ctx context.Context, mapping *models.MessageMapping) error {
	// Encrypt fields with randomized AEAD for storage
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

	// Compute lookup hashes for efficient, safe queries
	chatIDHash, err := d.encryptor.LookupHash(mapping.WhatsAppChatID)
	if err != nil {
		return fmt.Errorf("failed to compute chat ID hash: %w", err)
	}
	waMsgHash, err := d.encryptor.LookupHash(mapping.WhatsAppMsgID)
	if err != nil {
		return fmt.Errorf("failed to compute WhatsApp message ID hash: %w", err)
	}
	sigMsgHash, err := d.encryptor.LookupHash(mapping.SignalMsgID)
	if err != nil {
		return fmt.Errorf("failed to compute Signal message ID hash: %w", err)
	}

	var encryptedMediaPath *string
	if mapping.MediaPath != nil {
		encrypted, err := d.encryptor.EncryptIfEnabled(*mapping.MediaPath)
		if err != nil {
			return fmt.Errorf("failed to encrypt media path: %w", err)
		}
		encryptedMediaPath = &encrypted
	}

	sessionName := mapping.SessionName
	if sessionName == "" {
		return fmt.Errorf("session name is required in message mapping")
	}

	query := InsertMessageMappingQuery

	_, err = d.db.ExecContext(ctx, query,
		encryptedChatID,
		encryptedWhatsAppMsgID,
		encryptedSignalMsgID,
		mapping.SignalTimestamp,
		mapping.ForwardedAt,
		mapping.DeliveryStatus,
		encryptedMediaPath,
		sessionName,
		mapping.MediaType,
		chatIDHash,
		waMsgHash,
		sigMsgHash,
	)

	if err != nil {
		return fmt.Errorf("failed to save message mapping: %w", err)
	}

	return nil
}

func (d *Database) GetMessageMappingByWhatsAppID(ctx context.Context, whatsappID string) (*models.MessageMapping, error) {
	waHash, err := d.encryptor.LookupHash(whatsappID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute WhatsApp ID hash: %w", err)
	}

	runQuery := func(param string) (*models.MessageMapping, error) {
		query := SelectMessageMappingByWhatsAppIDQuery
		var encryptedChatID, encryptedWhatsAppMsgID, encryptedSignalMsgID string
		var encryptedMediaPath *string
		mapping := &models.MessageMapping{}

		err := d.db.QueryRowContext(ctx, query, param).Scan(
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
		if err != nil {
			return nil, err
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

	// Lookup by hash first
	mapping, qErr := runQuery(waHash)
	if qErr == nil {
		return mapping, nil
	}
	if qErr != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get message mapping: %w", qErr)
	}
	// Legacy fallback removed; mappings must be retrievable via hash
	return nil, nil
}

// GetMessageMapping retrieves a message mapping by either WhatsApp or Signal message ID
func (d *Database) GetMessageMapping(ctx context.Context, id string) (*models.MessageMapping, error) {
	// First try as WhatsApp ID
	mapping, err := d.GetMessageMappingByWhatsAppID(ctx, id)
	if err != nil {
		return nil, err
	}
	if mapping != nil {
		return mapping, nil
	}

	// If not found, try as Signal ID
	return d.GetMessageMappingBySignalID(ctx, id)
}

// GetMessageMappingBySignalID retrieves a message mapping by Signal message ID
func (d *Database) GetMessageMappingBySignalID(ctx context.Context, signalID string) (*models.MessageMapping, error) {
	sigHash, err := d.encryptor.LookupHash(signalID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute Signal ID hash: %w", err)
	}

	query := SelectMessageMappingBySignalIDQuery

	var encryptedChatID, encryptedWhatsAppMsgID, encryptedSignalMsgID string
	var encryptedMediaPath *string
	mapping := &models.MessageMapping{}

	err = d.db.QueryRowContext(ctx, query, sigHash).Scan(
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

	mapping.WhatsAppChatID, err = d.encryptor.DecryptAuto(encryptedChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt chat ID: %w", err)
	}

	mapping.WhatsAppMsgID, err = d.encryptor.DecryptAuto(encryptedWhatsAppMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt WhatsApp message ID: %w", err)
	}

	mapping.SignalMsgID, err = d.encryptor.DecryptAuto(encryptedSignalMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt Signal message ID: %w", err)
	}

	if encryptedMediaPath != nil {
		decryptedMediaPath, err := d.encryptor.DecryptAuto(*encryptedMediaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt media path: %w", err)
		}
		mapping.MediaPath = &decryptedMediaPath
	}

	return mapping, nil
}

func (d *Database) UpdateDeliveryStatusByWhatsAppID(ctx context.Context, whatsappID string, status string) error {
	hash, err := d.encryptor.LookupHash(whatsappID)
	if err != nil {
		return fmt.Errorf("failed to compute WhatsApp ID hash: %w", err)
	}

	query := UpdateDeliveryStatusByWhatsAppIDQuery

	result, err := d.db.ExecContext(ctx, query, status, hash)
	if err != nil {
		return fmt.Errorf("failed to update delivery status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no message found with WhatsApp ID: %s", whatsappID)
	}

	return nil
}

func (d *Database) UpdateDeliveryStatusBySignalID(ctx context.Context, signalID string, status string) error {
	hash, err := d.encryptor.LookupHash(signalID)
	if err != nil {
		return fmt.Errorf("failed to compute Signal ID hash: %w", err)
	}

	query := UpdateDeliveryStatusBySignalIDQuery

	result, err := d.db.ExecContext(ctx, query, status, hash)
	if err != nil {
		return fmt.Errorf("failed to update delivery status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no message found with Signal ID: %s", signalID)
	}

	return nil
}

func (d *Database) UpdateDeliveryStatus(ctx context.Context, id string, status string) error {
	return retryableDBOperationNoReturn(ctx, func() error {
		return d.updateDeliveryStatusInternal(ctx, id, status)
	}, "UpdateDeliveryStatus")
}

func (d *Database) updateDeliveryStatusInternal(ctx context.Context, id string, status string) error {
	// Try WhatsApp ID first
	err := d.UpdateDeliveryStatusByWhatsAppID(ctx, id, status)
	if err == nil {
		return nil
	}

	// If not found, try Signal ID
	err = d.UpdateDeliveryStatusBySignalID(ctx, id, status)
	if err == nil {
		return nil
	}

	return fmt.Errorf("no message found with ID: %s", id)
}

func (d *Database) GetLatestMessageMappingByWhatsAppChatID(ctx context.Context, whatsappChatID string) (*models.MessageMapping, error) {
	// Encrypt the chat ID for database query (deterministic for lookup)
	chatHash, err := d.encryptor.LookupHash(whatsappChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute WhatsApp chat ID hash: %w", err)
	}

	query := SelectLatestMessageMappingByWhatsAppChatIDQuery

	row := d.db.QueryRowContext(ctx, query, chatHash)

	var encryptedWAChatID, encryptedWAMsgID, encryptedSignalMsgID string
	var encryptedMediaPath *string
	mapping := &models.MessageMapping{}

	err = row.Scan(
		&mapping.ID,
		&encryptedWAChatID,
		&encryptedWAMsgID,
		&encryptedSignalMsgID,
		&mapping.SignalTimestamp,
		&mapping.ForwardedAt,
		&mapping.DeliveryStatus,
		&encryptedMediaPath,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No mapping found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest message mapping by WhatsApp chat ID: %w", err)
	}

	// Decrypt fields
	mapping.WhatsAppChatID, err = d.encryptor.DecryptIfEnabled(encryptedWAChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt WhatsApp chat ID: %w", err)
	}

	mapping.WhatsAppMsgID, err = d.encryptor.DecryptIfEnabled(encryptedWAMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt WhatsApp message ID: %w", err)
	}

	mapping.SignalMsgID, err = d.encryptor.DecryptIfEnabled(encryptedSignalMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt Signal message ID: %w", err)
	}

	if encryptedMediaPath != nil {
		decryptedPath, err := d.encryptor.DecryptIfEnabled(*encryptedMediaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt media path: %w", err)
		}
		mapping.MediaPath = &decryptedPath
	}

	return mapping, nil
}

func (d *Database) GetLatestMessageMapping(ctx context.Context) (*models.MessageMapping, error) {
	query := SelectLatestMessageMappingQuery

	row := d.db.QueryRowContext(ctx, query)

	var encryptedWAChatID, encryptedWAMsgID, encryptedSignalMsgID string
	var encryptedMediaPath *string
	mapping := &models.MessageMapping{}

	err := row.Scan(
		&mapping.ID,
		&encryptedWAChatID,
		&encryptedWAMsgID,
		&encryptedSignalMsgID,
		&mapping.SignalTimestamp,
		&mapping.ForwardedAt,
		&mapping.DeliveryStatus,
		&encryptedMediaPath,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No mapping found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest message mapping: %w", err)
	}

	// Decrypt fields
	mapping.WhatsAppChatID, err = d.encryptor.DecryptIfEnabled(encryptedWAChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt WhatsApp chat ID: %w", err)
	}

	mapping.WhatsAppMsgID, err = d.encryptor.DecryptIfEnabled(encryptedWAMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt WhatsApp message ID: %w", err)
	}

	mapping.SignalMsgID, err = d.encryptor.DecryptIfEnabled(encryptedSignalMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt Signal message ID: %w", err)
	}

	if encryptedMediaPath != nil {
		decryptedPath, err := d.encryptor.DecryptIfEnabled(*encryptedMediaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt media path: %w", err)
		}
		mapping.MediaPath = &decryptedPath
	}

	return mapping, nil
}

func (d *Database) GetLatestMessageMappingBySession(ctx context.Context, sessionName string) (*models.MessageMapping, error) {
	if sessionName == "" {
		return nil, fmt.Errorf("session name is required")
	}

	query := SelectLatestMessageMappingBySessionQuery

	row := d.db.QueryRowContext(ctx, query, sessionName)

	var encryptedWAChatID, encryptedWAMsgID, encryptedSignalMsgID string
	var encryptedMediaPath *string
	mapping := &models.MessageMapping{}

	err := row.Scan(
		&mapping.ID,
		&encryptedWAChatID,
		&encryptedWAMsgID,
		&encryptedSignalMsgID,
		&mapping.SignalTimestamp,
		&mapping.ForwardedAt,
		&mapping.DeliveryStatus,
		&encryptedMediaPath,
		&mapping.SessionName,
		&mapping.MediaType,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No mapping found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest message mapping by session: %w", err)
	}

	// Decrypt fields
	mapping.WhatsAppChatID, err = d.encryptor.DecryptIfEnabled(encryptedWAChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt WhatsApp chat ID: %w", err)
	}

	mapping.WhatsAppMsgID, err = d.encryptor.DecryptIfEnabled(encryptedWAMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt WhatsApp message ID: %w", err)
	}

	mapping.SignalMsgID, err = d.encryptor.DecryptIfEnabled(encryptedSignalMsgID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt Signal message ID: %w", err)
	}

	if encryptedMediaPath != nil {
		decryptedPath, err := d.encryptor.DecryptIfEnabled(*encryptedMediaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt media path: %w", err)
		}
		mapping.MediaPath = &decryptedPath
	}

	return mapping, nil
}

func (d *Database) CleanupOldRecords(ctx context.Context, retentionDays int) error {
	query := DeleteOldMessageMappingsQuery

	_, err := d.db.ExecContext(ctx, query, retentionDays)
	if err != nil {
		return fmt.Errorf("failed to cleanup old records: %w", err)
	}

	return nil
}

// Contact operations

// SaveContact saves or updates a contact in the database
func (d *Database) SaveContact(ctx context.Context, contact *models.Contact) error {
	// Deterministic encryption for IDs used in lookups
	encryptedContactID, err := d.encryptor.EncryptForLookupIfEnabled(contact.ContactID)
	if err != nil {
		return fmt.Errorf("failed to encrypt contact ID: %w", err)
	}

	encryptedPhone, err := d.encryptor.EncryptIfEnabled(contact.PhoneNumber)
	if err != nil {
		return fmt.Errorf("failed to encrypt phone number: %w", err)
	}

	encryptedName, err := d.encryptor.EncryptIfEnabled(contact.Name)
	if err != nil {
		return fmt.Errorf("failed to encrypt name: %w", err)
	}

	encryptedPushName, err := d.encryptor.EncryptIfEnabled(contact.PushName)
	if err != nil {
		return fmt.Errorf("failed to encrypt push name: %w", err)
	}

	query := InsertOrReplaceContactQuery

	_, err = d.db.ExecContext(ctx, query,
		encryptedContactID, encryptedPhone, encryptedName, encryptedPushName, contact.ShortName,
		contact.IsBlocked, contact.IsGroup, contact.IsMyContact)
	if err != nil {
		return fmt.Errorf("failed to save contact: %w", err)
	}

	return nil
}

// GetContact retrieves a contact by contact ID
func (d *Database) GetContact(ctx context.Context, contactID string) (*models.Contact, error) {
	encryptedContactID, err := d.encryptor.EncryptForLookupIfEnabled(contactID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt contact ID: %w", err)
	}

	query := SelectContactByIDQuery

	row := d.db.QueryRowContext(ctx, query, encryptedContactID)

	var contact models.Contact
	var encryptedPhone, encryptedName, encryptedPushName string

	err = row.Scan(&contact.ContactID, &encryptedPhone, &encryptedName, &encryptedPushName,
		&contact.ShortName, &contact.IsBlocked, &contact.IsGroup, &contact.IsMyContact, &contact.CachedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Contact not found
		}
		return nil, fmt.Errorf("failed to scan contact: %w", err)
	}

	// Decrypt fields
	contact.ContactID, err = d.encryptor.DecryptIfEnabled(contact.ContactID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt contact ID: %w", err)
	}

	contact.PhoneNumber, err = d.encryptor.DecryptIfEnabled(encryptedPhone)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt phone number: %w", err)
	}

	contact.Name, err = d.encryptor.DecryptIfEnabled(encryptedName)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt name: %w", err)
	}

	contact.PushName, err = d.encryptor.DecryptIfEnabled(encryptedPushName)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt push name: %w", err)
	}

	return &contact, nil
}

// GetContactByPhone retrieves a contact by phone number
func (d *Database) GetContactByPhone(ctx context.Context, phoneNumber string) (*models.Contact, error) {
	// Add @c.us suffix if not present
	contactID := phoneNumber
	if !strings.HasSuffix(contactID, "@c.us") {
		contactID = phoneNumber + "@c.us"
	}

	return d.GetContact(ctx, contactID)
}

// CleanupOldContacts removes contacts older than the specified days
func (d *Database) CleanupOldContacts(ctx context.Context, retentionDays int) error {
	query := DeleteOldContactsQuery

	_, err := d.db.ExecContext(ctx, query, retentionDays)
	if err != nil {
		return fmt.Errorf("failed to cleanup old contacts: %w", err)
	}

	return nil
}

// HasMessageHistoryBetween checks if there's any message history between a session and Signal sender
func (d *Database) HasMessageHistoryBetween(ctx context.Context, sessionName, signalSender string) (bool, error) {
	if sessionName == "" {
		return false, fmt.Errorf("session name is required")
	}

	// We need to check both directions:
	// 1. Messages from WhatsApp (this session) that were forwarded to Signal (signal_msg_id exists)
	// 2. Messages from Signal (this sender) that were forwarded to WhatsApp (this session)

	// For Signal messages, the sender is stored in the whatsapp_chat_id field (as phone@c.us)
	// We need to convert the Signal sender to WhatsApp chat ID format
	whatsappChatID := signalSender
	if !strings.HasSuffix(whatsappChatID, "@c.us") {
		whatsappChatID = signalSender + "@c.us"
	}

	chatHash, err := d.encryptor.LookupHash(whatsappChatID)
	if err != nil {
		return false, fmt.Errorf("failed to compute chat ID hash: %w", err)
	}

	query := `
		SELECT COUNT(*) FROM message_mappings
		WHERE session_name = ? AND chat_id_hash = ?
		LIMIT 1
	`

	var count int
	err = d.db.QueryRowContext(ctx, query, sessionName, chatHash).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check message history: %w", err)
	}

	return count > 0, nil
}

// HealthCheck performs a database health check by pinging the database connection
func (d *Database) HealthCheck(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Use PingContext to check if the database is reachable
	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Optional: run a simple query to ensure read access
	var result int
	if err := d.db.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
		return fmt.Errorf("database query failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("unexpected database query result: %d", result)
	}

	return nil
}
