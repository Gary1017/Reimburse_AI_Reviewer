package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/ai"
	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

func main() {
	// Parse command line flags
	apiKey := flag.String("key", "", "OpenAI API key (or set OPENAI_API_KEY env var)")
	policyFile := flag.String("policies", "configs/policies.json", "Path to policies.json")
	timeout := flag.Duration("timeout", 30*time.Second, "API call timeout")
	verbose := flag.Bool("verbose", false, "Verbose output")
	flag.Parse()

	// Initialize logger
	var logger *zap.Logger
	var err error
	if *verbose {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Get API key from flag or environment
	if *apiKey == "" {
		*apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if *apiKey == "" {
		fmt.Fprintf(os.Stderr, "ERROR: OPENAI_API_KEY not set and no --key flag provided\n")
		fmt.Fprintf(os.Stderr, "Usage: test-gpt-connection --key sk-... [--policies <path>] [--timeout 30s]\n")
		os.Exit(1)
	}

	fmt.Println("=== GPT-4 Connection Test ===\n")

	// Diagnostic info
	fmt.Println("Configuration:")
	fmt.Printf("  Policy file: %s\n", *policyFile)
	fmt.Printf("  API key length: %d chars\n", len(*apiKey))
	if len(*apiKey) >= 4 {
		fmt.Printf("  API key prefix: %s...\n", (*apiKey)[:4])
	}
	fmt.Printf("  Timeout: %v\n", *timeout)
	fmt.Println()

	// Check policy file exists
	if _, err := os.Stat(*policyFile); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Policy file not found: %s\n", *policyFile)
		os.Exit(1)
	}
	fmt.Printf("✓ Policy file found: %s\n\n", *policyFile)

	// Create policy validator
	fmt.Println("Initializing PolicyValidator...")
	validator, err := ai.NewPolicyValidator(*apiKey, "gpt-4", 0.3, *policyFile, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to create PolicyValidator: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ PolicyValidator initialized\n")

	// Test item
	testItem := &models.ReimbursementItem{
		ID:          1,
		ItemType:    "TRAVEL",
		Description: "Business flight to Beijing for client meeting - economy class",
		Amount:      1500.0,
		Currency:    "USD",
	}

	fmt.Println("Test Reimbursement Item:")
	fmt.Printf("  ID: %d\n", testItem.ID)
	fmt.Printf("  Type: %s\n", testItem.ItemType)
	fmt.Printf("  Description: %s\n", testItem.Description)
	fmt.Printf("  Amount: %.2f %s\n", testItem.Amount, testItem.Currency)
	fmt.Println()

	// Make API call with timeout
	fmt.Println("Sending request to OpenAI GPT-4 API...")
	fmt.Printf("Timeout: %v\n", *timeout)
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	startTime := time.Now()
	result, err := validator.Validate(ctx, testItem)
	duration := time.Since(startTime)

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ERROR: GPT-4 API call failed\n")
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		fmt.Fprintf(os.Stderr, "Possible causes:\n")
		fmt.Fprintf(os.Stderr, "  1. Invalid or expired OPENAI_API_KEY\n")
		fmt.Fprintf(os.Stderr, "  2. Network connectivity issue\n")
		fmt.Fprintf(os.Stderr, "  3. API quota exceeded\n")
		fmt.Fprintf(os.Stderr, "  4. API service unavailable\n")
		fmt.Fprintf(os.Stderr, "  5. Wrong API key format (should start with 'sk-')\n")
		os.Exit(1)
	}

	fmt.Println("✓ Received response from GPT-4!\n")
	fmt.Printf("API Response Time: %v\n", duration)
	fmt.Println()

	// Display results
	fmt.Println("=== Validation Result ===\n")
	fmt.Printf("Compliant: %v\n", result.Compliant)
	fmt.Printf("Confidence: %.2f (%.0f%%)\n", result.Confidence, result.Confidence*100)
	fmt.Printf("Reasoning: %s\n", result.Reasoning)

	if len(result.Violations) > 0 {
		fmt.Println("\nViolations:")
		for i, v := range result.Violations {
			fmt.Printf("  %d. %s\n", i+1, v)
		}
	} else {
		fmt.Println("\n✓ No violations detected")
	}

	// Show JSON response
	fmt.Println("\n=== Full Response (JSON) ===\n")
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonBytes))

	fmt.Println("\n✅ GPT-4 Connection Test PASSED!")
	os.Exit(0)
}
