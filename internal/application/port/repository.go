package port

import (
	"context"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// InstanceRepository defines persistence operations for ApprovalInstance
type InstanceRepository interface {
	Create(ctx context.Context, instance *entity.ApprovalInstance) error
	GetByID(ctx context.Context, id int64) (*entity.ApprovalInstance, error)
	GetByLarkInstanceID(ctx context.Context, larkID string) (*entity.ApprovalInstance, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
	SetApprovalTime(ctx context.Context, id int64, t time.Time) error
	List(ctx context.Context, limit, offset int) ([]*entity.ApprovalInstance, error)
}

// ItemRepository defines persistence operations for ReimbursementItem
type ItemRepository interface {
	Create(ctx context.Context, item *entity.ReimbursementItem) error
	GetByID(ctx context.Context, id int64) (*entity.ReimbursementItem, error)
	GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error)
	Update(ctx context.Context, item *entity.ReimbursementItem) error
}

// AttachmentRepository defines persistence operations for Attachment
type AttachmentRepository interface {
	Create(ctx context.Context, att *entity.Attachment) error
	GetByID(ctx context.Context, id int64) (*entity.Attachment, error)
	GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Attachment, error)
	GetPending(ctx context.Context, limit int) ([]*entity.Attachment, error)
	MarkCompleted(ctx context.Context, id int64, filePath string, fileSize int64) error
	UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error
}

// HistoryRepository defines persistence operations for ApprovalHistory
type HistoryRepository interface {
	Create(ctx context.Context, history *entity.ApprovalHistory) error
	GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ApprovalHistory, error)
}

// InvoiceRepository defines persistence operations for Invoice
type InvoiceRepository interface {
	Create(ctx context.Context, invoice *entity.Invoice) error
	GetByID(ctx context.Context, id int64) (*entity.Invoice, error)
	GetByAttachmentID(ctx context.Context, attachmentID int64) (*entity.Invoice, error)
	GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Invoice, error)
	Update(ctx context.Context, invoice *entity.Invoice) error
	GetByUniqueID(ctx context.Context, uniqueID string) (*entity.Invoice, error)
}

// VoucherRepository defines persistence operations for GeneratedVoucher
type VoucherRepository interface {
	Create(ctx context.Context, voucher *entity.GeneratedVoucher) error
	GetByInstanceID(ctx context.Context, instanceID int64) (*entity.GeneratedVoucher, error)
	Update(ctx context.Context, voucher *entity.GeneratedVoucher) error
}

// NotificationRepository defines persistence operations for AuditNotification
type NotificationRepository interface {
	Create(ctx context.Context, notification *entity.AuditNotification) error
	GetByInstanceID(ctx context.Context, instanceID int64) (*entity.AuditNotification, error)
	UpdateStatus(ctx context.Context, id int64, status string, errorMsg string) error
	MarkSent(ctx context.Context, id int64) error
}

// TransactionManager handles database transactions
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
