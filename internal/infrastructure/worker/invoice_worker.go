package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/dispatcher"
	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"github.com/garyjia/ai-reimbursement/internal/domain/event"
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
	invoiceV2Repo  port.InvoiceV2Repository
	fileStorage    port.FileStorage
	aiAuditor      port.AIAuditor
	dispatcher     dispatcher.Dispatcher
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
	invoiceV2Repo port.InvoiceV2Repository,
	fileStorage port.FileStorage,
	aiAuditor port.AIAuditor,
	eventDispatcher dispatcher.Dispatcher,
	logger *zap.Logger,
) *InvoiceWorker {
	return &InvoiceWorker{
		config:         config,
		attachmentRepo: attachmentRepo,
		itemRepo:       itemRepo,
		invoiceV2Repo:  invoiceV2Repo,
		fileStorage:    fileStorage,
		aiAuditor:      aiAuditor,
		dispatcher:     eventDispatcher,
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
	attachments, err := w.attachmentRepo.GetCompletedUnprocessed(ctx, w.config.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to get completed unprocessed attachments: %w", err)
	}

	if len(attachments) == 0 {
		return nil
	}

	w.logger.Debug("Processing batch of completed attachments",
		zap.Int("count", len(attachments)))

	// Track affected instances for completion check
	affectedInstances := make(map[int64]bool)

	// Process each attachment
	for _, att := range attachments {
		if err := w.processAttachment(ctx, att); err != nil {
			w.logger.Error("Failed to process attachment",
				zap.Int64("attachment_id", att.ID),
				zap.String("file_name", att.FileName),
				zap.Error(err))
			w.mu.Lock()
			w.failedCount++
			w.mu.Unlock()
		} else {
			w.mu.Lock()
			w.processedCount++
			w.mu.Unlock()
			affectedInstances[att.InstanceID] = true
		}
	}

	// Check completion status for all affected instances
	for instanceID := range affectedInstances {
		w.checkInstanceCompletion(ctx, instanceID)
	}

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
	if err := w.attachmentRepo.UpdateStatus(ctx, att.ID, entity.AttachmentStatusProcessing, ""); err != nil {
		return fmt.Errorf("failed to update status to PROCESSING: %w", err)
	}

	// Check if file type is supported
	ext := strings.ToLower(filepath.Ext(att.FileName))
	if ext != ".pdf" && ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		errMsg := fmt.Sprintf("unsupported file type: %s", ext)
		w.logger.Warn("Skipping unsupported file type",
			zap.Int64("attachment_id", att.ID),
			zap.String("extension", ext))

		// Mark as OTHER file type and PROCESSED
		if err := w.attachmentRepo.UpdateFileType(ctx, att.ID, entity.FileTypeOther); err != nil {
			w.logger.Error("Failed to update file type", zap.Error(err))
		}
		return w.attachmentRepo.UpdateStatus(ctx, att.ID, entity.AttachmentStatusProcessed, errMsg)
	}

	// Step 1: Read file content
	fileContent, err := w.fileStorage.Read(processCtx, att.FilePath)
	if err != nil {
		errMsg := fmt.Sprintf("failed to read file: %v", err)
		_ = w.attachmentRepo.UpdateStatus(ctx, att.ID, entity.AttachmentStatusAuditFailed, errMsg)
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

		// Mark as OTHER file type and AUDIT_FAILED
		if err := w.attachmentRepo.UpdateFileType(ctx, att.ID, entity.FileTypeOther); err != nil {
			w.logger.Error("Failed to update file type", zap.Error(err))
		}
		return w.attachmentRepo.UpdateStatus(ctx, att.ID, entity.AttachmentStatusAuditFailed, errMsg)
	}

	// Step 3: Check if extraction has invoice code and number
	if extractedData.InvoiceCode == "" || extractedData.InvoiceNumber == "" {
		w.logger.Info("Extraction succeeded but no invoice code/number found",
			zap.Int64("attachment_id", att.ID),
			zap.String("invoice_code", extractedData.InvoiceCode),
			zap.String("invoice_number", extractedData.InvoiceNumber))

		// Mark as OTHER file type and PROCESSED
		if err := w.attachmentRepo.UpdateFileType(ctx, att.ID, entity.FileTypeOther); err != nil {
			w.logger.Error("Failed to update file type", zap.Error(err))
		}
		return w.attachmentRepo.UpdateStatus(ctx, att.ID, entity.AttachmentStatusProcessed, "not an invoice")
	}

	w.logger.Info("Invoice data extracted",
		zap.Int64("attachment_id", att.ID),
		zap.String("invoice_code", extractedData.InvoiceCode),
		zap.String("invoice_number", extractedData.InvoiceNumber),
		zap.Float64("total_amount", extractedData.TotalAmount))

	// Step 4: Save InvoiceV2 record
	uniqueID := generateUniqueID(extractedData.InvoiceCode, extractedData.InvoiceNumber)

	// Parse invoice date
	var invoiceDate *time.Time
	if extractedData.InvoiceDate != "" {
		if t, err := time.Parse("2006-01-02", extractedData.InvoiceDate); err == nil {
			invoiceDate = &t
		}
	}

	// Convert amount to cents (分)
	invoiceAmountCents := int64(extractedData.TotalAmount * 100)

	// Serialize extracted data
	extractedDataJSON, _ := json.Marshal(extractedData.ExtractedData)

	invoiceV2 := &entity.InvoiceV2{
		InstanceID:         att.InstanceID,
		AttachmentID:       att.ID,
		ItemID:             att.ItemID,
		InvoiceCode:        extractedData.InvoiceCode,
		InvoiceNumber:      extractedData.InvoiceNumber,
		UniqueID:           uniqueID,
		InvoiceDate:        invoiceDate,
		InvoiceAmountCents: invoiceAmountCents,
		SellerName:         extractedData.SellerName,
		SellerTaxID:        extractedData.SellerTaxID,
		BuyerName:          extractedData.BuyerName,
		BuyerTaxID:         extractedData.BuyerTaxID,
		ExtractedData:      string(extractedDataJSON),
	}

	if err := w.invoiceV2Repo.Create(processCtx, invoiceV2); err != nil {
		errMsg := fmt.Sprintf("failed to save invoice: %v", err)
		w.logger.Error("Failed to save InvoiceV2 record",
			zap.Int64("attachment_id", att.ID),
			zap.Error(err))
		_ = w.attachmentRepo.UpdateStatus(ctx, att.ID, entity.AttachmentStatusAuditFailed, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// Step 5: Update attachment file_type to INVOICE
	if err := w.attachmentRepo.UpdateFileType(ctx, att.ID, entity.FileTypeInvoice); err != nil {
		w.logger.Error("Failed to update file type to INVOICE",
			zap.Int64("attachment_id", att.ID),
			zap.Error(err))
	}

	// Step 6: Mark attachment as PROCESSED
	if err := w.attachmentRepo.UpdateStatus(ctx, att.ID, entity.AttachmentStatusProcessed, ""); err != nil {
		return fmt.Errorf("failed to update status to PROCESSED: %w", err)
	}

	w.logger.Info("Attachment processed successfully",
		zap.Int64("attachment_id", att.ID),
		zap.Int64("invoice_id", invoiceV2.ID),
		zap.String("unique_id", uniqueID))

	return nil
}

// checkInstanceCompletion checks if all attachments for an instance are processed
// and emits TypeInvoiceExtracted event if complete
func (w *InvoiceWorker) checkInstanceCompletion(ctx context.Context, instanceID int64) {
	// Get all attachments for the instance
	totalAttachments, err := w.attachmentRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		w.logger.Error("Failed to get attachments for instance",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		return
	}

	if len(totalAttachments) == 0 {
		w.logger.Debug("No attachments found for instance",
			zap.Int64("instance_id", instanceID))
		return
	}

	// Count processed attachments (PROCESSED or AUDIT_FAILED)
	processedCount, err := w.attachmentRepo.CountByInstanceIDAndStatuses(ctx, instanceID,
		[]string{entity.AttachmentStatusProcessed, entity.AttachmentStatusAuditFailed})
	if err != nil {
		w.logger.Error("Failed to count processed attachments",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		return
	}

	w.logger.Debug("Checking instance completion",
		zap.Int64("instance_id", instanceID),
		zap.Int("total_attachments", len(totalAttachments)),
		zap.Int("processed_count", processedCount))

	// If all attachments are processed, emit event
	if processedCount >= len(totalAttachments) && len(totalAttachments) > 0 {
		w.logger.Info("All attachments processed for instance, emitting event",
			zap.Int64("instance_id", instanceID),
			zap.Int("total_attachments", len(totalAttachments)))

		if w.dispatcher != nil {
			evt := event.NewEvent(event.TypeInvoiceExtracted, instanceID, "", map[string]interface{}{
				"instance_id":       instanceID,
				"total_attachments": len(totalAttachments),
				"processed_count":   processedCount,
			})
			w.dispatcher.DispatchAsync(ctx, evt)
		} else {
			w.logger.Warn("Event dispatcher is nil, cannot emit invoice.extracted event",
				zap.Int64("instance_id", instanceID))
		}
	}
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
