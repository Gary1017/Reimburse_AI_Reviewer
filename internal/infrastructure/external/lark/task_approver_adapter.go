package lark

import (
	"context"

	"go.uber.org/zap"
)

// TaskApproverAdapter implements port.LarkTaskApprover by delegating to TaskAPI
type TaskApproverAdapter struct {
	taskAPI      *TaskAPI
	approvalCode string
	logger       *zap.Logger
}

// NewTaskApproverAdapter creates a new task approver adapter
func NewTaskApproverAdapter(taskAPI *TaskAPI, approvalCode string, logger *zap.Logger) *TaskApproverAdapter {
	return &TaskApproverAdapter{
		taskAPI:      taskAPI,
		approvalCode: approvalCode,
		logger:       logger,
	}
}

// ApproveTask approves a task using the pre-configured approval code
func (a *TaskApproverAdapter) ApproveTask(ctx context.Context, approvalCode, instanceCode, userID, taskID, comment string) error {
	// Use the adapter's approval code if the caller passes empty
	code := approvalCode
	if code == "" {
		code = a.approvalCode
	}
	return a.taskAPI.ApproveTask(ctx, code, instanceCode, userID, taskID, comment)
}

// RejectTask rejects a task using the pre-configured approval code
func (a *TaskApproverAdapter) RejectTask(ctx context.Context, approvalCode, instanceCode, userID, taskID, comment string) error {
	code := approvalCode
	if code == "" {
		code = a.approvalCode
	}
	return a.taskAPI.RejectTask(ctx, code, instanceCode, userID, taskID, comment)
}
