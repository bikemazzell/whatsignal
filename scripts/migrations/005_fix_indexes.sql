DROP INDEX IF EXISTS idx_whatsapp_msg_id;
DROP INDEX IF EXISTS idx_signal_msg_id;
DROP INDEX IF EXISTS idx_chat_time;
CREATE INDEX IF NOT EXISTS idx_delivery_status_forwarded ON message_mappings(delivery_status, forwarded_at);
CREATE INDEX IF NOT EXISTS idx_chat_hash_time ON message_mappings(chat_id_hash, forwarded_at);
