package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/ai"
	"github.com/garyjia/ai-reimbursement/internal/invoice"
	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// InvoiceProcessorRepositoryInterface defines the repository contract
type InvoiceProcessorRepositoryInterface interface {
	GetCompletedAttachments(limit int) ([]*models.Attachment, error)
	UpdateProcessingStatus(tx *sql.Tx, id int64, status string, auditResult string, errMsg string) error
	GetByID(id int64) (*models.Attachment, error)
}

// ReimbursementItemRepositoryInterface defines the item repository contract
type ReimbursementItemRepositoryInterface interface {
	GetByInstanceID(instanceID int64) ([]*models.ReimbursementItem, error)
}

// InvoiceRepositoryInterface defines the invoice repository contract
type InvoiceRepositoryInterface interface {
	Create(tx *sql.Tx, invoice *models.Invoice) error
	CheckUniqueness(uniqueID string) (*models.UniquenessCheckResult, error)
}

// InvoiceProcessorStatus reports current processor status
type InvoiceProcessorStatus struct {
	IsRunning       bool
	LastProcessed   time.Time
	ProcessedCount  int
	FailedCount     int
	UpSinceDuration time.Duration
	IsHealthy       bool
	LastError       error
}

// InvoiceProcessor processes downloaded attachments and performs AI audit
// ARCH-011-B: Invoice Processing Worker implementation
type InvoiceProcessor struct {
	// Configuration
	pollInterval    time.Duration
	batchSize       int
	processTimeout  time.Duration
	attachmentDir   string

	// Dependencies
	attachmentRepo InvoiceProcessorRepositoryInterface
	itemRepo       ReimbursementItemRepositoryInterface
	invoiceRepo    InvoiceRepositoryInterface
	pdfReader      *invoice.PDFReader
	invoiceAuditor *ai.InvoiceAuditor
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

// NewInvoiceProcessor creates a new invoice processor
func NewInvoiceProcessor(
	attachmentRepo InvoiceProcessorRepositoryInterface,
	itemRepo ReimbursementItemRepositoryInterface,
	invoiceRepo InvoiceRepositoryInterface,
	pdfReader *invoice.PDFReader,
	invoiceAuditor *ai.InvoiceAuditor,
	attachmentDir string,
	logger *zap.Logger,
) *InvoiceProcessor {
	return &InvoiceProcessor{
		pollInterval:   10 * time.Second,
		batchSize:      5,
		processTimeout: 120 * time.Second, // 2 minutes for Vision API + AI audit
		attachmentDir:  attachmentDir,
		attachmentRepo: attachmentRepo,
		itemRepo:       itemRepo,
		invoiceRepo:    invoiceRepo,
		pdfReader:      pdfReader,
		invoiceAuditor: invoiceAuditor,
		logger:         logger,
		lastProcessed:  time.Now(),
		startTime:      time.Now(),
	}
}

// Start begins the processor polling loop
func (p *InvoiceProcessor) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.isRunning {
		p.mu.Unlock()
		return fmt.Errorf("processor already running")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.isRunning = true
	p.startTime = time.Now()
	p.mu.Unlock()

	p.logger.Info("InvoiceProcessor started",
		zap.Duration("poll_interval", p.pollInterval),
		zap.Int("batch_size", p.batchSize))

	// Run polling loop in background
	go p.pollLoop()

	return nil
}

// Stop gracefully terminates the processor
func (p *InvoiceProcessor) Stop() {
	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return
	}

	p.isRunning = false
	p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
	}

	p.logger.Info("InvoiceProcessor stopped",
		zap.Int("processed_count", p.processedCount),
		zap.Int("failed_count", p.failedCount))
}

// GetStatus returns current processor status
func (p *InvoiceProcessor) GetStatus() InvoiceProcessorStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	upDuration := time.Since(p.startTime)
	isHealthy := time.Since(p.lastProcessed) < 5*time.Minute && p.isRunning

	return InvoiceProcessorStatus{
		IsRunning:       p.isRunning,
		LastProcessed:   p.lastProcessed,
		ProcessedCount:  p.processedCount,
		FailedCount:     p.failedCount,
		UpSinceDuration: upDuration,
		IsHealthy:       isHealthy,
		LastError:       p.lastError,
	}
}

// pollLoop runs the main polling loop
func (p *InvoiceProcessor) pollLoop() {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debug("Poll loop context cancelled")
			return

		case <-ticker.C:
			if err := p.processCompletedAttachments(); err != nil {
				p.mu.Lock()
				p.lastError = err
				p.mu.Unlock()
				p.logger.Error("Failed to process completed attachments", zap.Error(err))
			}

			p.mu.Lock()
			p.lastProcessed = time.Now()
			p.mu.Unlock()
		}
	}
}

// processCompletedAttachments processes attachments that have been downloaded
func (p *InvoiceProcessor) processCompletedAttachments() error {
	// Get completed attachments that haven't been processed yet
	attachments, err := p.attachmentRepo.GetCompletedAttachments(p.batchSize)
	if err != nil {
		return fmt.Errorf("failed to get completed attachments: %w", err)
	}

	if len(attachments) == 0 {
		return nil
	}

	p.logger.Debug("Processing completed attachments", zap.Int("count", len(attachments)))

	for _, att := range attachments {
		if err := p.processSingleAttachment(att); err != nil {
			p.logger.Warn("Failed to process attachment",
				zap.Int64("attachment_id", att.ID),
				zap.String("file_name", att.FileName),
				zap.Error(err))

			p.mu.Lock()
			p.failedCount++
			p.mu.Unlock()
		} else {
			p.mu.Lock()
			p.processedCount++
			p.mu.Unlock()
		}
	}

	return nil
}

// processSingleAttachment processes a single attachment through the AI pipeline
func (p *InvoiceProcessor) processSingleAttachment(att *models.Attachment) error {
	p.logger.Info("Processing attachment",
		zap.Int64("attachment_id", att.ID),
		zap.String("file_name", att.FileName),
		zap.String("file_path", att.FilePath))

	// Mark as processing
	if err := p.attachmentRepo.UpdateProcessingStatus(nil, att.ID,
		models.AttachmentStatusProcessing, "", ""); err != nil {
		return fmt.Errorf("failed to update status to PROCESSING: %w", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(p.ctx, p.processTimeout)
	defer cancel()

	// Check if file is a PDF or image
	ext := strings.ToLower(filepath.Ext(att.FileName))
	if ext != ".pdf" && ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		errMsg := fmt.Sprintf("unsupported file type: %s", ext)
		p.logger.Warn("Skipping unsupported file type",
			zap.Int64("attachment_id", att.ID),
			zap.String("extension", ext))
		return p.attachmentRepo.UpdateProcessingStatus(nil, att.ID,
			models.AttachmentStatusProcessed, "", errMsg)
	}

	// Step 1: Extract invoice data from PDF using Vision API
	fullPath := att.FilePath
	// Only prepend attachmentDir if the path doesn't already contain it
	if !filepath.IsAbs(fullPath) && !strings.HasPrefix(fullPath, p.attachmentDir) {
		fullPath = filepath.Join(p.attachmentDir, att.FilePath)
	}

	extractedData, err := p.pdfReader.ReadAndExtract(ctx, fullPath)
	if err != nil {
		errMsg := fmt.Sprintf("extraction failed: %v", err)
		p.logger.Error("Failed to extract invoice data",
			zap.Int64("attachment_id", att.ID),
			zap.Error(err))
		return p.attachmentRepo.UpdateProcessingStatus(nil, att.ID,
			models.AttachmentStatusAuditFailed, "", errMsg)
	}

	p.logger.Info("Invoice data extracted",
		zap.Int64("attachment_id", att.ID),
		zap.String("invoice_code", extractedData.InvoiceCode),
		zap.String("invoice_number", extractedData.InvoiceNumber),
		zap.Float64("total_amount", extractedData.TotalAmount))

	// Step 2: Get reimbursement item info for context
	items, err := p.itemRepo.GetByInstanceID(att.InstanceID)
	if err != nil {
		p.logger.Warn("Failed to get reimbursement items", zap.Error(err))
	}

	// Determine claimed amount and category
	var claimedAmount float64
	var expenseCategory string
	if len(items) > 0 {
		// Find the item this attachment belongs to
		for _, item := range items {
			if item.ID == att.ItemID {
				claimedAmount = item.Amount
				expenseCategory = item.ItemType
				break
			}
		}
		// Fallback: use first item if not found
		if claimedAmount == 0 && len(items) > 0 {
			claimedAmount = items[0].Amount
			expenseCategory = items[0].ItemType
		}
	}

	// If no claimed amount, use invoice amount
	if claimedAmount == 0 {
		claimedAmount = extractedData.TotalAmount
	}
	if expenseCategory == "" {
		expenseCategory = "OTHER"
	}

	// Step 3: Perform AI audit
	auditResult, err := p.invoiceAuditor.AuditInvoice(ctx, extractedData, claimedAmount, expenseCategory)
	if err != nil {
		errMsg := fmt.Sprintf("audit failed: %v", err)
		p.logger.Error("Invoice audit failed",
			zap.Int64("attachment_id", att.ID),
			zap.Error(err))
		return p.attachmentRepo.UpdateProcessingStatus(nil, att.ID,
			models.AttachmentStatusAuditFailed, "", errMsg)
	}

	p.logger.Info("Invoice audit completed",
		zap.Int64("attachment_id", att.ID),
		zap.String("decision", auditResult.OverallDecision),
		zap.Float64("confidence", auditResult.OverallConfidence))

	// Step 4: Save invoice record (for uniqueness tracking)
	if extractedData.InvoiceCode != "" && extractedData.InvoiceNumber != "" {
		uniqueID := models.GenerateUniqueID(extractedData.InvoiceCode, extractedData.InvoiceNumber)

		// Check uniqueness first
		uniqueCheck, err := p.invoiceRepo.CheckUniqueness(uniqueID)
		if err != nil {
			p.logger.Warn("Failed to check invoice uniqueness", zap.Error(err))
		} else if !uniqueCheck.IsUnique {
			p.logger.Warn("Duplicate invoice detected",
				zap.String("unique_id", uniqueID),
				zap.Int64("duplicate_invoice_id", uniqueCheck.DuplicateInvoiceID))
			// Add duplicate warning to audit result
			if auditResult.PolicyResult != nil {
				auditResult.PolicyResult.Violations = append(auditResult.PolicyResult.Violations,
					fmt.Sprintf("DUPLICATE: Invoice was previously submitted (ID: %d)", uniqueCheck.DuplicateInvoiceID))
				auditResult.PolicyResult.IsCompliant = false
				auditResult.OverallDecision = "FAIL"
			}
		} else {
			// Save new invoice record
			extractedDataJSON, _ := json.Marshal(extractedData)
			inv := &models.Invoice{
				InvoiceCode:   extractedData.InvoiceCode,
				InvoiceNumber: extractedData.InvoiceNumber,
				UniqueID:      uniqueID,
				InstanceID:    att.InstanceID,
				FilePath:      att.FilePath,
				InvoiceAmount: extractedData.TotalAmount,
				SellerName:    extractedData.SellerName,
				SellerTaxID:   extractedData.SellerTaxID,
				BuyerName:     extractedData.BuyerName,
				BuyerTaxID:    extractedData.BuyerTaxID,
				ExtractedData: string(extractedDataJSON),
			}
			if err := p.invoiceRepo.Create(nil, inv); err != nil {
				p.logger.Warn("Failed to save invoice record", zap.Error(err))
			}
		}
	}

	// Step 5: Serialize audit result and update attachment
	auditResultJSON, err := json.Marshal(auditResult)
	if err != nil {
		p.logger.Error("Failed to serialize audit result", zap.Error(err))
		auditResultJSON = []byte("{}")
	}

	// Update attachment with audit result
	return p.attachmentRepo.UpdateProcessingStatus(nil, att.ID,
		models.AttachmentStatusProcessed, string(auditResultJSON), "")
}

// SetPollInterval sets the polling interval (for testing)
func (p *InvoiceProcessor) SetPollInterval(interval time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pollInterval = interval
}

// SetBatchSize sets the batch size (for testing)
func (p *InvoiceProcessor) SetBatchSize(size int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.batchSize = size
}

// ProcessNow processes completed attachments immediately (for testing)
func (p *InvoiceProcessor) ProcessNow() error {
	return p.processCompletedAttachments()
}
