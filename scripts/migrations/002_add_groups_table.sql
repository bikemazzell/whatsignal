-- Add groups table for caching WhatsApp group metadata
-- Version: 1.0
-- Created: 2025-10-24

CREATE TABLE IF NOT EXISTS groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    description TEXT,
    participant_count INTEGER,
    session_name TEXT NOT NULL,
    cached_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(group_id, session_name)
);

CREATE INDEX IF NOT EXISTS idx_groups_group_id ON groups(group_id);
CREATE INDEX IF NOT EXISTS idx_groups_session_name ON groups(session_name);
CREATE INDEX IF NOT EXISTS idx_groups_cached_at ON groups(cached_at);
CREATE INDEX IF NOT EXISTS idx_groups_session_group ON groups(session_name, group_id);

CREATE TRIGGER IF NOT EXISTS groups_updated_at
AFTER UPDATE ON groups
BEGIN
    UPDATE groups SET updated_at = CURRENT_TIMESTAMP
    WHERE id = NEW.id;
END;
