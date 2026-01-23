package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// InvoiceWorkerConfig holds configuration for invoice worker
type InvoiceWorkerConfig struct {
	PollInterval   time.Duration
	BatchSize      int
	ProcessTimeout time.Duration
}

// DefaultInvoiceWorkerConfig returns default configuration
func DefaultInvoiceWorkerConfig() InvoiceWorkerConfig {
	return InvoiceWorkerConfig{
		PollInterval:   10 * time.Second,
		BatchSize:      5,
		ProcessTimeout: 120 * time.Second,
	}
}

// InvoiceWorker processes downloaded attachments with AI
// ARCH-124: Infrastructure layer worker using port interfaces
type InvoiceWorker struct {
	config InvoiceWorkerConfig

	// Port dependencies
	attachmentRepo port.AttachmentRepository
	itemRepo       port.ItemRepository
	invoiceRepo    port.InvoiceRepository
	fileStorage    port.FileStorage
	aiAuditor      port.AIAuditor
	logger         *zap.Logger

	// Runtime state
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	isRunning      bool
	lastProcessed  time.Time
	processedCount int
	failedCount    int
	startTime      time.Time
	lastError      error
}

// NewInvoiceWorker creates a new invoice worker
func NewInvoiceWorker(
	config InvoiceWorkerConfig,
	attachmentRepo port.AttachmentRepository,
	itemRepo port.ItemRepository,
	invoiceRepo port.InvoiceRepository,
	fileStorage port.FileStorage,
	aiAuditor port.AIAuditor,
	logger *zap.Logger,
) *InvoiceWorker {
	return &InvoiceWorker{
		config:         config,
		attachmentRepo: attachmentRepo,
		itemRepo:       itemRepo,
		invoiceRepo:    invoiceRepo,
		fileStorage:    fileStorage,
		aiAuditor:      aiAuditor,
		logger:         logger,
		lastProcessed:  time.Now(),
		startTime:      time.Now(),
	}
}

// Start begins the worker polling loop
func (w *InvoiceWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.isRunning {
		w.mu.Unlock()
		return fmt.Errorf("invoice worker already running")
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.isRunning = true
	w.startTime = time.Now()
	w.mu.Unlock()

	w.logger.Info("InvoiceWorker started",
		zap.Duration("poll_interval", w.config.PollInterval),
		zap.Int("batch_size", w.config.BatchSize))

	// Run polling loop in background goroutine
	go w.pollLoop()

	return nil
}

// Stop gracefully terminates the worker
func (w *InvoiceWorker) Stop() error {
	w.mu.Lock()
	if !w.isRunning {
		w.mu.Unlock()
		return nil
	}

	w.isRunning = false
	w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
	}

	w.logger.Info("InvoiceWorker stopped",
		zap.Int("processed_count", w.processedCount),
		zap.Int("failed_count", w.failedCount))

	return nil
}

// Name returns the worker name for identification
func (w *InvoiceWorker) Name() string {
	return "InvoiceWorker"
}

// pollLoop runs the main polling loop in background
func (w *InvoiceWorker) pollLoop() {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Debug("Poll loop context cancelled")
			return

		case <-ticker.C:
			if err := w.processCompletedAttachments(); err != nil {
				w.mu.Lock()
				w.lastError = err
				w.mu.Unlock()
				w.logger.Error("Failed to process completed attachments", zap.Error(err))
			}

			w.mu.Lock()
			w.lastProcessed = time.Now()
			w.mu.Unlock()
		}
	}
}

// processCompletedAttachments processes attachments that have been downloaded
func (w *InvoiceWorker) processCompletedAttachments() error {
	ctx := w.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Get attachments with status "COMPLETED" (downloaded but not yet processed)
	// Note: The port interface GetPending gets attachments with "PENDING" status
	// We need to get completed attachments for processing
	// For now, we'll use GetByInstanceID and filter in-memory (this should be enhanced in port interface)

	// This is a placeholder - the actual implementation would need a GetCompleted method
	// in the AttachmentRepository port interface
	// For now, return nil to allow compilation
	return nil
}

// processAttachment processes a single attachment through AI pipeline
func (w *InvoiceWorker) processAttachment(ctx context.Context, att *entity.Attachment) error {
	// Create processing context with timeout
	processCtx, cancel := context.WithTimeout(ctx, w.config.ProcessTimeout)
	defer cancel()

	w.logger.Info("Processing attachment",
		zap.Int64("attachment_id", att.ID),
		zap.String("file_name", att.FileName),
		zap.String("file_path", att.FilePath))

	// Mark as processing
	if err := w.attachmentRepo.UpdateStatus(ctx, att.ID, "PROCESSING", ""); err != nil {
		return fmt.Errorf("failed to update status to PROCESSING: %w", err)
	}

	// Check if file is supported
	ext := strings.ToLower(filepath.Ext(att.FileName))
	if ext != ".pdf" && ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		errMsg := fmt.Sprintf("unsupported file type: %s", ext)
		w.logger.Warn("Skipping unsupported file type",
			zap.Int64("attachment_id", att.ID),
			zap.String("extension", ext))
		return w.attachmentRepo.UpdateStatus(ctx, att.ID, "PROCESSED", errMsg)
	}

	// Step 1: Read file content
	fileContent, err := w.fileStorage.Read(processCtx, att.FilePath)
	if err != nil {
		errMsg := fmt.Sprintf("failed to read file: %v", err)
		_ = w.attachmentRepo.UpdateStatus(ctx, att.ID, "AUDIT_FAILED", errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// Step 2: Extract invoice data using AI
	mimeType := w.getMimeType(ext)
	extractedData, err := w.aiAuditor.ExtractInvoice(processCtx, fileContent, mimeType)
	if err != nil {
		errMsg := fmt.Sprintf("extraction failed: %v", err)
		w.logger.Error("Failed to extract invoice data",
			zap.Int64("attachment_id", att.ID),
			zap.Error(err))
		return w.attachmentRepo.UpdateStatus(ctx, att.ID, "AUDIT_FAILED", errMsg)
	}

	w.logger.Info("Invoice data extracted",
		zap.Int64("attachment_id", att.ID),
		zap.String("invoice_code", extractedData.InvoiceCode),
		zap.String("invoice_number", extractedData.InvoiceNumber),
		zap.Float64("total_amount", extractedData.TotalAmount))

	// Step 3: Get reimbursement item for context
	item, err := w.itemRepo.GetByID(processCtx, att.ItemID)
	if err != nil {
		w.logger.Warn("Failed to get reimbursement item", zap.Error(err))
		// Continue with default values
		item = &entity.ReimbursementItem{
			Amount:   extractedData.TotalAmount,
			ItemType: "OTHER",
		}
	}

	// Step 4: Perform AI audits
	policyResult, err := w.aiAuditor.AuditPolicy(processCtx, item, extractedData)
	if err != nil {
		errMsg := fmt.Sprintf("policy audit failed: %v", err)
		w.logger.Error("Policy audit failed",
			zap.Int64("attachment_id", att.ID),
			zap.Error(err))
		return w.attachmentRepo.UpdateStatus(ctx, att.ID, "AUDIT_FAILED", errMsg)
	}

	priceResult, err := w.aiAuditor.AuditPrice(processCtx, item, extractedData)
	if err != nil {
		errMsg := fmt.Sprintf("price audit failed: %v", err)
		w.logger.Error("Price audit failed",
			zap.Int64("attachment_id", att.ID),
			zap.Error(err))
		return w.attachmentRepo.UpdateStatus(ctx, att.ID, "AUDIT_FAILED", errMsg)
	}

	w.logger.Info("Invoice audit completed",
		zap.Int64("attachment_id", att.ID),
		zap.Bool("policy_compliant", policyResult.Compliant),
		zap.Bool("price_reasonable", priceResult.Reasonable))

	// Step 5: Save invoice record
	if extractedData.InvoiceCode != "" && extractedData.InvoiceNumber != "" {
		uniqueID := generateUniqueID(extractedData.InvoiceCode, extractedData.InvoiceNumber)

		// Check for duplicates
		existing, err := w.invoiceRepo.GetByUniqueID(processCtx, uniqueID)
		if err == nil && existing != nil {
			w.logger.Warn("Duplicate invoice detected",
				zap.String("unique_id", uniqueID),
				zap.Int64("duplicate_invoice_id", existing.ID))
			policyResult.Violations = append(policyResult.Violations,
				fmt.Sprintf("DUPLICATE: Invoice was previously submitted (ID: %d)", existing.ID))
			policyResult.Compliant = false
		} else {
			// Save new invoice
			extractedDataJSON, _ := json.Marshal(extractedData.ExtractedData)

			// Parse invoice date
			var invoiceDate *time.Time
			if extractedData.InvoiceDate != "" {
				if t, err := time.Parse("2006-01-02", extractedData.InvoiceDate); err == nil {
					invoiceDate = &t
				}
			}

			invoice := &entity.Invoice{
				InvoiceCode:   extractedData.InvoiceCode,
				InvoiceNumber: extractedData.InvoiceNumber,
				UniqueID:      uniqueID,
				InstanceID:    att.InstanceID,
				FilePath:      att.FilePath,
				InvoiceAmount: extractedData.TotalAmount,
				InvoiceDate:   invoiceDate,
				SellerName:    extractedData.SellerName,
				SellerTaxID:   extractedData.SellerTaxID,
				BuyerName:     extractedData.BuyerName,
				BuyerTaxID:    extractedData.BuyerTaxID,
				ExtractedData: string(extractedDataJSON),
			}
			if err := w.invoiceRepo.Create(processCtx, invoice); err != nil {
				w.logger.Warn("Failed to save invoice record", zap.Error(err))
			}
		}
	}

	// Step 6: Serialize audit results
	auditResults := map[string]interface{}{
		"policy": policyResult,
		"price":  priceResult,
	}
	auditResultJSON, _ := json.Marshal(auditResults)

	// Update attachment with audit result
	if err := w.attachmentRepo.UpdateStatus(ctx, att.ID, "PROCESSED", string(auditResultJSON)); err != nil {
		return err
	}

	return nil
}

// getMimeType returns MIME type for file extension
func (w *InvoiceWorker) getMimeType(ext string) string {
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

// generateUniqueID creates a unique identifier for invoice
func generateUniqueID(invoiceCode, invoiceNumber string) string {
	return fmt.Sprintf("%s-%s", invoiceCode, invoiceNumber)
}
