package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// AttachmentRepository handles attachment database operations
// Implements ARCH-004: Attachment metadata persistence
type AttachmentRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewAttachmentRepository creates a new attachment repository
func NewAttachmentRepository(db *sql.DB, logger *zap.Logger) *AttachmentRepository {
	return &AttachmentRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new attachment record
func (r *AttachmentRepository) Create(tx *sql.Tx, attachment *models.Attachment) error {
	query := `
		INSERT INTO attachments (
			item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			download_status, error_message, downloaded_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Use single timestamp for consistency between database and in-memory object
	now := time.Now()
	if attachment.CreatedAt.IsZero() {
		attachment.CreatedAt = now
	} else {
		now = attachment.CreatedAt
	}

	var result sql.Result
	var err error

	if tx != nil {
		result, err = tx.Exec(query,
			attachment.ItemID,
			attachment.InstanceID,
			attachment.FileName,
			attachment.URL,
			attachment.FilePath,
			attachment.FileSize,
			attachment.MimeType,
			attachment.DownloadStatus,
			attachment.ErrorMessage,
			attachment.DownloadedAt,
			now,
		)
	} else {
		result, err = r.db.Exec(query,
			attachment.ItemID,
			attachment.InstanceID,
			attachment.FileName,
			attachment.URL,
			attachment.FilePath,
			attachment.FileSize,
			attachment.MimeType,
			attachment.DownloadStatus,
			attachment.ErrorMessage,
			attachment.DownloadedAt,
			now,
		)
	}

	if err != nil {
		r.logger.Error("Failed to create attachment", zap.Error(err))
		return fmt.Errorf("failed to create attachment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	attachment.ID = id
	return nil
}

// GetByID retrieves an attachment by ID
func (r *AttachmentRepository) GetByID(id int64) (*models.Attachment, error) {
	query := `
		SELECT id, item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			download_status, error_message, downloaded_at, created_at
		FROM attachments
		WHERE id = ?
	`

	var attachment models.Attachment
	var downloadedAt sql.NullTime
	var url sql.NullString
	var errorMsg sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&attachment.ID,
		&attachment.ItemID,
		&attachment.InstanceID,
		&attachment.FileName,
		&url,
		&attachment.FilePath,
		&attachment.FileSize,
		&attachment.MimeType,
		&attachment.DownloadStatus,
		&errorMsg,
		&downloadedAt,
		&attachment.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get attachment by ID", zap.Int64("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to get attachment: %w", err)
	}

	if url.Valid {
		attachment.URL = url.String
	}

	if errorMsg.Valid {
		attachment.ErrorMessage = errorMsg.String
	}

	if downloadedAt.Valid {
		attachment.DownloadedAt = &downloadedAt.Time
	}

	return &attachment, nil
}

// GetByItemID retrieves all attachments for a reimbursement item
func (r *AttachmentRepository) GetByItemID(itemID int64) ([]*models.Attachment, error) {
	query := `
		SELECT id, item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			download_status, error_message, downloaded_at, created_at
		FROM attachments
		WHERE item_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(query, itemID)
	if err != nil {
		r.logger.Error("Failed to get attachments by item ID", zap.Int64("item_id", itemID), zap.Error(err))
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*models.Attachment
	for rows.Next() {
		var attachment models.Attachment
		var downloadedAt sql.NullTime
		var url sql.NullString

		err := rows.Scan(
			&attachment.ID,
			&attachment.ItemID,
			&attachment.InstanceID,
			&attachment.FileName,
			&url,
			&attachment.FilePath,
			&attachment.FileSize,
			&attachment.MimeType,
			&attachment.DownloadStatus,
			&attachment.ErrorMessage,
			&downloadedAt,
			&attachment.CreatedAt,
		)
		if err != nil {
			r.logger.Error("Failed to scan attachment", zap.Error(err))
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if url.Valid {
			attachment.URL = url.String
		}

		if downloadedAt.Valid {
			attachment.DownloadedAt = &downloadedAt.Time
		}

		attachments = append(attachments, &attachment)
	}

	return attachments, rows.Err()
}

// GetByInstanceID retrieves all attachments for an approval instance
func (r *AttachmentRepository) GetByInstanceID(instanceID int64) ([]*models.Attachment, error) {
	query := `
		SELECT id, item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			download_status, error_message, downloaded_at, created_at
		FROM attachments
		WHERE instance_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(query, instanceID)
	if err != nil {
		r.logger.Error("Failed to get attachments by instance ID",
			zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*models.Attachment
	for rows.Next() {
		var attachment models.Attachment
		var downloadedAt sql.NullTime

		err := rows.Scan(
			&attachment.ID,
			&attachment.ItemID,
			&attachment.InstanceID,
			&attachment.FileName,
			&attachment.URL,
			&attachment.FilePath,
			&attachment.FileSize,
			&attachment.MimeType,
			&attachment.DownloadStatus,
			&attachment.ErrorMessage,
			&downloadedAt,
			&attachment.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if downloadedAt.Valid {
			attachment.DownloadedAt = &downloadedAt.Time
		}

		attachments = append(attachments, &attachment)
	}

	return attachments, rows.Err()
}

// Update updates an attachment record
func (r *AttachmentRepository) Update(tx *sql.Tx, attachment *models.Attachment) error {
	query := `
		UPDATE attachments
		SET file_path = ?, file_size = ?, mime_type = ?,
			download_status = ?, error_message = ?, downloaded_at = ?
		WHERE id = ?
	`

	var err error
	if tx != nil {
		_, err = tx.Exec(query,
			attachment.FilePath,
			attachment.FileSize,
			attachment.MimeType,
			attachment.DownloadStatus,
			attachment.ErrorMessage,
			attachment.DownloadedAt,
			attachment.ID,
		)
	} else {
		_, err = r.db.Exec(query,
			attachment.FilePath,
			attachment.FileSize,
			attachment.MimeType,
			attachment.DownloadStatus,
			attachment.ErrorMessage,
			attachment.DownloadedAt,
			attachment.ID,
		)
	}

	if err != nil {
		r.logger.Error("Failed to update attachment", zap.Int64("id", attachment.ID), zap.Error(err))
		return fmt.Errorf("failed to update attachment: %w", err)
	}

	return nil
}

// UpdateStatus updates the download status and error message
func (r *AttachmentRepository) UpdateStatus(tx *sql.Tx, id int64, status, errorMessage string) error {
	query := `
		UPDATE attachments
		SET download_status = ?, error_message = ?
		WHERE id = ?
	`

	var err error
	if tx != nil {
		_, err = tx.Exec(query, status, errorMessage, id)
	} else {
		_, err = r.db.Exec(query, status, errorMessage, id)
	}

	if err != nil {
		r.logger.Error("Failed to update attachment status",
			zap.Int64("id", id),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// MarkDownloadCompleted marks attachment as downloaded with file info
func (r *AttachmentRepository) MarkDownloadCompleted(tx *sql.Tx, id int64, filePath string, fileSize int64) error {
	query := `
		UPDATE attachments
		SET download_status = ?, file_path = ?, file_size = ?, downloaded_at = ?
		WHERE id = ?
	`

	downloadedAt := time.Now()
	var err error
	if tx != nil {
		_, err = tx.Exec(query,
			models.AttachmentStatusCompleted,
			filePath,
			fileSize,
			downloadedAt,
			id,
		)
	} else {
		_, err = r.db.Exec(query,
			models.AttachmentStatusCompleted,
			filePath,
			fileSize,
			downloadedAt,
			id,
		)
	}

	if err != nil {
		r.logger.Error("Failed to mark download completed",
			zap.Int64("id", id),
			zap.Error(err))
		return fmt.Errorf("failed to mark download completed: %w", err)
	}

	return nil
}

// MarkDownloadFailed marks attachment as failed with error message
func (r *AttachmentRepository) MarkDownloadFailed(tx *sql.Tx, id int64, errorMessage string) error {
	query := `
		UPDATE attachments
		SET download_status = ?, error_message = ?
		WHERE id = ?
	`

	var err error
	if tx != nil {
		_, err = tx.Exec(query, models.AttachmentStatusFailed, errorMessage, id)
	} else {
		_, err = r.db.Exec(query, models.AttachmentStatusFailed, errorMessage, id)
	}

	if err != nil {
		r.logger.Error("Failed to mark download failed",
			zap.Int64("id", id),
			zap.Error(err))
		return fmt.Errorf("failed to mark download failed: %w", err)
	}

	return nil
}

// DeleteByInstanceID deletes all attachments for an instance
func (r *AttachmentRepository) DeleteByInstanceID(tx *sql.Tx, instanceID int64) error {
	query := `DELETE FROM attachments WHERE instance_id = ?`

	var err error
	if tx != nil {
		_, err = tx.Exec(query, instanceID)
	} else {
		_, err = r.db.Exec(query, instanceID)
	}

	if err != nil {
		r.logger.Error("Failed to delete attachments by instance ID",
			zap.Int64("instance_id", instanceID), zap.Error(err))
		return fmt.Errorf("failed to delete attachments: %w", err)
	}

	return nil
}

// GetPendingAttachments retrieves attachments that need to be downloaded
func (r *AttachmentRepository) GetPendingAttachments(limit int) ([]*models.Attachment, error) {
	query := `
		SELECT a.id, a.item_id, a.instance_id, i.lark_instance_id, a.file_name, a.url, a.file_path, a.file_size, a.mime_type,
			a.download_status, a.error_message, a.downloaded_at, a.created_at
		FROM attachments a
		JOIN approval_instances i ON a.instance_id = i.id
		WHERE a.download_status = ?
		ORDER BY a.created_at ASC
		LIMIT ?
	`

	rows, err := r.db.Query(query, models.AttachmentStatusPending, limit)
	if err != nil {
		r.logger.Error("Failed to get pending attachments", zap.Error(err))
		return nil, fmt.Errorf("failed to get pending attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*models.Attachment
	for rows.Next() {
		var attachment models.Attachment
		var downloadedAt sql.NullTime
		var url sql.NullString
		var errorMsg sql.NullString

		err := rows.Scan(
			&attachment.ID,
			&attachment.ItemID,
			&attachment.InstanceID,
			&attachment.LarkInstanceID,
			&attachment.FileName,
			&url,
			&attachment.FilePath,
			&attachment.FileSize,
			&attachment.MimeType,
			&attachment.DownloadStatus,
			&errorMsg,
			&downloadedAt,
			&attachment.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if url.Valid {
			attachment.URL = url.String
		}

		if errorMsg.Valid {
			attachment.ErrorMessage = errorMsg.String
		}

		if downloadedAt.Valid {
			attachment.DownloadedAt = &downloadedAt.Time
		}

		attachments = append(attachments, &attachment)
	}

	return attachments, rows.Err()
}

// GetCompletedAttachments retrieves downloaded attachments that need AI processing
// ARCH-011-E: Get attachments ready for invoice processing
func (r *AttachmentRepository) GetCompletedAttachments(limit int) ([]*models.Attachment, error) {
	query := `
		SELECT a.id, a.item_id, a.instance_id, i.lark_instance_id, a.file_name, a.url, a.file_path, a.file_size, a.mime_type,
			a.download_status, a.error_message, a.downloaded_at, a.created_at
		FROM attachments a
		JOIN approval_instances i ON a.instance_id = i.id
		WHERE a.download_status = ?
		ORDER BY a.downloaded_at ASC
		LIMIT ?
	`

	rows, err := r.db.Query(query, models.AttachmentStatusCompleted, limit)
	if err != nil {
		r.logger.Error("Failed to get completed attachments", zap.Error(err))
		return nil, fmt.Errorf("failed to get completed attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*models.Attachment
	for rows.Next() {
		var attachment models.Attachment
		var downloadedAt sql.NullTime
		var url sql.NullString
		var errorMsg sql.NullString

		err := rows.Scan(
			&attachment.ID,
			&attachment.ItemID,
			&attachment.InstanceID,
			&attachment.LarkInstanceID,
			&attachment.FileName,
			&url,
			&attachment.FilePath,
			&attachment.FileSize,
			&attachment.MimeType,
			&attachment.DownloadStatus,
			&errorMsg,
			&downloadedAt,
			&attachment.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if url.Valid {
			attachment.URL = url.String
		}

		if errorMsg.Valid {
			attachment.ErrorMessage = errorMsg.String
		}

		if downloadedAt.Valid {
			attachment.DownloadedAt = &downloadedAt.Time
		}

		attachments = append(attachments, &attachment)
	}

	return attachments, rows.Err()
}

// UpdateProcessingStatus updates attachment status with audit result
// ARCH-011-E: Update attachment with AI processing result
func (r *AttachmentRepository) UpdateProcessingStatus(tx *sql.Tx, id int64, status string, auditResult string, errMsg string) error {
	query := `
		UPDATE attachments
		SET download_status = ?, audit_result = ?, error_message = ?, processed_at = ?
		WHERE id = ?
	`

	processedAt := time.Now()
	var err error
	if tx != nil {
		_, err = tx.Exec(query, status, auditResult, errMsg, processedAt, id)
	} else {
		_, err = r.db.Exec(query, status, auditResult, errMsg, processedAt, id)
	}

	if err != nil {
		r.logger.Error("Failed to update processing status",
			zap.Int64("id", id),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update processing status: %w", err)
	}

	return nil
}

// GetProcessedByInstanceID retrieves all PROCESSED attachments for an instance
// ARCH-012: For audit notification aggregation
func (r *AttachmentRepository) GetProcessedByInstanceID(instanceID int64) ([]*models.Attachment, error) {
	query := `
		SELECT id, item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			download_status, error_message, downloaded_at, processed_at, audit_result, created_at
		FROM attachments
		WHERE instance_id = ? AND download_status = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(query, instanceID, models.AttachmentStatusProcessed)
	if err != nil {
		r.logger.Error("Failed to get processed attachments by instance ID",
			zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get processed attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*models.Attachment
	for rows.Next() {
		var attachment models.Attachment
		var downloadedAt, processedAt sql.NullTime
		var url, errorMsg, auditResult sql.NullString

		err := rows.Scan(
			&attachment.ID,
			&attachment.ItemID,
			&attachment.InstanceID,
			&attachment.FileName,
			&url,
			&attachment.FilePath,
			&attachment.FileSize,
			&attachment.MimeType,
			&attachment.DownloadStatus,
			&errorMsg,
			&downloadedAt,
			&processedAt,
			&auditResult,
			&attachment.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if url.Valid {
			attachment.URL = url.String
		}
		if errorMsg.Valid {
			attachment.ErrorMessage = errorMsg.String
		}
		if downloadedAt.Valid {
			attachment.DownloadedAt = &downloadedAt.Time
		}
		if processedAt.Valid {
			attachment.ProcessedAt = &processedAt.Time
		}
		if auditResult.Valid {
			attachment.AuditResult = auditResult.String
		}

		attachments = append(attachments, &attachment)
	}

	return attachments, rows.Err()
}

// GetUnprocessedCountByInstanceID returns count of attachments not yet fully processed
// ARCH-012: For checking if all attachments are audited
func (r *AttachmentRepository) GetUnprocessedCountByInstanceID(instanceID int64) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM attachments
		WHERE instance_id = ? AND download_status NOT IN (?, ?)
	`

	var count int
	err := r.db.QueryRow(query, instanceID, models.AttachmentStatusProcessed, models.AttachmentStatusAuditFailed).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to get unprocessed count by instance ID",
			zap.Int64("instance_id", instanceID), zap.Error(err))
		return 0, fmt.Errorf("failed to get unprocessed count: %w", err)
	}

	return count, nil
}

// GetTotalCountByInstanceID returns total count of attachments for an instance
// ARCH-012: For checking if all attachments are audited
func (r *AttachmentRepository) GetTotalCountByInstanceID(instanceID int64) (int, error) {
	query := `SELECT COUNT(*) FROM attachments WHERE instance_id = ?`

	var count int
	err := r.db.QueryRow(query, instanceID).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to get total count by instance ID",
			zap.Int64("instance_id", instanceID), zap.Error(err))
		return 0, fmt.Errorf("failed to get total count: %w", err)
	}

	return count, nil
}

// GetReimbursementItem retrieves a reimbursement item by ID
// ARCH-007: Required by AsyncDownloadWorker for filename generation
func (r *AttachmentRepository) GetReimbursementItem(itemID int64) (*models.ReimbursementItem, error) {
	query := `
		SELECT id, instance_id, item_type, description, amount, currency,
			receipt_attachment, ai_price_check, ai_policy_check,
			expense_date, vendor, business_purpose, created_at
		FROM reimbursement_items
		WHERE id = ?
	`

	var item models.ReimbursementItem
	var expenseDate sql.NullTime

	err := r.db.QueryRow(query, itemID).Scan(
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
		r.logger.Error("Failed to get reimbursement item by ID", zap.Int64("id", itemID), zap.Error(err))
		return nil, fmt.Errorf("failed to get reimbursement item: %w", err)
	}

	if expenseDate.Valid {
		item.ExpenseDate = &expenseDate.Time
	}

	return &item, nil
}
