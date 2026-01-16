package voucher

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/garyjia/ai-reimbursement/internal/lark"
	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/garyjia/ai-reimbursement/internal/repository"
	"github.com/garyjia/ai-reimbursement/pkg/database"
	"go.uber.org/zap"
)

// Generator orchestrates voucher generation
type Generator struct {
	db                *database.DB
	instanceRepo      *repository.InstanceRepository
	voucherRepo       *repository.VoucherRepository
	excelFiller       *ExcelFiller
	attachmentHandler *AttachmentHandler
	outputDir         string
	accountantEmail   string
	logger            *zap.Logger
}

// Config holds generator configuration
type Config struct {
	TemplatePath    string
	OutputDir       string
	CompanyName     string
	CompanyTaxID    string
	AccountantEmail string
}

// NewGenerator creates a new voucher generator
func NewGenerator(
	db *database.DB,
	instanceRepo *repository.InstanceRepository,
	voucherRepo *repository.VoucherRepository,
	approvalAPI *lark.ApprovalAPI,
	cfg Config,
	logger *zap.Logger,
) (*Generator, error) {
	excelFiller, err := NewExcelFiller(cfg.TemplatePath, cfg.CompanyName, cfg.CompanyTaxID, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Excel filler: %w", err)
	}

	attachmentHandler := NewAttachmentHandler(approvalAPI, cfg.OutputDir, logger)

	return &Generator{
		db:                db,
		instanceRepo:      instanceRepo,
		voucherRepo:       voucherRepo,
		excelFiller:       excelFiller,
		attachmentHandler: attachmentHandler,
		outputDir:         cfg.OutputDir,
		accountantEmail:   cfg.AccountantEmail,
		logger:            logger,
	}, nil
}

// GenerateVoucher generates a voucher for an approved instance
func (g *Generator) GenerateVoucher(ctx context.Context, instanceID int64) (*models.GeneratedVoucher, error) {
	g.logger.Info("Starting voucher generation", zap.Int64("instance_id", instanceID))

	// Get instance
	instance, err := g.instanceRepo.GetByID(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		return nil, fmt.Errorf("instance not found: %d", instanceID)
	}

	// Check if voucher already exists (idempotency)
	existing, err := g.voucherRepo.GetByInstanceID(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing voucher: %w", err)
	}
	if existing != nil {
		g.logger.Info("Voucher already exists", zap.Int64("instance_id", instanceID))
		return existing, nil
	}

	// Generate voucher number
	voucherNumber, err := g.voucherRepo.GenerateVoucherNumber()
	if err != nil {
		return nil, fmt.Errorf("failed to generate voucher number: %w", err)
	}

	// Generate output file path
	outputFileName := fmt.Sprintf("%s_%s.xlsx", voucherNumber, instance.ApplicantUserID)
	outputPath := filepath.Join(g.outputDir, outputFileName)

	// Fill Excel template
	if err := g.excelFiller.FillTemplate(instance, voucherNumber, outputPath); err != nil {
		return nil, fmt.Errorf("failed to fill template: %w", err)
	}

	// Download attachments
	attachmentPaths, err := g.attachmentHandler.DownloadAttachments(ctx, instance)
	if err != nil {
		g.logger.Error("Failed to download attachments", zap.Error(err))
		// Don't fail the entire process if attachments fail
		attachmentPaths = []string{}
	}

	// Store attachment paths in metadata
	g.logger.Info("Voucher generated successfully",
		zap.Int64("instance_id", instanceID),
		zap.String("voucher_number", voucherNumber),
		zap.String("output_path", outputPath),
		zap.Int("attachments", len(attachmentPaths)))

	// Create voucher record
	voucher := &models.GeneratedVoucher{
		InstanceID:      instanceID,
		VoucherNumber:   voucherNumber,
		FilePath:        outputPath,
		AccountantEmail: g.accountantEmail,
	}

	if err := g.voucherRepo.Create(nil, voucher); err != nil {
		return nil, fmt.Errorf("failed to create voucher record: %w", err)
	}

	return voucher, nil
}

// GetAttachmentPaths gets attachment paths for a voucher
func (g *Generator) GetAttachmentPaths(ctx context.Context, instance *models.ApprovalInstance) ([]string, error) {
	return g.attachmentHandler.DownloadAttachments(ctx, instance)
}
