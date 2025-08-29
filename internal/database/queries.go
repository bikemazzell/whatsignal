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
			   created_at, updated_at
		FROM message_mappings
		WHERE whatsapp_msg_id_hash = ?
	`

	SelectMessageMappingBySignalIDQuery = `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id,
			   signal_timestamp, forwarded_at, delivery_status, media_path,
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
		       forwarded_at, delivery_status, media_path,
		       created_at, updated_at
		FROM message_mappings
		WHERE chat_id_hash = ?
		ORDER BY forwarded_at DESC
		LIMIT 1
	`

	SelectLatestMessageMappingQuery = `
		SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id, signal_timestamp, 
		       forwarded_at, delivery_status, media_path,
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

	DeleteOldMessageMappingsQuery = `
		DELETE FROM message_mappings
		WHERE created_at < datetime('now', '-' || ? || ' days')
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