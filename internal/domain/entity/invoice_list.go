package entity

import "time"

// InvoiceList represents an aggregation of invoices for an approval instance.
// There is a 1:1 relationship between InvoiceList and ApprovalInstance.
type InvoiceList struct {
	ID                      int64     `json:"id"`
	InstanceID              int64     `json:"instance_id"`
	TotalInvoiceCount       int       `json:"total_invoice_count"`
	TotalInvoiceAmountCents int64     `json:"total_invoice_amount_cents"` // Amount in cents (分)
	TotalInvoiceAmount      float64   `json:"total_invoice_amount"`       // Deprecated: use TotalInvoiceAmountCents
	Status                  string    `json:"status"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// TotalAmountYuan returns the total amount in yuan (元) for display purposes.
func (l *InvoiceList) TotalAmountYuan() float64 {
	return float64(l.TotalInvoiceAmountCents) / 100.0
}

// InvoiceList status constants
const (
	InvoiceListStatusPending    = "PENDING"
	InvoiceListStatusProcessing = "PROCESSING"
	InvoiceListStatusCompleted  = "COMPLETED"
	InvoiceListStatusFailed     = "FAILED"
)
