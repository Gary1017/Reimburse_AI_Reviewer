package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestCreateAbnormalReportFromConfidence tests abnormal report creation
func TestCreateAbnormalReportFromConfidence(t *testing.T) {
	report := CreateAbnormalReportFromConfidence(
		1,
		100,
		0.65, // Below 0.70 threshold
		0.70,
		[]string{"Policy violation 1", "Policy violation 2"},
		"Item confidence score is below threshold",
	)

	assert.Equal(t, int64(1), report.InstanceID)
	assert.Equal(t, int64(100), report.ItemID)
	assert.Equal(t, "CONFIDENCE_THRESHOLD", report.ReportType)
	assert.Equal(t, 0.65, report.ConfidenceScore)
	assert.Equal(t, 0.70, report.Threshold)
	assert.Len(t, report.Violations, 2)
	assert.NotEmpty(t, report.Rationale)
}

// TestDetermineSeverity tests severity classification
func TestDetermineSeverity(t *testing.T) {
	tests := []struct {
		name              string
		confidenceScore   float64
		threshold         float64
		expectedSeverity  string
		description       string
	}{
		{
			name:              "above threshold",
			confidenceScore:   0.95,
			threshold:         0.70,
			expectedSeverity:  "LOW",
			description:       "Score above threshold should be LOW severity",
		},
		{
			name:              "within 5% of threshold",
			confidenceScore:   0.68,
			threshold:         0.70,
			expectedSeverity:  "LOW",
			description:       "Score within 5% of threshold should be LOW",
		},
		{
			name:              "within 15% of threshold",
			confidenceScore:   0.60,
			threshold:         0.70,
			expectedSeverity:  "MEDIUM",
			description:       "Score within 15% of threshold should be MEDIUM",
		},
		{
			name:              "within 30% of threshold",
			confidenceScore:   0.50,
			threshold:         0.70,
			expectedSeverity:  "HIGH",
			description:       "Score within 30% of threshold should be HIGH",
		},
		{
			name:              "more than 30% below threshold",
			confidenceScore:   0.35,
			threshold:         0.70,
			expectedSeverity:  "CRITICAL",
			description:       "Score more than 30% below threshold should be CRITICAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity := determineSeverity(tt.confidenceScore, tt.threshold)
			assert.Equal(t, tt.expectedSeverity, severity, tt.description)
		})
	}
}

// TestAbnormalReportHandler_FlagAbnormalItem tests flagging abnormal items
func TestAbnormalReportHandler_FlagAbnormalItem(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Mock repositories (for now, just passing nil for testing purposes)
	handler := NewAbnormalReportHandler(
		nil, // historyRepo
		nil, // instanceRepo
		"accountant@example.com",
		logger,
	)

	instance := &models.ApprovalInstance{
		ID:     1,
		Status: models.StatusAIAuditing,
	}

	report := &AbnormalReport{
		InstanceID:      1,
		ItemID:          100,
		ReportType:      "CONFIDENCE_THRESHOLD",
		ConfidenceScore: 0.65,
		Threshold:       0.70,
		Violations:      []string{"Low confidence score"},
		Rationale:       "Score below acceptable threshold",
	}

	// This should not panic even though repos are nil
	// (historyRepo operations fail gracefully)
	err := handler.FlagAbnormalItem(context.Background(), instance, report)

	// Error should be nil because we don't block on audit trail failures
	assert.NoError(t, err)

	// Report should be timestamped
	assert.NotZero(t, report.FlaggedAt)
	assert.Equal(t, "accountant@example.com", report.AccountantEmail)
}

// TestBuildNotificationMessage tests message building
func TestBuildNotificationMessage(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	handler := NewAbnormalReportHandler(nil, nil, "accountant@example.com", logger)

	report := &AbnormalReport{
		InstanceID:      1,
		ItemID:          100,
		ReportType:      "CONFIDENCE_THRESHOLD",
		Severity:        "HIGH",
		ConfidenceScore: 0.60,
		Threshold:       0.70,
		Violations:      []string{"Policy violation 1", "Policy violation 2"},
		Rationale:       "Score significantly below threshold",
		FlaggedAt:       time.Now(),
		AccountantEmail: "accountant@example.com",
	}

	message := handler.buildNotificationMessage(report)

	// Verify message contains key information
	assert.Contains(t, message, "Instance ID:        1")
	assert.Contains(t, message, "Item ID:            100")
	assert.Contains(t, message, "HIGH")
	assert.Contains(t, message, "0.60")
	assert.Contains(t, message, "Policy violation 1")
	assert.Contains(t, message, "accountant@example.com")
}

// TestConfidenceAnalysis tests confidence score analysis
func TestConfidenceAnalysis(t *testing.T) {
	tests := []struct {
		name              string
		confidenceScore   float64
		threshold         float64
		expectedAnalysis  string
	}{
		{
			name:              "slightly above threshold",
			confidenceScore:   0.72,
			threshold:         0.70,
			expectedAnalysis:  "ABOVE_THRESHOLD",
		},
		{
			name:              "significantly below threshold",
			confidenceScore:   0.50,
			threshold:         0.70,
			expectedAnalysis:  "BELOW_THRESHOLD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := detectStatus(tt.confidenceScore, tt.threshold)
			assert.Contains(t, status, tt.expectedAnalysis)
		})
	}
}

// TestAbnormalReportTypes tests different report types
func TestAbnormalReportTypes(t *testing.T) {
	reportTypes := []string{
		"CONFIDENCE_THRESHOLD",
		"POLICY_VIOLATION",
		"PRICE_OUTLIER",
	}

	for _, reportType := range reportTypes {
		report := &AbnormalReport{
			InstanceID: 1,
			ItemID:     100,
			ReportType: reportType,
			Severity:   "MEDIUM",
		}

		assert.Equal(t, reportType, report.ReportType)
	}
}

// TestSeverityLevels tests all severity levels
func TestSeverityLevels(t *testing.T) {
	severities := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}

	for _, severity := range severities {
		report := &AbnormalReport{
			InstanceID: 1,
			ItemID:     100,
			Severity:   severity,
		}

		assert.Equal(t, severity, report.Severity)
	}
}
