package voucher

import (
	"context"

	"github.com/garyjia/ai-reimbursement/internal/models"
)

// FormFillerInterface defines the contract for Excel template filling
// ARCH-013-A: Fill Excel template with form data
type FormFillerInterface interface {
	// FillTemplate fills the template with data and saves to outputPath
	// Returns the full path to the saved file or error
	FillTemplate(ctx context.Context, data *FormData, outputPath string) (string, error)

	// ValidateTemplate checks if template has expected structure
	ValidateTemplate() error
}

// FormDataAggregatorInterface defines the contract for data collection
// ARCH-013-C: Aggregate data from multiple repositories
type FormDataAggregatorInterface interface {
	// AggregateData collects all data needed for form generation
	AggregateData(ctx context.Context, instanceID int64) (*FormData, error)
}

// AccountingSubjectMapperInterface defines the contract for item type mapping
// ARCH-013-B: Map item types to Chinese accounting subjects
type AccountingSubjectMapperInterface interface {
	// MapToSubject converts ItemType to Chinese accounting subject
	MapToSubject(itemType string) string

	// MapToChineseName converts ItemType to Chinese display name
	MapToChineseName(itemType string) string
}

// VoucherGeneratorInterface defines the contract for voucher generation orchestration
// ARCH-013-D: Orchestrate voucher generation and file organization
type VoucherGeneratorInterface interface {
	// GenerateVoucher creates the complete voucher package for an instance
	GenerateVoucher(ctx context.Context, instanceID int64) (*VoucherResult, error)

	// GenerateVoucherWithOptions allows customization of generation behavior
	GenerateVoucherWithOptions(ctx context.Context, instanceID int64, opts *GenerationOptions) (*VoucherResult, error)

	// IsInstanceFullyProcessed checks if all attachments are ready
	IsInstanceFullyProcessed(instanceID int64) (bool, error)

	// GenerateVoucherAsync generates the voucher asynchronously
	GenerateVoucherAsync(ctx context.Context, instanceID int64)
}

// Deprecated: Use VoucherGeneratorInterface instead
type FormPackagerInterface = VoucherGeneratorInterface

// FolderManagerInterface defines the contract for folder operations
// ARCH-014-A: Manage instance-specific folders
type FolderManagerInterface interface {
	// CreateInstanceFolder creates folder for the given Lark instance ID
	// Returns the full path to the created folder
	CreateInstanceFolder(larkInstanceID string) (string, error)

	// GetInstanceFolderPath returns the path for an instance folder
	// Does not create the folder if it doesn't exist
	GetInstanceFolderPath(larkInstanceID string) string

	// FolderExists checks if instance folder already exists
	FolderExists(larkInstanceID string) bool

	// DeleteInstanceFolder removes an instance folder and all contents
	DeleteInstanceFolder(larkInstanceID string) error

	// SanitizeFolderName returns a filesystem-safe version of the name
	SanitizeFolderName(name string) string
}

// InstanceRepositoryInterface for dependency injection
type InstanceRepositoryInterface interface {
	GetByID(id int64) (*models.ApprovalInstance, error)
	GetByLarkInstanceID(larkInstanceID string) (*models.ApprovalInstance, error)
}

// ItemRepositoryInterface for dependency injection
type ItemRepositoryInterface interface {
	GetByInstanceID(instanceID int64) ([]*models.ReimbursementItem, error)
}

// AttachmentRepositoryInterface for dependency injection
type AttachmentRepositoryInterface interface {
	GetByInstanceID(instanceID int64) ([]*models.Attachment, error)
	GetByItemID(itemID int64) ([]*models.Attachment, error)
}
