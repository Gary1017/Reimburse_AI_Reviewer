package service

import (
	"context"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// InvoiceListService manages invoice list operations for approval instances.
// Each approval instance has exactly one invoice list (1:1 relationship).
type InvoiceListService interface {
	// CreateForInstance creates a new invoice list for an instance
	CreateForInstance(ctx context.Context, instanceID int64) (*entity.InvoiceList, error)

	// AddInvoice adds an invoice to the invoice list and updates totals
	AddInvoice(ctx context.Context, invoiceListID int64, invoice *entity.InvoiceV2) error

	// GetByInstanceID retrieves the invoice list for an instance
	GetByInstanceID(ctx context.Context, instanceID int64) (*entity.InvoiceList, error)

	// GetInvoicesForInstance retrieves all invoices for an instance
	GetInvoicesForInstance(ctx context.Context, instanceID int64) ([]*entity.InvoiceV2, error)

	// UpdateTotals recalculates totals from invoices
	UpdateTotals(ctx context.Context, invoiceListID int64) error

	// UpdateStatus updates the status of an invoice list
	UpdateStatus(ctx context.Context, invoiceListID int64, status string) error
}

type invoiceListServiceImpl struct {
	invoiceListRepo port.InvoiceListRepository
	invoiceV2Repo   port.InvoiceV2Repository
	txManager       port.TransactionManager
	logger          Logger
}

// NewInvoiceListService creates a new InvoiceListService
func NewInvoiceListService(
	invoiceListRepo port.InvoiceListRepository,
	invoiceV2Repo port.InvoiceV2Repository,
	txManager port.TransactionManager,
	logger Logger,
) InvoiceListService {
	return &invoiceListServiceImpl{
		invoiceListRepo: invoiceListRepo,
		invoiceV2Repo:   invoiceV2Repo,
		txManager:       txManager,
		logger:          logger,
	}
}

// CreateForInstance creates a new invoice list for an instance.
// Returns existing list if one already exists (idempotent).
func (s *invoiceListServiceImpl) CreateForInstance(ctx context.Context, instanceID int64) (*entity.InvoiceList, error) {
	// Check if invoice list already exists (idempotency)
	existing, err := s.invoiceListRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to check existing invoice list",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("check existing invoice list: %w", err)
	}
	if existing != nil {
		s.logger.Info("Invoice list already exists",
			"instance_id", instanceID,
			"invoice_list_id", existing.ID)
		return existing, nil
	}

	// Create new invoice list
	invoiceList := &entity.InvoiceList{
		InstanceID:         instanceID,
		TotalInvoiceCount:  0,
		TotalInvoiceAmount: 0,
		Status:             entity.InvoiceListStatusPending,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := s.invoiceListRepo.Create(ctx, invoiceList); err != nil {
		s.logger.Error("Failed to create invoice list",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("create invoice list: %w", err)
	}

	s.logger.Info("Invoice list created",
		"instance_id", instanceID,
		"invoice_list_id", invoiceList.ID)

	return invoiceList, nil
}

// AddInvoice adds an invoice to the invoice list and updates totals.
// Uses a transaction to ensure atomicity.
func (s *invoiceListServiceImpl) AddInvoice(ctx context.Context, invoiceListID int64, invoice *entity.InvoiceV2) error {
	// Set the invoice list ID
	invoice.InvoiceListID = invoiceListID

	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Create the invoice
		if err := s.invoiceV2Repo.Create(txCtx, invoice); err != nil {
			return fmt.Errorf("create invoice: %w", err)
		}

		// Update totals
		if err := s.recalculateTotals(txCtx, invoiceListID); err != nil {
			return fmt.Errorf("update totals: %w", err)
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Failed to add invoice",
			"error", err,
			"invoice_list_id", invoiceListID,
			"invoice_code", invoice.InvoiceCode)
		return err
	}

	s.logger.Info("Invoice added to list",
		"invoice_list_id", invoiceListID,
		"invoice_id", invoice.ID,
		"invoice_code", invoice.InvoiceCode)

	return nil
}

// GetByInstanceID retrieves the invoice list for an instance.
func (s *invoiceListServiceImpl) GetByInstanceID(ctx context.Context, instanceID int64) (*entity.InvoiceList, error) {
	invoiceList, err := s.invoiceListRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get invoice list",
			"error", err,
			"instance_id", instanceID)
		return nil, fmt.Errorf("get invoice list: %w", err)
	}
	return invoiceList, nil
}

// GetInvoicesForInstance retrieves all invoices for an instance.
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

// UpdateTotals recalculates totals from invoices.
func (s *invoiceListServiceImpl) UpdateTotals(ctx context.Context, invoiceListID int64) error {
	if err := s.recalculateTotals(ctx, invoiceListID); err != nil {
		s.logger.Error("Failed to update totals",
			"error", err,
			"invoice_list_id", invoiceListID)
		return err
	}

	s.logger.Info("Invoice list totals updated",
		"invoice_list_id", invoiceListID)

	return nil
}

// UpdateStatus updates the status of an invoice list.
func (s *invoiceListServiceImpl) UpdateStatus(ctx context.Context, invoiceListID int64, status string) error {
	if err := s.invoiceListRepo.UpdateStatus(ctx, invoiceListID, status); err != nil {
		s.logger.Error("Failed to update invoice list status",
			"error", err,
			"invoice_list_id", invoiceListID,
			"status", status)
		return fmt.Errorf("update invoice list status: %w", err)
	}

	s.logger.Info("Invoice list status updated",
		"invoice_list_id", invoiceListID,
		"status", status)

	return nil
}

// recalculateTotals recalculates the total count and amount from invoices.
func (s *invoiceListServiceImpl) recalculateTotals(ctx context.Context, invoiceListID int64) error {
	// Get all invoices in the list
	invoices, err := s.invoiceV2Repo.GetByInvoiceListID(ctx, invoiceListID)
	if err != nil {
		return fmt.Errorf("get invoices: %w", err)
	}

	// Calculate totals
	count := len(invoices)
	var totalAmount float64
	for _, inv := range invoices {
		totalAmount += inv.InvoiceAmount
	}

	// Update the invoice list
	if err := s.invoiceListRepo.UpdateTotals(ctx, invoiceListID, count, totalAmount); err != nil {
		return fmt.Errorf("update invoice list totals: %w", err)
	}

	return nil
}
