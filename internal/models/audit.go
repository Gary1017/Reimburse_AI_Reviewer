package models

import "time"

// ApprovalHistory represents the audit trail of an approval instance
type ApprovalHistory struct {
	ID             int64     `json:"id"`
	InstanceID     int64     `json:"instance_id"`
	ReviewerUserID string    `json:"reviewer_user_id"`
	PreviousStatus string    `json:"previous_status"`
	NewStatus      string    `json:"new_status"`
	ActionType     string    `json:"action_type"` // STATUS_CHANGE, HUMAN_REVIEW, AI_AUDIT
	ActionData     string    `json:"action_data"` // JSON blob
	Timestamp      time.Time `json:"timestamp"`
}

// SystemConfig represents system configuration key-value pairs
type SystemConfig struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Action type constants
const (
	ActionTypeStatusChange = "STATUS_CHANGE"
	ActionTypeHumanReview  = "HUMAN_REVIEW"
	ActionTypeAIAudit      = "AI_AUDIT"
	ActionTypeWebhook      = "WEBHOOK_EVENT"
)

// AIAuditResult represents the structure of AI audit results
type AIAuditResult struct {
	PolicyValidation PolicyValidationResult `json:"policy_validation"`
	PriceBenchmark   PriceBenchmarkResult   `json:"price_benchmark"`
	OverallDecision  string                 `json:"overall_decision"` // PASS, NEEDS_REVIEW, FAIL
	Confidence       float64                `json:"confidence"`
	Timestamp        time.Time              `json:"timestamp"`
}

// PolicyValidationResult represents policy validation results
type PolicyValidationResult struct {
	Compliant   bool     `json:"compliant"`
	Violations  []string `json:"violations"`
	Confidence  float64  `json:"confidence"`
	Reasoning   string   `json:"reasoning"`
}

// PriceBenchmarkResult represents price benchmarking results
type PriceBenchmarkResult struct {
	EstimatedPriceRange  []float64 `json:"estimated_price_range"` // [min, max]
	SubmittedPrice       float64   `json:"submitted_price"`
	DeviationPercentage  float64   `json:"deviation_percentage"`
	Reasonable           bool      `json:"reasonable"`
	Confidence           float64   `json:"confidence"`
	Reasoning            string    `json:"reasoning"`
}
