package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// Auditor implements port.AIAuditor using OpenAI
// ARCH-122: OpenAI client migration to infrastructure layer
type Auditor struct {
	client             *openai.Client
	policies           map[string]interface{}
	model              string
	deviationThreshold float64
	prompts            *PromptConfig
	logger             *zap.Logger
}

// NewAuditor creates a new OpenAI auditor
func NewAuditor(apiKey, model string, policies map[string]interface{}, deviationThreshold float64, prompts *PromptConfig, logger *zap.Logger) *Auditor {
	return &Auditor{
		client:             openai.NewClient(apiKey),
		policies:           policies,
		model:              model,
		deviationThreshold: deviationThreshold,
		prompts:            prompts,
		logger:             logger,
	}
}

// AuditPolicy validates a reimbursement item against company policies
func (a *Auditor) AuditPolicy(ctx context.Context, item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) (*port.PolicyAuditResult, error) {
	a.logger.Debug("Auditing policy",
		zap.Int64("item_id", item.ID),
		zap.String("item_type", item.ItemType))

	prompt, err := a.buildPolicyPrompt(item, invoiceData)
	if err != nil {
		a.logger.Error("Failed to build policy prompt", zap.Error(err))
		return nil, fmt.Errorf("failed to build policy prompt: %w", err)
	}

	req := openai.ChatCompletionRequest{
		Model:       a.model,
		Temperature: a.prompts.PolicyAudit.Temperature,
		MaxTokens:   a.prompts.PolicyAudit.MaxTokens,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: a.prompts.PolicyAudit.System,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		a.logger.Error("OpenAI API call failed", zap.Error(err))
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	content := resp.Choices[0].Message.Content

	var result entity.PolicyValidationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// Fallback: try to extract JSON from markdown code blocks
		if jsonStr := extractJSON(content); jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
				a.logger.Info("Extracted JSON from response")
				return a.toPolicyAuditResult(&result), nil
			}
		}

		a.logger.Error("Failed to parse OpenAI response",
			zap.Error(err),
			zap.String("content", content))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	a.logger.Info("Policy validation completed",
		zap.Int64("item_id", item.ID),
		zap.Bool("compliant", result.Compliant),
		zap.Float64("confidence", result.Confidence))

	return a.toPolicyAuditResult(&result), nil
}

// AuditPrice benchmarks a reimbursement item price against market rates
func (a *Auditor) AuditPrice(ctx context.Context, item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) (*port.PriceAuditResult, error) {
	a.logger.Debug("Auditing price",
		zap.Int64("item_id", item.ID),
		zap.Float64("amount", item.Amount))

	prompt, err := a.buildPricePrompt(item, invoiceData)
	if err != nil {
		a.logger.Error("Failed to build price prompt", zap.Error(err))
		return nil, fmt.Errorf("failed to build price prompt: %w", err)
	}

	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       a.model,
		Temperature: a.prompts.PriceAudit.Temperature,
		MaxTokens:   a.prompts.PriceAudit.MaxTokens,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: a.prompts.PriceAudit.System,
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
		a.logger.Error("OpenAI API call failed", zap.Error(err))
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	content := resp.Choices[0].Message.Content

	var result entity.PriceBenchmarkResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		a.logger.Error("Failed to parse OpenAI response",
			zap.Error(err),
			zap.String("content", content))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Calculate deviation if not provided
	if len(result.EstimatedPriceRange) == 2 && result.DeviationPercentage == 0 {
		avgPrice := (result.EstimatedPriceRange[0] + result.EstimatedPriceRange[1]) / 2
		if avgPrice > 0 {
			result.DeviationPercentage = ((item.Amount - avgPrice) / avgPrice) * 100
		}
	}

	// Determine if reasonable based on deviation threshold
	if result.DeviationPercentage > a.deviationThreshold*100 {
		result.Reasonable = false
	}

	a.logger.Info("Price audit completed",
		zap.Int64("item_id", item.ID),
		zap.Bool("reasonable", result.Reasonable),
		zap.Float64("deviation", result.DeviationPercentage),
		zap.Float64("confidence", result.Confidence))

	return a.toPriceAuditResult(&result, len(result.EstimatedPriceRange)), nil
}

// ExtractInvoice extracts invoice data from image using GPT-4 Vision
func (a *Auditor) ExtractInvoice(ctx context.Context, imageData []byte, mimeType string) (*port.InvoiceExtractionResult, error) {
	a.logger.Info("Extracting invoice data with Vision API", zap.String("mime_type", mimeType))

	// Encode image as base64
	base64Img := encodeBase64(imageData)

	// Build multi-modal message
	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       a.model,
		MaxTokens:   a.prompts.InvoiceExtraction.MaxTokens,
		Temperature: a.prompts.InvoiceExtraction.Temperature,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: a.prompts.InvoiceExtraction.System,
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: a.prompts.InvoiceExtraction.UserTemplate,
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL:    fmt.Sprintf("data:%s;base64,%s", mimeType, base64Img),
							Detail: openai.ImageURLDetailHigh,
						},
					},
				},
			},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})

	if err != nil {
		a.logger.Error("Vision API call failed", zap.Error(err))
		return &port.InvoiceExtractionResult{
			Success: false,
			Error:   fmt.Sprintf("Vision API call failed: %v", err),
		}, nil
	}

	if len(resp.Choices) == 0 {
		return &port.InvoiceExtractionResult{
			Success: false,
			Error:   "no response from Vision API",
		}, nil
	}

	content := resp.Choices[0].Message.Content

	var extractedData entity.ExtractedInvoiceData
	if err := json.Unmarshal([]byte(content), &extractedData); err != nil {
		a.logger.Error("Failed to parse Vision API response",
			zap.Error(err),
			zap.String("content", content))
		return &port.InvoiceExtractionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse response: %v", err),
		}, nil
	}

	a.logger.Info("Invoice data extracted successfully",
		zap.String("invoice_code", extractedData.InvoiceCode),
		zap.String("invoice_number", extractedData.InvoiceNumber),
		zap.Float64("total_amount", extractedData.TotalAmount))

	return a.toInvoiceExtractionResult(&extractedData), nil
}

// buildPolicyPrompt builds the policy validation prompt
func (a *Auditor) buildPolicyPrompt(item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) (string, error) {
	policiesJSON, _ := json.MarshalIndent(a.policies, "", "  ")

	invoiceInfo := "No invoice data available"
	if invoiceData != nil && invoiceData.Success {
		invoiceInfo = fmt.Sprintf("Invoice Amount: %.2f, Seller: %s, Date: %s",
			invoiceData.TotalAmount, invoiceData.SellerName, invoiceData.InvoiceDate)
	}

	data := map[string]interface{}{
		"Policies":    string(policiesJSON),
		"ItemType":    item.ItemType,
		"Description": item.Description,
		"Amount":      item.Amount,
		"Currency":    item.Currency,
		"InvoiceInfo": invoiceInfo,
	}

	return renderTemplate(a.prompts.PolicyAudit.UserTemplate, data)
}

// buildPricePrompt builds the price benchmarking prompt
func (a *Auditor) buildPricePrompt(item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) (string, error) {
	invoiceInfo := ""
	if invoiceData != nil && invoiceData.Success {
		invoiceInfo = fmt.Sprintf("\n- Invoice Amount: %.2f\n- Invoice Date: %s\n- Seller: %s",
			invoiceData.TotalAmount, invoiceData.InvoiceDate, invoiceData.SellerName)
	}

	data := map[string]interface{}{
		"ItemType":        item.ItemType,
		"Description":     item.Description,
		"Amount":          item.Amount,
		"Currency":        item.Currency,
		"InvoiceInfo":     invoiceInfo,
		"SubmittedPrice":  item.Amount,
	}

	return renderTemplate(a.prompts.PriceAudit.UserTemplate, data)
}

// toPolicyAuditResult converts entity.PolicyValidationResult to port.PolicyAuditResult
func (a *Auditor) toPolicyAuditResult(result *entity.PolicyValidationResult) *port.PolicyAuditResult {
	return &port.PolicyAuditResult{
		Compliant:  result.Compliant,
		Violations: result.Violations,
		Confidence: result.Confidence,
		Reasoning:  result.Reasoning,
	}
}

// toPriceAuditResult converts entity.PriceBenchmarkResult to port.PriceAuditResult
func (a *Auditor) toPriceAuditResult(result *entity.PriceBenchmarkResult, rangeLen int) *port.PriceAuditResult {
	var min, max float64
	if rangeLen >= 2 {
		min = result.EstimatedPriceRange[0]
		max = result.EstimatedPriceRange[1]
	}

	return &port.PriceAuditResult{
		Reasonable:          result.Reasonable,
		DeviationPercentage: result.DeviationPercentage,
		MarketPriceMin:      min,
		MarketPriceMax:      max,
		Confidence:          result.Confidence,
		Reasoning:           result.Reasoning,
	}
}

// toInvoiceExtractionResult converts entity.ExtractedInvoiceData to port.InvoiceExtractionResult
func (a *Auditor) toInvoiceExtractionResult(data *entity.ExtractedInvoiceData) *port.InvoiceExtractionResult {
	extractedData := make(map[string]interface{})
	extractedData["invoice_type"] = data.InvoiceType
	extractedData["amount_without_tax"] = data.AmountWithoutTax
	extractedData["seller_address"] = data.SellerAddress
	extractedData["seller_bank"] = data.SellerBank
	extractedData["buyer_address"] = data.BuyerAddress
	extractedData["buyer_bank"] = data.BuyerBank
	extractedData["items"] = data.Items
	extractedData["remarks"] = data.Remarks
	extractedData["check_code"] = data.CheckCode

	return &port.InvoiceExtractionResult{
		Success:       true,
		InvoiceCode:   data.InvoiceCode,
		InvoiceNumber: data.InvoiceNumber,
		TotalAmount:   data.TotalAmount,
		TaxAmount:     data.TaxAmount,
		InvoiceDate:   data.InvoiceDate,
		SellerName:    data.SellerName,
		SellerTaxID:   data.SellerTaxID,
		BuyerName:     data.BuyerName,
		BuyerTaxID:    data.BuyerTaxID,
		ExtractedData: extractedData,
		Confidence:    1.0, // Default confidence for successful extraction
		Error:         "",
	}
}

// extractJSON extracts JSON from markdown code blocks
func extractJSON(content string) string {
	start := findJSONStart(content)
	if start < 0 {
		return ""
	}
	end := findJSONEnd(content, start)
	if end <= start {
		return ""
	}
	return content[start:end]
}

// findJSONStart finds the start of JSON content in a string
func findJSONStart(content string) int {
	for i := 0; i < len(content); i++ {
		if content[i] == '{' {
			return i
		}
	}
	return -1
}

// findJSONEnd finds the end of JSON content starting at a given position
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

// encodeBase64 encodes byte data to base64 string
func encodeBase64(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	result := make([]byte, 0, ((len(data)+2)/3)*4)

	for i := 0; i < len(data); i += 3 {
		b1 := data[i]
		var b2, b3 byte

		if i+1 < len(data) {
			b2 = data[i+1]
		}
		if i+2 < len(data) {
			b3 = data[i+2]
		}

		result = append(result, base64Chars[b1>>2])
		result = append(result, base64Chars[((b1&0x03)<<4)|(b2>>4)])

		if i+1 < len(data) {
			result = append(result, base64Chars[((b2&0x0f)<<2)|(b3>>6)])
		} else {
			result = append(result, '=')
		}

		if i+2 < len(data) {
			result = append(result, base64Chars[b3&0x3f])
		} else {
			result = append(result, '=')
		}
	}

	return string(result)
}
