package lark

import (
	"context"
	"fmt"

	larkApproval "github.com/larksuite/oapi-sdk-go/v3/service/approval/v4"
	"go.uber.org/zap"
)

// TaskAPI handles Lark approval task operations (approve/reject)
type TaskAPI struct {
	client *SDKClient
	logger *zap.Logger
}

// NewTaskAPI creates a new task API handler
func NewTaskAPI(client *SDKClient, logger *zap.Logger) *TaskAPI {
	return &TaskAPI{
		client: client,
		logger: logger,
	}
}

// ApproveTask approves a Lark approval task
func (a *TaskAPI) ApproveTask(ctx context.Context, approvalCode, instanceCode, userID, taskID, comment string) error {
	a.logger.Info("Approving task",
		zap.String("approval_code", approvalCode),
		zap.String("instance_code", instanceCode),
		zap.String("user_id", userID),
		zap.String("task_id", taskID))

	taskApprove := larkApproval.NewTaskApproveBuilder().
		ApprovalCode(approvalCode).
		InstanceCode(instanceCode).
		UserId(userID).
		TaskId(taskID).
		Comment(comment).
		Build()

	req := larkApproval.NewApproveTaskReqBuilder().
		TaskApprove(taskApprove).
		Build()

	resp, err := a.client.client.Approval.Task.Approve(ctx, req)
	if err != nil {
		a.logger.Error("Failed to approve task",
			zap.String("task_id", taskID),
			zap.String("instance_code", instanceCode),
			zap.Error(err))
		return fmt.Errorf("failed to approve task: %w", err)
	}

	if !resp.Success() {
		a.logger.Error("Approve task API returned failure",
			zap.String("task_id", taskID),
			zap.Int("code", resp.Code),
			zap.String("msg", resp.Msg))
		return fmt.Errorf("approve task failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	a.logger.Info("Task approved successfully",
		zap.String("task_id", taskID),
		zap.String("instance_code", instanceCode))
	return nil
}

// RejectTask rejects a Lark approval task
func (a *TaskAPI) RejectTask(ctx context.Context, approvalCode, instanceCode, userID, taskID, comment string) error {
	a.logger.Info("Rejecting task",
		zap.String("approval_code", approvalCode),
		zap.String("instance_code", instanceCode),
		zap.String("user_id", userID),
		zap.String("task_id", taskID))

	taskApprove := larkApproval.NewTaskApproveBuilder().
		ApprovalCode(approvalCode).
		InstanceCode(instanceCode).
		UserId(userID).
		TaskId(taskID).
		Comment(comment).
		Build()

	req := larkApproval.NewRejectTaskReqBuilder().
		TaskApprove(taskApprove).
		Build()

	resp, err := a.client.client.Approval.Task.Reject(ctx, req)
	if err != nil {
		a.logger.Error("Failed to reject task",
			zap.String("task_id", taskID),
			zap.String("instance_code", instanceCode),
			zap.Error(err))
		return fmt.Errorf("failed to reject task: %w", err)
	}

	if !resp.Success() {
		a.logger.Error("Reject task API returned failure",
			zap.String("task_id", taskID),
			zap.Int("code", resp.Code),
			zap.String("msg", resp.Msg))
		return fmt.Errorf("reject task failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	a.logger.Info("Task rejected successfully",
		zap.String("task_id", taskID),
		zap.String("instance_code", instanceCode))
	return nil
}
