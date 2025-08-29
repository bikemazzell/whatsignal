-- Add session_name column for multi-channel support
-- Version: 2
-- Created: 2025-06-25

-- Add session_name column to message_mappings table
ALTER TABLE message_mappings ADD COLUMN session_name TEXT NOT NULL;

-- Add media_type column while we're at it (from the model)
ALTER TABLE message_mappings ADD COLUMN media_type TEXT;

-- Create index for session_name to improve query performance
CREATE INDEX IF NOT EXISTS idx_session_name ON message_mappings(session_name);

-- Create composite index for session-based queries
CREATE INDEX IF NOT EXISTS idx_session_chat ON message_mappings(session_name, whatsapp_chat_id);