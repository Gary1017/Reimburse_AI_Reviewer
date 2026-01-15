package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// VoucherRepository handles voucher database operations
type VoucherRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewVoucherRepository creates a new voucher repository
func NewVoucherRepository(db *sql.DB, logger *zap.Logger) *VoucherRepository {
	return &VoucherRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new voucher record
func (r *VoucherRepository) Create(tx *sql.Tx, voucher *models.GeneratedVoucher) error {
	query := `
		INSERT INTO generated_vouchers (
			instance_id, voucher_number, file_path, accountant_email
		) VALUES (?, ?, ?, ?)
	`

	var result sql.Result
	var err error

	if tx != nil {
		result, err = tx.Exec(query,
			voucher.InstanceID,
			voucher.VoucherNumber,
			voucher.FilePath,
			voucher.AccountantEmail,
		)
	} else {
		result, err = r.db.Exec(query,
			voucher.InstanceID,
			voucher.VoucherNumber,
			voucher.FilePath,
			voucher.AccountantEmail,
		)
	}

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

// UpdateEmailSent updates the email sent information
func (r *VoucherRepository) UpdateEmailSent(tx *sql.Tx, id int64, messageID string, sentAt time.Time) error {
	query := `
		UPDATE generated_vouchers 
		SET email_message_id = ?, sent_at = ? 
		WHERE id = ?
	`

	var err error
	if tx != nil {
		_, err = tx.Exec(query, messageID, sentAt, id)
	} else {
		_, err = r.db.Exec(query, messageID, sentAt, id)
	}

	if err != nil {
		r.logger.Error("Failed to update email sent", zap.Int64("id", id), zap.Error(err))
		return fmt.Errorf("failed to update email sent: %w", err)
	}

	return nil
}

// GetByInstanceID retrieves a voucher by instance ID
func (r *VoucherRepository) GetByInstanceID(instanceID int64) (*models.GeneratedVoucher, error) {
	query := `
		SELECT id, instance_id, voucher_number, file_path, email_message_id,
			sent_at, accountant_email, created_at
		FROM generated_vouchers
		WHERE instance_id = ?
	`

	var voucher models.GeneratedVoucher
	var emailMessageID sql.NullString
	var sentAt sql.NullTime

	err := r.db.QueryRow(query, instanceID).Scan(
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

// GenerateVoucherNumber generates a unique voucher number
func (r *VoucherRepository) GenerateVoucherNumber() (string, error) {
	// Format: RB-YYYYMMDD-NNNN (RB = Reimbursement)
	now := time.Now()
	prefix := fmt.Sprintf("RB-%s-", now.Format("20060102"))

	// Get the max sequence for today
	query := `
		SELECT voucher_number 
		FROM generated_vouchers 
		WHERE voucher_number LIKE ? 
		ORDER BY voucher_number DESC 
		LIMIT 1
	`

	var lastVoucherNumber string
	err := r.db.QueryRow(query, prefix+"%").Scan(&lastVoucherNumber)
	
	sequence := 1
	if err == nil && lastVoucherNumber != "" {
		// Extract sequence number from last voucher
		var seq int
		_, err := fmt.Sscanf(lastVoucherNumber, prefix+"%d", &seq)
		if err == nil {
			sequence = seq + 1
		}
	}

	return fmt.Sprintf("%s%04d", prefix, sequence), nil
}
