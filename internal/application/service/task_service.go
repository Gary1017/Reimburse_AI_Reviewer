package service

import (
	"context"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// LarkTask represents a task from Lark's task_list API response.
// Used for syncing tasks from Lark into our system.
type LarkTask struct {
	ID           string
	UserID       string
	OpenID       string
	Status       string
	NodeID       string
	NodeName     string
	CustomNodeID string
	Type         string
	StartTime    string
	EndTime      string
}

// TaskService manages approval task operations.
// Tasks are the unified model for both AI review and human approval.
type TaskService interface {
	// CreateAIReviewTask creates an AI review task for an instance
	CreateAIReviewTask(ctx context.Context, instanceID int64, assigneeUserID string) (*entity.ApprovalTask, error)

	// CreateHumanReviewTask creates a human review task from Lark
	CreateHumanReviewTask(ctx context.Context, instanceID int64, larkTaskID, nodeID, nodeName, assigneeUserID, assigneeOpenID string) (*entity.ApprovalTask, error)

	// CompleteTask marks a task as completed with result
	CompleteTask(ctx context.Context, taskID int64, decision string, confidence *float64, resultData, violations, completedBy string) error

	// GetTasksForInstance retrieves all tasks for an instance
	GetTasksForInstance(ctx context.Context, instanceID int64) ([]*entity.ApprovalTask, error)

	// GetCurrentTask retrieves the current active task for an instance
	GetCurrentTask(ctx context.Context, instanceID int64) (*entity.ApprovalTask, error)

	// GetAIReviewTask retrieves the AI review task for an instance
	GetAIReviewTask(ctx context.Context, instanceID int64) (*entity.ApprovalTask, error)

	// GetByID retrieves a task by its ID
	GetByID(ctx context.Context, taskID int64) (*entity.ApprovalTask, error)

	// SyncLarkTasks syncs tasks from Lark API into our system
	SyncLarkTasks(ctx context.Context, instanceID int64, larkTasks []LarkTask) error

	// SetCurrentTask sets a task as the current active task
	SetCurrentTask(ctx context.Context, instanceID int64, taskID int64) error

	// UpdateTaskStatus updates the status of a task
	UpdateTaskStatus(ctx context.Context, taskID int64, status string) error
}

type taskServiceImpl struct {
	taskRepo                   port.ApprovalTaskRepository
	reviewNotificationRepo     port.ReviewNotificationRepository
	txManager                  port.TransactionManager
	logger                     Logger
}

// NewTaskService creates a new TaskService
func NewTaskService(
	taskRepo port.ApprovalTaskRepository,
	reviewNotificationRepo port.ReviewNotificationRepository,
	txManager port.TransactionManager,
	logger Logger,
) TaskService {
	return &taskServiceImpl{
		taskRepo:               taskRepo,
		reviewNotificationRepo: reviewNotificationRepo,
		txManager:              txManager,
		logger:                 logger,
	}
}

// CreateAIReviewTask creates an AI review task for an instance.
// AI review tasks have sequence_number=0 and is_ai_decision=true.
func (s *taskServiceImpl) CreateAIReviewTask(ctx context.Context, instanceID int64, assigneeUserID string) (*entity.ApprovalTask, error) {
	// Check if AI review task already exists (idempotency)
	existing, err := s.taskRepo.GetAIReviewTask(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to check existing AI review task",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("check existing AI review task: %w", err)
	}
	if existing != nil {
		s.logger.Info("AI review task already exists",
			"instance_id", instanceID,
			"task_id", existing.ID)
		return existing, nil
	}

	// Create new AI review task
	task := &entity.ApprovalTask{
		InstanceID:     instanceID,
		TaskType:       entity.TaskTypeAIReview,
		SequenceNumber: 0, // AI review is always first
		Status:         entity.TaskStatusPending,
		IsCurrent:      true, // AI review starts as current
		IsAIDecision:   true,
		AssigneeUserID: assigneeUserID,
		StartTime:      time.Now().Format(time.RFC3339),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		s.logger.Error("Failed to create AI review task",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("create AI review task: %w", err)
	}

	s.logger.Info("AI review task created",
		"instance_id", instanceID,
		"task_id", task.ID)

	return task, nil
}

// CreateHumanReviewTask creates a human review task from Lark.
func (s *taskServiceImpl) CreateHumanReviewTask(ctx context.Context, instanceID int64, larkTaskID, nodeID, nodeName, assigneeUserID, assigneeOpenID string) (*entity.ApprovalTask, error) {
	// Check if task already exists by Lark task ID (idempotency)
	existing, err := s.taskRepo.GetByLarkTaskID(ctx, larkTaskID)
	if err != nil {
		s.logger.Error("Failed to check existing human review task",
			"error", err,
			"lark_task_id", larkTaskID)
		return nil, fmt.Errorf("check existing task: %w", err)
	}
	if existing != nil {
		s.logger.Info("Human review task already exists",
			"lark_task_id", larkTaskID,
			"task_id", existing.ID)
		return existing, nil
	}

	// Get existing tasks to determine sequence number
	existingTasks, err := s.taskRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get existing tasks",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("get existing tasks: %w", err)
	}

	// Sequence number is the count of existing tasks (AI task is 0)
	seqNum := len(existingTasks)

	// Create new human review task
	task := &entity.ApprovalTask{
		InstanceID:     instanceID,
		LarkTaskID:     larkTaskID,
		TaskType:       entity.TaskTypeHumanReview,
		SequenceNumber: seqNum,
		NodeID:         nodeID,
		NodeName:       nodeName,
		AssigneeUserID: assigneeUserID,
		AssigneeOpenID: assigneeOpenID,
		Status:         entity.TaskStatusPending,
		IsCurrent:      false,
		IsAIDecision:   false,
		StartTime:      time.Now().Format(time.RFC3339),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		s.logger.Error("Failed to create human review task",
			"error", err,
			"instance_id", instanceID,
			"lark_task_id", larkTaskID)
		return nil, fmt.Errorf("create human review task: %w", err)
	}

	s.logger.Info("Human review task created",
		"instance_id", instanceID,
		"task_id", task.ID,
		"lark_task_id", larkTaskID,
		"node_name", nodeName)

	return task, nil
}

// CompleteTask marks a task as completed with result.
func (s *taskServiceImpl) CompleteTask(ctx context.Context, taskID int64, decision string, confidence *float64, resultData, violations, completedBy string) error {
	if err := s.taskRepo.CompleteTask(ctx, taskID, decision, confidence, resultData, violations, completedBy); err != nil {
		s.logger.Error("Failed to complete task",
			"error", err,
			"task_id", taskID,
			"decision", decision)
		return fmt.Errorf("complete task: %w", err)
	}

	s.logger.Info("Task completed",
		"task_id", taskID,
		"decision", decision,
		"completed_by", completedBy)

	return nil
}

// GetTasksForInstance retrieves all tasks for an instance.
func (s *taskServiceImpl) GetTasksForInstance(ctx context.Context, instanceID int64) ([]*entity.ApprovalTask, error) {
	tasks, err := s.taskRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get tasks for instance",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("get tasks: %w", err)
	}
	return tasks, nil
}

// GetCurrentTask retrieves the current active task for an instance.
func (s *taskServiceImpl) GetCurrentTask(ctx context.Context, instanceID int64) (*entity.ApprovalTask, error) {
	task, err := s.taskRepo.GetCurrentTask(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get current task",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("get current task: %w", err)
	}
	return task, nil
}

// GetAIReviewTask retrieves the AI review task for an instance.
func (s *taskServiceImpl) GetAIReviewTask(ctx context.Context, instanceID int64) (*entity.ApprovalTask, error) {
	task, err := s.taskRepo.GetAIReviewTask(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get AI review task",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("get AI review task: %w", err)
	}
	return task, nil
}

// GetByID retrieves a task by its ID.
func (s *taskServiceImpl) GetByID(ctx context.Context, taskID int64) (*entity.ApprovalTask, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		s.logger.Error("Failed to get task by ID",
			"error", err,
			"task_id", taskID)
		return nil, fmt.Errorf("get task: %w", err)
	}
	return task, nil
}

// SyncLarkTasks syncs tasks from Lark API into our system.
// This creates or updates human review tasks based on Lark's task_list.
func (s *taskServiceImpl) SyncLarkTasks(ctx context.Context, instanceID int64, larkTasks []LarkTask) error {
	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		for _, lt := range larkTasks {
			// Check if task already exists
			existing, err := s.taskRepo.GetByLarkTaskID(txCtx, lt.ID)
			if err != nil {
				return fmt.Errorf("check existing task %s: %w", lt.ID, err)
			}

			if existing != nil {
				// Update existing task status if changed
				if existing.Status != lt.Status {
					if err := s.taskRepo.UpdateStatus(txCtx, existing.ID, lt.Status); err != nil {
						return fmt.Errorf("update task status %s: %w", lt.ID, err)
					}
				}
				continue
			}

			// Create new task
			_, err = s.CreateHumanReviewTask(txCtx, instanceID, lt.ID, lt.NodeID, lt.NodeName, lt.UserID, lt.OpenID)
			if err != nil {
				return fmt.Errorf("create task %s: %w", lt.ID, err)
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("Failed to sync Lark tasks",
			"error", err,
			"instance_id", instanceID,
			"task_count", len(larkTasks))
		return err
	}

	s.logger.Info("Lark tasks synced",
		"instance_id", instanceID,
		"task_count", len(larkTasks))

	return nil
}

// SetCurrentTask sets a task as the current active task.
func (s *taskServiceImpl) SetCurrentTask(ctx context.Context, instanceID int64, taskID int64) error {
	if err := s.taskRepo.SetCurrent(ctx, instanceID, taskID); err != nil {
		s.logger.Error("Failed to set current task",
			"error", err,
			"instance_id", instanceID,
			"task_id", taskID)
		return fmt.Errorf("set current task: %w", err)
	}

	s.logger.Info("Current task set",
		"instance_id", instanceID,
		"task_id", taskID)

	return nil
}

// UpdateTaskStatus updates the status of a task.
func (s *taskServiceImpl) UpdateTaskStatus(ctx context.Context, taskID int64, status string) error {
	if err := s.taskRepo.UpdateStatus(ctx, taskID, status); err != nil {
		s.logger.Error("Failed to update task status",
			"error", err,
			"task_id", taskID,
			"status", status)
		return fmt.Errorf("update task status: %w", err)
	}

	s.logger.Info("Task status updated",
		"task_id", taskID,
		"status", status)

	return nil
}
