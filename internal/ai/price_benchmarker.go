package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/models"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// PriceBenchmarker benchmarks expenses against market prices
type PriceBenchmarker struct {
	client             *openai.Client
	model              string
	temp               float32
	deviationThreshold float64
	logger             *zap.Logger
}

// NewPriceBenchmarker creates a new price benchmarker
func NewPriceBenchmarker(apiKey, model string, temperature float32, deviationThreshold float64, logger *zap.Logger) *PriceBenchmarker {
	client := openai.NewClient(apiKey)

	return &PriceBenchmarker{
		client:             client,
		model:              model,
		temp:               temperature,
		deviationThreshold: deviationThreshold,
		logger:             logger,
	}
}

// Benchmark benchmarks a reimbursement item price
func (pb *PriceBenchmarker) Benchmark(ctx context.Context, item *models.ReimbursementItem) (*models.PriceBenchmarkResult, error) {
	// Build the prompt
	prompt := pb.buildBenchmarkPrompt(item)

	pb.logger.Debug("Sending benchmark request to OpenAI",
		zap.Int64("item_id", item.ID),
		zap.Float64("amount", item.Amount))

	// Call OpenAI API
	resp, err := pb.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       pb.model,
		Temperature: pb.temp,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a market price analyst for business expenses in China. Estimate if expenses are reasonable based on market rates. Always respond with valid JSON.",
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
		pb.logger.Error("OpenAI API call failed", zap.Error(err))
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	// Parse response
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	content := resp.Choices[0].Message.Content

	var result models.PriceBenchmarkResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		pb.logger.Error("Failed to parse OpenAI response",
			zap.Error(err),
			zap.String("content", content))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Set submitted price
	result.SubmittedPrice = item.Amount

	// Calculate deviation if not provided
	if len(result.EstimatedPriceRange) == 2 && result.DeviationPercentage == 0 {
		avgPrice := (result.EstimatedPriceRange[0] + result.EstimatedPriceRange[1]) / 2
		if avgPrice > 0 {
			result.DeviationPercentage = ((item.Amount - avgPrice) / avgPrice) * 100
		}
	}

	// Determine if reasonable based on deviation threshold
	if result.DeviationPercentage > pb.deviationThreshold*100 {
		result.Reasonable = false
	}

	pb.logger.Info("Price benchmark completed",
		zap.Int64("item_id", item.ID),
		zap.Bool("reasonable", result.Reasonable),
		zap.Float64("deviation", result.DeviationPercentage),
		zap.Float64("confidence", result.Confidence))

	return &result, nil
}

// buildBenchmarkPrompt builds the benchmark prompt
func (pb *PriceBenchmarker) buildBenchmarkPrompt(item *models.ReimbursementItem) string {
	prompt := fmt.Sprintf(`Estimate if this business expense is reasonable based on typical market prices in China:

**Expense Details:**
- Category: %s
- Description: %s
- Submitted Amount: %.2f %s

**Context:**
Consider typical business expense patterns in China, regional price variations, and current market conditions.

**Required Response Format (JSON):**
{
  "estimated_price_range": [min, max],
  "submitted_price": float,
  "deviation_percentage": float,
  "reasonable": boolean,
  "confidence": float (0.0 to 1.0),
  "reasoning": string
}

Provide:
1. estimated_price_range: [minimum, maximum] reasonable price range
2. submitted_price: The amount being claimed (%.2f)
3. deviation_percentage: Percentage deviation from typical price
4. reasonable: Whether the price is within acceptable range
5. confidence: Your confidence in this assessment (0.0-1.0)
6. reasoning: Explanation of your assessment`,
		item.ItemType,
		item.Description,
		item.Amount,
		item.Currency,
		item.Amount,
	)

	return prompt
}
