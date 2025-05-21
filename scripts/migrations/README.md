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
   - Sets up indexes for efficient querying
   - Configures triggers for timestamp management

## Development

When adding a new migration:

1. Create a new file with the next number in sequence
2. Add both "up" and "down" migrations if applicable
3. Test the migration in isolation
4. Update this README with the new migration details

## Production

Migrations are automatically applied when the application starts. The application will:

1. Look for migration files in several locations:
   - Relative to the working directory
   - Relative to the executable
   - In parent directories until found
2. Apply any new migrations in order
3. Log the migration process

## Testing

To override the migrations directory location in tests:

```go
migrations.MigrationsDir = "testdata/migrations"
``` 