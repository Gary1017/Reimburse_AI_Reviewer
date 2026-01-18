package notification

import (
	"encoding/json"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// AuditAggregator consolidates audit results across all attachments for an instance
// ARCH-012: AI Audit Result Notification via Lark Approval Bot
type AuditAggregator struct {
	logger *zap.Logger
}

// NewAuditAggregator creates a new audit aggregator
func NewAuditAggregator(logger *zap.Logger) *AuditAggregator {
	return &AuditAggregator{
		logger: logger,
	}
}

// Aggregate consolidates audit results from all processed attachments
// Decision priority: FAIL > NEEDS_REVIEW > PASS
// Confidence = average across attachments
// Violations = union of all violations
func (a *AuditAggregator) Aggregate(
	attachments []*models.Attachment,
	instance *models.ApprovalInstance,
) (*models.AggregatedAuditResult, error) {
	if len(attachments) == 0 {
		return &models.AggregatedAuditResult{
			InstanceID:     instance.ID,
			LarkInstanceID: instance.LarkInstanceID,
			Decision:       models.AuditDecisionPass,
			Confidence:     1.0,
			TotalAmount:    0,
			Violations:     nil,
			AttachmentCount: 0,
			ProcessedCount:  0,
		}, nil
	}

	result := &models.AggregatedAuditResult{
		InstanceID:      instance.ID,
		LarkInstanceID:  instance.LarkInstanceID,
		Decision:        models.AuditDecisionPass,
		AttachmentCount: len(attachments),
		Violations:      []string{},
	}

	var totalConfidence float64
	var processedCount int
	violationSet := make(map[string]bool)

	for _, att := range attachments {
		if att.AuditResult == "" {
			continue
		}

		// Parse audit result JSON
		var auditResult models.InvoiceAuditResult
		if err := json.Unmarshal([]byte(att.AuditResult), &auditResult); err != nil {
			a.logger.Warn("Failed to parse audit result",
				zap.Int64("attachment_id", att.ID),
				zap.Error(err))
			continue
		}

		processedCount++
		totalConfidence += auditResult.OverallConfidence

		// Add to total amount (from extracted invoice data)
		if auditResult.ExtractedData != nil {
			result.TotalAmount += auditResult.ExtractedData.TotalAmount
		}

		// Decision priority: FAIL > NEEDS_REVIEW > PASS
		result.Decision = a.higherPriorityDecision(result.Decision, auditResult.OverallDecision)

		// Collect all violations from policy checks
		if auditResult.PolicyResult != nil {
			for _, v := range auditResult.PolicyResult.Violations {
				if !violationSet[v] {
					violationSet[v] = true
					result.Violations = append(result.Violations, v)
				}
			}
		}

		// Collect violations from price verification (amount mismatch)
		if auditResult.PriceVerification != nil {
			pv := auditResult.PriceVerification
			if !pv.AmountMatch && pv.DeviationPercent > 1.0 {
				// Format the amount mismatch violation
				amountViolation := fmt.Sprintf("金额不符：申请金额 ¥%.2f 与发票金额 ¥%.2f 不一致 (偏差 %.1f%%)",
					pv.ClaimedAmount, pv.InvoiceAmount, pv.DeviationPercent)
				if !violationSet[amountViolation] {
					violationSet[amountViolation] = true
					result.Violations = append(result.Violations, amountViolation)
				}
			}
			if !pv.IsReasonable {
				priceViolation := "价格不合理：发票金额超出市场价格合理范围"
				if !violationSet[priceViolation] {
					violationSet[priceViolation] = true
					result.Violations = append(result.Violations, priceViolation)
				}
			}
		}
	}

	result.ProcessedCount = processedCount

	// Calculate average confidence
	if processedCount > 0 {
		result.Confidence = totalConfidence / float64(processedCount)
	} else {
		result.Confidence = 0
	}

	a.logger.Info("Aggregated audit results",
		zap.Int64("instance_id", instance.ID),
		zap.String("decision", result.Decision),
		zap.Float64("confidence", result.Confidence),
		zap.Int("violation_count", len(result.Violations)))

	return result, nil
}

// higherPriorityDecision returns the higher priority decision
// Priority: FAIL > NEEDS_REVIEW > PASS
func (a *AuditAggregator) higherPriorityDecision(current, new string) string {
	priority := map[string]int{
		models.AuditDecisionPass:        0,
		models.AuditDecisionNeedsReview: 1,
		models.AuditDecisionFail:        2,
	}

	currentPriority := priority[current]
	newPriority := priority[new]

	if newPriority > currentPriority {
		return new
	}
	return current
}
