package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// Auditor orchestrates AI-driven auditing
type Auditor struct {
	policyValidator *PolicyValidator
	priceBenchmarker *PriceBenchmarker
	logger          *zap.Logger
}

// NewAuditor creates a new AI auditor
func NewAuditor(
	policyValidator *PolicyValidator,
	priceBenchmarker *PriceBenchmarker,
	logger *zap.Logger,
) *Auditor {
	return &Auditor{
		policyValidator:  policyValidator,
		priceBenchmarker: priceBenchmarker,
		logger:           logger,
	}
}

// AuditReimbursementItem audits a single reimbursement item
func (a *Auditor) AuditReimbursementItem(ctx context.Context, item *models.ReimbursementItem) (*models.AIAuditResult, error) {
	a.logger.Info("Starting AI audit",
		zap.Int64("item_id", item.ID),
		zap.String("item_type", item.ItemType),
		zap.Float64("amount", item.Amount))

	// Run policy validation
	policyResult, err := a.policyValidator.Validate(ctx, item)
	if err != nil {
		a.logger.Error("Policy validation failed", zap.Error(err))
		return nil, fmt.Errorf("policy validation failed: %w", err)
	}

	// Run price benchmarking
	priceResult, err := a.priceBenchmarker.Benchmark(ctx, item)
	if err != nil {
		a.logger.Error("Price benchmarking failed", zap.Error(err))
		return nil, fmt.Errorf("price benchmarking failed: %w", err)
	}

	// Calculate overall decision and confidence
	overallDecision := a.determineOverallDecision(policyResult, priceResult)
	overallConfidence := (policyResult.Confidence + priceResult.Confidence) / 2

	result := &models.AIAuditResult{
		PolicyValidation: *policyResult,
		PriceBenchmark:   *priceResult,
		OverallDecision:  overallDecision,
		Confidence:       overallConfidence,
	}

	a.logger.Info("AI audit completed",
		zap.Int64("item_id", item.ID),
		zap.String("decision", overallDecision),
		zap.Float64("confidence", overallConfidence))

	return result, nil
}

// AuditReimbursementInstance audits all items in an instance
func (a *Auditor) AuditReimbursementInstance(ctx context.Context, items []*models.ReimbursementItem) (*models.AIAuditResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to audit")
	}

	// Audit each item
	var allPolicyResults []models.PolicyValidationResult
	var allPriceResults []models.PriceBenchmarkResult
	var allViolations []string

	for _, item := range items {
		result, err := a.AuditReimbursementItem(ctx, item)
		if err != nil {
			a.logger.Error("Failed to audit item", zap.Int64("item_id", item.ID), zap.Error(err))
			// Continue with other items
			continue
		}

		// Store individual results in item
		policyJSON, _ := json.Marshal(result.PolicyValidation)
		item.AIPolicyCheck = string(policyJSON)

		priceJSON, _ := json.Marshal(result.PriceBenchmark)
		item.AIPriceCheck = string(priceJSON)

		allPolicyResults = append(allPolicyResults, result.PolicyValidation)
		allPriceResults = append(allPriceResults, result.PriceBenchmark)

		if !result.PolicyValidation.Compliant {
			allViolations = append(allViolations, result.PolicyValidation.Violations...)
		}
	}

	// Aggregate results
	aggregatedPolicy := a.aggregatePolicyResults(allPolicyResults, allViolations)
	aggregatedPrice := a.aggregatePriceResults(allPriceResults)

	overallDecision := a.determineOverallDecision(&aggregatedPolicy, &aggregatedPrice)
	overallConfidence := (aggregatedPolicy.Confidence + aggregatedPrice.Confidence) / 2

	return &models.AIAuditResult{
		PolicyValidation: aggregatedPolicy,
		PriceBenchmark:   aggregatedPrice,
		OverallDecision:  overallDecision,
		Confidence:       overallConfidence,
	}, nil
}

// determineOverallDecision determines the overall audit decision
func (a *Auditor) determineOverallDecision(policy *models.PolicyValidationResult, price *models.PriceBenchmarkResult) string {
	// FAIL if policy violations exist
	if !policy.Compliant {
		return "FAIL"
	}

	// NEEDS_REVIEW if price is unreasonable
	if !price.Reasonable {
		return "NEEDS_REVIEW"
	}

	// NEEDS_REVIEW if confidence is too low
	if policy.Confidence < 0.8 || price.Confidence < 0.7 {
		return "NEEDS_REVIEW"
	}

	// PASS otherwise
	return "PASS"
}

// aggregatePolicyResults aggregates multiple policy validation results
func (a *Auditor) aggregatePolicyResults(results []models.PolicyValidationResult, allViolations []string) models.PolicyValidationResult {
	if len(results) == 0 {
		return models.PolicyValidationResult{
			Compliant:  false,
			Confidence: 0,
			Reasoning:  "No items audited",
		}
	}

	compliant := true
	totalConfidence := 0.0

	for _, result := range results {
		if !result.Compliant {
			compliant = false
		}
		totalConfidence += result.Confidence
	}

	avgConfidence := totalConfidence / float64(len(results))

	return models.PolicyValidationResult{
		Compliant:  compliant,
		Violations: allViolations,
		Confidence: avgConfidence,
		Reasoning:  fmt.Sprintf("Aggregated from %d items", len(results)),
	}
}

// aggregatePriceResults aggregates multiple price benchmark results
func (a *Auditor) aggregatePriceResults(results []models.PriceBenchmarkResult) models.PriceBenchmarkResult {
	if len(results) == 0 {
		return models.PriceBenchmarkResult{
			Reasonable: false,
			Confidence: 0,
			Reasoning:  "No items audited",
		}
	}

	reasonable := true
	totalConfidence := 0.0
	totalDeviation := 0.0

	for _, result := range results {
		if !result.Reasonable {
			reasonable = false
		}
		totalConfidence += result.Confidence
		totalDeviation += result.DeviationPercentage
	}

	avgConfidence := totalConfidence / float64(len(results))
	avgDeviation := totalDeviation / float64(len(results))

	return models.PriceBenchmarkResult{
		DeviationPercentage: avgDeviation,
		Reasonable:          reasonable,
		Confidence:          avgConfidence,
		Reasoning:           fmt.Sprintf("Aggregated from %d items", len(results)),
	}
}
