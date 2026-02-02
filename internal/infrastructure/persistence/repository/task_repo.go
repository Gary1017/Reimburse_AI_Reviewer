package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// ApprovalTaskRepository implements port.ApprovalTaskRepository
type ApprovalTaskRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewApprovalTaskRepository creates a new approval task repository
func NewApprovalTaskRepository(db *sql.DB, logger *zap.Logger) port.ApprovalTaskRepository {
	return &ApprovalTaskRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new approval task
func (r *ApprovalTaskRepository) Create(ctx context.Context, task *entity.ApprovalTask) error {
	query := `
		INSERT INTO approval_tasks (
			instance_id, lark_task_id, task_type, sequence_number,
			node_id, node_name, custom_node_id, approval_type,
			assignee_user_id, assignee_open_id,
			status, start_time, end_time,
			is_current, is_ai_decision,
			decision, confidence, result_data, violations, completed_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Handle nullable fields
	var larkTaskID, nodeID, nodeName, customNodeID, approvalType sql.NullString
	var assigneeUserID, assigneeOpenID sql.NullString
	var startTime, endTime sql.NullString
	var decision, resultData, violations, completedBy sql.NullString
	var confidence sql.NullFloat64

	if task.LarkTaskID != "" {
		larkTaskID = sql.NullString{String: task.LarkTaskID, Valid: true}
	}
	if task.NodeID != "" {
		nodeID = sql.NullString{String: task.NodeID, Valid: true}
	}
	if task.NodeName != "" {
		nodeName = sql.NullString{String: task.NodeName, Valid: true}
	}
	if task.CustomNodeID != "" {
		customNodeID = sql.NullString{String: task.CustomNodeID, Valid: true}
	}
	if task.ApprovalType != "" {
		approvalType = sql.NullString{String: task.ApprovalType, Valid: true}
	}
	if task.AssigneeUserID != "" {
		assigneeUserID = sql.NullString{String: task.AssigneeUserID, Valid: true}
	}
	if task.AssigneeOpenID != "" {
		assigneeOpenID = sql.NullString{String: task.AssigneeOpenID, Valid: true}
	}
	if task.StartTime != "" {
		startTime = sql.NullString{String: task.StartTime, Valid: true}
	}
	if task.EndTime != "" {
		endTime = sql.NullString{String: task.EndTime, Valid: true}
	}
	if task.Decision != "" {
		decision = sql.NullString{String: task.Decision, Valid: true}
	}
	if task.ResultData != "" {
		resultData = sql.NullString{String: task.ResultData, Valid: true}
	}
	if task.Violations != "" {
		violations = sql.NullString{String: task.Violations, Valid: true}
	}
	if task.CompletedBy != "" {
		completedBy = sql.NullString{String: task.CompletedBy, Valid: true}
	}
	if task.Confidence != nil {
		confidence = sql.NullFloat64{Float64: *task.Confidence, Valid: true}
	}

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		task.InstanceID,
		larkTaskID,
		task.TaskType,
		task.SequenceNumber,
		nodeID,
		nodeName,
		customNodeID,
		approvalType,
		assigneeUserID,
		assigneeOpenID,
		task.Status,
		startTime,
		endTime,
		task.IsCurrent,
		task.IsAIDecision,
		decision,
		confidence,
		resultData,
		violations,
		completedBy,
	)
	if err != nil {
		r.logger.Error("Failed to create approval task",
			zap.Int64("instance_id", task.InstanceID),
			zap.String("task_type", task.TaskType),
			zap.Error(err))
		return fmt.Errorf("failed to create approval task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	task.ID = id
	return nil
}

// GetByID retrieves a task by its ID
func (r *ApprovalTaskRepository) GetByID(ctx context.Context, id int64) (*entity.ApprovalTask, error) {
	query := `
		SELECT id, instance_id, lark_task_id, task_type, sequence_number,
			node_id, node_name, custom_node_id, approval_type,
			assignee_user_id, assignee_open_id,
			status, start_time, end_time,
			is_current, is_ai_decision,
			decision, confidence, result_data, violations, completed_by,
			created_at, updated_at
		FROM approval_tasks
		WHERE id = ?
	`

	task, err := r.scanTask(r.getExecutor(ctx).QueryRowContext(ctx, query, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get approval task by ID",
			zap.Int64("id", id),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get approval task: %w", err)
	}

	return task, nil
}

// GetByLarkTaskID retrieves a task by Lark's task ID
func (r *ApprovalTaskRepository) GetByLarkTaskID(ctx context.Context, larkTaskID string) (*entity.ApprovalTask, error) {
	query := `
		SELECT id, instance_id, lark_task_id, task_type, sequence_number,
			node_id, node_name, custom_node_id, approval_type,
			assignee_user_id, assignee_open_id,
			status, start_time, end_time,
			is_current, is_ai_decision,
			decision, confidence, result_data, violations, completed_by,
			created_at, updated_at
		FROM approval_tasks
		WHERE lark_task_id = ?
	`

	task, err := r.scanTask(r.getExecutor(ctx).QueryRowContext(ctx, query, larkTaskID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get approval task by Lark task ID",
			zap.String("lark_task_id", larkTaskID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get approval task: %w", err)
	}

	return task, nil
}

// GetByInstanceID retrieves all tasks for an instance ordered by sequence
func (r *ApprovalTaskRepository) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ApprovalTask, error) {
	query := `
		SELECT id, instance_id, lark_task_id, task_type, sequence_number,
			node_id, node_name, custom_node_id, approval_type,
			assignee_user_id, assignee_open_id,
			status, start_time, end_time,
			is_current, is_ai_decision,
			decision, confidence, result_data, violations, completed_by,
			created_at, updated_at
		FROM approval_tasks
		WHERE instance_id = ?
		ORDER BY sequence_number
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, instanceID)
	if err != nil {
		r.logger.Error("Failed to get approval tasks by instance ID",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get approval tasks: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// GetCurrentTask retrieves the current active task for an instance
func (r *ApprovalTaskRepository) GetCurrentTask(ctx context.Context, instanceID int64) (*entity.ApprovalTask, error) {
	query := `
		SELECT id, instance_id, lark_task_id, task_type, sequence_number,
			node_id, node_name, custom_node_id, approval_type,
			assignee_user_id, assignee_open_id,
			status, start_time, end_time,
			is_current, is_ai_decision,
			decision, confidence, result_data, violations, completed_by,
			created_at, updated_at
		FROM approval_tasks
		WHERE instance_id = ? AND is_current = TRUE
	`

	task, err := r.scanTask(r.getExecutor(ctx).QueryRowContext(ctx, query, instanceID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get current approval task",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get current approval task: %w", err)
	}

	return task, nil
}

// GetAIReviewTask retrieves the AI review task for an instance (sequence=0)
func (r *ApprovalTaskRepository) GetAIReviewTask(ctx context.Context, instanceID int64) (*entity.ApprovalTask, error) {
	query := `
		SELECT id, instance_id, lark_task_id, task_type, sequence_number,
			node_id, node_name, custom_node_id, approval_type,
			assignee_user_id, assignee_open_id,
			status, start_time, end_time,
			is_current, is_ai_decision,
			decision, confidence, result_data, violations, completed_by,
			created_at, updated_at
		FROM approval_tasks
		WHERE instance_id = ? AND task_type = 'AI_REVIEW' AND sequence_number = 0
	`

	task, err := r.scanTask(r.getExecutor(ctx).QueryRowContext(ctx, query, instanceID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get AI review task",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get AI review task: %w", err)
	}

	return task, nil
}

// Update updates an existing task
func (r *ApprovalTaskRepository) Update(ctx context.Context, task *entity.ApprovalTask) error {
	query := `
		UPDATE approval_tasks
		SET lark_task_id = ?, task_type = ?, sequence_number = ?,
			node_id = ?, node_name = ?, custom_node_id = ?, approval_type = ?,
			assignee_user_id = ?, assignee_open_id = ?,
			status = ?, start_time = ?, end_time = ?,
			is_current = ?, is_ai_decision = ?,
			decision = ?, confidence = ?, result_data = ?, violations = ?, completed_by = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	// Handle nullable fields
	var larkTaskID, nodeID, nodeName, customNodeID, approvalType sql.NullString
	var assigneeUserID, assigneeOpenID sql.NullString
	var startTime, endTime sql.NullString
	var decision, resultData, violations, completedBy sql.NullString
	var confidence sql.NullFloat64

	if task.LarkTaskID != "" {
		larkTaskID = sql.NullString{String: task.LarkTaskID, Valid: true}
	}
	if task.NodeID != "" {
		nodeID = sql.NullString{String: task.NodeID, Valid: true}
	}
	if task.NodeName != "" {
		nodeName = sql.NullString{String: task.NodeName, Valid: true}
	}
	if task.CustomNodeID != "" {
		customNodeID = sql.NullString{String: task.CustomNodeID, Valid: true}
	}
	if task.ApprovalType != "" {
		approvalType = sql.NullString{String: task.ApprovalType, Valid: true}
	}
	if task.AssigneeUserID != "" {
		assigneeUserID = sql.NullString{String: task.AssigneeUserID, Valid: true}
	}
	if task.AssigneeOpenID != "" {
		assigneeOpenID = sql.NullString{String: task.AssigneeOpenID, Valid: true}
	}
	if task.StartTime != "" {
		startTime = sql.NullString{String: task.StartTime, Valid: true}
	}
	if task.EndTime != "" {
		endTime = sql.NullString{String: task.EndTime, Valid: true}
	}
	if task.Decision != "" {
		decision = sql.NullString{String: task.Decision, Valid: true}
	}
	if task.ResultData != "" {
		resultData = sql.NullString{String: task.ResultData, Valid: true}
	}
	if task.Violations != "" {
		violations = sql.NullString{String: task.Violations, Valid: true}
	}
	if task.CompletedBy != "" {
		completedBy = sql.NullString{String: task.CompletedBy, Valid: true}
	}
	if task.Confidence != nil {
		confidence = sql.NullFloat64{Float64: *task.Confidence, Valid: true}
	}

	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		larkTaskID,
		task.TaskType,
		task.SequenceNumber,
		nodeID,
		nodeName,
		customNodeID,
		approvalType,
		assigneeUserID,
		assigneeOpenID,
		task.Status,
		startTime,
		endTime,
		task.IsCurrent,
		task.IsAIDecision,
		decision,
		confidence,
		resultData,
		violations,
		completedBy,
		task.ID,
	)
	if err != nil {
		r.logger.Error("Failed to update approval task",
			zap.Int64("id", task.ID),
			zap.Error(err))
		return fmt.Errorf("failed to update approval task: %w", err)
	}

	return nil
}

// UpdateStatus updates task status
func (r *ApprovalTaskRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := `UPDATE approval_tasks SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query, status, id)
	if err != nil {
		r.logger.Error("Failed to update approval task status",
			zap.Int64("id", id),
			zap.String("status", status),
			zap.Error(err))
		return fmt.Errorf("failed to update approval task status: %w", err)
	}

	return nil
}

// CompleteTask marks a task as completed with result
func (r *ApprovalTaskRepository) CompleteTask(ctx context.Context, id int64, decision string, confidence *float64, resultData string, violations string, completedBy string) error {
	query := `
		UPDATE approval_tasks
		SET decision = ?, confidence = ?, result_data = ?, violations = ?,
			completed_by = ?, status = 'COMPLETED', end_time = datetime('now'),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	var conf sql.NullFloat64
	if confidence != nil {
		conf = sql.NullFloat64{Float64: *confidence, Valid: true}
	}

	var decisionVal, resultDataVal, violationsVal, completedByVal sql.NullString
	if decision != "" {
		decisionVal = sql.NullString{String: decision, Valid: true}
	}
	if resultData != "" {
		resultDataVal = sql.NullString{String: resultData, Valid: true}
	}
	if violations != "" {
		violationsVal = sql.NullString{String: violations, Valid: true}
	}
	if completedBy != "" {
		completedByVal = sql.NullString{String: completedBy, Valid: true}
	}

	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		decisionVal,
		conf,
		resultDataVal,
		violationsVal,
		completedByVal,
		id,
	)
	if err != nil {
		r.logger.Error("Failed to complete approval task",
			zap.Int64("id", id),
			zap.String("decision", decision),
			zap.Error(err))
		return fmt.Errorf("failed to complete approval task: %w", err)
	}

	return nil
}

// SetCurrent sets a task as the current active task (and clears others)
func (r *ApprovalTaskRepository) SetCurrent(ctx context.Context, instanceID int64, taskID int64) error {
	// First, clear is_current for all tasks of this instance
	clearQuery := `UPDATE approval_tasks SET is_current = FALSE, updated_at = CURRENT_TIMESTAMP WHERE instance_id = ?`
	_, err := r.getExecutor(ctx).ExecContext(ctx, clearQuery, instanceID)
	if err != nil {
		r.logger.Error("Failed to clear current task",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		return fmt.Errorf("failed to clear current task: %w", err)
	}

	// Then, set is_current for the specified task
	setQuery := `UPDATE approval_tasks SET is_current = TRUE, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err = r.getExecutor(ctx).ExecContext(ctx, setQuery, taskID)
	if err != nil {
		r.logger.Error("Failed to set current task",
			zap.Int64("task_id", taskID),
			zap.Error(err))
		return fmt.Errorf("failed to set current task: %w", err)
	}

	return nil
}

// scanTask scans a single task row
func (r *ApprovalTaskRepository) scanTask(row *sql.Row) (*entity.ApprovalTask, error) {
	var task entity.ApprovalTask
	var larkTaskID, nodeID, nodeName, customNodeID, approvalType sql.NullString
	var assigneeUserID, assigneeOpenID sql.NullString
	var startTime, endTime sql.NullString
	var decision, resultData, violations, completedBy sql.NullString
	var confidence sql.NullFloat64

	err := row.Scan(
		&task.ID,
		&task.InstanceID,
		&larkTaskID,
		&task.TaskType,
		&task.SequenceNumber,
		&nodeID,
		&nodeName,
		&customNodeID,
		&approvalType,
		&assigneeUserID,
		&assigneeOpenID,
		&task.Status,
		&startTime,
		&endTime,
		&task.IsCurrent,
		&task.IsAIDecision,
		&decision,
		&confidence,
		&resultData,
		&violations,
		&completedBy,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Map nullable fields
	if larkTaskID.Valid {
		task.LarkTaskID = larkTaskID.String
	}
	if nodeID.Valid {
		task.NodeID = nodeID.String
	}
	if nodeName.Valid {
		task.NodeName = nodeName.String
	}
	if customNodeID.Valid {
		task.CustomNodeID = customNodeID.String
	}
	if approvalType.Valid {
		task.ApprovalType = approvalType.String
	}
	if assigneeUserID.Valid {
		task.AssigneeUserID = assigneeUserID.String
	}
	if assigneeOpenID.Valid {
		task.AssigneeOpenID = assigneeOpenID.String
	}
	if startTime.Valid {
		task.StartTime = startTime.String
	}
	if endTime.Valid {
		task.EndTime = endTime.String
	}
	if decision.Valid {
		task.Decision = decision.String
	}
	if confidence.Valid {
		task.Confidence = &confidence.Float64
	}
	if resultData.Valid {
		task.ResultData = resultData.String
	}
	if violations.Valid {
		task.Violations = violations.String
	}
	if completedBy.Valid {
		task.CompletedBy = completedBy.String
	}

	return &task, nil
}

// scanTasks scans multiple task rows
func (r *ApprovalTaskRepository) scanTasks(rows *sql.Rows) ([]*entity.ApprovalTask, error) {
	var tasks []*entity.ApprovalTask

	for rows.Next() {
		var task entity.ApprovalTask
		var larkTaskID, nodeID, nodeName, customNodeID, approvalType sql.NullString
		var assigneeUserID, assigneeOpenID sql.NullString
		var startTime, endTime sql.NullString
		var decision, resultData, violations, completedBy sql.NullString
		var confidence sql.NullFloat64

		err := rows.Scan(
			&task.ID,
			&task.InstanceID,
			&larkTaskID,
			&task.TaskType,
			&task.SequenceNumber,
			&nodeID,
			&nodeName,
			&customNodeID,
			&approvalType,
			&assigneeUserID,
			&assigneeOpenID,
			&task.Status,
			&startTime,
			&endTime,
			&task.IsCurrent,
			&task.IsAIDecision,
			&decision,
			&confidence,
			&resultData,
			&violations,
			&completedBy,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan approval task: %w", err)
		}

		// Map nullable fields
		if larkTaskID.Valid {
			task.LarkTaskID = larkTaskID.String
		}
		if nodeID.Valid {
			task.NodeID = nodeID.String
		}
		if nodeName.Valid {
			task.NodeName = nodeName.String
		}
		if customNodeID.Valid {
			task.CustomNodeID = customNodeID.String
		}
		if approvalType.Valid {
			task.ApprovalType = approvalType.String
		}
		if assigneeUserID.Valid {
			task.AssigneeUserID = assigneeUserID.String
		}
		if assigneeOpenID.Valid {
			task.AssigneeOpenID = assigneeOpenID.String
		}
		if startTime.Valid {
			task.StartTime = startTime.String
		}
		if endTime.Valid {
			task.EndTime = endTime.String
		}
		if decision.Valid {
			task.Decision = decision.String
		}
		if confidence.Valid {
			task.Confidence = &confidence.Float64
		}
		if resultData.Valid {
			task.ResultData = resultData.String
		}
		if violations.Valid {
			task.Violations = violations.String
		}
		if completedBy.Valid {
			task.CompletedBy = completedBy.String
		}

		tasks = append(tasks, &task)
	}

	return tasks, rows.Err()
}

// getExecutor returns appropriate executor based on context
func (r *ApprovalTaskRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// Verify interface compliance
var _ port.ApprovalTaskRepository = (*ApprovalTaskRepository)(nil)
