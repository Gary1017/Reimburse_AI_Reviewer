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

// Matcher implements port.AIInvoiceMatcher using OpenAI
type Matcher struct {
	client *openai.Client
	model  string
	logger *zap.Logger
}

// NewMatcher creates a new OpenAI invoice matcher
func NewMatcher(apiKey, model string, logger *zap.Logger) *Matcher {
	return &Matcher{
		client: openai.NewClient(apiKey),
		model:  model,
		logger: logger,
	}
}

// MatchInvoicesToItems performs AI-powered matching of invoices to reimbursement items
func (m *Matcher) MatchInvoicesToItems(ctx context.Context, items []*entity.ReimbursementItem, invoices []*entity.InvoiceV2) (*port.InvoiceMatchingResult, error) {
	m.logger.Info("Starting invoice matching",
		zap.Int("item_count", len(items)),
		zap.Int("invoice_count", len(invoices)))

	// Build JSON prompt data
	itemsJSON, err := m.buildItemsJSON(items)
	if err != nil {
		m.logger.Error("Failed to build items JSON", zap.Error(err))
		return nil, fmt.Errorf("build items JSON: %w", err)
	}

	invoicesJSON, err := m.buildInvoicesJSON(invoices)
	if err != nil {
		m.logger.Error("Failed to build invoices JSON", zap.Error(err))
		return nil, fmt.Errorf("build invoices JSON: %w", err)
	}

	// Build user prompt
	userPrompt := fmt.Sprintf(`Match each invoice to the most likely reimbursement item based on:
- Amount (invoice amount should closely match item amount)
- Date (invoice date should be close to expense date)
- Vendor/Seller name (should match or be related)
- Description (item description should relate to invoice items)

Items:
%s

Invoices:
%s

Return a JSON object with this structure:
{
  "matches": [
    {
      "attachment_id": <invoice attachment_id>,
      "item_id": <matched item id>,
      "invoice_id": <invoice id>,
      "confidence": <0.0-1.0 confidence score>,
      "reasoning": "<brief explanation of why this match makes sense>"
    }
  ],
  "confidence": <0.0-1.0 overall confidence in the matching result>,
  "reasoning": "<overall reasoning about the matching process>"
}

Rules:
- Each invoice should match exactly one item (1:1 mapping)
- Only include matches with confidence >= 0.5
- If an invoice cannot be matched confidently, omit it from matches
- Consider amount tolerance (within 5%% is acceptable for rounding differences)`, itemsJSON, invoicesJSON)

	systemPrompt := `You are an intelligent invoice matching assistant. Your task is to match invoices to reimbursement items based on amount, date, vendor name, and description.

Return valid JSON only. Be conservative - only match invoices when you have reasonable confidence (>= 0.5). Consider:
1. Amount proximity (exact or within 5%%)
2. Date proximity (within 30 days is acceptable)
3. Vendor/seller name similarity
4. Description relevance

Provide clear reasoning for each match and overall confidence.`

	// Call OpenAI API
	resp, err := m.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       m.model,
		Temperature: 0.3, // Lower temperature for more deterministic matching
		MaxTokens:   2000,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})

	if err != nil {
		m.logger.Error("OpenAI API call failed", zap.Error(err))
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		m.logger.Error("No response from OpenAI")
		return &port.InvoiceMatchingResult{
			Matches:    []port.InvoiceItemMatch{},
			Confidence: 0.0,
			Reasoning:  "No response from AI model",
		}, nil
	}

	content := resp.Choices[0].Message.Content

	// Parse response
	var result struct {
		Matches []struct {
			AttachmentID int64   `json:"attachment_id"`
			ItemID       int64   `json:"item_id"`
			InvoiceID    int64   `json:"invoice_id"`
			Confidence   float64 `json:"confidence"`
			Reasoning    string  `json:"reasoning"`
		} `json:"matches"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		m.logger.Error("Failed to parse OpenAI response",
			zap.Error(err),
			zap.String("content", content))
		return &port.InvoiceMatchingResult{
			Matches:    []port.InvoiceItemMatch{},
			Confidence: 0.0,
			Reasoning:  fmt.Sprintf("Failed to parse AI response: %v", err),
		}, nil
	}

	// Convert to port types
	matches := make([]port.InvoiceItemMatch, 0, len(result.Matches))
	for _, m := range result.Matches {
		matches = append(matches, port.InvoiceItemMatch{
			AttachmentID: m.AttachmentID,
			ItemID:       m.ItemID,
			InvoiceID:    m.InvoiceID,
			Confidence:   m.Confidence,
			Reasoning:    m.Reasoning,
		})
	}

	m.logger.Info("Invoice matching completed",
		zap.Int("match_count", len(matches)),
		zap.Float64("confidence", result.Confidence))

	return &port.InvoiceMatchingResult{
		Matches:    matches,
		Confidence: result.Confidence,
		Reasoning:  result.Reasoning,
	}, nil
}

// buildItemsJSON converts items to JSON format for the prompt
func (m *Matcher) buildItemsJSON(items []*entity.ReimbursementItem) (string, error) {
	type itemData struct {
		ID           int64   `json:"id"`
		Description  string  `json:"description"`
		AmountCents  int64   `json:"amount_cents"`
		ItemType     string  `json:"item_type"`
		ExpenseDate  string  `json:"expense_date,omitempty"`
		Vendor       string  `json:"vendor,omitempty"`
	}

	itemList := make([]itemData, 0, len(items))
	for _, item := range items {
		data := itemData{
			ID:          item.ID,
			Description: item.Description,
			AmountCents: item.AmountCents,
			ItemType:    item.ItemType,
			Vendor:      item.Vendor,
		}
		if item.ExpenseDate != nil {
			data.ExpenseDate = item.ExpenseDate.Format("2006-01-02")
		}
		itemList = append(itemList, data)
	}

	jsonBytes, err := json.MarshalIndent(itemList, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// buildInvoicesJSON converts invoices to JSON format for the prompt
func (m *Matcher) buildInvoicesJSON(invoices []*entity.InvoiceV2) (string, error) {
	type invoiceData struct {
		ID           int64  `json:"id"`
		AttachmentID int64  `json:"attachment_id"`
		AmountCents  int64  `json:"amount_cents"`
		SellerName   string `json:"seller_name"`
		InvoiceDate  string `json:"invoice_date,omitempty"`
		InvoiceCode  string `json:"invoice_code"`
	}

	invoiceList := make([]invoiceData, 0, len(invoices))
	for _, inv := range invoices {
		data := invoiceData{
			ID:           inv.ID,
			AttachmentID: inv.AttachmentID,
			AmountCents:  inv.InvoiceAmountCents,
			SellerName:   inv.SellerName,
			InvoiceCode:  inv.InvoiceCode,
		}
		if inv.InvoiceDate != nil {
			data.InvoiceDate = inv.InvoiceDate.Format("2006-01-02")
		}
		invoiceList = append(invoiceList, data)
	}

	jsonBytes, err := json.MarshalIndent(invoiceList, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
