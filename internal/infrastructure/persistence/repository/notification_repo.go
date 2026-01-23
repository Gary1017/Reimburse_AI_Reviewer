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

// NotificationRepository implements port.NotificationRepository
type NotificationRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *sql.DB, logger *zap.Logger) port.NotificationRepository {
	return &NotificationRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new audit notification record
func (r *NotificationRepository) Create(ctx context.Context, notification *entity.AuditNotification) error {
	query := `
		INSERT INTO audit_notifications (
			instance_id, lark_instance_id, status, audit_decision,
			confidence, total_amount, approver_count, violations,
			sent_at, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = now
	}
	notification.UpdatedAt = now

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		notification.InstanceID,
		notification.LarkInstanceID,
		notification.Status,
		notification.AuditDecision,
		notification.Confidence,
		notification.TotalAmount,
		notification.ApproverCount,
		notification.Violations,
		notification.SentAt,
		notification.ErrorMessage,
	)
	if err != nil {
		r.logger.Error("Failed to create notification",
			zap.Int64("instance_id", notification.InstanceID),
			zap.Error(err))
		return fmt.Errorf("failed to create notification: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	notification.ID = id
	return nil
}

// GetByInstanceID retrieves a notification by instance ID
func (r *NotificationRepository) GetByInstanceID(ctx context.Context, instanceID int64) (*entity.AuditNotification, error) {
	query := `
		SELECT id, instance_id, lark_instance_id, status, audit_decision,
			confidence, total_amount, approver_count, violations,
			sent_at, error_message, created_at, updated_at
		FROM audit_notifications
		WHERE instance_id = ?
	`

	var notification entity.AuditNotification
	var sentAt sql.NullTime
	var errorMsg sql.NullString
	var violations sql.NullString

	err := r.getExecutor(ctx).QueryRowContext(ctx, query, instanceID).Scan(
		&notification.ID,
		&notification.InstanceID,
		&notification.LarkInstanceID,
		&notification.Status,
		&notification.AuditDecision,
		&notification.Confidence,
		&notification.TotalAmount,
		&notification.ApproverCount,
		&violations,
		&sentAt,
		&errorMsg,
		&notification.CreatedAt,
		&notification.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get notification by instance ID",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	if sentAt.Valid {
		notification.SentAt = &sentAt.Time
	}
	if errorMsg.Valid {
		notification.ErrorMessage = errorMsg.String
	}
	if violations.Valid {
		notification.Violations = violations.String
	}

	return &notification, nil
}

// UpdateStatus updates the notification status and optionally sets error message
func (r *NotificationRepository) UpdateStatus(ctx context.Context, id int64, status string, errorMsg string) error {
	query := `
		UPDATE audit_notifications
		SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	_, err := r.getExecutor(ctx).ExecContext(ctx, query, status, errorMsg, now, id)
	if err != nil {
		r.logger.Error("Failed to update notification status",
			zap.Int64("id", id),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// MarkSent marks notification as sent
func (r *NotificationRepository) MarkSent(ctx context.Context, id int64) error {
	query := `
		UPDATE audit_notifications
		SET status = 'SENT', sent_at = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	_, err := r.getExecutor(ctx).ExecContext(ctx, query, now, now, id)
	if err != nil {
		r.logger.Error("Failed to mark notification as sent",
			zap.Int64("id", id),
			zap.Error(err))
		return fmt.Errorf("failed to mark sent: %w", err)
	}

	return nil
}

// getExecutor returns appropriate executor based on context
func (r *NotificationRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// Verify interface compliance
var _ port.NotificationRepository = (*NotificationRepository)(nil)
