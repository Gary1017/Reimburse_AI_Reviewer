package entity

// Status constants for ApprovalInstance
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

// Item type constants for ReimbursementItem
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

// Attachment status constants
const (
	AttachmentStatusPending     = "PENDING"
	AttachmentStatusCompleted   = "COMPLETED"
	AttachmentStatusFailed      = "FAILED"
	AttachmentStatusProcessing  = "PROCESSING"
	AttachmentStatusProcessed   = "PROCESSED"
	AttachmentStatusAuditFailed = "AUDIT_FAILED"
)

// Notification status constants
const (
	NotificationStatusPending = "PENDING"
	NotificationStatusSent    = "SENT"
	NotificationStatusFailed  = "FAILED"
)

// Audit decision constants
const (
	AuditDecisionPass        = "PASS"
	AuditDecisionNeedsReview = "NEEDS_REVIEW"
	AuditDecisionFail        = "FAIL"
)
