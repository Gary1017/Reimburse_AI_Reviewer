package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// InstanceRepository handles approval instance database operations
type InstanceRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewInstanceRepository creates a new instance repository
func NewInstanceRepository(db *sql.DB, logger *zap.Logger) *InstanceRepository {
	return &InstanceRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new approval instance
func (r *InstanceRepository) Create(tx *sql.Tx, instance *models.ApprovalInstance) error {
	query := `
		INSERT INTO approval_instances (
			lark_instance_id, status, applicant_user_id, department,
			submission_time, form_data, ai_audit_result
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	var result sql.Result
	var err error

	if tx != nil {
		result, err = tx.Exec(query,
			instance.LarkInstanceID,
			instance.Status,
			instance.ApplicantUserID,
			instance.Department,
			instance.SubmissionTime,
			instance.FormData,
			instance.AIAuditResult,
		)
	} else {
		result, err = r.db.Exec(query,
			instance.LarkInstanceID,
			instance.Status,
			instance.ApplicantUserID,
			instance.Department,
			instance.SubmissionTime,
			instance.FormData,
			instance.AIAuditResult,
		)
	}

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
func (r *InstanceRepository) GetByID(id int64) (*models.ApprovalInstance, error) {
	query := `
		SELECT id, lark_instance_id, status, applicant_user_id, department,
			submission_time, approval_time, form_data, ai_audit_result,
			created_at, updated_at
		FROM approval_instances
		WHERE id = ?
	`

	var instance models.ApprovalInstance
	var approvalTime sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&instance.ID,
		&instance.LarkInstanceID,
		&instance.Status,
		&instance.ApplicantUserID,
		&instance.Department,
		&instance.SubmissionTime,
		&approvalTime,
		&instance.FormData,
		&instance.AIAuditResult,
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
func (r *InstanceRepository) GetByLarkInstanceID(larkInstanceID string) (*models.ApprovalInstance, error) {
	query := `
		SELECT id, lark_instance_id, status, applicant_user_id, department,
			submission_time, approval_time, form_data, ai_audit_result,
			created_at, updated_at
		FROM approval_instances
		WHERE lark_instance_id = ?
	`

	var instance models.ApprovalInstance
	var approvalTime sql.NullTime

	err := r.db.QueryRow(query, larkInstanceID).Scan(
		&instance.ID,
		&instance.LarkInstanceID,
		&instance.Status,
		&instance.ApplicantUserID,
		&instance.Department,
		&instance.SubmissionTime,
		&approvalTime,
		&instance.FormData,
		&instance.AIAuditResult,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get instance by Lark ID", zap.String("lark_instance_id", larkInstanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	if approvalTime.Valid {
		instance.ApprovalTime = &approvalTime.Time
	}

	return &instance, nil
}

// UpdateStatus updates the status of an approval instance
func (r *InstanceRepository) UpdateStatus(tx *sql.Tx, id int64, newStatus string) error {
	query := `UPDATE approval_instances SET status = ? WHERE id = ?`

	var err error
	if tx != nil {
		_, err = tx.Exec(query, newStatus, id)
	} else {
		_, err = r.db.Exec(query, newStatus, id)
	}

	if err != nil {
		r.logger.Error("Failed to update status", zap.Int64("id", id), zap.String("status", newStatus), zap.Error(err))
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// UpdateAIAuditResult updates the AI audit result
func (r *InstanceRepository) UpdateAIAuditResult(tx *sql.Tx, id int64, result *models.AIAuditResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal AI audit result: %w", err)
	}

	query := `UPDATE approval_instances SET ai_audit_result = ? WHERE id = ?`

	if tx != nil {
		_, err = tx.Exec(query, string(resultJSON), id)
	} else {
		_, err = r.db.Exec(query, string(resultJSON), id)
	}

	if err != nil {
		r.logger.Error("Failed to update AI audit result", zap.Int64("id", id), zap.Error(err))
		return fmt.Errorf("failed to update AI audit result: %w", err)
	}

	return nil
}

// SetApprovalTime sets the approval time for an instance
func (r *InstanceRepository) SetApprovalTime(tx *sql.Tx, id int64, approvalTime time.Time) error {
	query := `UPDATE approval_instances SET approval_time = ? WHERE id = ?`

	var err error
	if tx != nil {
		_, err = tx.Exec(query, approvalTime, id)
	} else {
		_, err = r.db.Exec(query, approvalTime, id)
	}

	if err != nil {
		r.logger.Error("Failed to set approval time", zap.Int64("id", id), zap.Error(err))
		return fmt.Errorf("failed to set approval time: %w", err)
	}

	return nil
}

// List retrieves approval instances with pagination
func (r *InstanceRepository) List(limit, offset int) ([]*models.ApprovalInstance, error) {
	query := `
		SELECT id, lark_instance_id, status, applicant_user_id, department,
			submission_time, approval_time, form_data, ai_audit_result,
			created_at, updated_at
		FROM approval_instances
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		r.logger.Error("Failed to list instances", zap.Error(err))
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}
	defer rows.Close()

	var instances []*models.ApprovalInstance
	for rows.Next() {
		var instance models.ApprovalInstance
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
			&instance.AIAuditResult,
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
