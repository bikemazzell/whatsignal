# Database Migrations

This directory contains all database migration files for the WhatsSignal project.

## Structure

- Files are named with a numeric prefix for ordering (e.g., `001_`, `002_`)
- Each file represents a single migration
- Files are SQL format
- Each file should be idempotent (safe to run multiple times)

## Current Migrations

1. `001_initial_schema.sql` - Initial database schema
   - Creates message_mappings table
   - Creates contacts table for caching
   - Sets up indexes for efficient querying
   - Configures triggers for timestamp management

2. `002_add_session_name.sql` - Multi-channel support
   - Adds session_name column to message_mappings
   - Adds media_type column to message_mappings
   - Creates indexes for session-based queries

## Development

When adding a new migration:

1. Create a new file with the next number in sequence
2. Add both "up" and "down" migrations if applicable
3. Test the migration in isolation
4. Update this README with the new migration details

## Production

For existing databases that need schema updates, use the migration tool:

```bash
# Apply pending migrations
go run ./cmd/migrate

# Or specify a different database path
go run ./cmd/migrate -db /path/to/your/database.db
```

The migration tool will:
1. Create a schema_migrations tracking table
2. Check which migrations have been applied
3. Apply any pending migrations in order
4. Log the migration process

New installations automatically get the latest schema.

## Testing

To override the migrations directory location in tests:

```go
migrations.MigrationsDir = "testdata/migrations"
``` 