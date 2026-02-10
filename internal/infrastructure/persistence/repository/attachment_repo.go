package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// AttachmentRepository implements port.AttachmentRepository
type AttachmentRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewAttachmentRepository creates a new attachment repository
func NewAttachmentRepository(db *sql.DB, logger *zap.Logger) port.AttachmentRepository {
	return &AttachmentRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new attachment record
func (r *AttachmentRepository) Create(ctx context.Context, att *entity.Attachment) error {
	query := `
		INSERT INTO attachments (
			item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			file_type, download_status, error_message, downloaded_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	if att.CreatedAt.IsZero() {
		att.CreatedAt = now
	} else {
		now = att.CreatedAt
	}

	// Default to INVOICE if not specified
	fileType := att.FileType
	if fileType == "" {
		fileType = entity.FileTypeInvoice
	}

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		att.ItemID,
		att.InstanceID,
		att.FileName,
		att.URL,
		att.FilePath,
		att.FileSize,
		att.MimeType,
		fileType,
		att.DownloadStatus,
		att.ErrorMessage,
		att.DownloadedAt,
		now,
	)
	if err != nil {
		r.logger.Error("Failed to create attachment", zap.Error(err))
		return fmt.Errorf("failed to create attachment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	att.ID = id
	return nil
}

// GetByID retrieves an attachment by ID
func (r *AttachmentRepository) GetByID(ctx context.Context, id int64) (*entity.Attachment, error) {
	query := `
		SELECT id, item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			COALESCE(file_type, 'INVOICE') as file_type,
			download_status, error_message, downloaded_at, created_at
		FROM attachments
		WHERE id = ?
	`

	var att entity.Attachment
	var downloadedAt sql.NullTime
	var url sql.NullString
	var errorMsg sql.NullString

	err := r.getExecutor(ctx).QueryRowContext(ctx, query, id).Scan(
		&att.ID,
		&att.ItemID,
		&att.InstanceID,
		&att.FileName,
		&url,
		&att.FilePath,
		&att.FileSize,
		&att.MimeType,
		&att.FileType,
		&att.DownloadStatus,
		&errorMsg,
		&downloadedAt,
		&att.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get attachment by ID", zap.Int64("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to get attachment: %w", err)
	}

	if url.Valid {
		att.URL = url.String
	}
	if errorMsg.Valid {
		att.ErrorMessage = errorMsg.String
	}
	if downloadedAt.Valid {
		att.DownloadedAt = &downloadedAt.Time
	}

	return &att, nil
}

// GetByInstanceID retrieves all attachments for an approval instance
func (r *AttachmentRepository) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Attachment, error) {
	query := `
		SELECT id, item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			COALESCE(file_type, 'INVOICE') as file_type,
			download_status, error_message, downloaded_at, created_at
		FROM attachments
		WHERE instance_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, instanceID)
	if err != nil {
		r.logger.Error("Failed to get attachments by instance ID",
			zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*entity.Attachment
	for rows.Next() {
		var att entity.Attachment
		var downloadedAt sql.NullTime
		var url sql.NullString
		var errorMsg sql.NullString

		err := rows.Scan(
			&att.ID,
			&att.ItemID,
			&att.InstanceID,
			&att.FileName,
			&url,
			&att.FilePath,
			&att.FileSize,
			&att.MimeType,
			&att.FileType,
			&att.DownloadStatus,
			&errorMsg,
			&downloadedAt,
			&att.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if url.Valid {
			att.URL = url.String
		}
		if errorMsg.Valid {
			att.ErrorMessage = errorMsg.String
		}
		if downloadedAt.Valid {
			att.DownloadedAt = &downloadedAt.Time
		}

		attachments = append(attachments, &att)
	}

	return attachments, rows.Err()
}

// GetPending retrieves attachments that need to be downloaded
func (r *AttachmentRepository) GetPending(ctx context.Context, limit int) ([]*entity.Attachment, error) {
	query := `
		SELECT id, item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			COALESCE(file_type, 'INVOICE') as file_type,
			download_status, error_message, downloaded_at, created_at
		FROM attachments
		WHERE download_status = 'PENDING'
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, limit)
	if err != nil {
		r.logger.Error("Failed to get pending attachments", zap.Error(err))
		return nil, fmt.Errorf("failed to get pending attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*entity.Attachment
	for rows.Next() {
		var att entity.Attachment
		var downloadedAt sql.NullTime
		var url sql.NullString
		var errorMsg sql.NullString

		err := rows.Scan(
			&att.ID,
			&att.ItemID,
			&att.InstanceID,
			&att.FileName,
			&url,
			&att.FilePath,
			&att.FileSize,
			&att.MimeType,
			&att.FileType,
			&att.DownloadStatus,
			&errorMsg,
			&downloadedAt,
			&att.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if url.Valid {
			att.URL = url.String
		}
		if errorMsg.Valid {
			att.ErrorMessage = errorMsg.String
		}
		if downloadedAt.Valid {
			att.DownloadedAt = &downloadedAt.Time
		}

		attachments = append(attachments, &att)
	}

	return attachments, rows.Err()
}

// MarkCompleted marks attachment as downloaded with file info
func (r *AttachmentRepository) MarkCompleted(ctx context.Context, id int64, filePath string, fileSize int64) error {
	query := `
		UPDATE attachments
		SET download_status = 'COMPLETED', file_path = ?, file_size = ?, downloaded_at = ?
		WHERE id = ?
	`

	downloadedAt := time.Now()
	_, err := r.getExecutor(ctx).ExecContext(ctx, query, filePath, fileSize, downloadedAt, id)
	if err != nil {
		r.logger.Error("Failed to mark download completed",
			zap.Int64("id", id),
			zap.Error(err))
		return fmt.Errorf("failed to mark download completed: %w", err)
	}

	return nil
}

// UpdateStatus updates the download status and error message
func (r *AttachmentRepository) UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error {
	query := `
		UPDATE attachments
		SET download_status = ?, error_message = ?
		WHERE id = ?
	`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query, status, errorMsg, id)
	if err != nil {
		r.logger.Error("Failed to update attachment status",
			zap.Int64("id", id),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// getExecutor returns appropriate executor based on context
func (r *AttachmentRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// GetCompletedUnprocessed retrieves attachments that are downloaded but not yet processed for invoice extraction.
// These are attachments with download_status=COMPLETED and empty/null file_type.
func (r *AttachmentRepository) GetCompletedUnprocessed(ctx context.Context, limit int) ([]*entity.Attachment, error) {
	query := `
		SELECT id, item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			COALESCE(file_type, '') as file_type,
			download_status, error_message, downloaded_at, created_at
		FROM attachments
		WHERE download_status = 'COMPLETED' AND (file_type IS NULL OR file_type = '')
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, limit)
	if err != nil {
		r.logger.Error("Failed to get completed unprocessed attachments", zap.Error(err))
		return nil, fmt.Errorf("failed to get completed unprocessed attachments: %w", err)
	}
	defer rows.Close()

	return r.scanAttachments(rows)
}

// UpdateItemID updates the item_id for an attachment (used by invoice-to-item matching)
func (r *AttachmentRepository) UpdateItemID(ctx context.Context, attachmentID int64, itemID int64) error {
	query := `UPDATE attachments SET item_id = ? WHERE id = ?`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query, itemID, attachmentID)
	if err != nil {
		r.logger.Error("Failed to update attachment item_id",
			zap.Int64("attachment_id", attachmentID),
			zap.Int64("item_id", itemID),
			zap.Error(err))
		return fmt.Errorf("failed to update attachment item_id: %w", err)
	}

	return nil
}

// UpdateFileType updates the file_type for an attachment (invoice vs other)
func (r *AttachmentRepository) UpdateFileType(ctx context.Context, attachmentID int64, fileType string) error {
	query := `UPDATE attachments SET file_type = ? WHERE id = ?`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query, fileType, attachmentID)
	if err != nil {
		r.logger.Error("Failed to update attachment file_type",
			zap.Int64("attachment_id", attachmentID),
			zap.String("file_type", fileType),
			zap.Error(err))
		return fmt.Errorf("failed to update attachment file_type: %w", err)
	}

	return nil
}

// GetByInstanceIDAndStatus retrieves attachments for an instance with a specific download status
func (r *AttachmentRepository) GetByInstanceIDAndStatus(ctx context.Context, instanceID int64, status string) ([]*entity.Attachment, error) {
	query := `
		SELECT id, item_id, instance_id, file_name, url, file_path, file_size, mime_type,
			COALESCE(file_type, 'INVOICE') as file_type,
			download_status, error_message, downloaded_at, created_at
		FROM attachments
		WHERE instance_id = ? AND download_status = ?
		ORDER BY created_at ASC
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, instanceID, status)
	if err != nil {
		r.logger.Error("Failed to get attachments by instance and status",
			zap.Int64("instance_id", instanceID),
			zap.String("status", status),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get attachments by instance and status: %w", err)
	}
	defer rows.Close()

	return r.scanAttachments(rows)
}

// CountByInstanceIDAndStatuses counts attachments for an instance with any of the given statuses
func (r *AttachmentRepository) CountByInstanceIDAndStatuses(ctx context.Context, instanceID int64, statuses []string) (int, error) {
	if len(statuses) == 0 {
		return 0, nil
	}

	placeholders := make([]string, len(statuses))
	args := make([]interface{}, 0, len(statuses)+1)
	args = append(args, instanceID)
	for i, s := range statuses {
		placeholders[i] = "?"
		args = append(args, s)
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM attachments
		WHERE instance_id = ? AND download_status IN (%s)
	`, strings.Join(placeholders, ","))

	var count int
	err := r.getExecutor(ctx).QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count attachments by instance and statuses",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		return 0, fmt.Errorf("failed to count attachments: %w", err)
	}

	return count, nil
}

// scanAttachments scans rows into attachment entities
func (r *AttachmentRepository) scanAttachments(rows *sql.Rows) ([]*entity.Attachment, error) {
	var attachments []*entity.Attachment
	for rows.Next() {
		var att entity.Attachment
		var downloadedAt sql.NullTime
		var url sql.NullString
		var errorMsg sql.NullString

		err := rows.Scan(
			&att.ID,
			&att.ItemID,
			&att.InstanceID,
			&att.FileName,
			&url,
			&att.FilePath,
			&att.FileSize,
			&att.MimeType,
			&att.FileType,
			&att.DownloadStatus,
			&errorMsg,
			&downloadedAt,
			&att.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}

		if url.Valid {
			att.URL = url.String
		}
		if errorMsg.Valid {
			att.ErrorMessage = errorMsg.String
		}
		if downloadedAt.Valid {
			att.DownloadedAt = &downloadedAt.Time
		}

		attachments = append(attachments, &att)
	}

	return attachments, rows.Err()
}

// Verify interface compliance
var _ port.AttachmentRepository = (*AttachmentRepository)(nil)
