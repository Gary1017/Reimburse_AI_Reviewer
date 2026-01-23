package entity

import "time"

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
	DownloadStatus string     `json:"download_status"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	DownloadedAt   *time.Time `json:"downloaded_at,omitempty"`
	ProcessedAt    *time.Time `json:"processed_at,omitempty"`
	AuditResult    string     `json:"audit_result,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
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
