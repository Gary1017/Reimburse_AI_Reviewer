package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// InvoiceRepository implements port.InvoiceRepository
type InvoiceRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewInvoiceRepository creates a new invoice repository
func NewInvoiceRepository(db *sql.DB, logger *zap.Logger) port.InvoiceRepository {
	return &InvoiceRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new invoice record
func (r *InvoiceRepository) Create(ctx context.Context, invoice *entity.Invoice) error {
	query := `
		INSERT INTO invoices (
			invoice_code, invoice_number, unique_id, instance_id, file_token,
			file_path, invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		invoice.InvoiceCode,
		invoice.InvoiceNumber,
		invoice.UniqueID,
		invoice.InstanceID,
		invoice.FileToken,
		invoice.FilePath,
		invoice.InvoiceDate,
		invoice.InvoiceAmount,
		invoice.SellerName,
		invoice.SellerTaxID,
		invoice.BuyerName,
		invoice.BuyerTaxID,
		invoice.ExtractedData,
	)
	if err != nil {
		r.logger.Error("Failed to create invoice", zap.Error(err))
		return fmt.Errorf("failed to create invoice: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	invoice.ID = id
	return nil
}

// GetByID retrieves an invoice by ID
func (r *InvoiceRepository) GetByID(ctx context.Context, id int64) (*entity.Invoice, error) {
	query := `
		SELECT id, invoice_code, invoice_number, unique_id, instance_id, file_token,
			file_path, invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices
		WHERE id = ?
	`

	var invoice entity.Invoice
	var invoiceDate sql.NullTime

	err := r.getExecutor(ctx).QueryRowContext(ctx, query, id).Scan(
		&invoice.ID,
		&invoice.InvoiceCode,
		&invoice.InvoiceNumber,
		&invoice.UniqueID,
		&invoice.InstanceID,
		&invoice.FileToken,
		&invoice.FilePath,
		&invoiceDate,
		&invoice.InvoiceAmount,
		&invoice.SellerName,
		&invoice.SellerTaxID,
		&invoice.BuyerName,
		&invoice.BuyerTaxID,
		&invoice.ExtractedData,
		&invoice.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get invoice by ID", zap.Int64("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	if invoiceDate.Valid {
		invoice.InvoiceDate = &invoiceDate.Time
	}

	return &invoice, nil
}

// GetByAttachmentID retrieves an invoice by attachment ID
func (r *InvoiceRepository) GetByAttachmentID(ctx context.Context, attachmentID int64) (*entity.Invoice, error) {
	// Note: The current schema doesn't have attachment_id in invoices table
	// This implementation assumes file_token is used for linking
	// You may need to adjust based on actual schema
	query := `
		SELECT id, invoice_code, invoice_number, unique_id, instance_id, file_token,
			file_path, invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices
		WHERE file_token = (SELECT file_name FROM attachments WHERE id = ?)
		LIMIT 1
	`

	var invoice entity.Invoice
	var invoiceDate sql.NullTime

	err := r.getExecutor(ctx).QueryRowContext(ctx, query, attachmentID).Scan(
		&invoice.ID,
		&invoice.InvoiceCode,
		&invoice.InvoiceNumber,
		&invoice.UniqueID,
		&invoice.InstanceID,
		&invoice.FileToken,
		&invoice.FilePath,
		&invoiceDate,
		&invoice.InvoiceAmount,
		&invoice.SellerName,
		&invoice.SellerTaxID,
		&invoice.BuyerName,
		&invoice.BuyerTaxID,
		&invoice.ExtractedData,
		&invoice.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get invoice by attachment ID", zap.Int64("attachment_id", attachmentID), zap.Error(err))
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	if invoiceDate.Valid {
		invoice.InvoiceDate = &invoiceDate.Time
	}

	return &invoice, nil
}

// GetByInstanceID retrieves all invoices for an instance
func (r *InvoiceRepository) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Invoice, error) {
	query := `
		SELECT id, invoice_code, invoice_number, unique_id, instance_id, file_token,
			file_path, invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices
		WHERE instance_id = ?
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, instanceID)
	if err != nil {
		r.logger.Error("Failed to get invoices by instance ID", zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get invoices: %w", err)
	}
	defer rows.Close()

	var invoices []*entity.Invoice
	for rows.Next() {
		var invoice entity.Invoice
		var invoiceDate sql.NullTime

		err := rows.Scan(
			&invoice.ID,
			&invoice.InvoiceCode,
			&invoice.InvoiceNumber,
			&invoice.UniqueID,
			&invoice.InstanceID,
			&invoice.FileToken,
			&invoice.FilePath,
			&invoiceDate,
			&invoice.InvoiceAmount,
			&invoice.SellerName,
			&invoice.SellerTaxID,
			&invoice.BuyerName,
			&invoice.BuyerTaxID,
			&invoice.ExtractedData,
			&invoice.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invoice: %w", err)
		}

		if invoiceDate.Valid {
			invoice.InvoiceDate = &invoiceDate.Time
		}

		invoices = append(invoices, &invoice)
	}

	return invoices, rows.Err()
}

// Update updates an invoice
func (r *InvoiceRepository) Update(ctx context.Context, invoice *entity.Invoice) error {
	query := `
		UPDATE invoices
		SET invoice_code = ?, invoice_number = ?, unique_id = ?, file_token = ?,
			file_path = ?, invoice_date = ?, invoice_amount = ?, seller_name = ?,
			seller_tax_id = ?, buyer_name = ?, buyer_tax_id = ?, extracted_data = ?
		WHERE id = ?
	`

	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		invoice.InvoiceCode,
		invoice.InvoiceNumber,
		invoice.UniqueID,
		invoice.FileToken,
		invoice.FilePath,
		invoice.InvoiceDate,
		invoice.InvoiceAmount,
		invoice.SellerName,
		invoice.SellerTaxID,
		invoice.BuyerName,
		invoice.BuyerTaxID,
		invoice.ExtractedData,
		invoice.ID,
	)
	if err != nil {
		r.logger.Error("Failed to update invoice", zap.Int64("id", invoice.ID), zap.Error(err))
		return fmt.Errorf("failed to update invoice: %w", err)
	}

	return nil
}

// GetByUniqueID retrieves an invoice by its unique ID
func (r *InvoiceRepository) GetByUniqueID(ctx context.Context, uniqueID string) (*entity.Invoice, error) {
	query := `
		SELECT id, invoice_code, invoice_number, unique_id, instance_id, file_token,
			file_path, invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices
		WHERE unique_id = ?
	`

	var invoice entity.Invoice
	var invoiceDate sql.NullTime

	err := r.getExecutor(ctx).QueryRowContext(ctx, query, uniqueID).Scan(
		&invoice.ID,
		&invoice.InvoiceCode,
		&invoice.InvoiceNumber,
		&invoice.UniqueID,
		&invoice.InstanceID,
		&invoice.FileToken,
		&invoice.FilePath,
		&invoiceDate,
		&invoice.InvoiceAmount,
		&invoice.SellerName,
		&invoice.SellerTaxID,
		&invoice.BuyerName,
		&invoice.BuyerTaxID,
		&invoice.ExtractedData,
		&invoice.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get invoice by unique ID", zap.String("unique_id", uniqueID), zap.Error(err))
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	if invoiceDate.Valid {
		invoice.InvoiceDate = &invoiceDate.Time
	}

	return &invoice, nil
}

// getExecutor returns appropriate executor based on context
func (r *InvoiceRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// Verify interface compliance
var _ port.InvoiceRepository = (*InvoiceRepository)(nil)
