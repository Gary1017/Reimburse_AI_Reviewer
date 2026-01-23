package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// ItemRepository implements port.ItemRepository
type ItemRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewItemRepository creates a new reimbursement item repository
func NewItemRepository(db *sql.DB, logger *zap.Logger) port.ItemRepository {
	return &ItemRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new reimbursement item
func (r *ItemRepository) Create(ctx context.Context, item *entity.ReimbursementItem) error {
	query := `
		INSERT INTO reimbursement_items (
			instance_id, item_type, description, amount, currency,
			receipt_attachment, ai_price_check, ai_policy_check,
			expense_date, vendor, business_purpose
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		item.InstanceID,
		item.ItemType,
		item.Description,
		item.Amount,
		item.Currency,
		item.ReceiptAttachment,
		item.AIPriceCheck,
		item.AIPolicyCheck,
		item.ExpenseDate,
		item.Vendor,
		item.BusinessPurpose,
	)
	if err != nil {
		r.logger.Error("Failed to create reimbursement item", zap.Error(err))
		return fmt.Errorf("failed to create reimbursement item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	item.ID = id
	return nil
}

// GetByID retrieves a reimbursement item by ID
func (r *ItemRepository) GetByID(ctx context.Context, id int64) (*entity.ReimbursementItem, error) {
	query := `
		SELECT id, instance_id, item_type, description, amount, currency,
			receipt_attachment, ai_price_check, ai_policy_check,
			expense_date, vendor, business_purpose, created_at
		FROM reimbursement_items
		WHERE id = ?
	`

	var item entity.ReimbursementItem
	var expenseDate sql.NullTime

	err := r.getExecutor(ctx).QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.InstanceID,
		&item.ItemType,
		&item.Description,
		&item.Amount,
		&item.Currency,
		&item.ReceiptAttachment,
		&item.AIPriceCheck,
		&item.AIPolicyCheck,
		&expenseDate,
		&item.Vendor,
		&item.BusinessPurpose,
		&item.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get item by ID", zap.Int64("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	if expenseDate.Valid {
		item.ExpenseDate = &expenseDate.Time
	}

	return &item, nil
}

// GetByInstanceID retrieves all reimbursement items for an instance
func (r *ItemRepository) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error) {
	query := `
		SELECT id, instance_id, item_type, description, amount, currency,
			receipt_attachment, ai_price_check, ai_policy_check,
			expense_date, vendor, business_purpose, created_at
		FROM reimbursement_items
		WHERE instance_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, instanceID)
	if err != nil {
		r.logger.Error("Failed to get items by instance ID", zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	defer rows.Close()

	var items []*entity.ReimbursementItem
	for rows.Next() {
		var item entity.ReimbursementItem
		var expenseDate sql.NullTime

		err := rows.Scan(
			&item.ID,
			&item.InstanceID,
			&item.ItemType,
			&item.Description,
			&item.Amount,
			&item.Currency,
			&item.ReceiptAttachment,
			&item.AIPriceCheck,
			&item.AIPolicyCheck,
			&expenseDate,
			&item.Vendor,
			&item.BusinessPurpose,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}

		if expenseDate.Valid {
			item.ExpenseDate = &expenseDate.Time
		}

		items = append(items, &item)
	}

	return items, rows.Err()
}

// Update updates a reimbursement item
func (r *ItemRepository) Update(ctx context.Context, item *entity.ReimbursementItem) error {
	query := `
		UPDATE reimbursement_items
		SET item_type = ?, description = ?, amount = ?, currency = ?,
			receipt_attachment = ?, ai_price_check = ?, ai_policy_check = ?,
			expense_date = ?, vendor = ?, business_purpose = ?
		WHERE id = ?
	`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		item.ItemType,
		item.Description,
		item.Amount,
		item.Currency,
		item.ReceiptAttachment,
		item.AIPriceCheck,
		item.AIPolicyCheck,
		item.ExpenseDate,
		item.Vendor,
		item.BusinessPurpose,
		item.ID,
	)
	if err != nil {
		r.logger.Error("Failed to update item", zap.Int64("id", item.ID), zap.Error(err))
		return fmt.Errorf("failed to update item: %w", err)
	}

	return nil
}

// getExecutor returns appropriate executor based on context
func (r *ItemRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// Verify interface compliance
var _ port.ItemRepository = (*ItemRepository)(nil)
