package invoice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/gen2brain/go-fitz"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// PDFReader reads PDF files and extracts invoice data using GPT-4 Vision
// ARCH-011-A: PDF Vision Reader implementation
type PDFReader struct {
	client *openai.Client
	model  string
	logger *zap.Logger
}

// NewPDFReader creates a new PDF reader with Vision API support
func NewPDFReader(apiKey, model string, logger *zap.Logger) *PDFReader {
	client := openai.NewClient(apiKey)
	return &PDFReader{
		client: client,
		model:  model,
		logger: logger,
	}
}

// ReadAndExtract reads a PDF file and extracts invoice data using Vision API
func (r *PDFReader) ReadAndExtract(ctx context.Context, pdfPath string) (*models.ExtractedInvoiceData, error) {
	r.logger.Info("Reading PDF for invoice extraction", zap.String("path", pdfPath))

	// Step 1: Convert PDF to images
	images, err := r.convertPDFToImages(pdfPath)
	if err != nil {
		r.logger.Error("Failed to convert PDF to images", zap.Error(err))
		return nil, fmt.Errorf("failed to convert PDF: %w", err)
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images extracted from PDF")
	}

	r.logger.Info("Converted PDF to images", zap.Int("page_count", len(images)))

	// Step 2: Extract invoice data using Vision API
	// Limit to first 2 pages to control costs
	maxPages := 2
	if len(images) < maxPages {
		maxPages = len(images)
	}

	return r.extractWithVision(ctx, images[:maxPages])
}

// convertPDFToImages converts PDF pages to JPEG images using mupdf
func (r *PDFReader) convertPDFToImages(pdfPath string) ([][]byte, error) {
	// Check if file exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("PDF file not found: %s", pdfPath)
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(pdfPath))
	if ext != ".pdf" {
		// If it's an image file, read it directly
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			return r.readImageFile(pdfPath)
		}
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}

	// Open PDF document
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	var images [][]byte
	pageCount := doc.NumPage()

	r.logger.Debug("Processing PDF", zap.Int("total_pages", pageCount))

	for pageNum := 0; pageNum < pageCount; pageNum++ {
		// Extract page as image (300 DPI for good quality)
		img, err := doc.Image(pageNum)
		if err != nil {
			r.logger.Warn("Failed to extract page as image",
				zap.Int("page", pageNum),
				zap.Error(err))
			continue
		}

		// Encode to JPEG
		imgBytes, err := r.encodeImageToJPEG(img)
		if err != nil {
			r.logger.Warn("Failed to encode page to JPEG",
				zap.Int("page", pageNum),
				zap.Error(err))
			continue
		}

		images = append(images, imgBytes)
	}

	return images, nil
}

// readImageFile reads an image file directly
func (r *PDFReader) readImageFile(imagePath string) ([][]byte, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(imagePath))
	var img image.Image

	switch ext {
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)
	case ".png":
		img, err = png.Decode(file)
	default:
		return nil, fmt.Errorf("unsupported image format: %s", ext)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	imgBytes, err := r.encodeImageToJPEG(img)
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	return [][]byte{imgBytes}, nil
}

// encodeImageToJPEG encodes an image to JPEG bytes
func (r *PDFReader) encodeImageToJPEG(img image.Image) ([]byte, error) {
	var buf strings.Builder
	// Use a temporary file approach since strings.Builder doesn't implement io.Writer properly
	tmpFile, err := os.CreateTemp("", "invoice_page_*.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Encode to JPEG with good quality
	if err := jpeg.Encode(tmpFile, img, &jpeg.Options{Quality: 85}); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}
	tmpFile.Close()

	// Read the file back
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file: %w", err)
	}

	_ = buf // Unused, but kept for potential future use
	return data, nil
}

// extractWithVision uses GPT-4 Vision to extract invoice data from images
func (r *PDFReader) extractWithVision(ctx context.Context, images [][]byte) (*models.ExtractedInvoiceData, error) {
	r.logger.Info("Extracting invoice data with Vision API", zap.Int("image_count", len(images)))

	// Build multi-modal message with images
	var contentParts []openai.ChatMessagePart

	// Add instruction text
	contentParts = append(contentParts, openai.ChatMessagePart{
		Type: openai.ChatMessagePartTypeText,
		Text: r.buildVisionPrompt(),
	})

	// Add images as base64
	for i, imgData := range images {
		base64Img := base64.StdEncoding.EncodeToString(imgData)
		contentParts = append(contentParts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    fmt.Sprintf("data:image/jpeg;base64,%s", base64Img),
				Detail: openai.ImageURLDetailHigh,
			},
		})
		r.logger.Debug("Added image to request", zap.Int("page", i+1), zap.Int("size_bytes", len(imgData)))
	}

	// Create chat completion request
	resp, err := r.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       r.model,
		MaxTokens:   4096,
		Temperature: 0.1,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert in reading and extracting data from Chinese invoices (发票). You have perfect accuracy in reading invoice codes, numbers, amounts, and all other fields. Always respond with valid JSON.",
			},
			{
				Role:         openai.ChatMessageRoleUser,
				MultiContent: contentParts,
			},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})

	if err != nil {
		r.logger.Error("Vision API call failed", zap.Error(err))
		return nil, fmt.Errorf("Vision API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from Vision API")
	}

	content := resp.Choices[0].Message.Content
	r.logger.Debug("Vision API response received", zap.Int("content_length", len(content)))

	// Parse the JSON response
	var extractedData models.ExtractedInvoiceData
	if err := json.Unmarshal([]byte(content), &extractedData); err != nil {
		r.logger.Error("Failed to parse Vision API response",
			zap.Error(err),
			zap.String("content", content))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Validate required fields
	if extractedData.InvoiceCode == "" && extractedData.InvoiceNumber == "" {
		r.logger.Warn("Could not extract invoice code or number",
			zap.String("raw_response", content))
	}

	r.logger.Info("Invoice data extracted successfully",
		zap.String("invoice_code", extractedData.InvoiceCode),
		zap.String("invoice_number", extractedData.InvoiceNumber),
		zap.Float64("total_amount", extractedData.TotalAmount))

	return &extractedData, nil
}

// buildVisionPrompt builds the prompt for Vision API extraction
func (r *PDFReader) buildVisionPrompt() string {
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
