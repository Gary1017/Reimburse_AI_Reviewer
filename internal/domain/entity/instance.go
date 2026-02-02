package entity

import "time"

// ApprovalInstance represents a reimbursement approval instance
type ApprovalInstance struct {
	ID              int64      `json:"id"`
	LarkInstanceID  string     `json:"lark_instance_id"`
	Status          string     `json:"status"`
	ApplicantUserID string     `json:"applicant_user_id"`
	Department      string     `json:"department"`
	SubmissionTime  time.Time  `json:"submission_time"`
	ApprovalTime    *time.Time `json:"approval_time,omitempty"`
	FormData        string     `json:"form_data"`
	// AIAuditResult has been moved to approval_tasks.result_data
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReimbursementItem represents a single expense item
type ReimbursementItem struct {
	ID                int64      `json:"id"`
	InstanceID        int64      `json:"instance_id"`
	ItemType          string     `json:"item_type"`
	Description       string     `json:"description"`
	Amount            float64    `json:"amount"`
	Currency          string     `json:"currency"`
	ReceiptAttachment string     `json:"receipt_attachment"`
	AIPriceCheck      string     `json:"ai_price_check"`
	AIPolicyCheck     string     `json:"ai_policy_check"`
	ExpenseDate       *time.Time `json:"expense_date,omitempty"`
	Vendor            string     `json:"vendor,omitempty"`
	BusinessPurpose   string     `json:"business_purpose,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}
