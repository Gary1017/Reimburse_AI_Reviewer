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
