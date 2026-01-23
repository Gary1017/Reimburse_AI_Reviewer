package entity

import "time"

// GeneratedVoucher represents a generated Excel voucher
type GeneratedVoucher struct {
	ID              int64      `json:"id"`
	InstanceID      int64      `json:"instance_id"`
	VoucherNumber   string     `json:"voucher_number"`
	FilePath        string     `json:"file_path"`
	EmailMessageID  string     `json:"email_message_id"`
	SentAt          *time.Time `json:"sent_at,omitempty"`
	AccountantEmail string     `json:"accountant_email"`
	CreatedAt       time.Time  `json:"created_at"`
}
