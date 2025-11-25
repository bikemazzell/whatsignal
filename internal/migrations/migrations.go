package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"whatsignal/internal/security"
)

var (
	// MigrationsDir can be overridden in tests or by the application
	MigrationsDir = getDefaultMigrationsDir()
)

// getDefaultMigrationsDir returns the migrations directory, checking environment variable first
func getDefaultMigrationsDir() string {
	if dir := os.Getenv("WHATSIGNAL_MIGRATIONS_DIR"); dir != "" {
		return dir
	}
	return "scripts/migrations"
}

// RunMigrations applies all pending database migrations
func RunMigrations(db *sql.DB) error {
	return RunMigrationsWithContext(context.Background(), db)
}

// RunMigrationsWithContext applies all pending database migrations with context support
func RunMigrationsWithContext(ctx context.Context, db *sql.DB) error {
	// Create migrations tracking table
	if err := createMigrationsTable(ctx, db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Find migration files
	migrationFiles, err := findMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to find migration files: %w", err)
	}

	if len(migrationFiles) == 0 {
		return fmt.Errorf("no migration files found in %s", MigrationsDir)
	}

	// Apply each migration
	for _, migrationFile := range migrationFiles {
		if err := applyMigration(ctx, db, migrationFile); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migrationFile, err)
		}
	}

	return nil
}

// createMigrationsTable creates the schema_migrations table if it doesn't exist
func createMigrationsTable(ctx context.Context, db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := db.ExecContext(ctx, query)
	return err
}

// findMigrationFiles finds and sorts all SQL migration files
func findMigrationFiles() ([]string, error) {
	searchPaths := []string{
		MigrationsDir,
		"/app/scripts/migrations",
	}

	var migrationsPath string
	for _, path := range searchPaths {
		if err := security.ValidateFilePath(path); err != nil {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			migrationsPath = path
			break
		}
	}

	if migrationsPath == "" {
		return nil, fmt.Errorf("migrations directory not found, searched: %v", searchPaths)
	}

	// Read migration files
	files, err := os.ReadDir(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrationFiles []string
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		fullPath := filepath.Join(migrationsPath, file.Name())
		if err := security.ValidateFilePath(fullPath); err != nil {
			continue
		}

		migrationFiles = append(migrationFiles, fullPath)
	}

	// Sort files numerically by their prefix
	sort.Slice(migrationFiles, func(i, j int) bool {
		return getMigrationNumber(migrationFiles[i]) < getMigrationNumber(migrationFiles[j])
	})

	return migrationFiles, nil
}

// getMigrationNumber extracts the numeric prefix from a migration filename
func getMigrationNumber(filename string) int {
	basename := filepath.Base(filename)
	parts := strings.SplitN(basename, "_", 2)
	if len(parts) < 2 {
		return 0
	}

	num, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	return num
}

// applyMigration applies a single migration if it hasn't been applied yet
func applyMigration(ctx context.Context, db *sql.DB, migrationFile string) error {
	filename := filepath.Base(migrationFile)

	// Check if migration already applied
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE filename = ?", filename).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if count > 0 {
		return nil // Already applied
	}

	// Read migration file
	content, err := os.ReadFile(migrationFile) // #nosec G304 - Path validated above
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Execute migration
	if _, err := db.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record migration as applied
	_, err = db.ExecContext(ctx, "INSERT INTO schema_migrations (filename) VALUES (?)", filename)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}
