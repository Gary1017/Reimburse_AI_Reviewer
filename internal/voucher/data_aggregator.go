package voucher

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// FormDataAggregator collects data from repositories for form generation
// ARCH-013-C: Data collection implementation
type FormDataAggregator struct {
	instanceRepo   InstanceRepositoryInterface
	itemRepo       ItemRepositoryInterface
	attachmentRepo AttachmentRepositoryInterface
	subjectMapper  AccountingSubjectMapperInterface
	logger         *zap.Logger
}

// NewFormDataAggregator creates a new FormDataAggregator
func NewFormDataAggregator(
	instanceRepo InstanceRepositoryInterface,
	itemRepo ItemRepositoryInterface,
	attachmentRepo AttachmentRepositoryInterface,
	subjectMapper AccountingSubjectMapperInterface,
	logger *zap.Logger,
) *FormDataAggregator {
	return &FormDataAggregator{
		instanceRepo:   instanceRepo,
		itemRepo:       itemRepo,
		attachmentRepo: attachmentRepo,
		subjectMapper:  subjectMapper,
		logger:         logger,
	}
}

// AggregateData collects all data needed for the reimbursement form
func (a *FormDataAggregator) AggregateData(ctx context.Context, instanceID int64) (*FormData, error) {
	a.logger.Debug("Aggregating form data", zap.Int64("instance_id", instanceID))

	// Step 1: Get approval instance
	instance, err := a.instanceRepo.GetByID(instanceID)
	if err != nil {
		a.logger.Error("Failed to get instance", zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	if instance == nil {
		return nil, fmt.Errorf("instance not found: %d", instanceID)
	}

	// Step 2: Get reimbursement items
	items, err := a.itemRepo.GetByInstanceID(instanceID)
	if err != nil {
		a.logger.Error("Failed to get items", zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get items: %w", err)
	}

	// Step 3: Build form data
	formData := &FormData{
		ApplicantName:  instance.ApplicantUserID,
		Department:     instance.Department,
		LarkInstanceID: instance.LarkInstanceID,
		SubmissionDate: instance.SubmissionTime,
		Items:          make([]FormItemData, 0, len(items)),
		TotalAmount:    0,
		TotalReceipts:  0,
	}

	// Step 4: Process each item
	for i, item := range items {
		// Get attachments for this item
		attachments, err := a.attachmentRepo.GetByItemID(item.ID)
		if err != nil {
			a.logger.Warn("Failed to get attachments for item",
				zap.Int64("item_id", item.ID),
				zap.Error(err))
			// Continue with zero attachments
			attachments = nil
		}

		receiptCount := len(attachments)

		formItemData := FormItemData{
			SequenceNumber:    i + 1, // 1-indexed
			AccountingSubject: a.subjectMapper.MapToSubject(item.ItemType),
			Description:       item.Description,
			ExpenseType:       a.subjectMapper.MapToChineseName(item.ItemType),
			ReceiptCount:      receiptCount,
			Amount:            item.Amount,
			Remarks:           item.BusinessPurpose,
		}

		formData.Items = append(formData.Items, formItemData)
		formData.TotalAmount += item.Amount
		formData.TotalReceipts += receiptCount
	}

	a.logger.Debug("Form data aggregation complete",
		zap.Int64("instance_id", instanceID),
		zap.Int("item_count", len(formData.Items)),
		zap.Float64("total_amount", formData.TotalAmount),
		zap.Int("total_receipts", formData.TotalReceipts))

	return formData, nil
}
