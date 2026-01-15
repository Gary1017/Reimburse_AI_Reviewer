package ai

import (
	"testing"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

func TestDetermineOverallDecision(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	auditor := &Auditor{logger: logger}

	tests := []struct {
		name         string
		policy       *models.PolicyValidationResult
		price        *models.PriceBenchmarkResult
		wantDecision string
	}{
		{
			name: "PASS when both compliant and reasonable",
			policy: &models.PolicyValidationResult{
				Compliant:  true,
				Confidence: 0.9,
			},
			price: &models.PriceBenchmarkResult{
				Reasonable: true,
				Confidence: 0.8,
			},
			wantDecision: "PASS",
		},
		{
			name: "FAIL when policy not compliant",
			policy: &models.PolicyValidationResult{
				Compliant:  false,
				Confidence: 0.9,
			},
			price: &models.PriceBenchmarkResult{
				Reasonable: true,
				Confidence: 0.8,
			},
			wantDecision: "FAIL",
		},
		{
			name: "NEEDS_REVIEW when price unreasonable",
			policy: &models.PolicyValidationResult{
				Compliant:  true,
				Confidence: 0.9,
			},
			price: &models.PriceBenchmarkResult{
				Reasonable: false,
				Confidence: 0.8,
			},
			wantDecision: "NEEDS_REVIEW",
		},
		{
			name: "NEEDS_REVIEW when confidence too low",
			policy: &models.PolicyValidationResult{
				Compliant:  true,
				Confidence: 0.6,
			},
			price: &models.PriceBenchmarkResult{
				Reasonable: true,
				Confidence: 0.5,
			},
			wantDecision: "NEEDS_REVIEW",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := auditor.determineOverallDecision(tt.policy, tt.price)
			if got != tt.wantDecision {
				t.Errorf("determineOverallDecision() = %v, want %v", got, tt.wantDecision)
			}
		})
	}
}
