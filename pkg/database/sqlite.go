package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// Config holds database configuration
type Config struct {
	Path            string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DB wraps sql.DB with additional functionality
type DB struct {
	*sql.DB
	logger *zap.Logger
}

// New creates a new database connection
func New(cfg Config, logger *zap.Logger) (*DB, error) {
	// Enable WAL mode for better concurrency
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", cfg.Path)

	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		DB:     sqlDB,
		logger: logger,
	}

	logger.Info("Database connection established", zap.String("path", cfg.Path))
	return db, nil
}

// BeginTx starts a new transaction
func (db *DB) BeginTx() (*sql.Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		db.logger.Error("Failed to begin transaction", zap.Error(err))
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// WithTransaction executes a function within a transaction
func (db *DB) WithTransaction(fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx()
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			db.logger.Error("Failed to rollback transaction", zap.Error(rbErr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		db.logger.Error("Failed to commit transaction", zap.Error(err))
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	db.logger.Info("Closing database connection")
	return db.DB.Close()
}
