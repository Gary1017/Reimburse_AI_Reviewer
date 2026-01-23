package service

import (
	"context"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// Logger interface for minimal logging dependency
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// ApprovalService manages approval instances
type ApprovalService interface {
	CreateInstance(ctx context.Context, larkInstanceID string, formData map[string]interface{}) (*entity.ApprovalInstance, error)
	GetInstance(ctx context.Context, id int64) (*entity.ApprovalInstance, error)
	GetInstanceByLarkID(ctx context.Context, larkInstanceID string) (*entity.ApprovalInstance, error)
	UpdateStatus(ctx context.Context, id int64, status string, actionData interface{}) error
	SetApprovalTime(ctx context.Context, id int64) error
	ListInstances(ctx context.Context, limit, offset int) ([]*entity.ApprovalInstance, error)
}

type approvalServiceImpl struct {
	instanceRepo port.InstanceRepository
	itemRepo     port.ItemRepository
	historyRepo  port.HistoryRepository
	txManager    port.TransactionManager
	logger       Logger
}

// NewApprovalService creates a new ApprovalService
func NewApprovalService(
	instanceRepo port.InstanceRepository,
	itemRepo port.ItemRepository,
	historyRepo port.HistoryRepository,
	txManager port.TransactionManager,
	logger Logger,
) ApprovalService {
	return &approvalServiceImpl{
		instanceRepo: instanceRepo,
		itemRepo:     itemRepo,
		historyRepo:  historyRepo,
		txManager:    txManager,
		logger:       logger,
	}
}

// CreateInstance creates a new approval instance
func (s *approvalServiceImpl) CreateInstance(ctx context.Context, larkInstanceID string, formData map[string]interface{}) (*entity.ApprovalInstance, error) {
	// Check if instance already exists (idempotency)
	existing, err := s.instanceRepo.GetByLarkInstanceID(ctx, larkInstanceID)
	if err == nil && existing != nil {
		s.logger.Info("Instance already exists", "lark_instance_id", larkInstanceID, "id", existing.ID)
		return existing, nil
	}

	// Create new instance
	instance := &entity.ApprovalInstance{
		LarkInstanceID: larkInstanceID,
		Status:         "CREATED",
		FormData:       marshalFormData(formData),
		SubmissionTime: time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Extract applicant info if available
	if userID, ok := formData["applicant_user_id"].(string); ok {
		instance.ApplicantUserID = userID
	}
	if dept, ok := formData["department"].(string); ok {
		instance.Department = dept
	}

	// Create instance and history in transaction
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.instanceRepo.Create(txCtx, instance); err != nil {
			return fmt.Errorf("create instance: %w", err)
		}

		// Create initial history entry
		history := &entity.ApprovalHistory{
			InstanceID:     instance.ID,
			ReviewerUserID: instance.ApplicantUserID,
			PreviousStatus: "",
			NewStatus:      "CREATED",
			ActionType:     "CREATE",
			ActionData:     "Instance created",
			Timestamp:      time.Now(),
		}

		if err := s.historyRepo.Create(txCtx, history); err != nil {
			return fmt.Errorf("create history: %w", err)
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Failed to create instance", "error", err, "lark_instance_id", larkInstanceID)
		return nil, err
	}

	s.logger.Info("Instance created", "id", instance.ID, "lark_instance_id", larkInstanceID)
	return instance, nil
}

// GetInstance retrieves an instance by ID
func (s *approvalServiceImpl) GetInstance(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
	instance, err := s.instanceRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get instance", "error", err, "id", id)
		return nil, err
	}
	return instance, nil
}

// GetInstanceByLarkID retrieves an instance by Lark instance ID
func (s *approvalServiceImpl) GetInstanceByLarkID(ctx context.Context, larkInstanceID string) (*entity.ApprovalInstance, error) {
	instance, err := s.instanceRepo.GetByLarkInstanceID(ctx, larkInstanceID)
	if err != nil {
		s.logger.Error("Failed to get instance by Lark ID", "error", err, "lark_instance_id", larkInstanceID)
		return nil, err
	}
	return instance, nil
}

// UpdateStatus updates the instance status
func (s *approvalServiceImpl) UpdateStatus(ctx context.Context, id int64, status string, actionData interface{}) error {
	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Update instance status
		if err := s.instanceRepo.UpdateStatus(txCtx, id, status); err != nil {
			return fmt.Errorf("update status: %w", err)
		}

		// Get current instance to know previous status
		instance, err := s.instanceRepo.GetByID(txCtx, id)
		if err != nil {
			return fmt.Errorf("get instance: %w", err)
		}

		// Create history entry
		history := &entity.ApprovalHistory{
			InstanceID:     id,
			PreviousStatus: instance.Status,
			NewStatus:      status,
			ActionType:     "UPDATE_STATUS",
			Timestamp:      time.Now(),
		}

		// Extract action details if available
		if data, ok := actionData.(map[string]interface{}); ok {
			if actionBy, ok := data["action_by"].(string); ok {
				history.ReviewerUserID = actionBy
			}
			if comment, ok := data["comment"].(string); ok {
				history.ActionData = comment
			}
		}

		if err := s.historyRepo.Create(txCtx, history); err != nil {
			return fmt.Errorf("create history: %w", err)
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Failed to update status", "error", err, "id", id, "status", status)
		return err
	}

	s.logger.Info("Status updated", "id", id, "status", status)
	return nil
}

// SetApprovalTime sets the approval time for an instance
func (s *approvalServiceImpl) SetApprovalTime(ctx context.Context, id int64) error {
	err := s.instanceRepo.SetApprovalTime(ctx, id, time.Now())
	if err != nil {
		s.logger.Error("Failed to set approval time", "error", err, "id", id)
		return err
	}

	s.logger.Info("Approval time set", "id", id)
	return nil
}

// ListInstances retrieves a paginated list of instances
func (s *approvalServiceImpl) ListInstances(ctx context.Context, limit, offset int) ([]*entity.ApprovalInstance, error) {
	instances, err := s.instanceRepo.List(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Failed to list instances", "error", err, "limit", limit, "offset", offset)
		return nil, err
	}
	return instances, nil
}

// marshalFormData converts form data to JSON string
func marshalFormData(data map[string]interface{}) string {
	// Simple implementation - in production, use json.Marshal
	return fmt.Sprintf("%v", data)
}
