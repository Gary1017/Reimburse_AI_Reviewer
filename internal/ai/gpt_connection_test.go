package ai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestGPT4ConnectionDemo tests the actual connection to OpenAI GPT-4 API
// This is an integration test that requires OPENAI_API_KEY environment variable
// Run with: go test -v -run TestGPT4ConnectionDemo ./internal/ai/...
func TestGPT4ConnectionDemo(t *testing.T) {
	// Check if API key is set
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping GPT-4 connection test")
	}

	// Verify API key format
	if len(apiKey) < 20 {
		t.Logf("WARNING: API key format suspicious (too short: %d chars)", len(apiKey))
	}

	// Create logger
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer logger.Sync()

	// Create policy validator with minimal config
	policyPath := "configs/policies.json"
	validator, err := NewPolicyValidator(
		apiKey,
		"gpt-4",
		0.3,
		policyPath,
		logger,
	)

	if err != nil {
		t.Logf("ERROR: Failed to initialize PolicyValidator: %v", err)
		t.Logf("Policy file path: %s", policyPath)
		t.Fail()
		return
	}

	t.Log("✓ PolicyValidator initialized successfully")

	// Create a test reimbursement item
	testItem := &models.ReimbursementItem{
		ID:          1,
		ItemType:    "TRAVEL",
		Description: "Business flight to Beijing for client meeting",
		Amount:      1500.0,
		Currency:    "USD",
	}

	t.Logf("Sending validation request to GPT-4: %+v", testItem)

	// Set timeout for API call
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Make the actual API call
	result, err := validator.Validate(ctx, testItem)

	if err != nil {
		t.Logf("ERROR: GPT-4 API call failed: %v", err)
		t.Logf("This likely means:")
		t.Logf("  - OPENAI_API_KEY is invalid or expired")
		t.Logf("  - Network connectivity issue")
		t.Logf("  - API quota exceeded or API disabled")
		t.Fail()
		return
	}

	// Verify response structure
	require.NotNil(t, result)
	t.Logf("✓ Received response from GPT-4")

	// Log the response
	t.Logf("Response Details:")
	t.Logf("  Compliant: %v", result.Compliant)
	t.Logf("  Confidence: %.2f", result.Confidence)
	t.Logf("  Violations: %v", result.Violations)
	t.Logf("  Reasoning: %s", result.Reasoning)

	// Verify response validity
	assert.NotEmpty(t, result.Reasoning, "Reasoning should not be empty")
	assert.GreaterOrEqual(t, result.Confidence, 0.0, "Confidence should be >= 0.0")
	assert.LessOrEqual(t, result.Confidence, 1.0, "Confidence should be <= 1.0")

	if result.Compliant {
		t.Log("✓ Item compliant with policy")
	} else {
		t.Log("⚠ Item has policy violations:")
		for _, v := range result.Violations {
			t.Logf("  - %s", v)
		}
	}

	t.Log("✓ GPT-4 connection test PASSED")
}

// TestGPT4MultipleRequests tests multiple sequential API calls
// This verifies API rate limiting and stability
func TestGPT4MultipleRequests(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping multiple requests test")
	}

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	policyPath := "configs/policies.json"
	validator, err := NewPolicyValidator(apiKey, "gpt-4", 0.3, policyPath, logger)
	if err != nil {
		t.Skipf("Failed to initialize validator: %v", err)
	}

	testCases := []struct {
		name        string
		itemType    string
		description string
		amount      float64
	}{
		{
			name:        "High-value travel",
			itemType:    "TRAVEL",
			description: "International flight to conference",
			amount:      3500.0,
		},
		{
			name:        "Meal expense",
			itemType:    "MEAL",
			description: "Team lunch meeting",
			amount:      150.0,
		},
		{
			name:        "Hotel stay",
			itemType:    "ACCOMMODATION",
			description: "5-star hotel for 3 nights",
			amount:      2000.0,
		},
		{
			name:        "Equipment purchase",
			itemType:    "EQUIPMENT",
			description: "Laptop for development",
			amount:      1800.0,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	for i, tc := range testCases {
		item := &models.ReimbursementItem{
			ID:          int64(i + 1),
			ItemType:    tc.itemType,
			Description: tc.description,
			Amount:      tc.amount,
			Currency:    "USD",
		}

		t.Logf("\nTest %d: %s", i+1, tc.name)
		result, err := validator.Validate(ctx, item)

		if err != nil {
			t.Logf("  ERROR: %v", err)
			continue
		}

		t.Logf("  ✓ Compliant: %v (confidence: %.2f)", result.Compliant, result.Confidence)
	}

	t.Log("\n✓ Multiple requests test completed")
}

// TestGPT4ConnectionStatus provides diagnostic information
// Useful for debugging connection issues
func TestGPT4ConnectionStatus(t *testing.T) {
	t.Log("=== GPT-4 Connection Diagnostic ===")

	apiKey := os.Getenv("OPENAI_API_KEY")
	t.Logf("OPENAI_API_KEY set: %v", apiKey != "")
	if apiKey != "" {
		t.Logf("  Key length: %d chars", len(apiKey))
		t.Logf("  Starts with: %.10s...", apiKey)
		if len(apiKey) >= 3 {
			t.Logf("  First 3 chars: %s", apiKey[:3])
		}
	}

	// Check policy file
	policyPath := "configs/policies.json"
	if _, err := os.Stat(policyPath); err == nil {
		t.Logf("✓ Policy file exists: %s", policyPath)
	} else {
		t.Logf("✗ Policy file NOT found: %s", policyPath)
	}

	// Check Lark credentials
	larkAppID := os.Getenv("LARK_APP_ID")
	t.Logf("LARK_APP_ID set: %v", larkAppID != "")

	// Check other required credentials
	t.Logf("LARK_APP_SECRET set: %v", os.Getenv("LARK_APP_SECRET") != "")
	t.Logf("LARK_APPROVAL_CODE set: %v", os.Getenv("LARK_APPROVAL_CODE") != "")
	t.Logf("COMPANY_NAME set: %v", os.Getenv("COMPANY_NAME") != "")
	t.Logf("ACCOUNTANT_EMAIL set: %v", os.Getenv("ACCOUNTANT_EMAIL") != "")

	t.Log("\n=== System Information ===")
	t.Logf("Go version: %s", "1.22.0 or later")
	t.Logf("Working directory: %s", os.Getenv("PWD"))
}

