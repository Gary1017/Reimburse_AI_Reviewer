package service

import (
	"context"
	"testing"

	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

type mockVoucherRepo struct {
	createFunc         func(ctx context.Context, voucher *entity.GeneratedVoucher) error
	getByInstanceIDFunc func(ctx context.Context, instanceID int64) (*entity.GeneratedVoucher, error)
}

func (m *mockVoucherRepo) Create(ctx context.Context, voucher *entity.GeneratedVoucher) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, voucher)
	}
	voucher.ID = 1
	return nil
}

func (m *mockVoucherRepo) GetByInstanceID(ctx context.Context, instanceID int64) (*entity.GeneratedVoucher, error) {
	if m.getByInstanceIDFunc != nil {
		return m.getByInstanceIDFunc(ctx, instanceID)
	}
	return nil, nil
}

func (m *mockVoucherRepo) Update(ctx context.Context, voucher *entity.GeneratedVoucher) error {
	return nil
}

func TestVoucherService_IsInstanceReady(t *testing.T) {
	tests := []struct {
		name             string
		instanceID       int64
		instanceStatus   string
		attachments      []*entity.Attachment
		items            []*entity.ReimbursementItem
		wantReady        bool
		wantErr          bool
	}{
		{
			name:           "ready instance",
			instanceID:     1,
			instanceStatus: "APPROVED",
			attachments: []*entity.Attachment{
				{ID: 1, DownloadStatus: "DOWNLOADED", FilePath: "/data/att1.pdf"},
			},
			items: []*entity.ReimbursementItem{
				{ID: 1, ItemType: "TRAVEL", Amount: 1000.0},
			},
			wantReady: true,
			wantErr:   false,
		},
		{
			name:           "not approved",
			instanceID:     2,
			instanceStatus: "PENDING",
			attachments: []*entity.Attachment{
				{ID: 1, DownloadStatus: "DOWNLOADED", FilePath: "/data/att1.pdf"},
			},
			items: []*entity.ReimbursementItem{
				{ID: 1, ItemType: "TRAVEL", Amount: 1000.0},
			},
			wantReady: false,
			wantErr:   false,
		},
		{
			name:           "attachment not downloaded",
			instanceID:     3,
			instanceStatus: "APPROVED",
			attachments: []*entity.Attachment{
				{ID: 1, DownloadStatus: "PENDING", FilePath: ""},
			},
			items: []*entity.ReimbursementItem{
				{ID: 1, ItemType: "TRAVEL", Amount: 1000.0},
			},
			wantReady: false,
			wantErr:   false,
		},
		{
			name:           "no attachments",
			instanceID:     4,
			instanceStatus: "APPROVED",
			attachments:    []*entity.Attachment{},
			items: []*entity.ReimbursementItem{
				{ID: 1, ItemType: "TRAVEL", Amount: 1000.0},
			},
			wantReady: false,
			wantErr:   false,
		},
		{
			name:           "no items",
			instanceID:     5,
			instanceStatus: "APPROVED",
			attachments: []*entity.Attachment{
				{ID: 1, DownloadStatus: "DOWNLOADED", FilePath: "/data/att1.pdf"},
			},
			items:     []*entity.ReimbursementItem{},
			wantReady: false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceRepo := &mockInstanceRepo{
				getByIDFunc: func(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
					return &entity.ApprovalInstance{
						ID:     id,
						Status: tt.instanceStatus,
					}, nil
				},
			}
			itemRepo := &mockItemRepo{
				getByInstanceIDFunc: func(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error) {
					return tt.items, nil
				},
			}
			attachmentRepo := &mockAttachmentRepo{
				getByInstanceIDFunc: func(ctx context.Context, instanceID int64) ([]*entity.Attachment, error) {
					return tt.attachments, nil
				},
			}
			voucherRepo := &mockVoucherRepo{}
			invoiceRepo := &mockInvoiceRepo{}
			txManager := &mockTxManager{}
			logger := &mockLogger{}

			service := NewVoucherService(instanceRepo, itemRepo, attachmentRepo, voucherRepo, invoiceRepo, txManager, logger)

			ready, err := service.IsInstanceReady(context.Background(), tt.instanceID)

			if (err != nil) != tt.wantErr {
				t.Errorf("IsInstanceReady() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if ready != tt.wantReady {
				t.Errorf("IsInstanceReady() ready = %v, want %v", ready, tt.wantReady)
			}
		})
	}
}

func TestVoucherService_GenerateVoucher(t *testing.T) {
	instanceRepo := &mockInstanceRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
			return &entity.ApprovalInstance{
				ID:             id,
				LarkInstanceID: "instance-123",
				Status:         "APPROVED",
			}, nil
		},
	}
	itemRepo := &mockItemRepo{
		getByInstanceIDFunc: func(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error) {
			return []*entity.ReimbursementItem{
				{ID: 1, ItemType: "TRAVEL", Amount: 1000.0},
			}, nil
		},
	}
	attachmentRepo := &mockAttachmentRepo{
		getByInstanceIDFunc: func(ctx context.Context, instanceID int64) ([]*entity.Attachment, error) {
			return []*entity.Attachment{
				{ID: 1, DownloadStatus: "DOWNLOADED", FilePath: "/data/att1.pdf"},
			}, nil
		},
	}
	voucherRepo := &mockVoucherRepo{}
	invoiceRepo := &mockInvoiceRepo{}
	txManager := &mockTxManager{}
	logger := &mockLogger{}

	service := NewVoucherService(instanceRepo, itemRepo, attachmentRepo, voucherRepo, invoiceRepo, txManager, logger)

	result, err := service.GenerateVoucher(context.Background(), 1)

	if err != nil {
		t.Errorf("GenerateVoucher() error = %v", err)
		return
	}

	if result == nil {
		t.Errorf("GenerateVoucher() returned nil result")
		return
	}

	if !result.Success {
		t.Errorf("GenerateVoucher() Success = false, want true")
	}

	if result.FolderPath == "" {
		t.Errorf("GenerateVoucher() FolderPath is empty")
	}

	if result.VoucherFilePath == "" {
		t.Errorf("GenerateVoucher() VoucherFilePath is empty")
	}

	if len(result.AttachmentPaths) == 0 {
		t.Errorf("GenerateVoucher() AttachmentPaths is empty")
	}
}

// Add mock methods for item repo
type mockItemRepoExtended struct {
	mockItemRepo
	getByInstanceIDFunc func(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error)
}

func (m *mockItemRepoExtended) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error) {
	if m.getByInstanceIDFunc != nil {
		return m.getByInstanceIDFunc(ctx, instanceID)
	}
	return []*entity.ReimbursementItem{}, nil
}

// Add mock methods for attachment repo
type mockAttachmentRepoExtended struct {
	mockAttachmentRepo
	getByInstanceIDFunc func(ctx context.Context, instanceID int64) ([]*entity.Attachment, error)
}

func (m *mockAttachmentRepoExtended) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Attachment, error) {
	if m.getByInstanceIDFunc != nil {
		return m.getByInstanceIDFunc(ctx, instanceID)
	}
	return []*entity.Attachment{}, nil
}
