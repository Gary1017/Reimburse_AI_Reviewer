package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// NotificationRepository handles audit notification database operations
// ARCH-012: AI Audit Result Notification via Lark Approval Bot
type NotificationRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *sql.DB, logger *zap.Logger) *NotificationRepository {
	return &NotificationRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new audit notification record
func (r *NotificationRepository) Create(tx *sql.Tx, notification *models.AuditNotification) error {
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

	var result sql.Result
	var err error

	if tx != nil {
		result, err = tx.Exec(query,
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
	} else {
		result, err = r.db.Exec(query,
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
	}

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

// GetByInstanceID retrieves a notification by instance ID (for idempotency check)
func (r *NotificationRepository) GetByInstanceID(instanceID int64) (*models.AuditNotification, error) {
	query := `
		SELECT id, instance_id, lark_instance_id, status, audit_decision,
			confidence, total_amount, approver_count, violations,
			sent_at, error_message, created_at, updated_at
		FROM audit_notifications
		WHERE instance_id = ?
	`

	var notification models.AuditNotification
	var sentAt sql.NullTime
	var errorMsg sql.NullString
	var violations sql.NullString

	err := r.db.QueryRow(query, instanceID).Scan(
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

// UpdateStatus updates the notification status and optionally sets sent_at or error_message
func (r *NotificationRepository) UpdateStatus(tx *sql.Tx, id int64, status string, errorMsg string) error {
	query := `
		UPDATE audit_notifications
		SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`

	var sentAtUpdate interface{}
	if status == models.NotificationStatusSent {
		now := time.Now()
		sentAtUpdate = now
		query = `
			UPDATE audit_notifications
			SET status = ?, sent_at = ?, error_message = ?, updated_at = ?
			WHERE id = ?
		`
	}

	now := time.Now()
	var err error

	if status == models.NotificationStatusSent {
		if tx != nil {
			_, err = tx.Exec(query, status, sentAtUpdate, errorMsg, now, id)
		} else {
			_, err = r.db.Exec(query, status, sentAtUpdate, errorMsg, now, id)
		}
	} else {
		if tx != nil {
			_, err = tx.Exec(query, status, errorMsg, now, id)
		} else {
			_, err = r.db.Exec(query, status, errorMsg, now, id)
		}
	}

	if err != nil {
		r.logger.Error("Failed to update notification status",
			zap.Int64("id", id),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// GetPendingNotifications retrieves notifications that need to be retried
func (r *NotificationRepository) GetPendingNotifications(limit int) ([]*models.AuditNotification, error) {
	query := `
		SELECT id, instance_id, lark_instance_id, status, audit_decision,
			confidence, total_amount, approver_count, violations,
			sent_at, error_message, created_at, updated_at
		FROM audit_notifications
		WHERE status = ?
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := r.db.Query(query, models.NotificationStatusPending, limit)
	if err != nil {
		r.logger.Error("Failed to get pending notifications", zap.Error(err))
		return nil, fmt.Errorf("failed to get pending notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*models.AuditNotification
	for rows.Next() {
		var notification models.AuditNotification
		var sentAt sql.NullTime
		var errorMsg sql.NullString
		var violations sql.NullString

		err := rows.Scan(
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
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
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

		notifications = append(notifications, &notification)
	}

	return notifications, rows.Err()
}
