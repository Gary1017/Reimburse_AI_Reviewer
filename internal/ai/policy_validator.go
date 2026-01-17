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

	// Build request (do NOT use JSON response format - causes issues with some models)
	// Instead, we rely on prompt engineering to get JSON output
	req := openai.ChatCompletionRequest{
		Model:       pv.model,
		Temperature: pv.temp,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a financial compliance auditor for a Chinese enterprise. Evaluate reimbursement items against company policies. Always respond with valid JSON wrapped in ```json and ``` markers.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	// Call OpenAI API
	resp, err := pv.client.CreateChatCompletion(ctx, req)

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
		pv.logger.Warn("Failed to parse OpenAI response as JSON, attempting to extract from text",
			zap.Error(err),
			zap.String("content", content))
		
		// Fallback: try to extract JSON from markdown code blocks
		var jsonStr string
		if start := findJSONStart(content); start >= 0 {
			if end := findJSONEnd(content, start); end > start {
				jsonStr = content[start:end]
				if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
					pv.logger.Info("Extracted JSON from response",
						zap.String("extracted", jsonStr[:50]+"..."))
					return &result, nil
				}
			}
		}
		
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

Please respond with ONLY a valid JSON object (no markdown, no explanation). The JSON must have this exact structure:
{
  "compliant": boolean,
  "violations": [string array of violation descriptions],
  "confidence": number between 0.0 and 1.0,
  "reasoning": string explaining your assessment
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

// findJSONStart finds the start of JSON content in a string
// Looks for '{' that starts valid JSON
func findJSONStart(content string) int {
	for i := 0; i < len(content); i++ {
		if content[i] == '{' {
			return i
		}
	}
	return -1
}

// findJSONEnd finds the end of JSON content starting at a given position
// Counts braces to find matching closing brace
func findJSONEnd(content string, start int) int {
	if start < 0 || start >= len(content) || content[start] != '{' {
		return -1
	}
	
	braceCount := 0
	inString := false
	escapeNext := false
	
	for i := start; i < len(content); i++ {
		char := content[i]
		
		if escapeNext {
			escapeNext = false
			continue
		}
		
		if char == '\\' {
			escapeNext = true
			continue
		}
		
		if char == '"' && !escapeNext {
			inString = !inString
			continue
		}
		
		if inString {
			continue
		}
		
		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 {
				return i + 1
			}
		}
	}
	
	return -1
}
