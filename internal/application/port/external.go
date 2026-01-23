package port

import (
	"context"

	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// LarkInstanceDetail represents details fetched from Lark API
type LarkInstanceDetail struct {
	InstanceCode string
	ApprovalCode string
	UserID       string
	OpenID       string
	Status       string
	FormData     string
	StartTime    int64
	EndTime      int64
}

// ApproverInfo represents an approver from Lark
type ApproverInfo struct {
	UserID string
	OpenID string
	Name   string
}

// LarkClient defines Lark API operations
type LarkClient interface {
	GetInstanceDetail(ctx context.Context, instanceID string) (*LarkInstanceDetail, error)
	GetApprovers(ctx context.Context, instanceID string) ([]ApproverInfo, error)
}

// LarkAttachmentDownloader defines attachment download operations
type LarkAttachmentDownloader interface {
	Download(ctx context.Context, url string) ([]byte, int64, error)
	DownloadWithRetry(ctx context.Context, url string, maxAttempts int) ([]byte, int64, error)
}

// LarkMessageSender defines message sending operations
type LarkMessageSender interface {
	SendMessage(ctx context.Context, openID string, content string) error
	SendCardMessage(ctx context.Context, openID string, cardContent interface{}) error
}

// PolicyAuditResult represents policy validation result
type PolicyAuditResult struct {
	Compliant  bool
	Violations []string
	Confidence float64
	Reasoning  string
}

// PriceAuditResult represents price benchmarking result
type PriceAuditResult struct {
	Reasonable          bool
	DeviationPercentage float64
	MarketPriceMin      float64
	MarketPriceMax      float64
	Confidence          float64
	Reasoning           string
}

// InvoiceExtractionResult represents extracted invoice data
type InvoiceExtractionResult struct {
	Success       bool
	InvoiceCode   string
	InvoiceNumber string
	TotalAmount   float64
	TaxAmount     float64
	InvoiceDate   string
	SellerName    string
	SellerTaxID   string
	BuyerName     string
	BuyerTaxID    string
	ExtractedData map[string]interface{}
	Confidence    float64
	Error         string
}

// AIAuditor defines AI auditing operations
type AIAuditor interface {
	AuditPolicy(ctx context.Context, item *entity.ReimbursementItem, invoiceData *InvoiceExtractionResult) (*PolicyAuditResult, error)
	AuditPrice(ctx context.Context, item *entity.ReimbursementItem, invoiceData *InvoiceExtractionResult) (*PriceAuditResult, error)
	ExtractInvoice(ctx context.Context, imageData []byte, mimeType string) (*InvoiceExtractionResult, error)
}
