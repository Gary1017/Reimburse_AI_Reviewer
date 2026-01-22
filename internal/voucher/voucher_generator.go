package voucher

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// VoucherGenerator orchestrates voucher generation and file organization
// ARCH-013-D: Main orchestration implementation
// This is the central manager for voucher/expense form processing that:
// 1. Extracts data from approval instances
// 2. Creates instance-scoped folders
// 3. Generates Excel voucher forms
// 4. Organizes all files (invoices + form) in the instance folder
// 5. Updates database records
type VoucherGenerator struct {
	filler         FormFillerInterface
	aggregator     FormDataAggregatorInterface
	folderManager  FolderManagerInterface
	attachmentRepo AttachmentRepositoryInterface
	instanceRepo   InstanceRepositoryInterface
	logger         *zap.Logger
}

// NewVoucherGenerator creates a new VoucherGenerator
func NewVoucherGenerator(
	filler FormFillerInterface,
	aggregator FormDataAggregatorInterface,
	folderManager FolderManagerInterface,
	attachmentRepo AttachmentRepositoryInterface,
	instanceRepo InstanceRepositoryInterface,
	logger *zap.Logger,
) *VoucherGenerator {
	return &VoucherGenerator{
		filler:         filler,
		aggregator:     aggregator,
		folderManager:  folderManager,
		attachmentRepo: attachmentRepo,
		instanceRepo:   instanceRepo,
		logger:         logger,
	}
}

// GenerateVoucher creates the complete voucher package for an instance
func (vg *VoucherGenerator) GenerateVoucher(ctx context.Context, instanceID int64) (*VoucherResult, error) {
	return vg.GenerateVoucherWithOptions(ctx, instanceID, &GenerationOptions{
		OverwriteExisting:  true,
		WaitForAttachments: false,
	})
}

// GenerateVoucherWithOptions allows customization of generation behavior
func (vg *VoucherGenerator) GenerateVoucherWithOptions(ctx context.Context, instanceID int64, opts *GenerationOptions) (*VoucherResult, error) {
	result := &VoucherResult{
		Success: false,
	}

	vg.logger.Info("Starting voucher generation",
		zap.Int64("instance_id", instanceID))

	// Step 1: Aggregate data from repositories (extract from JSON/DB)
	formData, err := vg.aggregator.AggregateData(ctx, instanceID)
	if err != nil {
		vg.logger.Error("Failed to aggregate voucher data",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		result.Error = err
		return result, err
	}

	// Step 2: Create instance folder (folder creation for this approval)
	folderPath, err := vg.folderManager.CreateInstanceFolder(formData.LarkInstanceID)
	if err != nil {
		vg.logger.Error("Failed to create instance folder",
			zap.String("lark_instance_id", formData.LarkInstanceID),
			zap.Error(err))
		result.Error = err
		return result, fmt.Errorf("failed to create folder: %w", err)
	}
	result.FolderPath = folderPath

	// Step 3: Generate voucher file path
	voucherFileName := fmt.Sprintf("form_%s.xlsx", formData.LarkInstanceID)
	voucherFilePath := filepath.Join(folderPath, voucherFileName)

	// Step 4: Fill template and save (generate voucher based on template)
	if vg.filler == nil {
		vg.logger.Warn("FormFiller is nil, skipping voucher generation",
			zap.String("lark_instance_id", formData.LarkInstanceID))
		// Don't fail the entire process if form filler is not available
		result.VoucherFilePath = ""
		result.FormFilePath = ""
	} else {
		savedPath, err := vg.filler.FillTemplate(ctx, formData, voucherFilePath)
		if err != nil {
			vg.logger.Error("Failed to fill voucher template",
				zap.String("output_path", voucherFilePath),
				zap.Error(err))
			// Log the error but don't fail the entire process
			// The attachments are still organized in the folder
			result.VoucherFilePath = ""
			result.FormFilePath = ""
		} else {
			result.VoucherFilePath = savedPath
			result.FormFilePath = savedPath // Backward compatibility
		}
	}

	// Step 5: Check attachment status (verify downloaded invoices)
	attachments, err := vg.attachmentRepo.GetByInstanceID(instanceID)
	if err != nil {
		vg.logger.Warn("Failed to get attachments for verification",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		// Continue without attachment verification
	} else {
		completePaths, incompleteCount := vg.verifyAttachments(attachments)
		result.AttachmentPaths = completePaths
		result.IncompleteCount = incompleteCount

		if incompleteCount > 0 {
			vg.logger.Warn("Some attachments are not yet downloaded",
				zap.Int64("instance_id", instanceID),
				zap.Int("incomplete_count", incompleteCount))
		}
	}

	result.Success = true
	vg.logger.Info("Voucher generated successfully",
		zap.Int64("instance_id", instanceID),
		zap.String("folder_path", result.FolderPath),
		zap.String("voucher_path", result.VoucherFilePath),
		zap.Int("attachment_count", len(result.AttachmentPaths)),
		zap.Int("incomplete_count", result.IncompleteCount))

	return result, nil
}

// verifyAttachments checks that all attachments are downloaded
// Returns list of completed attachment paths and count of incomplete attachments
func (vg *VoucherGenerator) verifyAttachments(attachments []*models.Attachment) ([]string, int) {
	var completePaths []string
	incompleteCount := 0

	for _, att := range attachments {
		if att.DownloadStatus == models.AttachmentStatusCompleted ||
			att.DownloadStatus == models.AttachmentStatusProcessed {
			if att.FilePath != "" {
				completePaths = append(completePaths, att.FilePath)
			}
		} else {
			incompleteCount++
		}
	}

	return completePaths, incompleteCount
}

// IsInstanceFullyProcessed checks if all attachments for an instance are downloaded
// Voucher generation triggers after download completion, not after AI processing
func (vg *VoucherGenerator) IsInstanceFullyProcessed(instanceID int64) (bool, error) {
	attachments, err := vg.attachmentRepo.GetByInstanceID(instanceID)
	if err != nil {
		return false, fmt.Errorf("failed to get attachments: %w", err)
	}

	vg.logger.Debug("Checking instance attachment statuses",
		zap.Int64("instance_id", instanceID),
		zap.Int("attachment_count", len(attachments)))

	if len(attachments) == 0 {
		// No attachments - still generate voucher
		vg.logger.Debug("No attachments found, will generate voucher",
			zap.Int64("instance_id", instanceID))
		return true, nil
	}

	for _, att := range attachments {
		vg.logger.Debug("Checking attachment status",
			zap.Int64("instance_id", instanceID),
			zap.Int64("attachment_id", att.ID),
			zap.String("status", att.DownloadStatus))

		// Check if attachment is at least downloaded (COMPLETED or beyond)
		if att.DownloadStatus == models.AttachmentStatusPending ||
			att.DownloadStatus == models.AttachmentStatusFailed {
			return false, nil
		}
	}

	return true, nil
}

// GenerateVoucherAsync generates the voucher asynchronously
// It first checks if all attachments are processed before generating
func (vg *VoucherGenerator) GenerateVoucherAsync(ctx context.Context, instanceID int64) {
	vg.logger.Info("GenerateVoucherAsync called",
		zap.Int64("instance_id", instanceID))

	go func() {
		// Check if all attachments are processed
		isReady, err := vg.IsInstanceFullyProcessed(instanceID)
		if err != nil {
			vg.logger.Warn("Failed to check if instance is fully processed",
				zap.Int64("instance_id", instanceID),
				zap.Error(err))
			return
		}

		vg.logger.Info("Instance readiness check result",
			zap.Int64("instance_id", instanceID),
			zap.Bool("is_ready", isReady))

		if !isReady {
			vg.logger.Debug("Instance not fully processed yet, skipping voucher generation",
				zap.Int64("instance_id", instanceID))
			return
		}

		// Generate voucher
		result, err := vg.GenerateVoucher(ctx, instanceID)
		if err != nil {
			vg.logger.Warn("Failed to generate voucher (non-blocking)",
				zap.Int64("instance_id", instanceID),
				zap.Error(err))
			return
		}

		vg.logger.Info("Voucher generated successfully (async)",
			zap.Int64("instance_id", instanceID),
			zap.String("voucher_path", result.VoucherFilePath),
			zap.Int("attachment_count", len(result.AttachmentPaths)))
	}()
}

// GenerateFormPackageAsync is a backward compatibility alias for GenerateVoucherAsync
// Deprecated: Use GenerateVoucherAsync instead
func (vg *VoucherGenerator) GenerateFormPackageAsync(ctx context.Context, instanceID int64) {
	vg.GenerateVoucherAsync(ctx, instanceID)
}

// GenerateFormPackage is a backward compatibility alias for GenerateVoucher
// Deprecated: Use GenerateVoucher instead
func (vg *VoucherGenerator) GenerateFormPackage(ctx context.Context, instanceID int64) (*VoucherResult, error) {
	return vg.GenerateVoucher(ctx, instanceID)
}

// GenerateFormPackageWithOptions is a backward compatibility alias
// Deprecated: Use GenerateVoucherWithOptions instead
func (vg *VoucherGenerator) GenerateFormPackageWithOptions(ctx context.Context, instanceID int64, opts *GenerationOptions) (*VoucherResult, error) {
	return vg.GenerateVoucherWithOptions(ctx, instanceID, opts)
}

// NewFormPackager is a backward compatibility constructor
// Deprecated: Use NewVoucherGenerator instead
func NewFormPackager(
	filler FormFillerInterface,
	aggregator FormDataAggregatorInterface,
	folderManager FolderManagerInterface,
	attachmentRepo AttachmentRepositoryInterface,
	instanceRepo InstanceRepositoryInterface,
	logger *zap.Logger,
) *VoucherGenerator {
	return NewVoucherGenerator(filler, aggregator, folderManager, attachmentRepo, instanceRepo, logger)
}
