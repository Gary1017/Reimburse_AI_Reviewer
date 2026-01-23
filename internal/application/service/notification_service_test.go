package service

import (
	"context"
	"testing"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

type mockLarkClient struct{}

func (m *mockLarkClient) GetInstanceDetail(ctx context.Context, instanceID string) (*port.LarkInstanceDetail, error) {
	return &port.LarkInstanceDetail{
		InstanceCode: instanceID,
		UserID:       "user-001",
		OpenID:       "ou-12345678",
		Status:       "APPROVED",
	}, nil
}

func (m *mockLarkClient) GetApprovers(ctx context.Context, instanceID string) ([]port.ApproverInfo, error) {
	return []port.ApproverInfo{}, nil
}

type mockMessageSender struct {
	sendMessageFunc     func(ctx context.Context, openID string, content string) error
	sendCardMessageFunc func(ctx context.Context, openID string, cardContent interface{}) error
}

func (m *mockMessageSender) SendMessage(ctx context.Context, openID string, content string) error {
	if m.sendMessageFunc != nil {
		return m.sendMessageFunc(ctx, openID, content)
	}
	return nil
}

func (m *mockMessageSender) SendCardMessage(ctx context.Context, openID string, cardContent interface{}) error {
	if m.sendCardMessageFunc != nil {
		return m.sendCardMessageFunc(ctx, openID, cardContent)
	}
	return nil
}

type mockNotificationRepo struct {
	createFunc       func(ctx context.Context, notification *entity.AuditNotification) error
	updateStatusFunc func(ctx context.Context, id int64, status string, errorMsg string) error
	markSentFunc     func(ctx context.Context, id int64) error
}

func (m *mockNotificationRepo) Create(ctx context.Context, notification *entity.AuditNotification) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, notification)
	}
	notification.ID = 1
	return nil
}

func (m *mockNotificationRepo) GetByInstanceID(ctx context.Context, instanceID int64) (*entity.AuditNotification, error) {
	return &entity.AuditNotification{ID: 1}, nil
}

func (m *mockNotificationRepo) UpdateStatus(ctx context.Context, id int64, status string, errorMsg string) error {
	if m.updateStatusFunc != nil {
		return m.updateStatusFunc(ctx, id, status, errorMsg)
	}
	return nil
}

func (m *mockNotificationRepo) MarkSent(ctx context.Context, id int64) error {
	if m.markSentFunc != nil {
		return m.markSentFunc(ctx, id)
	}
	return nil
}

func TestNotificationService_NotifyApplicant(t *testing.T) {
	instanceRepo := &mockInstanceRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
			return &entity.ApprovalInstance{
				ID:             id,
				LarkInstanceID: "instance-123",
				Status:         "APPROVED",
			}, nil
		},
	}
	notificationRepo := &mockNotificationRepo{}
	larkClient := &mockLarkClient{}
	messageSender := &mockMessageSender{}
	txManager := &mockTxManager{}
	logger := &mockLogger{}

	service := NewNotificationService(instanceRepo, notificationRepo, larkClient, messageSender, txManager, logger)

	err := service.NotifyApplicant(context.Background(), 1, "Test message")

	if err != nil {
		t.Errorf("NotifyApplicant() error = %v", err)
	}
}

func TestNotificationService_NotifyAuditResult(t *testing.T) {
	tests := []struct {
		name        string
		result      *AuditResult
		wantErr     bool
	}{
		{
			name: "passed audit",
			result: &AuditResult{
				OverallPass: true,
				Confidence:  0.95,
				Reasoning:   "All checks passed",
				PolicyResult: &port.PolicyAuditResult{
					Compliant:  true,
					Violations: []string{},
					Confidence: 0.95,
				},
				PriceResult: &port.PriceAuditResult{
					Reasonable: true,
					Confidence: 0.9,
				},
			},
			wantErr: false,
		},
		{
			name: "failed audit",
			result: &AuditResult{
				OverallPass: false,
				Confidence:  0.8,
				Reasoning:   "Policy violations detected",
				PolicyResult: &port.PolicyAuditResult{
					Compliant:  false,
					Violations: []string{"Missing company name", "Invalid date"},
					Confidence: 0.8,
				},
				PriceResult: &port.PriceAuditResult{
					Reasonable: true,
					Confidence: 0.9,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceRepo := &mockInstanceRepo{
				getByIDFunc: func(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
					return &entity.ApprovalInstance{
						ID:             id,
						LarkInstanceID: "instance-123",
						Status:         "AI_AUDITING",
					}, nil
				},
			}
			notificationRepo := &mockNotificationRepo{}
			larkClient := &mockLarkClient{}
			messageSender := &mockMessageSender{}
			txManager := &mockTxManager{}
			logger := &mockLogger{}

			service := NewNotificationService(instanceRepo, notificationRepo, larkClient, messageSender, txManager, logger)

			err := service.NotifyAuditResult(context.Background(), 1, tt.result)

			if (err != nil) != tt.wantErr {
				t.Errorf("NotifyAuditResult() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNotificationService_NotifyVoucherReady(t *testing.T) {
	instanceRepo := &mockInstanceRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
			return &entity.ApprovalInstance{
				ID:             id,
				LarkInstanceID: "instance-123",
				Status:         "COMPLETED",
			}, nil
		},
	}
	notificationRepo := &mockNotificationRepo{}
	larkClient := &mockLarkClient{}
	messageSender := &mockMessageSender{}
	txManager := &mockTxManager{}
	logger := &mockLogger{}

	service := NewNotificationService(instanceRepo, notificationRepo, larkClient, messageSender, txManager, logger)

	err := service.NotifyVoucherReady(context.Background(), 1, "/data/vouchers/instance_1/voucher.xlsx")

	if err != nil {
		t.Errorf("NotifyVoucherReady() error = %v", err)
	}
}
