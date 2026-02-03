package entity

import "time"

// Attachment file type constants
const (
	FileTypeInvoice = "INVOICE" // Tax invoice (发票) - can be extracted and validated
	FileTypeOther   = "OTHER"   // Other supporting documents (receipts, contracts, etc.)
)

// Attachment represents attachment metadata
type Attachment struct {
	ID             int64      `json:"id"`
	ItemID         int64      `json:"item_id"`
	InstanceID     int64      `json:"instance_id"`
	LarkInstanceID string     `json:"lark_instance_id,omitempty"`
	FileName       string     `json:"file_name"`
	URL            string     `json:"url,omitempty"`
	FilePath       string     `json:"file_path"`
	FileSize       int64      `json:"file_size"`
	MimeType       string     `json:"mime_type"`
	FileType       string     `json:"file_type"` // INVOICE, RECEIPT, CONTRACT, OTHER
	DownloadStatus string     `json:"download_status"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	DownloadedAt   *time.Time `json:"downloaded_at,omitempty"`
	ProcessedAt    *time.Time `json:"processed_at,omitempty"`
	AuditResult    string     `json:"audit_result,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// IsInvoice returns true if this attachment is a tax invoice
func (a *Attachment) IsInvoice() bool {
	return a.FileType == FileTypeInvoice
}

// AttachmentReference represents attachment data extracted from Lark form
type AttachmentReference struct {
	OriginalName string
	URL          string
	ItemID       int64
}

// AttachmentFile represents downloaded file content
type AttachmentFile struct {
	Content  []byte
	FileName string
	MimeType string
	Size     int64
}
