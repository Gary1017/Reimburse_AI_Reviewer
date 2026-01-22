package voucher

import "time"

// FormData represents aggregated data for filling the reimbursement form
// ARCH-013-C: Output structure of FormDataAggregator
type FormData struct {
	ApplicantName  string         // From ApprovalInstance.ApplicantUserID (resolved to name)
	Department     string         // From ApprovalInstance.Department
	LarkInstanceID string         // From ApprovalInstance.LarkInstanceID
	SubmissionDate time.Time      // From ApprovalInstance.SubmissionTime
	Items          []FormItemData // From ReimbursementItem records
	TotalAmount    float64        // Calculated sum of all item amounts
	TotalReceipts  int            // Total attachment count
}

// FormItemData represents a single line item for the form
type FormItemData struct {
	SequenceNumber    int     // 1-indexed row number
	AccountingSubject string  // Chinese accounting category (会计科目)
	Description       string  // From ReimbursementItem.Description
	ExpenseType       string  // Chinese name for ItemType
	ReceiptCount      int     // Number of attachments for this item
	Amount            float64 // From ReimbursementItem.Amount
	Remarks           string  // From ReimbursementItem.BusinessPurpose
}

// FormPackageResult represents the output of form package generation
// ARCH-013-D: Result structure for FormPackager
type FormPackageResult struct {
	FolderPath      string   // Full path to instance folder
	FormFilePath    string   // Full path to generated Excel file
	AttachmentPaths []string // Full paths to all attachments
	IncompleteCount int      // Number of attachments not yet downloaded
	Success         bool     // True if all operations completed
	Error           error    // Non-nil if any operation failed
}

// PackageOptions customizes form package generation behavior
type PackageOptions struct {
	OverwriteExisting  bool // If true, overwrite existing folder/files
	WaitForAttachments bool // If true, wait for incomplete downloads
	AttachmentTimeout  int  // Seconds to wait for attachments (if WaitForAttachments)
}
