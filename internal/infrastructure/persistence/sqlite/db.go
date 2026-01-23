package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"go.uber.org/zap"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const txKey contextKey = "tx"

// DB wraps sql.DB and implements TransactionManager
type DB struct {
	*sql.DB
	logger *zap.Logger
}

// NewDB creates a new database wrapper
func NewDB(sqlDB *sql.DB, logger *zap.Logger) *DB {
	return &DB{
		DB:     sqlDB,
		logger: logger,
	}
}

// WithTransaction implements port.TransactionManager
// Executes the provided function within a database transaction
func (db *DB) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	// Check if already in a transaction
	if tx := extractTx(ctx); tx != nil {
		// Reuse existing transaction
		return fn(ctx)
	}

	// Start new transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		db.logger.Error("Failed to begin transaction", zap.Error(err))
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Add transaction to context
	txCtx := context.WithValue(ctx, txKey, tx)

	// Handle panic and ensure rollback
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			db.logger.Error("Transaction panicked, rolled back", zap.Any("panic", p))
			panic(p)
		}
	}()

	// Execute function
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			db.logger.Error("Failed to rollback transaction", zap.Error(rbErr))
		}
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		db.logger.Error("Failed to commit transaction", zap.Error(err))
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// extractTx retrieves transaction from context if present
func extractTx(ctx context.Context) *sql.Tx {
	if tx, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return tx
	}
	return nil
}

// getExecutor returns appropriate executor (transaction or database)
func (db *DB) getExecutor(ctx context.Context) executor {
	if tx := extractTx(ctx); tx != nil {
		return tx
	}
	return db.DB
}

// executor interface covers both *sql.DB and *sql.Tx
type executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Verify interface compliance
var _ port.TransactionManager = (*DB)(nil)
