package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"github.com/garyjia/ai-reimbursement/internal/models"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// Auditor implements port.AIAuditor using OpenAI
// ARCH-122: OpenAI client migration to infrastructure layer
type Auditor struct {
	client             *openai.Client
	policies           map[string]interface{}
	model              string
	temperature        float32
	deviationThreshold float64
	logger             *zap.Logger
}

// NewAuditor creates a new OpenAI auditor
func NewAuditor(apiKey, model string, temperature float32, policies map[string]interface{}, deviationThreshold float64, logger *zap.Logger) *Auditor {
	return &Auditor{
		client:             openai.NewClient(apiKey),
		policies:           policies,
		model:              model,
		temperature:        temperature,
		deviationThreshold: deviationThreshold,
		logger:             logger,
	}
}

// AuditPolicy validates a reimbursement item against company policies
func (a *Auditor) AuditPolicy(ctx context.Context, item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) (*port.PolicyAuditResult, error) {
	a.logger.Debug("Auditing policy",
		zap.Int64("item_id", item.ID),
		zap.String("item_type", item.ItemType))

	prompt := a.buildPolicyPrompt(item, invoiceData)

	req := openai.ChatCompletionRequest{
		Model:       a.model,
		Temperature: a.temperature,
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

	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		a.logger.Error("OpenAI API call failed", zap.Error(err))
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	content := resp.Choices[0].Message.Content

	var result models.PolicyValidationResult
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

	prompt := a.buildPricePrompt(item, invoiceData)

	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       a.model,
		Temperature: a.temperature,
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
		a.logger.Error("OpenAI API call failed", zap.Error(err))
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	content := resp.Choices[0].Message.Content

	var result models.PriceBenchmarkResult
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

	// Build vision prompt
	prompt := a.buildVisionPrompt()

	// Encode image as base64
	base64Img := encodeBase64(imageData)

	// Build multi-modal message
	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       a.model,
		MaxTokens:   4096,
		Temperature: 0.1,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert in reading and extracting data from Chinese invoices (发票). You have perfect accuracy in reading invoice codes, numbers, amounts, and all other fields. Always respond with valid JSON.",
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: prompt,
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

	var extractedData models.ExtractedInvoiceData
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
func (a *Auditor) buildPolicyPrompt(item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) string {
	policiesJSON, _ := json.MarshalIndent(a.policies, "", "  ")

	invoiceInfo := "No invoice data available"
	if invoiceData != nil && invoiceData.Success {
		invoiceInfo = fmt.Sprintf("Invoice Amount: %.2f, Seller: %s, Date: %s",
			invoiceData.TotalAmount, invoiceData.SellerName, invoiceData.InvoiceDate)
	}

	prompt := fmt.Sprintf(`Evaluate this reimbursement item against company policies:

**Company Policies:**
%s

**Reimbursement Item:**
- Type: %s
- Description: %s
- Amount: %.2f %s

**Invoice Information:**
%s

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
		invoiceInfo,
	)

	return prompt
}

// buildPricePrompt builds the price benchmarking prompt
func (a *Auditor) buildPricePrompt(item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) string {
	invoiceInfo := ""
	if invoiceData != nil && invoiceData.Success {
		invoiceInfo = fmt.Sprintf("\n- Invoice Amount: %.2f\n- Invoice Date: %s\n- Seller: %s",
			invoiceData.TotalAmount, invoiceData.InvoiceDate, invoiceData.SellerName)
	}

	prompt := fmt.Sprintf(`Estimate if this business expense is reasonable based on typical market prices in China:

**Expense Details:**
- Category: %s
- Description: %s
- Submitted Amount: %.2f %s%s

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
		invoiceInfo,
		item.Amount,
	)

	return prompt
}

// buildVisionPrompt builds the prompt for Vision API extraction
func (a *Auditor) buildVisionPrompt() string {
	return `Carefully examine this Chinese invoice image (发票) and extract ALL information.

This is a critical financial document. Please extract with 100% accuracy:

REQUIRED FIELDS (发票基本信息):
- invoice_code (发票代码): Usually 10 or 12 digits at the top
- invoice_number (发票号码): Usually 8 digits
- invoice_type (发票类型): 增值税专用发票, 增值税普通发票, 电子发票, etc.
- invoice_date (开票日期): Date in YYYY-MM-DD format

AMOUNT FIELDS (金额信息):
- total_amount (价税合计): Total amount INCLUDING tax - this is the main amount
- tax_amount (税额): Tax amount only
- amount_without_tax (金额): Amount BEFORE tax

SELLER INFORMATION (销售方信息):
- seller_name (销售方名称): Company name
- seller_tax_id (销售方税号): Tax registration number (15-20 chars)
- seller_address (销售方地址电话)
- seller_bank (销售方开户行及账号)

BUYER INFORMATION (购买方信息):
- buyer_name (购买方名称): Company name
- buyer_tax_id (购买方税号): Tax registration number
- buyer_address (购买方地址电话)
- buyer_bank (购买方开户行及账号)

LINE ITEMS (明细项目) - Extract ALL line items as an array:
- name (项目名称)
- specification (规格型号)
- unit (单位)
- quantity (数量)
- unit_price (单价)
- amount (金额)
- tax_rate (税率) - as decimal, e.g. 0.13 for 13%
- tax_amount (税额)

OTHER FIELDS:
- check_code (校验码): Verification code if visible
- remarks (备注): Any remarks or notes

Return a JSON object with this exact structure:
{
  "invoice_code": "string",
  "invoice_number": "string",
  "invoice_type": "string",
  "invoice_date": "YYYY-MM-DD",
  "total_amount": number,
  "tax_amount": number,
  "amount_without_tax": number,
  "seller_name": "string",
  "seller_tax_id": "string",
  "seller_address": "string",
  "seller_bank": "string",
  "buyer_name": "string",
  "buyer_tax_id": "string",
  "buyer_address": "string",
  "buyer_bank": "string",
  "items": [{"name": "string", "specification": "string", "unit": "string", "quantity": number, "unit_price": number, "amount": number, "tax_rate": number, "tax_amount": number}],
  "check_code": "string",
  "remarks": "string"
}

IMPORTANT:
- Extract EXACTLY what you see. Do not guess or make up values.
- For amounts, use numbers without currency symbols.
- If a field is not visible or unclear, use empty string "" or 0.`
}

// toPolicyAuditResult converts models.PolicyValidationResult to port.PolicyAuditResult
func (a *Auditor) toPolicyAuditResult(result *models.PolicyValidationResult) *port.PolicyAuditResult {
	return &port.PolicyAuditResult{
		Compliant:  result.Compliant,
		Violations: result.Violations,
		Confidence: result.Confidence,
		Reasoning:  result.Reasoning,
	}
}

// toPriceAuditResult converts models.PriceBenchmarkResult to port.PriceAuditResult
func (a *Auditor) toPriceAuditResult(result *models.PriceBenchmarkResult, rangeLen int) *port.PriceAuditResult {
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

// toInvoiceExtractionResult converts models.ExtractedInvoiceData to port.InvoiceExtractionResult
func (a *Auditor) toInvoiceExtractionResult(data *models.ExtractedInvoiceData) *port.InvoiceExtractionResult {
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
