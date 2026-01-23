package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// VoucherRepository implements port.VoucherRepository
type VoucherRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewVoucherRepository creates a new voucher repository
func NewVoucherRepository(db *sql.DB, logger *zap.Logger) port.VoucherRepository {
	return &VoucherRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new voucher record
func (r *VoucherRepository) Create(ctx context.Context, voucher *entity.GeneratedVoucher) error {
	query := `
		INSERT INTO generated_vouchers (
			instance_id, voucher_number, file_path, accountant_email
		) VALUES (?, ?, ?, ?)
	`

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		voucher.InstanceID,
		voucher.VoucherNumber,
		voucher.FilePath,
		voucher.AccountantEmail,
	)
	if err != nil {
		r.logger.Error("Failed to create voucher", zap.Error(err))
		return fmt.Errorf("failed to create voucher: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	voucher.ID = id
	return nil
}

// GetByInstanceID retrieves a voucher by instance ID
func (r *VoucherRepository) GetByInstanceID(ctx context.Context, instanceID int64) (*entity.GeneratedVoucher, error) {
	query := `
		SELECT id, instance_id, voucher_number, file_path, email_message_id,
			sent_at, accountant_email, created_at
		FROM generated_vouchers
		WHERE instance_id = ?
	`

	var voucher entity.GeneratedVoucher
	var emailMessageID sql.NullString
	var sentAt sql.NullTime

	err := r.getExecutor(ctx).QueryRowContext(ctx, query, instanceID).Scan(
		&voucher.ID,
		&voucher.InstanceID,
		&voucher.VoucherNumber,
		&voucher.FilePath,
		&emailMessageID,
		&sentAt,
		&voucher.AccountantEmail,
		&voucher.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get voucher by instance ID", zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get voucher: %w", err)
	}

	if emailMessageID.Valid {
		voucher.EmailMessageID = emailMessageID.String
	}
	if sentAt.Valid {
		voucher.SentAt = &sentAt.Time
	}

	return &voucher, nil
}

// Update updates a voucher
func (r *VoucherRepository) Update(ctx context.Context, voucher *entity.GeneratedVoucher) error {
	query := `
		UPDATE generated_vouchers
		SET voucher_number = ?, file_path = ?, email_message_id = ?,
			sent_at = ?, accountant_email = ?
		WHERE id = ?
	`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		voucher.VoucherNumber,
		voucher.FilePath,
		voucher.EmailMessageID,
		voucher.SentAt,
		voucher.AccountantEmail,
		voucher.ID,
	)
	if err != nil {
		r.logger.Error("Failed to update voucher", zap.Int64("id", voucher.ID), zap.Error(err))
		return fmt.Errorf("failed to update voucher: %w", err)
	}

	return nil
}

// getExecutor returns appropriate executor based on context
func (r *VoucherRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// Verify interface compliance
var _ port.VoucherRepository = (*VoucherRepository)(nil)
