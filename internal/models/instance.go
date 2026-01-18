package models

import "time"

// ApprovalInstance represents a reimbursement approval instance
type ApprovalInstance struct {
	ID              int64      `json:"id"`
	LarkInstanceID  string     `json:"lark_instance_id"`
	Status          string     `json:"status"` // CREATED, PENDING, AI_AUDITING, IN_REVIEW, APPROVED, REJECTED, COMPLETED
	ApplicantUserID string     `json:"applicant_user_id"`
	Department      string     `json:"department"`
	SubmissionTime  time.Time  `json:"submission_time"`
	ApprovalTime    *time.Time `json:"approval_time,omitempty"`
	FormData        string     `json:"form_data"`       // JSON blob
	AIAuditResult   string     `json:"ai_audit_result"` // JSON blob
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ReimbursementItem represents a single expense item
type ReimbursementItem struct {
	ID                int64      `json:"id"`
	InstanceID        int64      `json:"instance_id"`
	ItemType          string     `json:"item_type"` // TRAVEL, MEAL, ACCOMMODATION, EQUIPMENT
	Description       string     `json:"description"`
	Amount            float64    `json:"amount"`
	Currency          string     `json:"currency"`
	ReceiptAttachment string     `json:"receipt_attachment"`         // Lark file token
	AIPriceCheck      string     `json:"ai_price_check"`             // JSON blob
	AIPolicyCheck     string     `json:"ai_policy_check"`            // JSON blob
	ExpenseDate       *time.Time `json:"expense_date,omitempty"`     // Date of expense
	Vendor            string     `json:"vendor,omitempty"`           // Vendor/merchant name
	BusinessPurpose   string     `json:"business_purpose,omitempty"` // Business purpose
	CreatedAt         time.Time  `json:"created_at"`
}

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

// Status constants
const (
	StatusCreated           = "CREATED"
	StatusPending           = "PENDING"
	StatusAIAuditing        = "AI_AUDITING"
	StatusAIAudited         = "AI_AUDITED"
	StatusInReview          = "IN_REVIEW"
	StatusAutoApproved      = "AUTO_APPROVED"
	StatusApproved          = "APPROVED"
	StatusRejected          = "REJECTED"
	StatusVoucherGenerating = "VOUCHER_GENERATING"
	StatusCompleted         = "COMPLETED"
)

// Item type constants
const (
	ItemTypeTravel         = "TRAVEL"         // 差旅费
	ItemTypeMeal           = "MEAL"           // 餐费
	ItemTypeAccommodation  = "ACCOMMODATION"  // 住宿费
	ItemTypeEquipment      = "EQUIPMENT"      // 设备/办公用品
	ItemTypeTransportation = "TRANSPORTATION" // 交通费
	ItemTypeEntertainment  = "ENTERTAINMENT"  // 招待费
	ItemTypeTeamBuilding   = "TEAM_BUILDING"  // 团建费
	ItemTypeCommunication  = "COMMUNICATION"  // 通讯费
	ItemTypeOther          = "OTHER"          // 其他
)
