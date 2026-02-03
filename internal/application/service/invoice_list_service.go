package service

import (
	"context"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// InvoiceListService manages invoice list operations for approval instances.
// Deprecated: This service is deprecated as of migration 019. The invoice_lists table
// has been removed. Use InvoiceV2Repository.GetByInstanceID() and GetTotalsByInstanceID()
// directly instead. This service is kept for backwards compatibility but may be removed
// in a future version.
type InvoiceListService interface {
	// CreateForInstance creates a new invoice list for an instance
	// Deprecated: Invoice lists are no longer used. Returns nil, nil.
	CreateForInstance(ctx context.Context, instanceID int64) (*entity.InvoiceList, error)

	// AddInvoice adds an invoice directly to the instance (invoice_list_id is ignored)
	// Deprecated: Use InvoiceV2Repository.Create() directly instead.
	AddInvoice(ctx context.Context, invoiceListID int64, invoice *entity.InvoiceV2) error

	// GetByInstanceID retrieves invoice totals for an instance
	// Deprecated: Use InvoiceV2Repository.GetTotalsByInstanceID() instead.
	GetByInstanceID(ctx context.Context, instanceID int64) (*entity.InvoiceList, error)

	// GetInvoicesForInstance retrieves all invoices for an instance
	// Deprecated: Use InvoiceV2Repository.GetByInstanceID() instead.
	GetInvoicesForInstance(ctx context.Context, instanceID int64) ([]*entity.InvoiceV2, error)

	// UpdateTotals is a no-op. Totals are computed dynamically.
	// Deprecated: Totals are computed on-the-fly via GetTotalsByInstanceID.
	UpdateTotals(ctx context.Context, invoiceListID int64) error

	// UpdateStatus is a no-op. Invoice list status is no longer tracked.
	// Deprecated: Invoice list status is no longer used.
	UpdateStatus(ctx context.Context, invoiceListID int64, status string) error
}

type invoiceListServiceImpl struct {
	invoiceV2Repo port.InvoiceV2Repository
	txManager     port.TransactionManager
	logger        Logger
}

// NewInvoiceListService creates a new InvoiceListService
// Deprecated: This service is deprecated. Use InvoiceV2Repository directly.
func NewInvoiceListService(
	invoiceListRepo port.InvoiceListRepository, // Deprecated: ignored
	invoiceV2Repo port.InvoiceV2Repository,
	txManager port.TransactionManager,
	logger Logger,
) InvoiceListService {
	return &invoiceListServiceImpl{
		invoiceV2Repo: invoiceV2Repo,
		txManager:     txManager,
		logger:        logger,
	}
}

// CreateForInstance is a no-op. Invoice lists are no longer used.
// Deprecated: Returns nil, nil. Invoices link directly to instances.
func (s *invoiceListServiceImpl) CreateForInstance(ctx context.Context, instanceID int64) (*entity.InvoiceList, error) {
	s.logger.Info("CreateForInstance called (deprecated - no-op)",
		"instance_id", instanceID)
	return nil, nil
}

// AddInvoice adds an invoice directly to the instance.
// The invoiceListID parameter is ignored as invoice lists are deprecated.
// Deprecated: Use InvoiceV2Repository.Create() directly.
func (s *invoiceListServiceImpl) AddInvoice(ctx context.Context, invoiceListID int64, invoice *entity.InvoiceV2) error {
	// invoiceListID is ignored - invoices link directly to instances now
	s.logger.Info("AddInvoice called (deprecated - invoiceListID ignored)",
		"invoice_list_id", invoiceListID,
		"instance_id", invoice.InstanceID)

	if err := s.invoiceV2Repo.Create(ctx, invoice); err != nil {
		s.logger.Error("Failed to add invoice",
			"error", err,
			"instance_id", invoice.InstanceID,
			"invoice_code", invoice.InvoiceCode)
		return fmt.Errorf("create invoice: %w", err)
	}

	s.logger.Info("Invoice added",
		"invoice_id", invoice.ID,
		"instance_id", invoice.InstanceID,
		"invoice_code", invoice.InvoiceCode)

	return nil
}

// GetByInstanceID returns a synthetic InvoiceList computed from invoices.
// Deprecated: Use InvoiceV2Repository.GetTotalsByInstanceID() instead.
func (s *invoiceListServiceImpl) GetByInstanceID(ctx context.Context, instanceID int64) (*entity.InvoiceList, error) {
	totals, err := s.invoiceV2Repo.GetTotalsByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get invoice totals",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("get invoice totals: %w", err)
	}

	// Return a synthetic InvoiceList for backwards compatibility
	return &entity.InvoiceList{
		ID:                      0, // No real ID - computed dynamically
		InstanceID:              instanceID,
		TotalInvoiceCount:       totals.Count,
		TotalInvoiceAmountCents: totals.AmountCents,
		TotalInvoiceAmount:      float64(totals.AmountCents) / 100.0, // Deprecated field
		Status:                  entity.InvoiceListStatusCompleted,   // Always "completed"
	}, nil
}

// GetInvoicesForInstance retrieves all invoices for an instance.
// Deprecated: Use InvoiceV2Repository.GetByInstanceID() instead.
func (s *invoiceListServiceImpl) GetInvoicesForInstance(ctx context.Context, instanceID int64) ([]*entity.InvoiceV2, error) {
	invoices, err := s.invoiceV2Repo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get invoices for instance",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("get invoices: %w", err)
	}
	return invoices, nil
}

// UpdateTotals is a no-op. Totals are computed dynamically.
// Deprecated: Totals are computed on-the-fly via GetTotalsByInstanceID.
func (s *invoiceListServiceImpl) UpdateTotals(ctx context.Context, invoiceListID int64) error {
	s.logger.Info("UpdateTotals called (deprecated - no-op)",
		"invoice_list_id", invoiceListID)
	return nil
}

// UpdateStatus is a no-op. Invoice list status is no longer tracked.
// Deprecated: Invoice list status is no longer used.
func (s *invoiceListServiceImpl) UpdateStatus(ctx context.Context, invoiceListID int64, status string) error {
	s.logger.Info("UpdateStatus called (deprecated - no-op)",
		"invoice_list_id", invoiceListID,
		"status", status)
	return nil
}
