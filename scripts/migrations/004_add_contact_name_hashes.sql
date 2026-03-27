-- Add hash columns for encrypted name lookups in contacts table
ALTER TABLE contacts ADD COLUMN name_hash TEXT;
ALTER TABLE contacts ADD COLUMN push_name_hash TEXT;
ALTER TABLE contacts ADD COLUMN short_name_hash TEXT;

CREATE INDEX IF NOT EXISTS idx_contact_name_hash ON contacts(name_hash);
CREATE INDEX IF NOT EXISTS idx_contact_push_name_hash ON contacts(push_name_hash);
CREATE INDEX IF NOT EXISTS idx_contact_short_name_hash ON contacts(short_name_hash);
