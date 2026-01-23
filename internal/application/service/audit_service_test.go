package service

import (
	"context"
	"testing"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

type mockAttachmentRepo struct{
	getByInstanceIDFunc func(ctx context.Context, instanceID int64) ([]*entity.Attachment, error)
}

func (m *mockAttachmentRepo) Create(ctx context.Context, att *entity.Attachment) error {
	return nil
}

func (m *mockAttachmentRepo) GetByID(ctx context.Context, id int64) (*entity.Attachment, error) {
	return &entity.Attachment{
		ID:             id,
		DownloadStatus: "DOWNLOADED",
		FilePath:       "/data/attachments/test.pdf",
	}, nil
}

func (m *mockAttachmentRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Attachment, error) {
	if m.getByInstanceIDFunc != nil {
		return m.getByInstanceIDFunc(ctx, instanceID)
	}
	return []*entity.Attachment{
		{
			ID:             1,
			DownloadStatus: "DOWNLOADED",
			FilePath:       "/data/attachments/test.pdf",
		},
	}, nil
}

func (m *mockAttachmentRepo) GetPending(ctx context.Context, limit int) ([]*entity.Attachment, error) {
	return []*entity.Attachment{}, nil
}

func (m *mockAttachmentRepo) MarkCompleted(ctx context.Context, id int64, filePath string, fileSize int64) error {
	return nil
}

func (m *mockAttachmentRepo) UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error {
	return nil
}

type mockInvoiceRepo struct{}

func (m *mockInvoiceRepo) Create(ctx context.Context, invoice *entity.Invoice) error {
	return nil
}

func (m *mockInvoiceRepo) GetByID(ctx context.Context, id int64) (*entity.Invoice, error) {
	return &entity.Invoice{ID: id}, nil
}

func (m *mockInvoiceRepo) GetByAttachmentID(ctx context.Context, attachmentID int64) (*entity.Invoice, error) {
	return &entity.Invoice{ID: 1}, nil
}

func (m *mockInvoiceRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Invoice, error) {
	return []*entity.Invoice{
		{
			ID:            1,
			InvoiceCode:   "001",
			InvoiceNumber: "12345678",
			InvoiceAmount: 1000.0,
		},
	}, nil
}

func (m *mockInvoiceRepo) Update(ctx context.Context, invoice *entity.Invoice) error {
	return nil
}

func (m *mockInvoiceRepo) GetByUniqueID(ctx context.Context, uniqueID string) (*entity.Invoice, error) {
	return nil, nil
}

type mockAIAuditor struct{}

func (m *mockAIAuditor) AuditPolicy(ctx context.Context, item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) (*port.PolicyAuditResult, error) {
	return &port.PolicyAuditResult{
		Compliant:  true,
		Violations: []string{},
		Confidence: 0.95,
		Reasoning:  "All policies compliant",
	}, nil
}

func (m *mockAIAuditor) AuditPrice(ctx context.Context, item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) (*port.PriceAuditResult, error) {
	return &port.PriceAuditResult{
		Reasonable:          true,
		DeviationPercentage: 5.0,
		MarketPriceMin:      900.0,
		MarketPriceMax:      1100.0,
		Confidence:          0.9,
		Reasoning:           "Price within market range",
	}, nil
}

func (m *mockAIAuditor) ExtractInvoice(ctx context.Context, imageData []byte, mimeType string) (*port.InvoiceExtractionResult, error) {
	return &port.InvoiceExtractionResult{
		Success:     true,
		Confidence:  0.95,
		TotalAmount: 1000.0,
	}, nil
}

func TestAuditService_AuditInstance(t *testing.T) {
	instanceRepo := &mockInstanceRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
			return &entity.ApprovalInstance{
				ID:     id,
				Status: "PENDING",
			}, nil
		},
	}
	itemRepo := &mockItemRepo{}
	attachmentRepo := &mockAttachmentRepo{}
	invoiceRepo := &mockInvoiceRepo{}
	aiAuditor := &mockAIAuditor{}
	logger := &mockLogger{}

	service := NewAuditService(instanceRepo, itemRepo, attachmentRepo, invoiceRepo, aiAuditor, logger)

	result, err := service.AuditInstance(context.Background(), 1)

	if err != nil {
		t.Errorf("AuditInstance() error = %v", err)
		return
	}

	if result == nil {
		t.Errorf("AuditInstance() returned nil result")
		return
	}

	if !result.OverallPass {
		t.Errorf("AuditInstance() OverallPass = false, want true")
	}

	if result.Confidence == 0 {
		t.Errorf("AuditInstance() Confidence = 0, want > 0")
	}
}

func TestAuditService_AuditItem(t *testing.T) {
	instanceRepo := &mockInstanceRepo{}
	itemRepo := &mockItemRepo{}
	attachmentRepo := &mockAttachmentRepo{}
	invoiceRepo := &mockInvoiceRepo{}
	aiAuditor := &mockAIAuditor{}
	logger := &mockLogger{}

	service := NewAuditService(instanceRepo, itemRepo, attachmentRepo, invoiceRepo, aiAuditor, logger)

	item := &entity.ReimbursementItem{
		ID:         1,
		InstanceID: 1,
		ItemType:   "TRAVEL",
		Amount:     1000.0,
	}

	result, err := service.AuditItem(context.Background(), item)

	if err != nil {
		t.Errorf("AuditItem() error = %v", err)
		return
	}

	if result == nil {
		t.Errorf("AuditItem() returned nil result")
		return
	}

	if !result.OverallPass {
		t.Errorf("AuditItem() OverallPass = false, want true")
	}

	if result.PolicyResult == nil {
		t.Errorf("AuditItem() PolicyResult is nil")
	}

	if result.PriceResult == nil {
		t.Errorf("AuditItem() PriceResult is nil")
	}
}

func TestAuditService_ExtractInvoice(t *testing.T) {
	instanceRepo := &mockInstanceRepo{}
	itemRepo := &mockItemRepo{}
	attachmentRepo := &mockAttachmentRepo{}
	invoiceRepo := &mockInvoiceRepo{}
	aiAuditor := &mockAIAuditor{}
	logger := &mockLogger{}

	service := NewAuditService(instanceRepo, itemRepo, attachmentRepo, invoiceRepo, aiAuditor, logger)

	result, err := service.ExtractInvoice(context.Background(), 1)

	if err != nil {
		t.Errorf("ExtractInvoice() error = %v", err)
		return
	}

	if result == nil {
		t.Errorf("ExtractInvoice() returned nil result")
		return
	}

	if !result.Success {
		t.Errorf("ExtractInvoice() Success = false, want true")
	}
}
