package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// ReimbursementItemRepository handles reimbursement item database operations
type ReimbursementItemRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewReimbursementItemRepository creates a new reimbursement item repository
func NewReimbursementItemRepository(db *sql.DB, logger *zap.Logger) *ReimbursementItemRepository {
	return &ReimbursementItemRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new reimbursement item
func (r *ReimbursementItemRepository) Create(tx *sql.Tx, item *models.ReimbursementItem) error {
	query := `
		INSERT INTO reimbursement_items (
			instance_id, item_type, description, amount, currency,
			receipt_attachment, ai_price_check, ai_policy_check,
			expense_date, vendor, business_purpose
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var result sql.Result
	var err error

	if tx != nil {
		result, err = tx.Exec(query,
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
	} else {
		result, err = r.db.Exec(query,
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
	}

	if err != nil {
		r.logger.Error("Failed to create reimbursement item", zap.Error(err))
		return fmt.Errorf("failed to create reimbursement item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	item.ID = id
	item.CreatedAt = time.Now()
	return nil
}

// GetByID retrieves a reimbursement item by ID
func (r *ReimbursementItemRepository) GetByID(id int64) (*models.ReimbursementItem, error) {
	query := `
		SELECT id, instance_id, item_type, description, amount, currency,
			receipt_attachment, ai_price_check, ai_policy_check,
			expense_date, vendor, business_purpose, created_at
		FROM reimbursement_items
		WHERE id = ?
	`

	var item models.ReimbursementItem
	var expenseDate sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
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
func (r *ReimbursementItemRepository) GetByInstanceID(instanceID int64) ([]*models.ReimbursementItem, error) {
	query := `
		SELECT id, instance_id, item_type, description, amount, currency,
			receipt_attachment, ai_price_check, ai_policy_check,
			expense_date, vendor, business_purpose, created_at
		FROM reimbursement_items
		WHERE instance_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(query, instanceID)
	if err != nil {
		r.logger.Error("Failed to get items by instance ID", zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get items: %w", err)
	}
	defer rows.Close()

	var items []*models.ReimbursementItem
	for rows.Next() {
		var item models.ReimbursementItem
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
func (r *ReimbursementItemRepository) Update(tx *sql.Tx, item *models.ReimbursementItem) error {
	query := `
		UPDATE reimbursement_items
		SET item_type = ?, description = ?, amount = ?, currency = ?,
			receipt_attachment = ?, ai_price_check = ?, ai_policy_check = ?,
			expense_date = ?, vendor = ?, business_purpose = ?
		WHERE id = ?
	`

	var err error
	if tx != nil {
		_, err = tx.Exec(query,
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
	} else {
		_, err = r.db.Exec(query,
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
	}

	if err != nil {
		r.logger.Error("Failed to update item", zap.Int64("id", item.ID), zap.Error(err))
		return fmt.Errorf("failed to update item: %w", err)
	}

	return nil
}

// DeleteByInstanceID deletes all reimbursement items for an instance
func (r *ReimbursementItemRepository) DeleteByInstanceID(tx *sql.Tx, instanceID int64) error {
	query := `DELETE FROM reimbursement_items WHERE instance_id = ?`

	var err error
	if tx != nil {
		_, err = tx.Exec(query, instanceID)
	} else {
		_, err = r.db.Exec(query, instanceID)
	}

	if err != nil {
		r.logger.Error("Failed to delete items by instance ID", zap.Int64("instance_id", instanceID), zap.Error(err))
		return fmt.Errorf("failed to delete items: %w", err)
	}

	return nil
}
