#!/bin/bash

# Migration runner for WhatSignal database
# Usage: ./run-migrations.sh [database_path]

set -e

DB_PATH=${1:-"./whatsignal.db"}

if [ ! -f "$DB_PATH" ]; then
    echo "Error: Database file not found: $DB_PATH"
    exit 1
fi

echo "Running migrations on database: $DB_PATH"

# Check if sqlite3 is available
if ! command -v sqlite3 &> /dev/null; then
    echo "Error: sqlite3 command not found. Please install sqlite3."
    exit 1
fi

# Create migrations table if it doesn't exist
sqlite3 "$DB_PATH" "CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);"

# Function to check if migration is already applied
is_migration_applied() {
    local version=$1
    local count=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM schema_migrations WHERE version = $version;")
    [ "$count" -gt 0 ]
}

# Function to apply migration
apply_migration() {
    local version=$1
    local file=$2
    
    if is_migration_applied $version; then
        echo "Migration $version already applied, skipping..."
        return
    fi
    
    echo "Applying migration $version: $file"
    
    # Apply the migration
    sqlite3 "$DB_PATH" < "$file"
    
    # Record that migration was applied
    sqlite3 "$DB_PATH" "INSERT INTO schema_migrations (version) VALUES ($version);"
    
    echo "Migration $version applied successfully"
}

# Apply migrations in order
MIGRATIONS_DIR="$(dirname "$0")/migrations"

if [ -f "$MIGRATIONS_DIR/002_add_session_name.sql" ]; then
    apply_migration 2 "$MIGRATIONS_DIR/002_add_session_name.sql"
fi

echo "All migrations completed successfully"