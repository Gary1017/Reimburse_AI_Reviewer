package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// InvoiceListRepository implements port.InvoiceListRepository
type InvoiceListRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewInvoiceListRepository creates a new invoice list repository
func NewInvoiceListRepository(db *sql.DB, logger *zap.Logger) port.InvoiceListRepository {
	return &InvoiceListRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new invoice list for an instance
func (r *InvoiceListRepository) Create(ctx context.Context, list *entity.InvoiceList) error {
	query := `
		INSERT INTO invoice_lists (
			instance_id, total_invoice_count, total_invoice_amount, total_invoice_amount_cents, status
		) VALUES (?, ?, ?, ?, ?)
	`

	// Write to both amount columns for backwards compatibility
	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		list.InstanceID,
		list.TotalInvoiceCount,
		float64(list.TotalInvoiceAmountCents)/100.0, // Deprecated
		list.TotalInvoiceAmountCents,                 // Primary
		list.Status,
	)
	if err != nil {
		r.logger.Error("Failed to create invoice list",
			zap.Int64("instance_id", list.InstanceID),
			zap.Error(err))
		return fmt.Errorf("failed to create invoice list: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	list.ID = id
	return nil
}

// GetByID retrieves an invoice list by its ID
func (r *InvoiceListRepository) GetByID(ctx context.Context, id int64) (*entity.InvoiceList, error) {
	query := `
		SELECT id, instance_id, total_invoice_count,
			COALESCE(total_invoice_amount_cents, CAST(total_invoice_amount * 100 AS INTEGER)) as total_invoice_amount_cents,
			total_invoice_amount, status, created_at, updated_at
		FROM invoice_lists
		WHERE id = ?
	`

	var list entity.InvoiceList
	err := r.getExecutor(ctx).QueryRowContext(ctx, query, id).Scan(
		&list.ID,
		&list.InstanceID,
		&list.TotalInvoiceCount,
		&list.TotalInvoiceAmountCents,
		&list.TotalInvoiceAmount, // Deprecated
		&list.Status,
		&list.CreatedAt,
		&list.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get invoice list by ID",
			zap.Int64("id", id),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get invoice list: %w", err)
	}

	return &list, nil
}

// GetByInstanceID retrieves the invoice list for an instance (1:1 relationship)
func (r *InvoiceListRepository) GetByInstanceID(ctx context.Context, instanceID int64) (*entity.InvoiceList, error) {
	query := `
		SELECT id, instance_id, total_invoice_count,
			COALESCE(total_invoice_amount_cents, CAST(total_invoice_amount * 100 AS INTEGER)) as total_invoice_amount_cents,
			total_invoice_amount, status, created_at, updated_at
		FROM invoice_lists
		WHERE instance_id = ?
	`

	var list entity.InvoiceList
	err := r.getExecutor(ctx).QueryRowContext(ctx, query, instanceID).Scan(
		&list.ID,
		&list.InstanceID,
		&list.TotalInvoiceCount,
		&list.TotalInvoiceAmountCents,
		&list.TotalInvoiceAmount, // Deprecated
		&list.Status,
		&list.CreatedAt,
		&list.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get invoice list by instance ID",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get invoice list: %w", err)
	}

	return &list, nil
}

// Update updates an existing invoice list
func (r *InvoiceListRepository) Update(ctx context.Context, list *entity.InvoiceList) error {
	query := `
		UPDATE invoice_lists
		SET total_invoice_count = ?, total_invoice_amount = ?, total_invoice_amount_cents = ?,
			status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	// Write to both amount columns for backwards compatibility
	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		list.TotalInvoiceCount,
		float64(list.TotalInvoiceAmountCents)/100.0, // Deprecated
		list.TotalInvoiceAmountCents,                 // Primary
		list.Status,
		list.ID,
	)
	if err != nil {
		r.logger.Error("Failed to update invoice list",
			zap.Int64("id", list.ID),
			zap.Error(err))
		return fmt.Errorf("failed to update invoice list: %w", err)
	}

	return nil
}

// UpdateStatus updates the status of an invoice list
func (r *InvoiceListRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := `UPDATE invoice_lists SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query, status, id)
	if err != nil {
		r.logger.Error("Failed to update invoice list status",
			zap.Int64("id", id),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update invoice list status: %w", err)
	}

	return nil
}

// UpdateTotals updates the count and amount totals (amount in cents)
func (r *InvoiceListRepository) UpdateTotals(ctx context.Context, id int64, count int, amountCents int64) error {
	query := `
		UPDATE invoice_lists
		SET total_invoice_count = ?, total_invoice_amount = ?, total_invoice_amount_cents = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	// Write to both amount columns for backwards compatibility
	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		count,
		float64(amountCents)/100.0, // Deprecated
		amountCents,                 // Primary
		id,
	)
	if err != nil {
		r.logger.Error("Failed to update invoice list totals",
			zap.Int64("id", id),
			zap.Int("count", count),
			zap.Int64("amount_cents", amountCents),
			zap.Error(err))
		return fmt.Errorf("failed to update invoice list totals: %w", err)
	}

	return nil
}

// getExecutor returns appropriate executor based on context
func (r *InvoiceListRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// Verify interface compliance
var _ port.InvoiceListRepository = (*InvoiceListRepository)(nil)
