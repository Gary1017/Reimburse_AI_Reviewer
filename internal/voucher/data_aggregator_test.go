package voucher

import (
	"context"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ARCH-013-C: FormDataAggregator tests
// Tests for aggregating data from repositories for form generation

// MockInstanceRepository implements InstanceRepositoryInterface for testing
type MockInstanceRepository struct {
	instances map[int64]*models.ApprovalInstance
	err       error
}

func (m *MockInstanceRepository) GetByID(id int64) (*models.ApprovalInstance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.instances[id], nil
}

func (m *MockInstanceRepository) GetByLarkInstanceID(larkInstanceID string) (*models.ApprovalInstance, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, inst := range m.instances {
		if inst.LarkInstanceID == larkInstanceID {
			return inst, nil
		}
	}
	return nil, nil
}

// MockItemRepository implements ItemRepositoryInterface for testing
type MockItemRepository struct {
	items map[int64][]*models.ReimbursementItem
	err   error
}

func (m *MockItemRepository) GetByInstanceID(instanceID int64) ([]*models.ReimbursementItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items[instanceID], nil
}

// MockAttachmentRepository implements AttachmentRepositoryInterface for testing
type MockAttachmentRepository struct {
	attachmentsByInstance map[int64][]*models.Attachment
	attachmentsByItem     map[int64][]*models.Attachment
	err                   error
}

func (m *MockAttachmentRepository) GetByInstanceID(instanceID int64) ([]*models.Attachment, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.attachmentsByInstance[instanceID], nil
}

func (m *MockAttachmentRepository) GetByItemID(itemID int64) ([]*models.Attachment, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.attachmentsByItem[itemID], nil
}

func TestFormDataAggregator_AggregateData(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()

	submissionTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	t.Run("aggregates data from all repositories", func(t *testing.T) {
		instanceRepo := &MockInstanceRepository{
			instances: map[int64]*models.ApprovalInstance{
				1: {
					ID:              1,
					LarkInstanceID:  "ABC123-XYZ",
					ApplicantUserID: "user001",
					Department:      "Engineering",
					SubmissionTime:  submissionTime,
					Status:          models.StatusApproved,
				},
			},
		}

		itemRepo := &MockItemRepository{
			items: map[int64][]*models.ReimbursementItem{
				1: {
					{ID: 101, InstanceID: 1, ItemType: models.ItemTypeTravel, Description: "出差北京", Amount: 500.00, BusinessPurpose: "客户会议"},
					{ID: 102, InstanceID: 1, ItemType: models.ItemTypeMeal, Description: "工作餐", Amount: 150.00, BusinessPurpose: "加班"},
				},
			},
		}

		attachmentRepo := &MockAttachmentRepository{
			attachmentsByItem: map[int64][]*models.Attachment{
				101: {
					{ID: 1001, ItemID: 101, FileName: "receipt1.pdf", DownloadStatus: models.AttachmentStatusCompleted},
					{ID: 1002, ItemID: 101, FileName: "receipt2.pdf", DownloadStatus: models.AttachmentStatusCompleted},
				},
				102: {
					{ID: 1003, ItemID: 102, FileName: "meal_receipt.jpg", DownloadStatus: models.AttachmentStatusCompleted},
				},
			},
		}

		aggregator := NewFormDataAggregator(instanceRepo, itemRepo, attachmentRepo, NewAccountingSubjectMapper(), logger)

		formData, err := aggregator.AggregateData(ctx, 1)

		require.NoError(t, err)
		assert.NotNil(t, formData)
		assert.Equal(t, "user001", formData.ApplicantName)
		assert.Equal(t, "Engineering", formData.Department)
		assert.Equal(t, "ABC123-XYZ", formData.LarkInstanceID)
		assert.Equal(t, submissionTime, formData.SubmissionDate)
		assert.Equal(t, 650.00, formData.TotalAmount)
		assert.Equal(t, 3, formData.TotalReceipts)
		assert.Len(t, formData.Items, 2)

		// Verify first item
		assert.Equal(t, 1, formData.Items[0].SequenceNumber)
		assert.Equal(t, "差旅费", formData.Items[0].AccountingSubject)
		assert.Equal(t, "出差北京", formData.Items[0].Description)
		assert.Equal(t, "差旅费", formData.Items[0].ExpenseType)
		assert.Equal(t, 2, formData.Items[0].ReceiptCount)
		assert.Equal(t, 500.00, formData.Items[0].Amount)
		assert.Equal(t, "客户会议", formData.Items[0].Remarks)

		// Verify second item
		assert.Equal(t, 2, formData.Items[1].SequenceNumber)
		assert.Equal(t, "餐费", formData.Items[1].AccountingSubject)
		assert.Equal(t, 1, formData.Items[1].ReceiptCount)
	})

	t.Run("returns error when instance not found", func(t *testing.T) {
		instanceRepo := &MockInstanceRepository{
			instances: map[int64]*models.ApprovalInstance{}, // empty
		}

		aggregator := NewFormDataAggregator(instanceRepo, &MockItemRepository{}, &MockAttachmentRepository{}, NewAccountingSubjectMapper(), logger)

		formData, err := aggregator.AggregateData(ctx, 999)

		assert.Error(t, err)
		assert.Nil(t, formData)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("handles instance with no items", func(t *testing.T) {
		instanceRepo := &MockInstanceRepository{
			instances: map[int64]*models.ApprovalInstance{
				1: {
					ID:              1,
					LarkInstanceID:  "NO-ITEMS",
					ApplicantUserID: "user001",
					Department:      "Engineering",
					SubmissionTime:  submissionTime,
				},
			},
		}

		itemRepo := &MockItemRepository{
			items: map[int64][]*models.ReimbursementItem{}, // no items
		}

		aggregator := NewFormDataAggregator(instanceRepo, itemRepo, &MockAttachmentRepository{}, NewAccountingSubjectMapper(), logger)

		formData, err := aggregator.AggregateData(ctx, 1)

		require.NoError(t, err)
		assert.NotNil(t, formData)
		assert.Empty(t, formData.Items)
		assert.Equal(t, 0.0, formData.TotalAmount)
		assert.Equal(t, 0, formData.TotalReceipts)
	})

	t.Run("handles items with no attachments", func(t *testing.T) {
		instanceRepo := &MockInstanceRepository{
			instances: map[int64]*models.ApprovalInstance{
				1: {
					ID:              1,
					LarkInstanceID:  "NO-ATTACHMENTS",
					ApplicantUserID: "user001",
					Department:      "Engineering",
					SubmissionTime:  submissionTime,
				},
			},
		}

		itemRepo := &MockItemRepository{
			items: map[int64][]*models.ReimbursementItem{
				1: {
					{ID: 101, InstanceID: 1, ItemType: models.ItemTypeMeal, Description: "午餐", Amount: 50.00},
				},
			},
		}

		attachmentRepo := &MockAttachmentRepository{
			attachmentsByItem: map[int64][]*models.Attachment{}, // no attachments
		}

		aggregator := NewFormDataAggregator(instanceRepo, itemRepo, attachmentRepo, NewAccountingSubjectMapper(), logger)

		formData, err := aggregator.AggregateData(ctx, 1)

		require.NoError(t, err)
		assert.Len(t, formData.Items, 1)
		assert.Equal(t, 0, formData.Items[0].ReceiptCount)
		assert.Equal(t, 0, formData.TotalReceipts)
	})

	t.Run("calculates total amount correctly", func(t *testing.T) {
		instanceRepo := &MockInstanceRepository{
			instances: map[int64]*models.ApprovalInstance{
				1: {
					ID:              1,
					LarkInstanceID:  "CALC-TEST",
					ApplicantUserID: "user001",
					Department:      "Engineering",
					SubmissionTime:  submissionTime,
				},
			},
		}

		itemRepo := &MockItemRepository{
			items: map[int64][]*models.ReimbursementItem{
				1: {
					{ID: 1, InstanceID: 1, ItemType: models.ItemTypeTravel, Amount: 100.50},
					{ID: 2, InstanceID: 1, ItemType: models.ItemTypeMeal, Amount: 50.25},
					{ID: 3, InstanceID: 1, ItemType: models.ItemTypeAccommodation, Amount: 200.00},
				},
			},
		}

		aggregator := NewFormDataAggregator(instanceRepo, itemRepo, &MockAttachmentRepository{}, NewAccountingSubjectMapper(), logger)

		formData, err := aggregator.AggregateData(ctx, 1)

		require.NoError(t, err)
		assert.Equal(t, 350.75, formData.TotalAmount)
	})
}

func TestNewFormDataAggregator(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	aggregator := NewFormDataAggregator(
		&MockInstanceRepository{},
		&MockItemRepository{},
		&MockAttachmentRepository{},
		NewAccountingSubjectMapper(),
		logger,
	)

	assert.NotNil(t, aggregator)
}
