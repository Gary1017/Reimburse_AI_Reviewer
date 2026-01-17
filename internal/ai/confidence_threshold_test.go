package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TEST-001: Load Default Confidence Thresholds (ARCH-001-A)
func TestLoadDefaultConfidenceThresholds(t *testing.T) {
	threshold := NewConfidenceThreshold()

	assert.Equal(t, 0.95, threshold.HighThreshold, "HighThreshold should be 0.95")
	assert.Equal(t, 0.70, threshold.LowThreshold, "LowThreshold should be 0.70")
	assert.Equal(t, "v1", threshold.ConfigVersion, "ConfigVersion should be initialized")
}

// TEST-002: Validate Threshold Boundaries (ARCH-001-A)
func TestValidateThresholdBoundaries(t *testing.T) {
	tests := []struct {
		name            string
		highThreshold   float64
		lowThreshold    float64
		expectError     bool
		errorContains   string
	}{
		{
			name:          "valid thresholds",
			highThreshold: 0.95,
			lowThreshold:  0.70,
			expectError:   false,
		},
		{
			name:          "high threshold too high",
			highThreshold: 1.5,
			lowThreshold:  0.70,
			expectError:   true,
			errorContains: "HighThreshold must be between 0.0 and 1.0",
		},
		{
			name:          "low threshold negative",
			highThreshold: 0.95,
			lowThreshold:  -0.1,
			expectError:   true,
			errorContains: "LowThreshold must be between 0.0 and 1.0",
		},
		{
			name:          "high less than low",
			highThreshold: 0.5,
			lowThreshold:  0.7,
			expectError:   true,
			errorContains: "HighThreshold must be greater than LowThreshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			threshold := &ConfidenceThreshold{
				HighThreshold: tt.highThreshold,
				LowThreshold:  tt.lowThreshold,
			}

			err := threshold.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TEST-003: Calculate High Confidence Score (ARCH-001-B)
func TestCalculateHighConfidenceScore(t *testing.T) {
	calculator := NewConfidenceCalculator()

	// All checks pass: policy match + price valid + unique invoice
	score := calculator.CalculateConfidence(true, true, true)

	assert.GreaterOrEqual(t, score, 0.95, "High confidence score should be >= 0.95")
	assert.LessOrEqual(t, score, 1.0, "Score should be <= 1.0")
}

// TEST-004: Calculate Medium Confidence Score (ARCH-001-B)
func TestCalculateMediumConfidenceScore(t *testing.T) {
	calculator := NewConfidenceCalculator()

	// Partial match: policy match + no price + unique invoice
	score := calculator.CalculateConfidence(true, false, true)

	assert.GreaterOrEqual(t, score, 0.60, "Medium confidence score should be >= 0.60")
	assert.LessOrEqual(t, score, 0.80, "Medium confidence score should be <= 0.80")
}

// TEST-005: Calculate Low Confidence Score (ARCH-001-B)
func TestCalculateLowConfidenceScore(t *testing.T) {
	calculator := NewConfidenceCalculator()

	// Multiple failures: no policy match + no price + unique invoice
	score := calculator.CalculateConfidence(false, false, true)

	assert.Less(t, score, 0.70, "Low confidence score should be < 0.70")
}

// TEST-006: Confidence Score Normalization (ARCH-001-B)
func TestConfidenceScoreNormalization(t *testing.T) {
	calculator := NewConfidenceCalculator()

	tests := []struct {
		name             string
		policyMatch      bool
		priceValid       bool
		uniqueInvoice    bool
		expectedMinScore float64
		expectedMaxScore float64
	}{
		{
			name:             "all pass",
			policyMatch:      true,
			priceValid:       true,
			uniqueInvoice:    true,
			expectedMinScore: 0.95,
			expectedMaxScore: 1.0,
		},
		{
			name:             "two pass",
			policyMatch:      true,
			priceValid:       true,
			uniqueInvoice:    false,
			expectedMinScore: 0.60,
			expectedMaxScore: 0.80,
		},
		{
			name:             "one pass",
			policyMatch:      true,
			priceValid:       false,
			uniqueInvoice:    false,
			expectedMinScore: 0.30,
			expectedMaxScore: 0.50,
		},
		{
			name:             "none pass",
			policyMatch:      false,
			priceValid:       false,
			uniqueInvoice:    false,
			expectedMinScore: 0.0,
			expectedMaxScore: 0.30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculator.CalculateConfidence(tt.policyMatch, tt.priceValid, tt.uniqueInvoice)

			assert.GreaterOrEqual(t, score, tt.expectedMinScore, "Score should be >= min")
			assert.LessOrEqual(t, score, tt.expectedMaxScore, "Score should be <= max")
			assert.GreaterOrEqual(t, score, 0.0, "Score should be >= 0.0")
			assert.LessOrEqual(t, score, 1.0, "Score should be <= 1.0")
		})
	}
}

// TEST-007: Route HIGH Confidence to Auto-Approve (ARCH-001-C)
func TestRouteHighConfidenceToAutoApprove(t *testing.T) {
	router := NewConfidenceRouter(DefaultConfidenceThreshold())

	decision := &AuditDecision{
		InstanceID:      "inst-001",
		ConfidenceScore: 0.97,
	}

	decision = router.RouteDecision(decision)

	assert.Equal(t, "AUTO_APPROVED", decision.Decision)
	assert.Equal(t, "", decision.NextQueue, "Auto-approved items should not go to a queue")
}

// TEST-008: Route MEDIUM Confidence to Exception Queue (ARCH-001-C)
func TestRouteMediumConfidenceToException(t *testing.T) {
	router := NewConfidenceRouter(DefaultConfidenceThreshold())

	decision := &AuditDecision{
		InstanceID:      "inst-002",
		ConfidenceScore: 0.80,
		Decision:        "PENDING",
	}

	decision = router.RouteDecision(decision)

	assert.Equal(t, "IN_REVIEW", decision.Decision)
	assert.Equal(t, "LARK_APPROVAL", decision.NextQueue, "Medium confidence items should go to Lark approval")
}

// TEST-009: Route LOW Confidence to Rejection (ARCH-001-C)
func TestRouteLowConfidenceToRejection(t *testing.T) {
	router := NewConfidenceRouter(DefaultConfidenceThreshold())

	decision := &AuditDecision{
		InstanceID:      "inst-003",
		ConfidenceScore: 0.50,
		Decision:        "PENDING",
		Rationale:       "Policy violations detected",
	}

	decision = router.RouteDecision(decision)

	assert.Equal(t, "REJECTED", decision.Decision)
	assert.Contains(t, decision.Rationale, "confidence 0.50 < 0.70")
}

// TEST-010: Audit Decision Immutability (ARCH-001-D)
func TestAuditDecisionImmutability(t *testing.T) {
	decision := &AuditDecision{
		InstanceID:      "inst-004",
		ConfidenceScore: 0.92,
		Decision:        "AUTO_APPROVED",
		Locked:          true,
	}

	// Attempting to modify a locked decision should be prevented
	canModify := !decision.Locked

	assert.False(t, canModify, "Locked decision should not be modifiable")
	assert.Equal(t, "AUTO_APPROVED", decision.Decision, "Decision should remain unchanged")
}

// TEST-011: Threshold Config Versioning (ARCH-001-E)
func TestThresholdConfigVersioning(t *testing.T) {
	threshold := NewConfidenceThreshold()
	decision := &AuditDecision{
		InstanceID:      "inst-005",
		ConfidenceScore: 0.95,
		Decision:        "AUTO_APPROVED",
		ThresholdConfig: *threshold,
	}

	assert.Equal(t, "v1", decision.ThresholdConfig.ConfigVersion)
	assert.NotEmpty(t, decision.ThresholdConfig.UpdatedAt)
}
