package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := flag.String("db", "./whatsignal.db", "Path to the database file")
	flag.Parse()

	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		log.Fatalf("Database file not found: %s", *dbPath)
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create migrations table if it doesn't exist
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatalf("Failed to create migrations table: %v", err)
	}

	// Check if migration 2 is already applied
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 2").Scan(&count)
	if err != nil {
		log.Fatalf("Failed to check migration status: %v", err)
	}

	if count > 0 {
		fmt.Println("Migration 2 already applied, skipping...")
		return
	}

	fmt.Println("Applying migration 2: Add session_name and media_type columns")

	// Apply migration 2
	migrations := []string{
		"ALTER TABLE message_mappings ADD COLUMN session_name TEXT NOT NULL DEFAULT 'default'",
		"ALTER TABLE message_mappings ADD COLUMN media_type TEXT",
		"CREATE INDEX IF NOT EXISTS idx_session_name ON message_mappings(session_name)",
		"CREATE INDEX IF NOT EXISTS idx_session_chat ON message_mappings(session_name, whatsapp_chat_id)",
		"INSERT INTO schema_migrations (version) VALUES (2)",
	}

	for i, migration := range migrations {
		fmt.Printf("Executing step %d/%d...\n", i+1, len(migrations))
		_, err = db.Exec(migration)
		if err != nil {
			log.Fatalf("Failed to execute migration step %d: %v", i+1, err)
		}
	}

	fmt.Println("Migration 2 applied successfully")
	fmt.Println("Database schema updated. You can now restart WhatSignal.")
}