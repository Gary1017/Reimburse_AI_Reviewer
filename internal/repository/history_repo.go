package repository

import (
	"database/sql"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// HistoryRepository handles approval history database operations
type HistoryRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewHistoryRepository creates a new history repository
func NewHistoryRepository(db *sql.DB, logger *zap.Logger) *HistoryRepository {
	return &HistoryRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new history record
func (r *HistoryRepository) Create(tx *sql.Tx, history *models.ApprovalHistory) error {
	query := `
		INSERT INTO approval_history (
			instance_id, reviewer_user_id, previous_status, new_status,
			action_type, action_data
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	var result sql.Result
	var err error

	if tx != nil {
		result, err = tx.Exec(query,
			history.InstanceID,
			history.ReviewerUserID,
			history.PreviousStatus,
			history.NewStatus,
			history.ActionType,
			history.ActionData,
		)
	} else {
		result, err = r.db.Exec(query,
			history.InstanceID,
			history.ReviewerUserID,
			history.PreviousStatus,
			history.NewStatus,
			history.ActionType,
			history.ActionData,
		)
	}

	if err != nil {
		r.logger.Error("Failed to create history record", zap.Error(err))
		return fmt.Errorf("failed to create history: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	history.ID = id
	return nil
}

// GetByInstanceID retrieves all history records for an instance
func (r *HistoryRepository) GetByInstanceID(instanceID int64) ([]*models.ApprovalHistory, error) {
	query := `
		SELECT id, instance_id, reviewer_user_id, previous_status, new_status,
			action_type, action_data, timestamp
		FROM approval_history
		WHERE instance_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := r.db.Query(query, instanceID)
	if err != nil {
		r.logger.Error("Failed to get history by instance ID", zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	defer rows.Close()

	var records []*models.ApprovalHistory
	for rows.Next() {
		var record models.ApprovalHistory
		err := rows.Scan(
			&record.ID,
			&record.InstanceID,
			&record.ReviewerUserID,
			&record.PreviousStatus,
			&record.NewStatus,
			&record.ActionType,
			&record.ActionData,
			&record.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history record: %w", err)
		}
		records = append(records, &record)
	}

	return records, rows.Err()
}
