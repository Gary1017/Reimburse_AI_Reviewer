package workflow

import (
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// ExceptionManager handles exception-based routing
type ExceptionManager struct {
	logger *zap.Logger
}

// NewExceptionManager creates a new exception manager
func NewExceptionManager(logger *zap.Logger) *ExceptionManager {
	return &ExceptionManager{
		logger: logger,
	}
}

// ExceptionDecision represents the decision on exception handling
type ExceptionDecision struct {
	RequiresManualReview bool
	Reason               string
	Confidence           float64
	Violations           []string
}

// EvaluateAIAuditResult evaluates AI audit results for exceptions
func (em *ExceptionManager) EvaluateAIAuditResult(result *models.AIAuditResult) *ExceptionDecision {
	decision := &ExceptionDecision{
		RequiresManualReview: false,
		Confidence:           result.Confidence,
	}

	// Check policy violations
	if !result.PolicyValidation.Compliant {
		decision.RequiresManualReview = true
		decision.Reason = "Policy violations detected"
		decision.Violations = result.PolicyValidation.Violations
		return decision
	}

	// Check price benchmark deviations
	if !result.PriceBenchmark.Reasonable {
		decision.RequiresManualReview = true
		decision.Reason = fmt.Sprintf("Price deviation: %.2f%%", result.PriceBenchmark.DeviationPercentage)
		decision.Violations = append(decision.Violations, decision.Reason)
		return decision
	}

	// Check confidence thresholds
	if result.PolicyValidation.Confidence < 0.8 {
		decision.RequiresManualReview = true
		decision.Reason = "Low AI confidence in policy validation"
		decision.Violations = append(decision.Violations, decision.Reason)
		return decision
	}

	if result.PriceBenchmark.Confidence < 0.7 {
		decision.RequiresManualReview = true
		decision.Reason = "Low AI confidence in price estimation"
		decision.Violations = append(decision.Violations, decision.Reason)
		return decision
	}

	// All checks passed
	decision.Reason = "All automated checks passed"
	return decision
}

// DetermineNextStatus determines the next status based on exception decision
func (em *ExceptionManager) DetermineNextStatus(decision *ExceptionDecision) string {
	if decision.RequiresManualReview {
		em.logger.Info("Routing to manual review",
			zap.String("reason", decision.Reason),
			zap.Float64("confidence", decision.Confidence))
		return models.StatusInReview
	}

	em.logger.Info("Auto-approving based on AI confidence",
		zap.Float64("confidence", decision.Confidence))
	return models.StatusAutoApproved
}

// ShouldEscalate determines if an instance should be escalated
func (em *ExceptionManager) ShouldEscalate(amount float64, violations []string) bool {
	// Escalation rules
	const highValueThreshold = 10000.0 // CNY

	// Escalate high-value reimbursements
	if amount >= highValueThreshold {
		em.logger.Info("Escalating high-value reimbursement",
			zap.Float64("amount", amount))
		return true
	}

	// Escalate if there are multiple violations
	if len(violations) >= 3 {
		em.logger.Info("Escalating due to multiple violations",
			zap.Int("violation_count", len(violations)))
		return true
	}

	return false
}

// GetReviewPriority determines review priority
func (em *ExceptionManager) GetReviewPriority(result *models.AIAuditResult, amount float64) string {
	// Priority levels: LOW, MEDIUM, HIGH, URGENT

	if !result.PolicyValidation.Compliant {
		return "HIGH"
	}

	if amount >= 10000 {
		return "HIGH"
	}

	if result.PriceBenchmark.DeviationPercentage > 50 {
		return "URGENT"
	}

	if result.Confidence < 0.6 {
		return "MEDIUM"
	}

	return "LOW"
}
