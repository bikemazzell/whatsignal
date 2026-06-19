CREATE INDEX IF NOT EXISTS idx_message_mappings_session_forwarded
ON message_mappings(session_name, forwarded_at DESC);

CREATE INDEX IF NOT EXISTS idx_message_mappings_created_at
ON message_mappings(created_at);
