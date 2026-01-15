package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/garyjia/ai-reimbursement/internal/models"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// PolicyValidator validates expenses against company policies
type PolicyValidator struct {
	client   *openai.Client
	policies map[string]interface{}
	model    string
	temp     float32
	logger   *zap.Logger
}

// NewPolicyValidator creates a new policy validator
func NewPolicyValidator(apiKey, model string, temperature float32, policiesPath string, logger *zap.Logger) (*PolicyValidator, error) {
	client := openai.NewClient(apiKey)

	// Load policies from file
	policies, err := loadPolicies(policiesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load policies: %w", err)
	}

	return &PolicyValidator{
		client:   client,
		policies: policies,
		model:    model,
		temp:     temperature,
		logger:   logger,
	}, nil
}

// Validate validates a reimbursement item against policies
func (pv *PolicyValidator) Validate(ctx context.Context, item *models.ReimbursementItem) (*models.PolicyValidationResult, error) {
	// Build the prompt
	prompt := pv.buildValidationPrompt(item)

	pv.logger.Debug("Sending validation request to OpenAI",
		zap.Int64("item_id", item.ID),
		zap.String("item_type", item.ItemType))

	// Call OpenAI API
	resp, err := pv.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       pv.model,
		Temperature: pv.temp,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a financial compliance auditor for a Chinese enterprise. Evaluate reimbursement items against company policies. Always respond with valid JSON.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})

	if err != nil {
		pv.logger.Error("OpenAI API call failed", zap.Error(err))
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	// Parse response
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	content := resp.Choices[0].Message.Content

	var result models.PolicyValidationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		pv.logger.Error("Failed to parse OpenAI response",
			zap.Error(err),
			zap.String("content", content))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	pv.logger.Info("Policy validation completed",
		zap.Int64("item_id", item.ID),
		zap.Bool("compliant", result.Compliant),
		zap.Float64("confidence", result.Confidence))

	return &result, nil
}

// buildValidationPrompt builds the validation prompt
func (pv *PolicyValidator) buildValidationPrompt(item *models.ReimbursementItem) string {
	policiesJSON, _ := json.MarshalIndent(pv.policies, "", "  ")

	prompt := fmt.Sprintf(`Evaluate this reimbursement item against company policies:

**Company Policies:**
%s

**Reimbursement Item:**
- Type: %s
- Description: %s
- Amount: %.2f %s

**Required Response Format (JSON):**
{
  "compliant": boolean,
  "violations": [string],
  "confidence": float (0.0 to 1.0),
  "reasoning": string
}

Provide a detailed evaluation. If there are any policy violations, list them in the violations array. Set confidence to reflect your certainty in the assessment.`,
		string(policiesJSON),
		item.ItemType,
		item.Description,
		item.Amount,
		item.Currency,
	)

	return prompt
}

// loadPolicies loads policies from a JSON file
func loadPolicies(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var policies map[string]interface{}
	if err := json.Unmarshal(data, &policies); err != nil {
		return nil, err
	}

	return policies, nil
}
