package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// HistoryRepository implements port.HistoryRepository
type HistoryRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewHistoryRepository creates a new history repository
func NewHistoryRepository(db *sql.DB, logger *zap.Logger) port.HistoryRepository {
	return &HistoryRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new history record
func (r *HistoryRepository) Create(ctx context.Context, history *entity.ApprovalHistory) error {
	query := `
		INSERT INTO approval_history (
			instance_id, reviewer_user_id, previous_status, new_status,
			action_type, action_data
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		history.InstanceID,
		history.ReviewerUserID,
		history.PreviousStatus,
		history.NewStatus,
		history.ActionType,
		history.ActionData,
	)
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
func (r *HistoryRepository) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ApprovalHistory, error) {
	query := `
		SELECT id, instance_id, reviewer_user_id, previous_status, new_status,
			action_type, action_data, timestamp
		FROM approval_history
		WHERE instance_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, instanceID)
	if err != nil {
		r.logger.Error("Failed to get history by instance ID", zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	defer rows.Close()

	var records []*entity.ApprovalHistory
	for rows.Next() {
		var record entity.ApprovalHistory
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

// getExecutor returns appropriate executor based on context
func (r *HistoryRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// Verify interface compliance
var _ port.HistoryRepository = (*HistoryRepository)(nil)
