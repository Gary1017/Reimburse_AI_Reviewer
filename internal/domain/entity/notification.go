package entity

import "time"

// AuditNotification represents a notification record
type AuditNotification struct {
	ID             int64      `json:"id"`
	InstanceID     int64      `json:"instance_id"`
	LarkInstanceID string     `json:"lark_instance_id"`
	Status         string     `json:"status"`
	AuditDecision  string     `json:"audit_decision"`
	Confidence     float64    `json:"confidence"`
	TotalAmount    float64    `json:"total_amount"`
	ApproverCount  int        `json:"approver_count"`
	Violations     string     `json:"violations"`
	SentAt         *time.Time `json:"sent_at,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// AggregatedAuditResult represents consolidated audit results across all attachments for an instance
type AggregatedAuditResult struct {
	InstanceID      int64    `json:"instance_id"`
	LarkInstanceID  string   `json:"lark_instance_id"`
	Decision        string   `json:"decision"`
	Confidence      float64  `json:"confidence"`
	TotalAmount     float64  `json:"total_amount"`
	Violations      []string `json:"violations"`
	AttachmentCount int      `json:"attachment_count"`
	ProcessedCount  int      `json:"processed_count"`
}

// ApproverInfo represents an approver extracted from Lark instance
type ApproverInfo struct {
	UserID string `json:"user_id"`
	OpenID string `json:"open_id"`
	Name   string `json:"name,omitempty"`
	Email  string `json:"email,omitempty"`
}

// AuditNotificationRequest represents a request to send audit notification via Lark Bot
type AuditNotificationRequest struct {
	ApprovalCode   string                 `json:"approval_code"`
	InstanceCode   string                 `json:"instance_code"`
	LarkInstanceID string                 `json:"lark_instance_id"`
	OpenID         string                 `json:"open_id"`
	AuditResult    *AggregatedAuditResult `json:"audit_result"`
}

// AuditNotificationResponse represents the response from Lark Bot API
type AuditNotificationResponse struct {
	Success      bool   `json:"success"`
	MessageID    string `json:"message_id,omitempty"`
	ErrorCode    int    `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// NotificationMessageContent represents the message content for Lark Bot notification
type NotificationMessageContent struct {
	InstanceCode string   `json:"instance_code"`
	TotalAmount  float64  `json:"total_amount"`
	Decision     string   `json:"decision"`
	DecisionText string   `json:"decision_text"`
	Confidence   float64  `json:"confidence"`
	Violations   []string `json:"violations"`
	InstanceLink string   `json:"instance_link"`
}
