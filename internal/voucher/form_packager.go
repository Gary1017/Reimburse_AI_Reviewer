package voucher

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// FormPackager orchestrates form generation and file organization
// ARCH-013-D: Main orchestration implementation
type FormPackager struct {
	filler         FormFillerInterface
	aggregator     FormDataAggregatorInterface
	folderManager  FolderManagerInterface
	attachmentRepo AttachmentRepositoryInterface
	instanceRepo   InstanceRepositoryInterface
	logger         *zap.Logger
}

// NewFormPackager creates a new FormPackager
func NewFormPackager(
	filler FormFillerInterface,
	aggregator FormDataAggregatorInterface,
	folderManager FolderManagerInterface,
	attachmentRepo AttachmentRepositoryInterface,
	instanceRepo InstanceRepositoryInterface,
	logger *zap.Logger,
) *FormPackager {
	return &FormPackager{
		filler:         filler,
		aggregator:     aggregator,
		folderManager:  folderManager,
		attachmentRepo: attachmentRepo,
		instanceRepo:   instanceRepo,
		logger:         logger,
	}
}

// GenerateFormPackage creates the complete form package for an instance
func (p *FormPackager) GenerateFormPackage(ctx context.Context, instanceID int64) (*FormPackageResult, error) {
	return p.GenerateFormPackageWithOptions(ctx, instanceID, &PackageOptions{
		OverwriteExisting:  true,
		WaitForAttachments: false,
	})
}

// GenerateFormPackageWithOptions allows customization of generation behavior
func (p *FormPackager) GenerateFormPackageWithOptions(ctx context.Context, instanceID int64, opts *PackageOptions) (*FormPackageResult, error) {
	result := &FormPackageResult{
		Success: false,
	}

	p.logger.Info("Starting form package generation",
		zap.Int64("instance_id", instanceID))

	// Step 1: Aggregate data from repositories
	formData, err := p.aggregator.AggregateData(ctx, instanceID)
	if err != nil {
		p.logger.Error("Failed to aggregate form data",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		result.Error = err
		return result, err
	}

	// Step 2: Create instance folder
	folderPath, err := p.folderManager.CreateInstanceFolder(formData.LarkInstanceID)
	if err != nil {
		p.logger.Error("Failed to create instance folder",
			zap.String("lark_instance_id", formData.LarkInstanceID),
			zap.Error(err))
		result.Error = err
		return result, fmt.Errorf("failed to create folder: %w", err)
	}
	result.FolderPath = folderPath

	// Step 3: Generate form file path
	formFileName := fmt.Sprintf("form_%s.xlsx", formData.LarkInstanceID)
	formFilePath := filepath.Join(folderPath, formFileName)

	// Step 4: Fill template and save
	if p.filler == nil {
		p.logger.Warn("FormFiller is nil, skipping form generation",
			zap.String("lark_instance_id", formData.LarkInstanceID))
		result.Error = fmt.Errorf("form filler not initialized")
		return result, result.Error
	}
	savedPath, err := p.filler.FillTemplate(ctx, formData, formFilePath)
	if err != nil {
		p.logger.Error("Failed to fill form template",
			zap.String("output_path", formFilePath),
			zap.Error(err))
		result.Error = err
		return result, fmt.Errorf("failed to fill template: %w", err)
	}
	result.FormFilePath = savedPath

	// Step 5: Check attachment status
	attachments, err := p.attachmentRepo.GetByInstanceID(instanceID)
	if err != nil {
		p.logger.Warn("Failed to get attachments for verification",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		// Continue without attachment verification
	} else {
		completePaths, incompleteCount := p.verifyAttachments(attachments)
		result.AttachmentPaths = completePaths
		result.IncompleteCount = incompleteCount

		if incompleteCount > 0 {
			p.logger.Warn("Some attachments are not yet downloaded",
				zap.Int64("instance_id", instanceID),
				zap.Int("incomplete_count", incompleteCount))
		}
	}

	result.Success = true
	p.logger.Info("Form package generated successfully",
		zap.Int64("instance_id", instanceID),
		zap.String("folder_path", result.FolderPath),
		zap.String("form_path", result.FormFilePath),
		zap.Int("attachment_count", len(result.AttachmentPaths)),
		zap.Int("incomplete_count", result.IncompleteCount))

	return result, nil
}

// verifyAttachments checks that all attachments are downloaded
// Returns list of completed attachment paths and count of incomplete attachments
func (p *FormPackager) verifyAttachments(attachments []*models.Attachment) ([]string, int) {
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
// Form generation triggers after download completion, not after AI processing
func (p *FormPackager) IsInstanceFullyProcessed(instanceID int64) (bool, error) {
	attachments, err := p.attachmentRepo.GetByInstanceID(instanceID)
	if err != nil {
		return false, fmt.Errorf("failed to get attachments: %w", err)
	}

	p.logger.Debug("Checking instance attachment statuses",
		zap.Int64("instance_id", instanceID),
		zap.Int("attachment_count", len(attachments)))

	if len(attachments) == 0 {
		// No attachments - still generate form
		p.logger.Debug("No attachments found, will generate form",
			zap.Int64("instance_id", instanceID))
		return true, nil
	}

	for _, att := range attachments {
		p.logger.Debug("Checking attachment status",
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

// GenerateFormPackageAsync generates the form package asynchronously
// It first checks if all attachments are processed before generating
func (p *FormPackager) GenerateFormPackageAsync(ctx context.Context, instanceID int64) {
	p.logger.Info("GenerateFormPackageAsync called",
		zap.Int64("instance_id", instanceID))

	go func() {
		// Check if all attachments are processed
		isReady, err := p.IsInstanceFullyProcessed(instanceID)
		if err != nil {
			p.logger.Warn("Failed to check if instance is fully processed",
				zap.Int64("instance_id", instanceID),
				zap.Error(err))
			return
		}

		p.logger.Info("Instance readiness check result",
			zap.Int64("instance_id", instanceID),
			zap.Bool("is_ready", isReady))

		if !isReady {
			p.logger.Debug("Instance not fully processed yet, skipping form generation",
				zap.Int64("instance_id", instanceID))
			return
		}

		// Generate form package
		result, err := p.GenerateFormPackage(ctx, instanceID)
		if err != nil {
			p.logger.Warn("Failed to generate form package (non-blocking)",
				zap.Int64("instance_id", instanceID),
				zap.Error(err))
			return
		}

		p.logger.Info("Form package generated successfully (async)",
			zap.Int64("instance_id", instanceID),
			zap.String("form_path", result.FormFilePath),
			zap.Int("attachment_count", len(result.AttachmentPaths)))
	}()
}
