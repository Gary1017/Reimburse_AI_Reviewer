package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// ReviewNotificationRepository implements port.ReviewNotificationRepository
type ReviewNotificationRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewReviewNotificationRepository creates a new review notification repository
func NewReviewNotificationRepository(db *sql.DB, logger *zap.Logger) port.ReviewNotificationRepository {
	return &ReviewNotificationRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new review notification
func (r *ReviewNotificationRepository) Create(ctx context.Context, notification *entity.ReviewNotification) error {
	query := `
		INSERT INTO review_notifications (
			task_id, lark_instance_id, status, approver_count, error_message
		) VALUES (?, ?, ?, ?, ?)
	`

	var errorMsg sql.NullString
	if notification.ErrorMessage != "" {
		errorMsg = sql.NullString{String: notification.ErrorMessage, Valid: true}
	}

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		notification.TaskID,
		notification.LarkInstanceID,
		notification.Status,
		notification.ApproverCount,
		errorMsg,
	)
	if err != nil {
		r.logger.Error("Failed to create review notification",
			zap.Int64("task_id", notification.TaskID),
			zap.Error(err))
		return fmt.Errorf("failed to create review notification: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	notification.ID = id
	return nil
}

// GetByID retrieves a notification by its ID
func (r *ReviewNotificationRepository) GetByID(ctx context.Context, id int64) (*entity.ReviewNotification, error) {
	query := `
		SELECT id, task_id, lark_instance_id, status, approver_count,
			sent_at, error_message, created_at, updated_at
		FROM review_notifications
		WHERE id = ?
	`

	notification, err := r.scanNotification(r.getExecutor(ctx).QueryRowContext(ctx, query, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get review notification by ID",
			zap.Int64("id", id),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get review notification: %w", err)
	}

	return notification, nil
}

// GetByTaskID retrieves notification by task ID (1:1 relationship)
func (r *ReviewNotificationRepository) GetByTaskID(ctx context.Context, taskID int64) (*entity.ReviewNotification, error) {
	query := `
		SELECT id, task_id, lark_instance_id, status, approver_count,
			sent_at, error_message, created_at, updated_at
		FROM review_notifications
		WHERE task_id = ?
	`

	notification, err := r.scanNotification(r.getExecutor(ctx).QueryRowContext(ctx, query, taskID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get review notification by task ID",
			zap.Int64("task_id", taskID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get review notification: %w", err)
	}

	return notification, nil
}

// UpdateStatus updates notification status and optional error message
func (r *ReviewNotificationRepository) UpdateStatus(ctx context.Context, id int64, status string, errorMsg string) error {
	query := `
		UPDATE review_notifications
		SET status = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	var errorMsgVal sql.NullString
	if errorMsg != "" {
		errorMsgVal = sql.NullString{String: errorMsg, Valid: true}
	}

	_, err := r.getExecutor(ctx).ExecContext(ctx, query, status, errorMsgVal, id)
	if err != nil {
		r.logger.Error("Failed to update review notification status",
			zap.Int64("id", id),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update review notification status: %w", err)
	}

	return nil
}

// MarkSent marks notification as sent with timestamp
func (r *ReviewNotificationRepository) MarkSent(ctx context.Context, id int64) error {
	query := `
		UPDATE review_notifications
		SET status = 'SENT', sent_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		r.logger.Error("Failed to mark review notification as sent",
			zap.Int64("id", id),
			zap.Error(err))
		return fmt.Errorf("failed to mark review notification as sent: %w", err)
	}

	return nil
}

// scanNotification scans a single notification row
func (r *ReviewNotificationRepository) scanNotification(row *sql.Row) (*entity.ReviewNotification, error) {
	var notification entity.ReviewNotification
	var sentAt sql.NullTime
	var errorMsg sql.NullString

	err := row.Scan(
		&notification.ID,
		&notification.TaskID,
		&notification.LarkInstanceID,
		&notification.Status,
		&notification.ApproverCount,
		&sentAt,
		&errorMsg,
		&notification.CreatedAt,
		&notification.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if sentAt.Valid {
		notification.SentAt = &sentAt.Time
	}
	if errorMsg.Valid {
		notification.ErrorMessage = errorMsg.String
	}

	return &notification, nil
}

// getExecutor returns appropriate executor based on context
func (r *ReviewNotificationRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// Verify interface compliance
var _ port.ReviewNotificationRepository = (*ReviewNotificationRepository)(nil)
