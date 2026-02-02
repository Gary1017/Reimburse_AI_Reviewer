package entity

import "time"

// InvoiceList represents an aggregation of invoices for an approval instance.
// There is a 1:1 relationship between InvoiceList and ApprovalInstance.
type InvoiceList struct {
	ID                 int64     `json:"id"`
	InstanceID         int64     `json:"instance_id"`
	TotalInvoiceCount  int       `json:"total_invoice_count"`
	TotalInvoiceAmount float64   `json:"total_invoice_amount"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// InvoiceList status constants
const (
	InvoiceListStatusPending    = "PENDING"
	InvoiceListStatusProcessing = "PROCESSING"
	InvoiceListStatusCompleted  = "COMPLETED"
	InvoiceListStatusFailed     = "FAILED"
)
