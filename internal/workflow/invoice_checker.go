package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/invoice"
	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/garyjia/ai-reimbursement/internal/repository"
	"github.com/garyjia/ai-reimbursement/pkg/database"
	"go.uber.org/zap"
)

// InvoiceChecker checks invoice uniqueness and validity
type InvoiceChecker struct {
	db              *database.DB
	invoiceRepo     *repository.InvoiceRepository
	invoiceExtractor *invoice.Extractor
	logger          *zap.Logger
}

// NewInvoiceChecker creates a new invoice checker
func NewInvoiceChecker(
	db *database.DB,
	invoiceRepo *repository.InvoiceRepository,
	invoiceExtractor *invoice.Extractor,
	logger *zap.Logger,
) *InvoiceChecker {
	return &InvoiceChecker{
		db:              db,
		invoiceRepo:     invoiceRepo,
		invoiceExtractor: invoiceExtractor,
		logger:          logger,
	}
}

// CheckInstanceInvoices checks all invoices for an approval instance
func (ic *InvoiceChecker) CheckInstanceInvoices(ctx context.Context, instanceID int64, attachmentPaths []string) error {
	ic.logger.Info("Checking invoices for instance", 
		zap.Int64("instance_id", instanceID),
		zap.Int("attachment_count", len(attachmentPaths)))

	if len(attachmentPaths) == 0 {
		ic.logger.Warn("No attachments to check", zap.Int64("instance_id", instanceID))
		return nil
	}

	for _, path := range attachmentPaths {
		// Only process PDF files (invoices are typically PDFs)
		if !ic.isPDF(path) {
			ic.logger.Debug("Skipping non-PDF file", zap.String("path", path))
			continue
		}

		if err := ic.processInvoice(ctx, instanceID, path); err != nil {
			ic.logger.Error("Failed to process invoice",
				zap.String("path", path),
				zap.Error(err))
			// Continue with other invoices even if one fails
			continue
		}
	}

	return nil
}

// processInvoice processes a single invoice file
func (ic *InvoiceChecker) processInvoice(ctx context.Context, instanceID int64, filePath string) error {
	ic.logger.Info("Processing invoice", 
		zap.Int64("instance_id", instanceID),
		zap.String("file_path", filePath))

	// Extract invoice data using AI
	extractedData, err := ic.invoiceExtractor.ExtractFromPDF(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to extract invoice data: %w", err)
	}

	// Generate unique ID
	uniqueID := models.GenerateUniqueID(extractedData.InvoiceCode, extractedData.InvoiceNumber)

	// Check uniqueness
	uniquenessResult, err := ic.invoiceRepo.CheckUniqueness(uniqueID)
	if err != nil {
		return fmt.Errorf("failed to check uniqueness: %w", err)
	}

	if !uniquenessResult.IsUnique {
		ic.logger.Warn("Duplicate invoice detected",
			zap.String("unique_id", uniqueID),
			zap.Int64("duplicate_instance_id", uniquenessResult.DuplicateInstanceID),
			zap.Time("first_seen_at", *uniquenessResult.FirstSeenAt))
		
		// Record validation failure
		return fmt.Errorf("duplicate invoice: %s (first seen at instance %d on %s)",
			uniqueID,
			uniquenessResult.DuplicateInstanceID,
			uniquenessResult.FirstSeenAt.Format("2006-01-02"))
	}

	// Convert extracted data to JSON
	extractedJSON, err := json.Marshal(extractedData)
	if err != nil {
		return fmt.Errorf("failed to marshal extracted data: %w", err)
	}

	// Create invoice record
	invoiceRecord := &models.Invoice{
		InvoiceCode:   extractedData.InvoiceCode,
		InvoiceNumber: extractedData.InvoiceNumber,
		UniqueID:      uniqueID,
		InstanceID:    instanceID,
		FilePath:      filePath,
		InvoiceAmount: extractedData.TotalAmount,
		SellerName:    extractedData.SellerName,
		SellerTaxID:   extractedData.SellerTaxID,
		BuyerName:     extractedData.BuyerName,
		BuyerTaxID:    extractedData.BuyerTaxID,
		ExtractedData: string(extractedJSON),
	}

	// Parse invoice date if available
	if extractedData.InvoiceDate != "" {
		// TODO: Parse date from string format
	}

	// Save invoice in transaction
	err = ic.db.WithTransaction(func(tx error) error {
		if err := ic.invoiceRepo.Create(nil, invoiceRecord); err != nil {
			return fmt.Errorf("failed to create invoice record: %w", err)
		}

		// Record successful validation
		validation := &models.InvoiceValidation{
			InvoiceID:      invoiceRecord.ID,
			ValidationType: models.ValidationTypeUniqueness,
			IsValid:        true,
			ValidationData: string(extractedJSON),
		}

		if err := ic.invoiceRepo.CreateValidation(nil, validation); err != nil {
			return fmt.Errorf("failed to create validation record: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	ic.logger.Info("Invoice processed successfully",
		zap.String("unique_id", uniqueID),
		zap.Int64("invoice_id", invoiceRecord.ID),
		zap.Float64("amount", invoiceRecord.InvoiceAmount))

	return nil
}

// isPDF checks if a file is a PDF based on extension
func (ic *InvoiceChecker) isPDF(filePath string) bool {
	// Simple check - in production, you might want to check magic bytes
	return len(filePath) > 4 && filePath[len(filePath)-4:] == ".pdf"
}
