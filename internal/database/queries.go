package database

// Message mapping queries
const (
	InsertMessageMappingQuery = `
		INSERT INTO message_mappings (
			whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			signal_timestamp, forwarded_at, delivery_status, media_path,
			session_name, media_type,
			chat_id_hash, whatsapp_msg_id_hash, signal_msg_id_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	SelectMessageMappingByWhatsAppIDQuery = `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			   signal_timestamp, forwarded_at, delivery_status, media_path,
			   session_name, media_type,
			   created_at, updated_at
		FROM message_mappings
		WHERE whatsapp_msg_id_hash = ?
	`

	SelectMessageMappingBySignalIDQuery = `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			   signal_timestamp, forwarded_at, delivery_status, media_path,
			   session_name, media_type,
			   created_at, updated_at
		FROM message_mappings
		WHERE signal_msg_id_hash = ?
	`

	UpdateDeliveryStatusByWhatsAppIDQuery = `
		UPDATE message_mappings
		SET delivery_status = ?
		WHERE whatsapp_msg_id_hash = ?
	`

	UpdateDeliveryStatusBySignalIDQuery = `
		UPDATE message_mappings
		SET delivery_status = ?
		WHERE signal_msg_id_hash = ?
	`

	SelectLatestMessageMappingByWhatsAppChatIDQuery = `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id, signal_timestamp,
		       forwarded_at, delivery_status, media_path, session_name, media_type,
		       created_at, updated_at
		FROM message_mappings
		WHERE chat_id_hash = ?
		ORDER BY forwarded_at DESC
		LIMIT 1
	`

	SelectLatestMessageMappingQuery = `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id, signal_timestamp,
		       forwarded_at, delivery_status, media_path, session_name, media_type,
		       created_at, updated_at
		FROM message_mappings
		ORDER BY forwarded_at DESC
		LIMIT 1
	`

	SelectLatestMessageMappingBySessionQuery = `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id, signal_timestamp,
		       forwarded_at, delivery_status, media_path, session_name, media_type,
		       created_at, updated_at
		FROM message_mappings
		WHERE session_name = ?
		ORDER BY forwarded_at DESC
		LIMIT 1
	`

	// Select a recent window of message mappings for a session (for post-filtering in code)
	SelectRecentMessageMappingsBySessionQuery = `
			SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id, signal_timestamp,
			       forwarded_at, delivery_status, media_path, session_name, media_type,
			       created_at, updated_at
			FROM message_mappings
			WHERE session_name = ?
			ORDER BY forwarded_at DESC
			LIMIT ?
		`

	DeleteOldMessageMappingsQuery = `
		DELETE FROM message_mappings
		WHERE created_at < datetime('now', '-' || ? || ' days')
	`

	CountStaleMessagesQuery = `
		SELECT COUNT(*)
		FROM message_mappings
		WHERE delivery_status = 'sent'
		  AND forwarded_at < datetime('now', '-' || ? || ' seconds')
	`
)

// Contact queries
const (
	InsertOrReplaceContactQuery = `
		INSERT OR REPLACE INTO contacts (
			contact_id, phone_number, name, push_name, short_name,
			is_blocked, is_group, is_my_contact, cached_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	SelectContactByIDQuery = `
		SELECT contact_id, phone_number, name, push_name, short_name,
			   is_blocked, is_group, is_my_contact, cached_at
		FROM contacts
		WHERE contact_id = ?
	`

	DeleteOldContactsQuery = `
		DELETE FROM contacts
		WHERE cached_at < datetime('now', '-' || ? || ' days')
	`
)

// Group queries
const (
	InsertOrReplaceGroupQuery = `
		INSERT OR REPLACE INTO groups (
			group_id, subject, description, participant_count, session_name, cached_at
		) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	SelectGroupByIDQuery = `
		SELECT id, group_id, subject, description, participant_count, session_name,
		       cached_at, updated_at
		FROM groups
		WHERE group_id = ? AND session_name = ?
	`

	DeleteOldGroupsQuery = `
		DELETE FROM groups
		WHERE cached_at < datetime('now', '-' || ? || ' days')
	`
)

// Pending signal message queries
const (
	InsertPendingSignalMessageQuery = `
		INSERT OR IGNORE INTO pending_signal_messages (
			message_id, message_id_hash, sender, message, group_id,
			timestamp, raw_json, destination, retry_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
	`

	SelectPendingSignalMessagesQuery = `
		SELECT id, message_id, sender, message, group_id,
			   timestamp, raw_json, destination, retry_count, created_at
		FROM pending_signal_messages
		ORDER BY created_at ASC
		LIMIT ?
	`

	DeletePendingSignalMessageQuery = `
		DELETE FROM pending_signal_messages
		WHERE message_id_hash = ? AND destination = ?
	`

	IncrementPendingRetryCountQuery = `
		UPDATE pending_signal_messages
		SET retry_count = retry_count + 1
		WHERE message_id_hash = ? AND destination = ?
	`
)
