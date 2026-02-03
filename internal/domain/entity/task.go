package entity

import "time"

// ApprovalTask represents a unified task in the approval workflow.
// It combines task definition and result, aligned with Lark's task_list structure.
// AI review is treated as "just another task" with IsAIDecision flag.
//
// Governance: All tasks (including AI review) must have an assignee for accountability.
// For AI tasks, the assignee is the person configured as AI_APPROVER_OPEN_ID who is
// accountable for AI decisions. This satisfies Lark's requirement that all decisions
// must be attributable to a physical person.
type ApprovalTask struct {
	ID         int64 `json:"id"`
	InstanceID int64 `json:"instance_id"`

	// Lark task_list mapping (NULL for AI-generated tasks, but AI tasks still have assignee)
	LarkTaskID string `json:"lark_task_id,omitempty"`

	// Task type and workflow position
	TaskType       string `json:"task_type"`
	SequenceNumber int    `json:"sequence_number"`

	// Lark node information
	NodeID       string `json:"node_id,omitempty"`
	NodeName     string `json:"node_name,omitempty"`
	CustomNodeID string `json:"custom_node_id,omitempty"`
	ApprovalType string `json:"approval_type,omitempty"`

	// Assignee (for Lark governance accountability)
	AssigneeUserID string `json:"assignee_user_id,omitempty"`
	AssigneeOpenID string `json:"assignee_open_id,omitempty"`

	// Status and timing
	Status    string `json:"status"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`

	// Workflow control
	IsCurrent bool `json:"is_current"`

	// AI decision tracking (for technical auditing)
	IsAIDecision bool `json:"is_ai_decision"`

	// Result fields (merged from task_results + approval_history)
	Decision    string   `json:"decision,omitempty"`
	Confidence  *float64 `json:"confidence,omitempty"`
	ResultData  string   `json:"result_data,omitempty"`
	Violations  string   `json:"violations,omitempty"`
	CompletedBy string   `json:"completed_by,omitempty"`

	// Notification tracking (for AI review tasks)
	NotificationSentAt *time.Time `json:"notification_sent_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Task type constants
const (
	TaskTypeAIReview    = "AI_REVIEW"
	TaskTypeHumanReview = "HUMAN_REVIEW"
)

// Task status constants
const (
	TaskStatusPending    = "PENDING"
	TaskStatusInProgress = "IN_PROGRESS"
	TaskStatusCompleted  = "COMPLETED"
	TaskStatusRejected   = "REJECTED"
)

// Decision constants
const (
	DecisionPass        = "PASS"
	DecisionNeedsReview = "NEEDS_REVIEW"
	DecisionFail        = "FAIL"
	DecisionApproved    = "APPROVED"
	DecisionRejected    = "REJECTED"
)
