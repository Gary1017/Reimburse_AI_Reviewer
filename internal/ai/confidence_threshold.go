package ai

import (
	"fmt"
	"time"
)

// ConfidenceThreshold defines the decision boundaries for AI audit routing
// ARCH-001-A: Define confidence threshold model
type ConfidenceThreshold struct {
	HighThreshold  float64   // Default: 0.95 - Auto-approve threshold
	LowThreshold   float64   // Default: 0.70 - Rejection threshold
	ConfigVersion  string    // Version identifier for audit trail
	UpdatedAt      time.Time // When this config was created
}

// AuditDecision represents the outcome of an AI audit with routing decision
// ARCH-001-C, ARCH-001-D: Audit decision with routing and immutability
type AuditDecision struct {
	InstanceID       string
	ConfidenceScore  float64
	Decision         string // "AUTO_APPROVED", "IN_REVIEW", "REJECTED"
	NextQueue        string // "LARK_APPROVAL" or empty for auto-approved
	Rationale        string // Explanation of the decision
	ThresholdConfig  ConfidenceThreshold
	Timestamp        time.Time
	Locked           bool // Immutability flag
}

// ConfidenceCalculator computes confidence scores from audit components
// ARCH-001-B: Implement confidence calculation
type ConfidenceCalculator struct{}

// ConfidenceRouter routes audit decisions based on confidence thresholds
// ARCH-001-C: Route decisions based on thresholds
type ConfidenceRouter struct {
	thresholds ConfidenceThreshold
}

// NewConfidenceThreshold creates a new threshold model with default values
func NewConfidenceThreshold() *ConfidenceThreshold {
	return &ConfidenceThreshold{
		HighThreshold: 0.95,    // 95% for auto-approval
		LowThreshold:  0.70,    // 70% for exception routing
		ConfigVersion: "v1",
		UpdatedAt:     time.Now(),
	}
}

// DefaultConfidenceThreshold returns the default threshold configuration
func DefaultConfidenceThreshold() ConfidenceThreshold {
	return ConfidenceThreshold{
		HighThreshold: 0.95,
		LowThreshold:  0.70,
		ConfigVersion: "v1",
		UpdatedAt:     time.Now(),
	}
}

// Validate ensures threshold values are within valid ranges and logically consistent
// ARCH-001-A: Thresholds must be > 0.0, ≤ 1.0; HighThreshold > LowThreshold
func (ct *ConfidenceThreshold) Validate() error {
	if ct.HighThreshold < 0.0 || ct.HighThreshold > 1.0 {
		return fmt.Errorf("HighThreshold must be between 0.0 and 1.0, got %.2f", ct.HighThreshold)
	}

	if ct.LowThreshold < 0.0 || ct.LowThreshold > 1.0 {
		return fmt.Errorf("LowThreshold must be between 0.0 and 1.0, got %.2f", ct.LowThreshold)
	}

	if ct.HighThreshold <= ct.LowThreshold {
		return fmt.Errorf("HighThreshold must be greater than LowThreshold (high: %.2f, low: %.2f)", ct.HighThreshold, ct.LowThreshold)
	}

	return nil
}

// NewConfidenceCalculator creates a new confidence calculator
func NewConfidenceCalculator() *ConfidenceCalculator {
	return &ConfidenceCalculator{}
}

// CalculateConfidence computes normalized confidence score from three boolean checks
// ARCH-001-B: Score = (policy_matches + price_within_range + no_duplicates) / 3
func (cc *ConfidenceCalculator) CalculateConfidence(policyMatch, priceValid, uniqueInvoice bool) float64 {
	var score float64

	if policyMatch {
		score += 1.0
	}
	if priceValid {
		score += 1.0
	}
	if uniqueInvoice {
		score += 1.0
	}

	// Normalize to [0.0, 1.0]
	normalizedScore := score / 3.0
	return normalizedScore
}

// NewConfidenceRouter creates a new decision router with given thresholds
func NewConfidenceRouter(thresholds ConfidenceThreshold) *ConfidenceRouter {
	return &ConfidenceRouter{
		thresholds: thresholds,
	}
}

// RouteDecision assigns a routing decision based on confidence score and thresholds
// ARCH-001-C: HIGH→auto-approve; MEDIUM→exception; LOW→reject
func (cr *ConfidenceRouter) RouteDecision(decision *AuditDecision) *AuditDecision {
	decision.ThresholdConfig = cr.thresholds
	decision.Timestamp = time.Now()

	switch {
	case decision.ConfidenceScore >= cr.thresholds.HighThreshold:
		// Auto-approve high-confidence items
		decision.Decision = "AUTO_APPROVED"
		decision.NextQueue = ""
		decision.Rationale = fmt.Sprintf("Auto-approved: confidence score %.2f >= threshold %.2f",
			decision.ConfidenceScore, cr.thresholds.HighThreshold)
		decision.Locked = true // Immutable once routed

	case decision.ConfidenceScore >= cr.thresholds.LowThreshold:
		// Route medium-confidence to manual review
		decision.Decision = "IN_REVIEW"
		decision.NextQueue = "LARK_APPROVAL"
		decision.Rationale = fmt.Sprintf("Manual review required: confidence score %.2f between thresholds (%.2f-%.2f)",
			decision.ConfidenceScore, cr.thresholds.LowThreshold, cr.thresholds.HighThreshold)
		decision.Locked = true

	default:
		// Reject low-confidence items
		decision.Decision = "REJECTED"
		decision.NextQueue = ""
		if decision.Rationale == "" {
			decision.Rationale = fmt.Sprintf("Rejected: confidence score %.2f below low confidence threshold %.2f",
				decision.ConfidenceScore, cr.thresholds.LowThreshold)
		} else {
			decision.Rationale += fmt.Sprintf(" (confidence %.2f < %.2f)",
				decision.ConfidenceScore, cr.thresholds.LowThreshold)
		}
		decision.Locked = true
	}

	return decision
}

// IsLocked returns whether the decision is immutable
func (ad *AuditDecision) IsLocked() bool {
	return ad.Locked
}

// String returns a human-readable representation of the decision
func (ad *AuditDecision) String() string {
	return fmt.Sprintf("AuditDecision{InstanceID: %s, Decision: %s, Confidence: %.2f, Queue: %s}",
		ad.InstanceID, ad.Decision, ad.ConfidenceScore, ad.NextQueue)
}
