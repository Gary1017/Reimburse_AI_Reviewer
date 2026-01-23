package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// Mock repositories
type mockInstanceRepo struct {
	createFunc            func(ctx context.Context, instance *entity.ApprovalInstance) error
	getByIDFunc           func(ctx context.Context, id int64) (*entity.ApprovalInstance, error)
	getByLarkInstanceIDFunc func(ctx context.Context, larkID string) (*entity.ApprovalInstance, error)
	updateStatusFunc      func(ctx context.Context, id int64, status string) error
	setApprovalTimeFunc   func(ctx context.Context, id int64, t time.Time) error
	listFunc              func(ctx context.Context, limit, offset int) ([]*entity.ApprovalInstance, error)
}

func (m *mockInstanceRepo) Create(ctx context.Context, instance *entity.ApprovalInstance) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, instance)
	}
	instance.ID = 1 // Set ID for created instance
	return nil
}

func (m *mockInstanceRepo) GetByID(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return &entity.ApprovalInstance{ID: id, Status: "CREATED"}, nil
}

func (m *mockInstanceRepo) GetByLarkInstanceID(ctx context.Context, larkID string) (*entity.ApprovalInstance, error) {
	if m.getByLarkInstanceIDFunc != nil {
		return m.getByLarkInstanceIDFunc(ctx, larkID)
	}
	return nil, errors.New("not found")
}

func (m *mockInstanceRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	if m.updateStatusFunc != nil {
		return m.updateStatusFunc(ctx, id, status)
	}
	return nil
}

func (m *mockInstanceRepo) SetApprovalTime(ctx context.Context, id int64, t time.Time) error {
	if m.setApprovalTimeFunc != nil {
		return m.setApprovalTimeFunc(ctx, id, t)
	}
	return nil
}

func (m *mockInstanceRepo) List(ctx context.Context, limit, offset int) ([]*entity.ApprovalInstance, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, limit, offset)
	}
	return []*entity.ApprovalInstance{}, nil
}

type mockItemRepo struct{
	getByInstanceIDFunc func(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error)
}

func (m *mockItemRepo) Create(ctx context.Context, item *entity.ReimbursementItem) error {
	return nil
}

func (m *mockItemRepo) GetByID(ctx context.Context, id int64) (*entity.ReimbursementItem, error) {
	return &entity.ReimbursementItem{ID: id}, nil
}

func (m *mockItemRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error) {
	if m.getByInstanceIDFunc != nil {
		return m.getByInstanceIDFunc(ctx, instanceID)
	}
	return []*entity.ReimbursementItem{}, nil
}

func (m *mockItemRepo) Update(ctx context.Context, item *entity.ReimbursementItem) error {
	return nil
}

type mockHistoryRepo struct {
	createFunc func(ctx context.Context, history *entity.ApprovalHistory) error
}

func (m *mockHistoryRepo) Create(ctx context.Context, history *entity.ApprovalHistory) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, history)
	}
	return nil
}

func (m *mockHistoryRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ApprovalHistory, error) {
	return []*entity.ApprovalHistory{}, nil
}

type mockTxManager struct {
	withTransactionFunc func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.withTransactionFunc != nil {
		return m.withTransactionFunc(ctx, fn)
	}
	return fn(ctx)
}

type mockLogger struct{}

func (m *mockLogger) Info(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {}

func TestApprovalService_CreateInstance(t *testing.T) {
	tests := []struct {
		name           string
		larkInstanceID string
		formData       map[string]interface{}
		existingFunc   func(ctx context.Context, larkID string) (*entity.ApprovalInstance, error)
		wantErr        bool
	}{
		{
			name:           "create new instance",
			larkInstanceID: "instance-123",
			formData: map[string]interface{}{
				"applicant_user_id": "user-001",
				"department":        "Engineering",
			},
			existingFunc: func(ctx context.Context, larkID string) (*entity.ApprovalInstance, error) {
				return nil, errors.New("not found")
			},
			wantErr: false,
		},
		{
			name:           "instance already exists",
			larkInstanceID: "instance-456",
			formData:       map[string]interface{}{},
			existingFunc: func(ctx context.Context, larkID string) (*entity.ApprovalInstance, error) {
				return &entity.ApprovalInstance{
					ID:             1,
					LarkInstanceID: larkID,
					Status:         "CREATED",
				}, nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceRepo := &mockInstanceRepo{
				getByLarkInstanceIDFunc: tt.existingFunc,
			}
			historyRepo := &mockHistoryRepo{}
			itemRepo := &mockItemRepo{}
			txManager := &mockTxManager{}
			logger := &mockLogger{}

			service := NewApprovalService(instanceRepo, itemRepo, historyRepo, txManager, logger)

			instance, err := service.CreateInstance(context.Background(), tt.larkInstanceID, tt.formData)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateInstance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if instance == nil {
				t.Errorf("CreateInstance() returned nil instance")
				return
			}

			if instance.LarkInstanceID != tt.larkInstanceID {
				t.Errorf("CreateInstance() instance.LarkInstanceID = %v, want %v",
					instance.LarkInstanceID, tt.larkInstanceID)
			}
		})
	}
}

func TestApprovalService_GetInstance(t *testing.T) {
	instanceRepo := &mockInstanceRepo{}
	historyRepo := &mockHistoryRepo{}
	itemRepo := &mockItemRepo{}
	txManager := &mockTxManager{}
	logger := &mockLogger{}

	service := NewApprovalService(instanceRepo, itemRepo, historyRepo, txManager, logger)

	instance, err := service.GetInstance(context.Background(), 1)

	if err != nil {
		t.Errorf("GetInstance() error = %v", err)
		return
	}

	if instance == nil {
		t.Errorf("GetInstance() returned nil instance")
		return
	}

	if instance.ID != 1 {
		t.Errorf("GetInstance() instance.ID = %v, want %v", instance.ID, 1)
	}
}

func TestApprovalService_UpdateStatus(t *testing.T) {
	instanceRepo := &mockInstanceRepo{}
	historyRepo := &mockHistoryRepo{}
	itemRepo := &mockItemRepo{}
	txManager := &mockTxManager{}
	logger := &mockLogger{}

	service := NewApprovalService(instanceRepo, itemRepo, historyRepo, txManager, logger)

	err := service.UpdateStatus(context.Background(), 1, "APPROVED", map[string]interface{}{
		"action_by": "user-001",
		"comment":   "Looks good",
	})

	if err != nil {
		t.Errorf("UpdateStatus() error = %v", err)
	}
}

func TestApprovalService_SetApprovalTime(t *testing.T) {
	instanceRepo := &mockInstanceRepo{}
	historyRepo := &mockHistoryRepo{}
	itemRepo := &mockItemRepo{}
	txManager := &mockTxManager{}
	logger := &mockLogger{}

	service := NewApprovalService(instanceRepo, itemRepo, historyRepo, txManager, logger)

	err := service.SetApprovalTime(context.Background(), 1)

	if err != nil {
		t.Errorf("SetApprovalTime() error = %v", err)
	}
}

func TestApprovalService_ListInstances(t *testing.T) {
	instanceRepo := &mockInstanceRepo{}
	historyRepo := &mockHistoryRepo{}
	itemRepo := &mockItemRepo{}
	txManager := &mockTxManager{}
	logger := &mockLogger{}

	service := NewApprovalService(instanceRepo, itemRepo, historyRepo, txManager, logger)

	instances, err := service.ListInstances(context.Background(), 10, 0)

	if err != nil {
		t.Errorf("ListInstances() error = %v", err)
		return
	}

	if instances == nil {
		t.Errorf("ListInstances() returned nil")
	}
}
