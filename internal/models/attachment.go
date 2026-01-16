package models

import "time"

// Attachment represents attachment metadata in database
type Attachment struct {
	ID             int64      `json:"id"`
	ItemID         int64      `json:"item_id"`
	InstanceID     int64      `json:"instance_id"`
	FileName       string     `json:"file_name"`
	FilePath       string     `json:"file_path"`
	FileSize       int64      `json:"file_size"`
	MimeType       string     `json:"mime_type"`
	DownloadStatus string     `json:"download_status"` // PENDING, COMPLETED, FAILED
	ErrorMessage   string     `json:"error_message,omitempty"`
	DownloadedAt   *time.Time `json:"downloaded_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Attachment status constants
const (
	AttachmentStatusPending   = "PENDING"
	AttachmentStatusCompleted = "COMPLETED"
	AttachmentStatusFailed    = "FAILED"
)

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
