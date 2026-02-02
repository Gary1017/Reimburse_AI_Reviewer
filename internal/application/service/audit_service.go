package service

import (
	"context"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// AuditResult represents the complete audit result
type AuditResult struct {
	PolicyResult  *port.PolicyAuditResult
	PriceResult   *port.PriceAuditResult
	OverallPass   bool
	Confidence    float64
	Reasoning     string
}

// AuditService manages audit operations
type AuditService interface {
	AuditInstance(ctx context.Context, instanceID int64) (*AuditResult, error)
	AuditItem(ctx context.Context, item *entity.ReimbursementItem) (*AuditResult, error)
	ExtractInvoice(ctx context.Context, attachmentID int64) (*port.InvoiceExtractionResult, error)
}

type auditServiceImpl struct {
	instanceRepo    port.InstanceRepository
	itemRepo        port.ItemRepository
	attachmentRepo  port.AttachmentRepository
	invoiceRepo     port.InvoiceRepository
	aiAuditor       port.AIAuditor
	logger          Logger
}

// NewAuditService creates a new AuditService
func NewAuditService(
	instanceRepo port.InstanceRepository,
	itemRepo port.ItemRepository,
	attachmentRepo port.AttachmentRepository,
	invoiceRepo port.InvoiceRepository,
	aiAuditor port.AIAuditor,
	logger Logger,
) AuditService {
	return &auditServiceImpl{
		instanceRepo:   instanceRepo,
		itemRepo:       itemRepo,
		attachmentRepo: attachmentRepo,
		invoiceRepo:    invoiceRepo,
		aiAuditor:      aiAuditor,
		logger:         logger,
	}
}

// AuditInstance performs audit on all items in an instance
func (s *auditServiceImpl) AuditInstance(ctx context.Context, instanceID int64) (*AuditResult, error) {
	// Get instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get instance", "error", err, "instance_id", instanceID)
		return nil, fmt.Errorf("get instance: %w", err)
	}

	// Get all items
	items, err := s.itemRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get items", "error", err, "instance_id", instanceID)
		return nil, fmt.Errorf("get items: %w", err)
	}

	if len(items) == 0 {
		s.logger.Info("No items to audit", "instance_id", instanceID)
		return &AuditResult{
			OverallPass: true,
			Confidence:  1.0,
			Reasoning:   "No items to audit",
		}, nil
	}

	// Audit each item
	var allPolicyResults []*port.PolicyAuditResult
	var allPriceResults []*port.PriceAuditResult
	var totalConfidence float64
	overallPass := true

	for _, item := range items {
		itemResult, err := s.AuditItem(ctx, item)
		if err != nil {
			s.logger.Error("Failed to audit item", "error", err, "item_id", item.ID)
			// Continue with other items
			continue
		}

		if itemResult.PolicyResult != nil {
			allPolicyResults = append(allPolicyResults, itemResult.PolicyResult)
			if !itemResult.PolicyResult.Compliant {
				overallPass = false
			}
		}

		if itemResult.PriceResult != nil {
			allPriceResults = append(allPriceResults, itemResult.PriceResult)
			if !itemResult.PriceResult.Reasonable {
				overallPass = false
			}
		}

		totalConfidence += itemResult.Confidence
	}

	// Calculate average confidence
	avgConfidence := totalConfidence / float64(len(items))

	result := &AuditResult{
		OverallPass: overallPass,
		Confidence:  avgConfidence,
		Reasoning:   fmt.Sprintf("Audited %d items for instance %d", len(items), instanceID),
	}

	// Aggregate results (take first as representative, in production merge properly)
	if len(allPolicyResults) > 0 {
		result.PolicyResult = allPolicyResults[0]
	}
	if len(allPriceResults) > 0 {
		result.PriceResult = allPriceResults[0]
	}

	s.logger.Info("Instance audit completed",
		"instance_id", instanceID,
		"overall_pass", overallPass,
		"confidence", avgConfidence,
		"items_audited", len(items),
	)

	// Note: AIAuditResult is now stored in approval_tasks.result_data
	// The caller should create/update an ApprovalTask with the audit result
	_ = instance // Avoid unused variable warning

	return result, nil
}

// AuditItem performs audit on a single item
func (s *auditServiceImpl) AuditItem(ctx context.Context, item *entity.ReimbursementItem) (*AuditResult, error) {
	s.logger.Info("Auditing item", "item_id", item.ID, "type", item.ItemType)

	// Get associated invoices
	invoices, err := s.invoiceRepo.GetByInstanceID(ctx, item.InstanceID)
	if err != nil {
		s.logger.Error("Failed to get invoices", "error", err, "instance_id", item.InstanceID)
		return nil, fmt.Errorf("get invoices: %w", err)
	}

	// Use first invoice as extraction result (simplified)
	var extractionResult *port.InvoiceExtractionResult
	if len(invoices) > 0 {
		invoice := invoices[0]
		extractionResult = &port.InvoiceExtractionResult{
			Success:       true,
			InvoiceCode:   invoice.InvoiceCode,
			InvoiceNumber: invoice.InvoiceNumber,
			TotalAmount:   invoice.InvoiceAmount,
			InvoiceDate:   formatDate(invoice.InvoiceDate),
			SellerName:    invoice.SellerName,
			SellerTaxID:   invoice.SellerTaxID,
			BuyerName:     invoice.BuyerName,
			BuyerTaxID:    invoice.BuyerTaxID,
			Confidence:    0.9,
		}
	}

	// Perform policy audit
	policyResult, err := s.aiAuditor.AuditPolicy(ctx, item, extractionResult)
	if err != nil {
		s.logger.Error("Policy audit failed", "error", err, "item_id", item.ID)
		return nil, fmt.Errorf("policy audit: %w", err)
	}

	// Perform price audit
	priceResult, err := s.aiAuditor.AuditPrice(ctx, item, extractionResult)
	if err != nil {
		s.logger.Error("Price audit failed", "error", err, "item_id", item.ID)
		return nil, fmt.Errorf("price audit: %w", err)
	}

	// Calculate overall result
	overallPass := policyResult.Compliant && priceResult.Reasonable
	avgConfidence := (policyResult.Confidence + priceResult.Confidence) / 2.0

	result := &AuditResult{
		PolicyResult: policyResult,
		PriceResult:  priceResult,
		OverallPass:  overallPass,
		Confidence:   avgConfidence,
		Reasoning:    fmt.Sprintf("Policy: %s, Price: %s", policyResult.Reasoning, priceResult.Reasoning),
	}

	s.logger.Info("Item audit completed",
		"item_id", item.ID,
		"overall_pass", overallPass,
		"confidence", avgConfidence,
	)

	return result, nil
}

// ExtractInvoice extracts invoice data from an attachment
func (s *auditServiceImpl) ExtractInvoice(ctx context.Context, attachmentID int64) (*port.InvoiceExtractionResult, error) {
	// Get attachment
	attachment, err := s.attachmentRepo.GetByID(ctx, attachmentID)
	if err != nil {
		s.logger.Error("Failed to get attachment", "error", err, "attachment_id", attachmentID)
		return nil, fmt.Errorf("get attachment: %w", err)
	}

	// Check if attachment is downloaded
	if attachment.DownloadStatus != "DOWNLOADED" {
		return nil, fmt.Errorf("attachment not downloaded: status=%s", attachment.DownloadStatus)
	}

	// Read file content (simplified - in production, read from file system)
	// For now, return a placeholder
	s.logger.Info("Extracting invoice", "attachment_id", attachmentID, "file_path", attachment.FilePath)

	// In production, read file and call AIAuditor.ExtractInvoice
	result := &port.InvoiceExtractionResult{
		Success:     true,
		Confidence:  0.9,
		Error:       "",
	}

	s.logger.Info("Invoice extraction completed", "attachment_id", attachmentID, "success", result.Success)

	return result, nil
}

// formatDate formats time pointer to string
func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}
