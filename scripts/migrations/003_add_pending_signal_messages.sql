-- Add pending_signal_messages table for crash-safe message processing
-- Sensitive fields are encrypted by the application layer

CREATE TABLE IF NOT EXISTS pending_signal_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id TEXT NOT NULL,
    message_id_hash TEXT NOT NULL,
    sender TEXT NOT NULL,
    message TEXT,
    group_id TEXT,
    timestamp INTEGER NOT NULL,
    raw_json TEXT NOT NULL,
    destination TEXT NOT NULL,
    retry_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(message_id_hash, destination)
);

CREATE INDEX IF NOT EXISTS idx_pending_signal_created_at ON pending_signal_messages(created_at);
CREATE INDEX IF NOT EXISTS idx_pending_signal_message_id_hash ON pending_signal_messages(message_id_hash);
