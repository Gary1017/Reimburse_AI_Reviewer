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

// InstanceRepository implements port.InstanceRepository
type InstanceRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewInstanceRepository creates a new instance repository
func NewInstanceRepository(db *sql.DB, logger *zap.Logger) port.InstanceRepository {
	return &InstanceRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new approval instance
func (r *InstanceRepository) Create(ctx context.Context, instance *entity.ApprovalInstance) error {
	query := `
		INSERT INTO approval_instances (
			lark_instance_id, status, applicant_user_id, department,
			submission_time, form_data
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		instance.LarkInstanceID,
		instance.Status,
		instance.ApplicantUserID,
		instance.Department,
		instance.SubmissionTime,
		instance.FormData,
	)
	if err != nil {
		r.logger.Error("Failed to create instance", zap.Error(err))
		return fmt.Errorf("failed to create instance: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	instance.ID = id
	return nil
}

// GetByID retrieves an approval instance by ID
func (r *InstanceRepository) GetByID(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
	query := `
		SELECT id, lark_instance_id, status, applicant_user_id, department,
			submission_time, approval_time, form_data,
			created_at, updated_at
		FROM approval_instances
		WHERE id = ?
	`

	var instance entity.ApprovalInstance
	var approvalTime sql.NullTime

	err := r.getExecutor(ctx).QueryRowContext(ctx, query, id).Scan(
		&instance.ID,
		&instance.LarkInstanceID,
		&instance.Status,
		&instance.ApplicantUserID,
		&instance.Department,
		&instance.SubmissionTime,
		&approvalTime,
		&instance.FormData,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get instance by ID", zap.Int64("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	if approvalTime.Valid {
		instance.ApprovalTime = &approvalTime.Time
	}

	return &instance, nil
}

// GetByLarkInstanceID retrieves an approval instance by Lark instance ID
func (r *InstanceRepository) GetByLarkInstanceID(ctx context.Context, larkID string) (*entity.ApprovalInstance, error) {
	query := `
		SELECT id, lark_instance_id, status, applicant_user_id, department,
			submission_time, approval_time, form_data,
			created_at, updated_at
		FROM approval_instances
		WHERE lark_instance_id = ?
	`

	var instance entity.ApprovalInstance
	var approvalTime sql.NullTime

	err := r.getExecutor(ctx).QueryRowContext(ctx, query, larkID).Scan(
		&instance.ID,
		&instance.LarkInstanceID,
		&instance.Status,
		&instance.ApplicantUserID,
		&instance.Department,
		&instance.SubmissionTime,
		&approvalTime,
		&instance.FormData,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get instance by Lark ID", zap.String("lark_instance_id", larkID), zap.Error(err))
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	if approvalTime.Valid {
		instance.ApprovalTime = &approvalTime.Time
	}

	return &instance, nil
}

// UpdateStatus updates the status of an approval instance
func (r *InstanceRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := `UPDATE approval_instances SET status = ? WHERE id = ?`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query, status, id)
	if err != nil {
		r.logger.Error("Failed to update status", zap.Int64("id", id), zap.String("status", status), zap.Error(err))
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// SetApprovalTime sets the approval time for an instance
func (r *InstanceRepository) SetApprovalTime(ctx context.Context, id int64, t time.Time) error {
	query := `UPDATE approval_instances SET approval_time = ? WHERE id = ?`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query, t, id)
	if err != nil {
		r.logger.Error("Failed to set approval time", zap.Int64("id", id), zap.Error(err))
		return fmt.Errorf("failed to set approval time: %w", err)
	}

	return nil
}

// List retrieves approval instances with pagination
func (r *InstanceRepository) List(ctx context.Context, limit, offset int) ([]*entity.ApprovalInstance, error) {
	query := `
		SELECT id, lark_instance_id, status, applicant_user_id, department,
			submission_time, approval_time, form_data,
			created_at, updated_at
		FROM approval_instances
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, limit, offset)
	if err != nil {
		r.logger.Error("Failed to list instances", zap.Error(err))
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}
	defer rows.Close()

	var instances []*entity.ApprovalInstance
	for rows.Next() {
		var instance entity.ApprovalInstance
		var approvalTime sql.NullTime

		err := rows.Scan(
			&instance.ID,
			&instance.LarkInstanceID,
			&instance.Status,
			&instance.ApplicantUserID,
			&instance.Department,
			&instance.SubmissionTime,
			&approvalTime,
			&instance.FormData,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan instance: %w", err)
		}

		if approvalTime.Valid {
			instance.ApprovalTime = &approvalTime.Time
		}

		instances = append(instances, &instance)
	}

	return instances, rows.Err()
}

// getExecutor returns appropriate executor based on context
func (r *InstanceRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// executor interface covers both *sql.DB and *sql.Tx
type executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type contextKey string

// Verify interface compliance
var _ port.InstanceRepository = (*InstanceRepository)(nil)
