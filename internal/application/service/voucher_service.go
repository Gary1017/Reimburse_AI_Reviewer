package service

import (
	"context"
	"fmt"
	"path/filepath"
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
	renderer       port.VoucherRenderer
	fileStorage    port.FileStorage
	folderManager  port.FolderManager
	templatePath   string
	outputDir      string
	companyName    string
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
	renderer port.VoucherRenderer,
	fileStorage port.FileStorage,
	folderManager port.FolderManager,
	templatePath string,
	outputDir string,
	companyName string,
	logger Logger,
) VoucherService {
	return &voucherServiceImpl{
		instanceRepo:   instanceRepo,
		itemRepo:       itemRepo,
		attachmentRepo: attachmentRepo,
		voucherRepo:    voucherRepo,
		invoiceRepo:    invoiceRepo,
		txManager:      txManager,
		renderer:       renderer,
		fileStorage:    fileStorage,
		folderManager:  folderManager,
		templatePath:   templatePath,
		outputDir:      outputDir,
		companyName:    companyName,
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

	// Create output folder: {outputDir}/{timestamp}+{lark_instance_id}/
	timestamp := time.Now().Unix()
	folderName := s.folderManager.SanitizeName(fmt.Sprintf("%d+%s", timestamp, instance.LarkInstanceID))
	folderPath, err := s.folderManager.CreateFolder(ctx, folderName)
	if err != nil {
		s.logger.Error("Failed to create output folder", "error", err, "folder_name", folderName)
		return &VoucherResult{
			Success: false,
			Error:   fmt.Errorf("create output folder: %w", err),
		}, err
	}
	s.logger.Info("Created output folder", "folder_path", folderPath)

	// Calculate total amount from items
	var totalAmountCents int64
	for _, item := range items {
		totalAmountCents += item.AmountCents
	}

	// Build VoucherData from instance/items/attachments
	voucherData := &port.VoucherData{
		CompanyName:      s.companyName,
		ApplicantName:    instance.ApplicantUserID, // Use UserID as placeholder; in production, fetch name from Lark
		Department:       instance.Department,
		ApprovalID:       instance.LarkInstanceID,
		Items:            items,
		Attachments:      attachments,
		TotalAmountCents: totalAmountCents,
	}

	// Render voucher Excel file
	voucherFileName := "reimbursement_form.xlsx"
	voucherFilePath := filepath.Join(folderPath, voucherFileName)
	renderResult, err := s.renderer.Render(ctx, voucherData, s.templatePath, voucherFilePath)
	if err != nil {
		s.logger.Error("Failed to render voucher", "error", err, "output_path", voucherFilePath)
		return &VoucherResult{
			Success: false,
			Error:   fmt.Errorf("render voucher: %w", err),
		}, err
	}
	s.logger.Info("Rendered voucher Excel", "file_path", renderResult.FilePath)

	// Copy downloaded attachments to folder as {attachment_id}_{amount}.ext
	var attachmentPaths []string
	for _, att := range attachments {
		if att.DownloadStatus != "DOWNLOADED" || att.FilePath == "" {
			s.logger.Info("Skipping attachment (not downloaded)", "attachment_id", att.ID, "status", att.DownloadStatus)
			continue
		}

		// Read attachment from storage
		content, err := s.fileStorage.Read(ctx, att.FilePath)
		if err != nil {
			s.logger.Error("Failed to read attachment", "error", err, "attachment_id", att.ID, "path", att.FilePath)
			return &VoucherResult{
				Success: false,
				Error:   fmt.Errorf("read attachment %d: %w", att.ID, err),
			}, err
		}

		// Find matching item to get amount
		var itemAmount int64
		for _, item := range items {
			if item.ID == att.ItemID {
				itemAmount = item.AmountCents
				break
			}
		}

		// Generate new filename: {attachment_id}_{amount}.ext
		ext := filepath.Ext(att.FileName)
		amountYuan := float64(itemAmount) / 100.0
		newFileName := fmt.Sprintf("%d_%.2f%s", att.ID, amountYuan, ext)
		newFilePath := filepath.Join(folderPath, newFileName)

		// Save attachment to folder
		if err := s.fileStorage.Save(ctx, newFilePath, content); err != nil {
			s.logger.Error("Failed to copy attachment", "error", err, "attachment_id", att.ID, "new_path", newFilePath)
			return &VoucherResult{
				Success: false,
				Error:   fmt.Errorf("copy attachment %d: %w", att.ID, err),
			}, err
		}

		attachmentPaths = append(attachmentPaths, newFilePath)
		s.logger.Info("Copied attachment", "attachment_id", att.ID, "new_path", newFilePath)
	}

	// Create voucher DB record in transaction
	voucher := &entity.GeneratedVoucher{
		InstanceID:    instanceID,
		FilePath:      renderResult.FilePath,
		VoucherNumber: fmt.Sprintf("VOUCHER-%d-%d", instanceID, timestamp),
		CreatedAt:     time.Now(),
	}

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Check if voucher already exists (idempotency)
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
		VoucherFilePath: renderResult.FilePath,
		AttachmentPaths: attachmentPaths,
		Error:           nil,
	}

	s.logger.Info("Voucher generated successfully",
		"instance_id", instanceID,
		"folder_path", folderPath,
		"voucher_path", renderResult.FilePath,
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
