-- Initial schema for WhatsSignal
-- Version: 3 (includes all required columns)
-- Created: 2024-05-21
-- Updated: 2025-09-03 (consolidated all migrations into initial schema)

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
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Add hash columns if they don't exist (for existing databases)
-- We need to handle the case where table exists but columns don't
-- This approach recreates the table if it exists without the hash columns

-- Create temporary table with all columns
CREATE TABLE IF NOT EXISTS message_mappings_temp (
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

-- Copy data from old table if it exists and has different schema
INSERT OR IGNORE INTO message_mappings_temp 
    (id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id, signal_timestamp, 
     forwarded_at, delivery_status, media_path, session_name, media_type, created_at, updated_at)
SELECT id, whatsapp_chat_id, whatsapp_msg_id, signal_msg_id, signal_timestamp,
       forwarded_at, delivery_status, media_path, 
       COALESCE(session_name, 'default') as session_name,
       media_type, created_at, updated_at
FROM message_mappings
WHERE EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name='message_mappings');

-- Drop old table and rename new one
DROP TABLE IF EXISTS message_mappings;
ALTER TABLE message_mappings_temp RENAME TO message_mappings;

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

-- Contacts table for caching WhatsApp contact information
CREATE TABLE IF NOT EXISTS contacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    contact_id TEXT NOT NULL UNIQUE,  -- WhatsApp ID like "1234567890@c.us"
    phone_number TEXT NOT NULL,       -- Just the phone number "1234567890"
    name TEXT,                        -- Contact book name (highest priority)
    push_name TEXT,                   -- User's display name (fallback)
    short_name TEXT,                  -- Shortened name
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
END;