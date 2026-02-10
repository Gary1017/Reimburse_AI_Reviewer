package service

import (
	"context"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// TaskEventHandler handles task events and DataReady events to trigger AI review.
// It coordinates the two-path trigger:
//   - Path A: Lark task event arrives first (AI approver's task is created) -> waits for data
//   - Path B: DataReady event arrives first (all invoices extracted/matched) -> waits for task
//
// Review starts only when BOTH conditions are met.
type TaskEventHandler interface {
	HandleTaskEvent(ctx context.Context, larkInstanceID string, payload map[string]interface{}) error
	HandleDataReady(ctx context.Context, instanceID int64) error
}

type taskEventHandlerImpl struct {
	instanceRepo     port.InstanceRepository
	taskRepo         port.ApprovalTaskRepository
	reviewService    ReviewService
	aiApproverOpenID string
	logger           Logger
}

// NewTaskEventHandler creates a new task event handler
func NewTaskEventHandler(
	instanceRepo port.InstanceRepository,
	taskRepo port.ApprovalTaskRepository,
	reviewService ReviewService,
	aiApproverOpenID string,
	logger Logger,
) TaskEventHandler {
	return &taskEventHandlerImpl{
		instanceRepo:     instanceRepo,
		taskRepo:         taskRepo,
		reviewService:    reviewService,
		aiApproverOpenID: aiApproverOpenID,
		logger:           logger,
	}
}

// HandleTaskEvent handles a Lark approval task event (approval_task).
// If the task belongs to the AI approver, it updates the AI review task's LarkTaskID.
// If data is already ready (DATA_READY state), it triggers review immediately.
func (h *taskEventHandlerImpl) HandleTaskEvent(ctx context.Context, larkInstanceID string, payload map[string]interface{}) error {
	// Extract fields from payload
	openID, _ := payload["open_id"].(string)
	if openID == "" {
		// Try user_id extraction from nested structure
		if userInfo, ok := payload["user_id_list"].([]interface{}); ok && len(userInfo) > 0 {
			openID, _ = userInfo[0].(string)
		}
	}

	// Only handle AI approver's task events
	if openID != h.aiApproverOpenID {
		h.logger.Info("Ignoring task event for non-AI approver",
			"open_id", openID, "lark_instance_id", larkInstanceID)
		return nil
	}

	taskID, _ := payload["task_id"].(string)
	if taskID == "" {
		h.logger.Info("Task event missing task_id", "lark_instance_id", larkInstanceID)
		return nil
	}

	h.logger.Info("Handling AI approver task event",
		"lark_instance_id", larkInstanceID,
		"task_id", taskID)

	// Find instance by Lark ID
	instance, err := h.instanceRepo.GetByLarkInstanceID(ctx, larkInstanceID)
	if err != nil {
		return fmt.Errorf("get instance by lark ID %s: %w", larkInstanceID, err)
	}
	if instance == nil {
		h.logger.Info("Instance not found for lark ID, may not be created yet",
			"lark_instance_id", larkInstanceID)
		return nil
	}

	// Get AI review task
	aiTask, err := h.taskRepo.GetAIReviewTask(ctx, instance.ID)
	if err != nil {
		return fmt.Errorf("get AI review task: %w", err)
	}
	if aiTask == nil {
		h.logger.Info("No AI review task found", "instance_id", instance.ID)
		return nil
	}

	// Update LarkTaskID if not set
	if aiTask.LarkTaskID == "" {
		aiTask.LarkTaskID = taskID
		if err := h.taskRepo.Update(ctx, aiTask); err != nil {
			return fmt.Errorf("update AI task lark_task_id: %w", err)
		}
		h.logger.Info("Set LarkTaskID on AI review task",
			"task_id", aiTask.ID, "lark_task_id", taskID)
	}

	// Check if data is ready -> trigger review
	if instance.Status == entity.StatusDataReady {
		h.logger.Info("Data already ready, triggering review",
			"instance_id", instance.ID)
		return h.triggerReview(ctx, instance.ID)
	}

	h.logger.Info("Data not ready yet, review will be triggered by HandleDataReady",
		"instance_id", instance.ID, "status", instance.Status)
	return nil
}

// HandleDataReady handles a DataReady event.
// If the AI review task already has a LarkTaskID (task event received), triggers review.
func (h *taskEventHandlerImpl) HandleDataReady(ctx context.Context, instanceID int64) error {
	h.logger.Info("Handling DataReady for review trigger", "instance_id", instanceID)

	// Get AI review task
	aiTask, err := h.taskRepo.GetAIReviewTask(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("get AI review task: %w", err)
	}
	if aiTask == nil {
		h.logger.Info("No AI review task found", "instance_id", instanceID)
		return nil
	}

	// Check if task event has been received (LarkTaskID is set)
	if aiTask.LarkTaskID != "" {
		h.logger.Info("Task event already received, triggering review",
			"instance_id", instanceID, "lark_task_id", aiTask.LarkTaskID)
		return h.triggerReview(ctx, instanceID)
	}

	h.logger.Info("Task event not received yet, review will be triggered by HandleTaskEvent",
		"instance_id", instanceID)
	return nil
}

// triggerReview starts the AI review process
func (h *taskEventHandlerImpl) triggerReview(ctx context.Context, instanceID int64) error {
	decision, err := h.reviewService.ReviewInstance(ctx, instanceID)
	if err != nil {
		h.logger.Error("Review failed", "error", err, "instance_id", instanceID)
		return fmt.Errorf("review instance %d: %w", instanceID, err)
	}

	h.logger.Info("Review completed",
		"instance_id", instanceID,
		"decision", decision.Decision,
		"confidence", decision.Confidence)
	return nil
}
