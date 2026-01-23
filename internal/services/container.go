package services

import (
	"os"

	"github.com/garyjia/ai-reimbursement/internal/ai"
	"github.com/garyjia/ai-reimbursement/internal/invoice"
	"github.com/garyjia/ai-reimbursement/internal/lark"
	"github.com/garyjia/ai-reimbursement/internal/notification"
	"github.com/garyjia/ai-reimbursement/internal/storage"
	"github.com/garyjia/ai-reimbursement/internal/voucher"
	"go.uber.org/zap"
)

// Container holds all service-layer components (stateless/reactive)
// Services are called on-demand by workers, not continuously running
type Container struct {
	// AI Processing Services
	PDFReader      *invoice.PDFReader
	InvoiceAuditor *ai.InvoiceAuditor

	// Notification Services
	AuditAggregator *notification.AuditAggregator
	AuditNotifier   *notification.AuditNotifier

	// Voucher/Form Services
	SubjectMapper      *voucher.AccountingSubjectMapper
	FormDataAggregator *voucher.FormDataAggregator
	FormFiller         *voucher.ReimbursementFormFiller
	VoucherGenerator   *voucher.VoucherGenerator

	// Storage Services
	FolderManager *storage.FolderManager
	FileStorage   *storage.LocalFileStorage

	// Lark Services
	ApprovalAPI       *lark.ApprovalAPI
	ApprovalBotAPI    *lark.ApprovalBotAPI
	AttachmentHandler *lark.AttachmentHandler
	EventProcessor    *lark.EventProcessor

	logger *zap.Logger
}

// ServiceConfig holds configuration for service initialization
type ServiceConfig struct {
	OpenAIAPIKey     string
	OpenAIModel      string
	CompanyName      string
	CompanyTaxID     string
	PriceDeviation   float64
	AttachmentDir    string
	FormTemplatePath string
	FontPath         string
	LarkApprovalCode string
}

// NewContainer creates and initializes all services
func NewContainer(
	cfg ServiceConfig,
	infrastructure *Infrastructure,
	logger *zap.Logger,
) (*Container, error) {
	c := &Container{
		logger: logger,
	}

	// Initialize services by category
	c.initializeAIServices(cfg, logger)
	c.initializeLarkServices(infrastructure, cfg, logger)
	c.initializeNotificationServices(infrastructure, cfg, logger)
	c.initializeStorageServices(cfg, logger)
	c.initializeVoucherServices(infrastructure, cfg, logger)

	logger.Info("Service container initialized",
		zap.Bool("form_generation_enabled", c.FormFiller != nil))

	// Validate CJK font support for Excel generation
	c.ValidateCJKFontSupport(cfg, logger)

	return c, nil
}

// initializeAIServices initializes AI processing services
func (c *Container) initializeAIServices(cfg ServiceConfig, logger *zap.Logger) {
	c.PDFReader = invoice.NewPDFReader(cfg.OpenAIAPIKey, cfg.OpenAIModel, logger)
	c.InvoiceAuditor = ai.NewInvoiceAuditor(
		cfg.OpenAIAPIKey,
		cfg.OpenAIModel,
		cfg.CompanyName,
		cfg.CompanyTaxID,
		cfg.PriceDeviation,
		logger,
	)
}

// initializeLarkServices initializes Lark API services
func (c *Container) initializeLarkServices(infrastructure *Infrastructure, cfg ServiceConfig, logger *zap.Logger) {
	c.ApprovalAPI = lark.NewApprovalAPI(infrastructure.LarkClient, logger)
	c.ApprovalBotAPI = lark.NewApprovalBotAPI(infrastructure.LarkClient, logger)
	c.AttachmentHandler = lark.NewAttachmentHandler(logger, cfg.AttachmentDir)

	// Event processing (must be initialized after workflow engine)
	// This will be wired to the engine in Infrastructure.InitializeWorkflowEngine
	c.EventProcessor = lark.NewEventProcessor(
		cfg.LarkApprovalCode,
		nil, // Will be set later by SetWorkflowHandler
		logger,
	)
}

// initializeNotificationServices initializes notification services
func (c *Container) initializeNotificationServices(infrastructure *Infrastructure, cfg ServiceConfig, logger *zap.Logger) {
	c.AuditAggregator = notification.NewAuditAggregator(logger)
	c.AuditNotifier = notification.NewAuditNotifier(
		infrastructure.Repositories.Attachment,
		infrastructure.Repositories.Instance,
		infrastructure.Repositories.Notification,
		c.ApprovalAPI,
		c.ApprovalBotAPI,
		c.AuditAggregator,
		cfg.LarkApprovalCode,
		logger,
	)
}

// initializeStorageServices initializes storage services
func (c *Container) initializeStorageServices(cfg ServiceConfig, logger *zap.Logger) {
	c.FolderManager = storage.NewFolderManager(cfg.AttachmentDir, logger)
	c.FileStorage = storage.NewLocalFileStorage(cfg.AttachmentDir, logger)
}

// initializeVoucherServices initializes voucher/form services
func (c *Container) initializeVoucherServices(infrastructure *Infrastructure, cfg ServiceConfig, logger *zap.Logger) {
	c.SubjectMapper = voucher.NewAccountingSubjectMapper()
	c.FormDataAggregator = voucher.NewFormDataAggregator(
		infrastructure.Repositories.Instance,
		infrastructure.Repositories.Item,
		infrastructure.Repositories.Attachment,
		c.SubjectMapper,
		logger,
	)

	// FormFiller may fail to load template - this is non-fatal
	formFiller, err := voucher.NewReimbursementFormFiller(cfg.FormTemplatePath, cfg.FontPath, logger)
	if err != nil {
		logger.Warn("Failed to initialize FormFiller, form generation disabled", zap.Error(err))
		c.FormFiller = nil
	} else {
		c.FormFiller = formFiller
	}

	c.VoucherGenerator = voucher.NewVoucherGenerator(
		c.FormFiller,
		c.FormDataAggregator,
		c.FolderManager,
		infrastructure.Repositories.Attachment,
		infrastructure.Repositories.Instance,
		logger,
	)
}

// ValidateCJKFontSupport checks if CJK font is available for Excel generation
// This is a health check that logs warnings if font support is not available
func (c *Container) ValidateCJKFontSupport(cfg ServiceConfig, logger *zap.Logger) {
	if cfg.FontPath == "" {
		logger.Warn("CJK font path not configured",
			zap.String("recommendation", "Set 'voucher.font_path' in config.yaml for proper Chinese character support in Excel"),
			zap.String("default_path", "configs/NotoSansCJKsc-Regular.otf"))
		return
	}

	if _, err := os.Stat(cfg.FontPath); os.IsNotExist(err) {
		logger.Warn("CJK font file not found - Excel generation may have formatting issues",
			zap.String("expected_path", cfg.FontPath),
			zap.String("impact", "Chinese characters may not display correctly in generated Excel vouchers"),
			zap.String("resolution", "Ensure font file exists at configured path, or disable Excel generation"),
			zap.String("graceful_degradation", "System will continue - attachments will still be delivered without formatted Excel"))
		return
	}

	logger.Info("CJK font support validated",
		zap.String("font_path", cfg.FontPath),
		zap.String("status", "OK - Excel generation will include proper Chinese font support"))
}

