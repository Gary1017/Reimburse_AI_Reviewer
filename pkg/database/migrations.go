package database

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"
)

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Migrator handles database migrations
type Migrator struct {
	db     *DB
	logger *zap.Logger
}

// NewMigrator creates a new migrator
func NewMigrator(db *DB, logger *zap.Logger) *Migrator {
	return &Migrator{
		db:     db,
		logger: logger,
	}
}

// createMigrationsTable creates the migrations tracking table
func (m *Migrator) createMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err := m.db.Exec(query)
	return err
}

// getAppliedMigrations returns the list of applied migration versions
func (m *Migrator) getAppliedMigrations() (map[int]bool, error) {
	rows, err := m.db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

// RunMigrations executes all pending migrations from a directory
func (m *Migrator) RunMigrations(migrationsDir string) error {
	m.logger.Info("Starting database migrations", zap.String("dir", migrationsDir))

	// Create migrations table if it doesn't exist
	if err := m.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := m.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Load migration files
	migrations, err := m.loadMigrations(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Apply pending migrations
	for _, migration := range migrations {
		if applied[migration.Version] {
			m.logger.Debug("Skipping applied migration",
				zap.Int("version", migration.Version),
				zap.String("name", migration.Name))
			continue
		}

		m.logger.Info("Applying migration",
			zap.Int("version", migration.Version),
			zap.String("name", migration.Name))

		if err := m.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}
	}

	m.logger.Info("Database migrations completed successfully")
	return nil
}

// loadMigrations loads all migration files from a directory
func (m *Migrator) loadMigrations(dir string) ([]Migration, error) {
	var migrations []Migration

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}

		// Extract version from filename (e.g., "001_initial_schema.sql" -> version 1)
		filename := filepath.Base(path)
		var version int
		var name string
		if _, err := fmt.Sscanf(filename, "%d", &version); err != nil {
			return fmt.Errorf("invalid migration filename format: %s", filename)
		}

		// Extract name (remove version and .sql extension)
		parts := strings.SplitN(filename, "_", 2)
		if len(parts) == 2 {
			name = strings.TrimSuffix(parts[1], ".sql")
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			SQL:     string(content),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// applyMigration applies a single migration within a transaction
func (m *Migrator) applyMigration(migration Migration) error {
	return m.db.WithTransaction(func(tx *sql.Tx) error {
		// Execute migration SQL
		if _, err := tx.Exec(migration.SQL); err != nil {
			return fmt.Errorf("failed to execute migration SQL: %w", err)
		}

		// Record migration
		_, err := tx.Exec(
			"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
			migration.Version,
			migration.Name,
		)
		if err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}

		return nil
	})
}
