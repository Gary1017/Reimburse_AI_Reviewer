package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// InvoiceV2Repository implements port.InvoiceV2Repository
type InvoiceV2Repository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewInvoiceV2Repository creates a new invoice v2 repository
func NewInvoiceV2Repository(db *sql.DB, logger *zap.Logger) port.InvoiceV2Repository {
	return &InvoiceV2Repository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new invoice record
func (r *InvoiceV2Repository) Create(ctx context.Context, invoice *entity.InvoiceV2) error {
	query := `
		INSERT INTO invoices_v2 (
			invoice_list_id, attachment_id, item_id,
			invoice_code, invoice_number, unique_id,
			invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Handle nullable invoice_date
	var invoiceDate interface{}
	if invoice.InvoiceDate != nil {
		invoiceDate = invoice.InvoiceDate
	}

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		invoice.InvoiceListID,
		invoice.AttachmentID,
		invoice.ItemID,
		invoice.InvoiceCode,
		invoice.InvoiceNumber,
		invoice.UniqueID,
		invoiceDate,
		invoice.InvoiceAmount,
		invoice.SellerName,
		invoice.SellerTaxID,
		invoice.BuyerName,
		invoice.BuyerTaxID,
		invoice.ExtractedData,
	)
	if err != nil {
		r.logger.Error("Failed to create invoice v2",
			zap.Int64("invoice_list_id", invoice.InvoiceListID),
			zap.Int64("attachment_id", invoice.AttachmentID),
			zap.Error(err))
		return fmt.Errorf("failed to create invoice v2: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	invoice.ID = id
	return nil
}

// GetByID retrieves an invoice by its ID
func (r *InvoiceV2Repository) GetByID(ctx context.Context, id int64) (*entity.InvoiceV2, error) {
	query := `
		SELECT id, invoice_list_id, attachment_id, item_id,
			invoice_code, invoice_number, unique_id,
			invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices_v2
		WHERE id = ?
	`

	invoice, err := r.scanInvoice(r.getExecutor(ctx).QueryRowContext(ctx, query, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get invoice v2 by ID",
			zap.Int64("id", id),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get invoice v2: %w", err)
	}

	return invoice, nil
}

// GetByAttachmentID retrieves invoice by attachment ID (1:1 relationship)
func (r *InvoiceV2Repository) GetByAttachmentID(ctx context.Context, attachmentID int64) (*entity.InvoiceV2, error) {
	query := `
		SELECT id, invoice_list_id, attachment_id, item_id,
			invoice_code, invoice_number, unique_id,
			invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices_v2
		WHERE attachment_id = ?
	`

	invoice, err := r.scanInvoice(r.getExecutor(ctx).QueryRowContext(ctx, query, attachmentID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get invoice v2 by attachment ID",
			zap.Int64("attachment_id", attachmentID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get invoice v2: %w", err)
	}

	return invoice, nil
}

// GetByItemID retrieves invoice by item ID
func (r *InvoiceV2Repository) GetByItemID(ctx context.Context, itemID int64) (*entity.InvoiceV2, error) {
	query := `
		SELECT id, invoice_list_id, attachment_id, item_id,
			invoice_code, invoice_number, unique_id,
			invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices_v2
		WHERE item_id = ?
	`

	invoice, err := r.scanInvoice(r.getExecutor(ctx).QueryRowContext(ctx, query, itemID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get invoice v2 by item ID",
			zap.Int64("item_id", itemID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get invoice v2: %w", err)
	}

	return invoice, nil
}

// GetByInvoiceListID retrieves all invoices in an invoice list
func (r *InvoiceV2Repository) GetByInvoiceListID(ctx context.Context, invoiceListID int64) ([]*entity.InvoiceV2, error) {
	query := `
		SELECT id, invoice_list_id, attachment_id, item_id,
			invoice_code, invoice_number, unique_id,
			invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices_v2
		WHERE invoice_list_id = ?
		ORDER BY id
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, invoiceListID)
	if err != nil {
		r.logger.Error("Failed to get invoices v2 by invoice list ID",
			zap.Int64("invoice_list_id", invoiceListID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get invoices v2: %w", err)
	}
	defer rows.Close()

	return r.scanInvoices(rows)
}

// GetByInstanceID retrieves all invoices for an instance (via invoice_list)
func (r *InvoiceV2Repository) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.InvoiceV2, error) {
	query := `
		SELECT iv.id, iv.invoice_list_id, iv.attachment_id, iv.item_id,
			iv.invoice_code, iv.invoice_number, iv.unique_id,
			iv.invoice_date, iv.invoice_amount, iv.seller_name, iv.seller_tax_id,
			iv.buyer_name, iv.buyer_tax_id, iv.extracted_data, iv.created_at
		FROM invoices_v2 iv
		INNER JOIN invoice_lists il ON iv.invoice_list_id = il.id
		WHERE il.instance_id = ?
		ORDER BY iv.id
	`

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, instanceID)
	if err != nil {
		r.logger.Error("Failed to get invoices v2 by instance ID",
			zap.Int64("instance_id", instanceID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get invoices v2: %w", err)
	}
	defer rows.Close()

	return r.scanInvoices(rows)
}

// GetByUniqueID retrieves invoice by unique ID (code + number)
func (r *InvoiceV2Repository) GetByUniqueID(ctx context.Context, uniqueID string) (*entity.InvoiceV2, error) {
	query := `
		SELECT id, invoice_list_id, attachment_id, item_id,
			invoice_code, invoice_number, unique_id,
			invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices_v2
		WHERE unique_id = ?
	`

	invoice, err := r.scanInvoice(r.getExecutor(ctx).QueryRowContext(ctx, query, uniqueID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get invoice v2 by unique ID",
			zap.String("unique_id", uniqueID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get invoice v2: %w", err)
	}

	return invoice, nil
}

// Update updates an existing invoice
func (r *InvoiceV2Repository) Update(ctx context.Context, invoice *entity.InvoiceV2) error {
	query := `
		UPDATE invoices_v2
		SET invoice_list_id = ?, attachment_id = ?, item_id = ?,
			invoice_code = ?, invoice_number = ?, unique_id = ?,
			invoice_date = ?, invoice_amount = ?, seller_name = ?, seller_tax_id = ?,
			buyer_name = ?, buyer_tax_id = ?, extracted_data = ?
		WHERE id = ?
	`

	// Handle nullable invoice_date
	var invoiceDate interface{}
	if invoice.InvoiceDate != nil {
		invoiceDate = invoice.InvoiceDate
	}

	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		invoice.InvoiceListID,
		invoice.AttachmentID,
		invoice.ItemID,
		invoice.InvoiceCode,
		invoice.InvoiceNumber,
		invoice.UniqueID,
		invoiceDate,
		invoice.InvoiceAmount,
		invoice.SellerName,
		invoice.SellerTaxID,
		invoice.BuyerName,
		invoice.BuyerTaxID,
		invoice.ExtractedData,
		invoice.ID,
	)
	if err != nil {
		r.logger.Error("Failed to update invoice v2",
			zap.Int64("id", invoice.ID),
			zap.Error(err))
		return fmt.Errorf("failed to update invoice v2: %w", err)
	}

	return nil
}

// scanInvoice scans a single invoice row
func (r *InvoiceV2Repository) scanInvoice(row *sql.Row) (*entity.InvoiceV2, error) {
	var invoice entity.InvoiceV2
	var invoiceDate sql.NullTime
	var itemID sql.NullInt64

	err := row.Scan(
		&invoice.ID,
		&invoice.InvoiceListID,
		&invoice.AttachmentID,
		&itemID,
		&invoice.InvoiceCode,
		&invoice.InvoiceNumber,
		&invoice.UniqueID,
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
		return nil, err
	}

	if invoiceDate.Valid {
		invoice.InvoiceDate = &invoiceDate.Time
	}
	if itemID.Valid {
		invoice.ItemID = itemID.Int64
	}

	return &invoice, nil
}

// scanInvoices scans multiple invoice rows
func (r *InvoiceV2Repository) scanInvoices(rows *sql.Rows) ([]*entity.InvoiceV2, error) {
	var invoices []*entity.InvoiceV2

	for rows.Next() {
		var invoice entity.InvoiceV2
		var invoiceDate sql.NullTime
		var itemID sql.NullInt64

		err := rows.Scan(
			&invoice.ID,
			&invoice.InvoiceListID,
			&invoice.AttachmentID,
			&itemID,
			&invoice.InvoiceCode,
			&invoice.InvoiceNumber,
			&invoice.UniqueID,
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
			return nil, fmt.Errorf("failed to scan invoice v2: %w", err)
		}

		if invoiceDate.Valid {
			invoice.InvoiceDate = &invoiceDate.Time
		}
		if itemID.Valid {
			invoice.ItemID = itemID.Int64
		}

		invoices = append(invoices, &invoice)
	}

	return invoices, rows.Err()
}

// getExecutor returns appropriate executor based on context
func (r *InvoiceV2Repository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// Verify interface compliance
var _ port.InvoiceV2Repository = (*InvoiceV2Repository)(nil)
