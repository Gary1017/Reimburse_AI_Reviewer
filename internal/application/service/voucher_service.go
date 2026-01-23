package service

import (
	"context"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// VoucherResult represents the result of voucher generation
type VoucherResult struct {
	Success         bool
	FolderPath      string
	VoucherFilePath string
	AttachmentPaths []string
	Error           error
}

// VoucherService manages voucher generation
type VoucherService interface {
	GenerateVoucher(ctx context.Context, instanceID int64) (*VoucherResult, error)
	IsInstanceReady(ctx context.Context, instanceID int64) (bool, error)
}

type voucherServiceImpl struct {
	instanceRepo   port.InstanceRepository
	itemRepo       port.ItemRepository
	attachmentRepo port.AttachmentRepository
	voucherRepo    port.VoucherRepository
	invoiceRepo    port.InvoiceRepository
	txManager      port.TransactionManager
	logger         Logger
}

// NewVoucherService creates a new VoucherService
func NewVoucherService(
	instanceRepo port.InstanceRepository,
	itemRepo port.ItemRepository,
	attachmentRepo port.AttachmentRepository,
	voucherRepo port.VoucherRepository,
	invoiceRepo port.InvoiceRepository,
	txManager port.TransactionManager,
	logger Logger,
) VoucherService {
	return &voucherServiceImpl{
		instanceRepo:   instanceRepo,
		itemRepo:       itemRepo,
		attachmentRepo: attachmentRepo,
		voucherRepo:    voucherRepo,
		invoiceRepo:    invoiceRepo,
		txManager:      txManager,
		logger:         logger,
	}
}

// GenerateVoucher generates a voucher for an instance
func (s *voucherServiceImpl) GenerateVoucher(ctx context.Context, instanceID int64) (*VoucherResult, error) {
	s.logger.Info("Generating voucher", "instance_id", instanceID)

	// Check if instance is ready
	ready, err := s.IsInstanceReady(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to check instance readiness", "error", err, "instance_id", instanceID)
		return &VoucherResult{
			Success: false,
			Error:   err,
		}, err
	}

	if !ready {
		err := fmt.Errorf("instance not ready for voucher generation")
		s.logger.Error("Instance not ready", "instance_id", instanceID)
		return &VoucherResult{
			Success: false,
			Error:   err,
		}, err
	}

	// Get instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get instance", "error", err, "instance_id", instanceID)
		return &VoucherResult{
			Success: false,
			Error:   err,
		}, err
	}

	// Get items
	items, err := s.itemRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get items", "error", err, "instance_id", instanceID)
		return &VoucherResult{
			Success: false,
			Error:   err,
		}, err
	}

	// Get attachments
	attachments, err := s.attachmentRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get attachments", "error", err, "instance_id", instanceID)
		return &VoucherResult{
			Success: false,
			Error:   err,
		}, err
	}

	// Generate folder path
	folderPath := fmt.Sprintf("/data/vouchers/instance_%d_%s", instanceID, instance.LarkInstanceID)

	// Generate voucher file path (Excel or PDF)
	voucherFilePath := fmt.Sprintf("%s/reimbursement_voucher_%d.xlsx", folderPath, instanceID)

	// Collect attachment paths
	var attachmentPaths []string
	for _, att := range attachments {
		if att.DownloadStatus == "DOWNLOADED" && att.FilePath != "" {
			attachmentPaths = append(attachmentPaths, att.FilePath)
		}
	}

	// In production, this would:
	// 1. Create folder structure
	// 2. Generate Excel/PDF voucher with item details
	// 3. Copy attachments to folder
	// 4. Save voucher record to database

	// Create voucher record
	voucher := &entity.GeneratedVoucher{
		InstanceID:    instanceID,
		FilePath:      voucherFilePath,
		VoucherNumber: fmt.Sprintf("VOUCHER-%d-%d", instanceID, time.Now().Unix()),
		CreatedAt:     time.Now(),
	}

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Check if voucher already exists
		existing, err := s.voucherRepo.GetByInstanceID(txCtx, instanceID)
		if err == nil && existing != nil {
			s.logger.Info("Voucher already exists", "instance_id", instanceID, "voucher_id", existing.ID)
			return nil
		}

		if err := s.voucherRepo.Create(txCtx, voucher); err != nil {
			return fmt.Errorf("create voucher record: %w", err)
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Failed to save voucher record", "error", err, "instance_id", instanceID)
		return &VoucherResult{
			Success: false,
			Error:   err,
		}, err
	}

	result := &VoucherResult{
		Success:         true,
		FolderPath:      folderPath,
		VoucherFilePath: voucherFilePath,
		AttachmentPaths: attachmentPaths,
		Error:           nil,
	}

	s.logger.Info("Voucher generated successfully",
		"instance_id", instanceID,
		"voucher_path", voucherFilePath,
		"attachment_count", len(attachmentPaths),
		"item_count", len(items),
	)

	return result, nil
}

// IsInstanceReady checks if an instance is ready for voucher generation
func (s *voucherServiceImpl) IsInstanceReady(ctx context.Context, instanceID int64) (bool, error) {
	// Get instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return false, fmt.Errorf("get instance: %w", err)
	}

	// Check status - instance must be approved
	if instance.Status != "APPROVED" {
		s.logger.Info("Instance not approved", "instance_id", instanceID, "status", instance.Status)
		return false, nil
	}

	// Check if attachments are downloaded
	attachments, err := s.attachmentRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return false, fmt.Errorf("get attachments: %w", err)
	}

	if len(attachments) == 0 {
		s.logger.Info("No attachments found", "instance_id", instanceID)
		return false, nil
	}

	// Check if all attachments are downloaded
	for _, att := range attachments {
		if att.DownloadStatus != "DOWNLOADED" {
			s.logger.Info("Attachment not downloaded",
				"instance_id", instanceID,
				"attachment_id", att.ID,
				"status", att.DownloadStatus,
			)
			return false, nil
		}
	}

	// Check if items exist
	items, err := s.itemRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return false, fmt.Errorf("get items: %w", err)
	}

	if len(items) == 0 {
		s.logger.Info("No items found", "instance_id", instanceID)
		return false, nil
	}

	s.logger.Info("Instance ready for voucher generation",
		"instance_id", instanceID,
		"attachment_count", len(attachments),
		"item_count", len(items),
	)

	return true, nil
}
