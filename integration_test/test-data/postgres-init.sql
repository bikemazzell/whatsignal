-- PostgreSQL initialization script for WhatsSignal integration testing

-- Create additional test databases
CREATE DATABASE whatsignal_test_multi;
CREATE DATABASE whatsignal_benchmark;

-- Create test user with limited permissions
CREATE USER whatsignal_readonly WITH PASSWORD 'readonly_password';
GRANT CONNECT ON DATABASE whatsignal_test TO whatsignal_readonly;

-- Create test data for benchmarking
\c whatsignal_test;

-- Create tables if they don't exist
CREATE TABLE IF NOT EXISTS contacts (
    contact_id VARCHAR(255) PRIMARY KEY,
    phone_number VARCHAR(50),
    name VARCHAR(255),
    cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS message_mappings (
    id SERIAL PRIMARY KEY,
    whatsapp_chat_id VARCHAR(255),
    whatsapp_msg_id VARCHAR(255) UNIQUE,
    signal_msg_id VARCHAR(255),
    session_name VARCHAR(100),
    delivery_status VARCHAR(50),
    signal_timestamp TIMESTAMP,
    forwarded_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Performance testing functions
CREATE OR REPLACE FUNCTION generate_test_contacts(num_contacts INTEGER)
RETURNS VOID AS $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 1..num_contacts LOOP
        INSERT INTO contacts (contact_id, phone_number, name, cached_at)
        VALUES (
            'test' || i || '@c.us',
            '+1' || LPAD(i::text, 10, '0'),
            'Test User ' || i,
            NOW() - (RANDOM() * INTERVAL '30 days')
        ) ON CONFLICT (contact_id) DO NOTHING;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Message mapping generation for load testing
CREATE OR REPLACE FUNCTION generate_test_mappings(num_mappings INTEGER)
RETURNS VOID AS $$
DECLARE
    i INTEGER;
    sessions TEXT[] := ARRAY['personal', 'business', 'emergency'];
    statuses TEXT[] := ARRAY['pending', 'delivered', 'failed'];
BEGIN
    FOR i IN 1..num_mappings LOOP
        INSERT INTO message_mappings (
            whatsapp_chat_id,
            whatsapp_msg_id,
            signal_msg_id,
            session_name,
            delivery_status,
            signal_timestamp,
            forwarded_at,
            created_at,
            updated_at
        ) VALUES (
            'test' || (i % 1000) || '@c.us',
            'wamid.test' || i,
            'signal-msg-' || i,
            sessions[1 + (i % 3)],
            statuses[1 + (i % 3)],
            NOW() - (RANDOM() * INTERVAL '24 hours'),
            NOW() - (RANDOM() * INTERVAL '23 hours'),
            NOW() - (RANDOM() * INTERVAL '24 hours'),
            NOW() - (RANDOM() * INTERVAL '12 hours')
        );
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Create indexes for performance testing
CREATE INDEX IF NOT EXISTS idx_contacts_phone_test ON contacts(phone_number) WHERE contact_id LIKE 'test%';
CREATE INDEX IF NOT EXISTS idx_mappings_session_test ON message_mappings(session_name) WHERE whatsapp_msg_id LIKE 'wamid.test%';
CREATE INDEX IF NOT EXISTS idx_mappings_timestamp_test ON message_mappings(signal_timestamp) WHERE whatsapp_msg_id LIKE 'wamid.test%';

-- Grant permissions to the main user
GRANT EXECUTE ON FUNCTION generate_test_contacts(INTEGER) TO whatsignal;
GRANT EXECUTE ON FUNCTION generate_test_mappings(INTEGER) TO whatsignal;

-- Grant permissions to readonly user on the tables
GRANT USAGE ON SCHEMA public TO whatsignal_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO whatsignal_readonly;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO whatsignal_readonly;