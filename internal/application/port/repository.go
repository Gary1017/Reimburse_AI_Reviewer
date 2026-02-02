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
// Deprecated: Use ApprovalTaskRepository instead. History is now merged into approval_tasks.
type HistoryRepository interface {
	Create(ctx context.Context, history *entity.ApprovalHistory) error
	GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ApprovalHistory, error)
}

// =============================================================================
// NEW INTERFACES FOR SCHEMA REFACTORING
// =============================================================================

// InvoiceListRepository defines persistence operations for InvoiceList
// Each approval instance has exactly one invoice list (1:1 relationship)
type InvoiceListRepository interface {
	// Create creates a new invoice list for an instance
	Create(ctx context.Context, list *entity.InvoiceList) error

	// GetByID retrieves an invoice list by its ID
	GetByID(ctx context.Context, id int64) (*entity.InvoiceList, error)

	// GetByInstanceID retrieves the invoice list for an instance (1:1)
	GetByInstanceID(ctx context.Context, instanceID int64) (*entity.InvoiceList, error)

	// Update updates an existing invoice list
	Update(ctx context.Context, list *entity.InvoiceList) error

	// UpdateStatus updates the status of an invoice list
	UpdateStatus(ctx context.Context, id int64, status string) error

	// UpdateTotals updates the count and amount totals
	UpdateTotals(ctx context.Context, id int64, count int, amount float64) error
}

// InvoiceV2Repository defines persistence operations for InvoiceV2
// Each invoice links to an invoice_list, attachment (1:1), and item
type InvoiceV2Repository interface {
	// Create creates a new invoice record
	Create(ctx context.Context, invoice *entity.InvoiceV2) error

	// GetByID retrieves an invoice by its ID
	GetByID(ctx context.Context, id int64) (*entity.InvoiceV2, error)

	// GetByAttachmentID retrieves invoice by attachment ID (1:1 relationship)
	GetByAttachmentID(ctx context.Context, attachmentID int64) (*entity.InvoiceV2, error)

	// GetByItemID retrieves invoice by item ID
	GetByItemID(ctx context.Context, itemID int64) (*entity.InvoiceV2, error)

	// GetByInvoiceListID retrieves all invoices in an invoice list
	GetByInvoiceListID(ctx context.Context, invoiceListID int64) ([]*entity.InvoiceV2, error)

	// GetByInstanceID retrieves all invoices for an instance (via invoice_list)
	GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.InvoiceV2, error)

	// GetByUniqueID retrieves invoice by unique ID (code + number)
	GetByUniqueID(ctx context.Context, uniqueID string) (*entity.InvoiceV2, error)

	// Update updates an existing invoice
	Update(ctx context.Context, invoice *entity.InvoiceV2) error
}

// ApprovalTaskRepository defines persistence operations for ApprovalTask
// Unified task table containing both task definition and result (replaces HistoryRepository)
type ApprovalTaskRepository interface {
	// Create creates a new approval task
	Create(ctx context.Context, task *entity.ApprovalTask) error

	// GetByID retrieves a task by its ID
	GetByID(ctx context.Context, id int64) (*entity.ApprovalTask, error)

	// GetByLarkTaskID retrieves a task by Lark's task ID
	GetByLarkTaskID(ctx context.Context, larkTaskID string) (*entity.ApprovalTask, error)

	// GetByInstanceID retrieves all tasks for an instance ordered by sequence
	GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ApprovalTask, error)

	// GetCurrentTask retrieves the current active task for an instance
	GetCurrentTask(ctx context.Context, instanceID int64) (*entity.ApprovalTask, error)

	// GetAIReviewTask retrieves the AI review task for an instance (sequence=0)
	GetAIReviewTask(ctx context.Context, instanceID int64) (*entity.ApprovalTask, error)

	// Update updates an existing task
	Update(ctx context.Context, task *entity.ApprovalTask) error

	// UpdateStatus updates task status
	UpdateStatus(ctx context.Context, id int64, status string) error

	// CompleteTask marks a task as completed with result
	CompleteTask(ctx context.Context, id int64, decision string, confidence *float64, resultData string, violations string, completedBy string) error

	// SetCurrent sets a task as the current active task (and clears others)
	SetCurrent(ctx context.Context, instanceID int64, taskID int64) error
}

// ReviewNotificationRepository defines persistence operations for ReviewNotification
// Each notification links to an AI_REVIEW task (1:1 relationship)
type ReviewNotificationRepository interface {
	// Create creates a new review notification
	Create(ctx context.Context, notification *entity.ReviewNotification) error

	// GetByID retrieves a notification by its ID
	GetByID(ctx context.Context, id int64) (*entity.ReviewNotification, error)

	// GetByTaskID retrieves notification by task ID (1:1 relationship)
	GetByTaskID(ctx context.Context, taskID int64) (*entity.ReviewNotification, error)

	// UpdateStatus updates notification status and optional error message
	UpdateStatus(ctx context.Context, id int64, status string, errorMsg string) error

	// MarkSent marks notification as sent with timestamp
	MarkSent(ctx context.Context, id int64) error
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
