package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// InvoiceRepository handles invoice database operations
type InvoiceRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewInvoiceRepository creates a new invoice repository
func NewInvoiceRepository(db *sql.DB, logger *zap.Logger) *InvoiceRepository {
	return &InvoiceRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new invoice record
func (r *InvoiceRepository) Create(tx *sql.Tx, invoice *models.Invoice) error {
	query := `
		INSERT INTO invoices (
			invoice_code, invoice_number, unique_id, instance_id, file_token,
			file_path, invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var result sql.Result
	var err error

	if tx != nil {
		result, err = tx.Exec(query,
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
	} else {
		result, err = r.db.Exec(query,
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
	}

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

// CheckUniqueness checks if an invoice with the same unique ID exists
func (r *InvoiceRepository) CheckUniqueness(uniqueID string) (*models.UniquenessCheckResult, error) {
	query := `
		SELECT id, instance_id, created_at
		FROM invoices
		WHERE unique_id = ?
		LIMIT 1
	`

	var invoiceID, instanceID int64
	var createdAt time.Time

	err := r.db.QueryRow(query, uniqueID).Scan(&invoiceID, &instanceID, &createdAt)

	if err == sql.ErrNoRows {
		// Invoice is unique
		return &models.UniquenessCheckResult{
			IsUnique: true,
			Message:  "Invoice is unique",
		}, nil
	}

	if err != nil {
		r.logger.Error("Failed to check invoice uniqueness", zap.Error(err))
		return nil, fmt.Errorf("failed to check uniqueness: %w", err)
	}

	// Invoice already exists
	return &models.UniquenessCheckResult{
		IsUnique:            false,
		DuplicateInvoiceID:  invoiceID,
		DuplicateInstanceID: instanceID,
		FirstSeenAt:         &createdAt,
		Message:             fmt.Sprintf("Duplicate invoice found (first seen: %s)", createdAt.Format("2006-01-02")),
	}, nil
}

// GetByUniqueID retrieves an invoice by its unique ID
func (r *InvoiceRepository) GetByUniqueID(uniqueID string) (*models.Invoice, error) {
	query := `
		SELECT id, invoice_code, invoice_number, unique_id, instance_id, file_token,
			file_path, invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices
		WHERE unique_id = ?
	`

	var invoice models.Invoice
	var invoiceDate sql.NullTime

	err := r.db.QueryRow(query, uniqueID).Scan(
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

// GetByInstanceID retrieves all invoices for an instance
func (r *InvoiceRepository) GetByInstanceID(instanceID int64) ([]*models.Invoice, error) {
	query := `
		SELECT id, invoice_code, invoice_number, unique_id, instance_id, file_token,
			file_path, invoice_date, invoice_amount, seller_name, seller_tax_id,
			buyer_name, buyer_tax_id, extracted_data, created_at
		FROM invoices
		WHERE instance_id = ?
	`

	rows, err := r.db.Query(query, instanceID)
	if err != nil {
		r.logger.Error("Failed to get invoices by instance ID", zap.Int64("instance_id", instanceID), zap.Error(err))
		return nil, fmt.Errorf("failed to get invoices: %w", err)
	}
	defer rows.Close()

	var invoices []*models.Invoice
	for rows.Next() {
		var invoice models.Invoice
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

// CreateValidation creates an invoice validation record
func (r *InvoiceRepository) CreateValidation(tx *sql.Tx, validation *models.InvoiceValidation) error {
	query := `
		INSERT INTO invoice_validations (
			invoice_id, validation_type, is_valid, error_message, validation_data
		) VALUES (?, ?, ?, ?, ?)
	`

	var result sql.Result
	var err error

	if tx != nil {
		result, err = tx.Exec(query,
			validation.InvoiceID,
			validation.ValidationType,
			validation.IsValid,
			validation.ErrorMessage,
			validation.ValidationData,
		)
	} else {
		result, err = r.db.Exec(query,
			validation.InvoiceID,
			validation.ValidationType,
			validation.IsValid,
			validation.ErrorMessage,
			validation.ValidationData,
		)
	}

	if err != nil {
		r.logger.Error("Failed to create validation", zap.Error(err))
		return fmt.Errorf("failed to create validation: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	validation.ID = id
	return nil
}
