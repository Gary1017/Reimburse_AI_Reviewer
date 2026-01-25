package invoice

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// Extractor extracts invoice data from PDF files using AI
type Extractor struct {
	client *openai.Client
	model  string
	logger *zap.Logger
}

// NewExtractor creates a new invoice extractor
func NewExtractor(apiKey, model string, logger *zap.Logger) *Extractor {
	client := openai.NewClient(apiKey)
	return &Extractor{
		client: client,
		model:  model,
		logger: logger,
	}
}

// ExtractFromPDF extracts invoice data from a PDF file
func (e *Extractor) ExtractFromPDF(ctx context.Context, pdfPath string) (*entity.ExtractedInvoiceData, error) {
	e.logger.Info("Extracting invoice data from PDF", zap.String("pdf_path", pdfPath))

	// Read PDF file
	// Note: For production, you should use a PDF parsing library like pdfcpu or use OCR
	// Here we'll use GPT-4 Vision API with the PDF converted to images
	// For now, we'll implement a placeholder that uses OCR text

	// TODO: Implement actual PDF reading and OCR
	// This is a simplified version using AI to extract from text

	prompt := e.buildExtractionPrompt()

	// Call OpenAI API
	resp, err := e.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       "gpt-4",
		Temperature: 0.1,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert in reading Chinese invoices (发票). Extract structured data from invoice images or text.",
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
		e.logger.Error("Failed to call OpenAI API", zap.Error(err))
		return nil, fmt.Errorf("failed to extract invoice data: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	content := resp.Choices[0].Message.Content

	var extractedData entity.ExtractedInvoiceData
	if err := json.Unmarshal([]byte(content), &extractedData); err != nil {
		e.logger.Error("Failed to parse extraction result",
			zap.Error(err),
			zap.String("content", content))
		return nil, fmt.Errorf("failed to parse extraction result: %w", err)
	}

	// Validate required fields
	if extractedData.InvoiceCode == "" || extractedData.InvoiceNumber == "" {
		return nil, fmt.Errorf("failed to extract invoice code or number")
	}

	e.logger.Info("Invoice data extracted successfully",
		zap.String("invoice_code", extractedData.InvoiceCode),
		zap.String("invoice_number", extractedData.InvoiceNumber),
		zap.Float64("total_amount", extractedData.TotalAmount))

	return &extractedData, nil
}

// ExtractFromText extracts invoice data from OCR text
func (e *Extractor) ExtractFromText(ctx context.Context, ocrText string) (*entity.ExtractedInvoiceData, error) {
	// Use regex patterns to extract key invoice fields
	data := &entity.ExtractedInvoiceData{
		RawData: make(map[string]interface{}),
	}

	// Extract invoice code (发票代码) - typically 10 or 12 digits
	codePattern := regexp.MustCompile(`发票代码[：:]\s*(\d{10,12})`)
	if matches := codePattern.FindStringSubmatch(ocrText); len(matches) > 1 {
		data.InvoiceCode = matches[1]
	}

	// Extract invoice number (发票号码) - typically 8 digits
	numberPattern := regexp.MustCompile(`发票号码[：:]\s*(\d{8})`)
	if matches := numberPattern.FindStringSubmatch(ocrText); len(matches) > 1 {
		data.InvoiceNumber = matches[1]
	}

	// Extract total amount (价税合计)
	amountPattern := regexp.MustCompile(`价税合计[（(].*?[)）][：:¥]?\s*[¥￥]?\s*([\d,]+\.?\d*)`)
	if matches := amountPattern.FindStringSubmatch(ocrText); len(matches) > 1 {
		amountStr := strings.ReplaceAll(matches[1], ",", "")
		if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
			data.TotalAmount = amount
		}
	}

	// Extract seller tax ID (销售方税号)
	sellerTaxPattern := regexp.MustCompile(`销售方.*?税号[：:]\s*([A-Z0-9]{15,20})`)
	if matches := sellerTaxPattern.FindStringSubmatch(ocrText); len(matches) > 1 {
		data.SellerTaxID = matches[1]
	}

	// Extract buyer tax ID (购买方税号)
	buyerTaxPattern := regexp.MustCompile(`购买方.*?税号[：:]\s*([A-Z0-9]{15,20})`)
	if matches := buyerTaxPattern.FindStringSubmatch(ocrText); len(matches) > 1 {
		data.BuyerTaxID = matches[1]
	}

	// If we couldn't extract code or number with regex, use AI
	if data.InvoiceCode == "" || data.InvoiceNumber == "" {
		e.logger.Warn("Failed to extract invoice fields with regex, falling back to AI")
		return e.extractWithAI(ctx, ocrText)
	}

	return data, nil
}

// extractWithAI uses AI to extract invoice data when regex fails
func (e *Extractor) extractWithAI(ctx context.Context, text string) (*entity.ExtractedInvoiceData, error) {
	prompt := fmt.Sprintf(`Extract invoice information from this Chinese invoice text:

%s

Return JSON with the following structure:
{
  "invoice_code": "10-digit or 12-digit code",
  "invoice_number": "8-digit number",
  "invoice_type": "增值税专用发票 or 增值税普通发票",
  "invoice_date": "YYYY-MM-DD",
  "total_amount": float,
  "tax_amount": float,
  "amount_without_tax": float,
  "seller_name": "seller company name",
  "seller_tax_id": "seller tax ID",
  "buyer_name": "buyer company name",
  "buyer_tax_id": "buyer tax ID",
  "check_code": "verification code if available"
}`, text)

	resp, err := e.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       e.model,
		Temperature: 0.1,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert in reading Chinese invoices. Extract all fields accurately.",
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
		return nil, fmt.Errorf("AI extraction failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no AI response")
	}

	var data entity.ExtractedInvoiceData
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &data); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return &data, nil
}

// buildExtractionPrompt builds the prompt for invoice extraction
func (e *Extractor) buildExtractionPrompt() string {
	return `Extract all information from this Chinese invoice (发票). 

Required fields:
- 发票代码 (Invoice Code): Usually 10 or 12 digits
- 发票号码 (Invoice Number): Usually 8 digits  
- 开票日期 (Invoice Date)
- 价税合计 (Total Amount including tax)
- 税额 (Tax Amount)
- 金额 (Amount without tax)
- 销售方名称 (Seller Name)
- 销售方税号 (Seller Tax ID)
- 购买方名称 (Buyer Name)
- 购买方税号 (Buyer Tax ID)

Optional fields:
- 发票类型 (Invoice Type)
- 明细项目 (Line items)
- 备注 (Remarks)
- 校验码 (Check Code)

Return as structured JSON matching the ExtractedInvoiceData format.`
}
