package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/garyjia/ai-reimbursement/internal/models"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// Language constants for response language configuration
const (
	LangChinese = "zh"
	LangEnglish = "en"
)

// InvoiceAuditor performs comprehensive audit of invoice data
// ARCH-011-C/D: Invoice auditing with policy check and completeness verification
type InvoiceAuditor struct {
	client             *openai.Client
	model              string
	companyName        string
	companyTaxID       string
	deviationThreshold float64
	logger             *zap.Logger
}

// NewInvoiceAuditor creates a new invoice auditor
func NewInvoiceAuditor(
	apiKey, model string,
	companyName, companyTaxID string,
	deviationThreshold float64,
	logger *zap.Logger,
) *InvoiceAuditor {
	client := openai.NewClient(apiKey)
	return &InvoiceAuditor{
		client:             client,
		model:              model,
		companyName:        companyName,
		companyTaxID:       companyTaxID,
		deviationThreshold: deviationThreshold,
		logger:             logger,
	}
}

// AuditInvoice performs a complete audit of extracted invoice data
func (a *InvoiceAuditor) AuditInvoice(
	ctx context.Context,
	invoice *models.ExtractedInvoiceData,
	claimedAmount float64,
	expenseCategory string,
) (*models.InvoiceAuditResult, error) {
	a.logger.Info("Starting invoice audit",
		zap.String("invoice_code", invoice.InvoiceCode),
		zap.String("invoice_number", invoice.InvoiceNumber),
		zap.Float64("claimed_amount", claimedAmount))

	// Run all checks in parallel
	policyResultCh := make(chan *models.AccountingPolicyCheckResult, 1)
	policyErrCh := make(chan error, 1)

	priceResultCh := make(chan *models.PriceVerificationResult, 1)
	priceErrCh := make(chan error, 1)

	// Policy check
	go func() {
		result, err := a.checkAccountingPolicy(ctx, invoice, expenseCategory)
		if err != nil {
			policyErrCh <- err
			return
		}
		policyResultCh <- result
	}()

	// Price verification
	go func() {
		result, err := a.verifyPrice(ctx, invoice, claimedAmount)
		if err != nil {
			priceErrCh <- err
			return
		}
		priceResultCh <- result
	}()

	// Wait for results
	var policyResult *models.AccountingPolicyCheckResult
	var priceResult *models.PriceVerificationResult

	select {
	case policyResult = <-policyResultCh:
	case err := <-policyErrCh:
		a.logger.Error("Policy check failed", zap.Error(err))
		policyResult = &models.AccountingPolicyCheckResult{
			IsCompliant: false,
			Violations:  []string{"Policy check failed: " + err.Error()},
			Confidence:  0.0,
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case priceResult = <-priceResultCh:
	case err := <-priceErrCh:
		a.logger.Error("Price verification failed", zap.Error(err))
		priceResult = &models.PriceVerificationResult{
			IsReasonable: false,
			Confidence:   0.0,
			Reasoning:    "Price verification failed: " + err.Error(),
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Completeness check (synchronous, fast)
	completeness := a.checkCompleteness(invoice)

	// Calculate overall result
	overallConfidence := (policyResult.Confidence + priceResult.Confidence + completeness.Score) / 3
	overallDecision := a.determineDecision(policyResult, priceResult, completeness, overallConfidence)

	// Build reasoning
	reasoning := a.buildOverallReasoning(policyResult, priceResult, completeness)

	result := &models.InvoiceAuditResult{
		ExtractedData:     invoice,
		PolicyResult:      policyResult,
		PriceVerification: priceResult,
		Completeness:      completeness,
		OverallConfidence: overallConfidence,
		OverallDecision:   overallDecision,
		Reasoning:         reasoning,
		ProcessedAt:       time.Now(),
	}

	a.logger.Info("Invoice audit completed",
		zap.String("decision", overallDecision),
		zap.Float64("confidence", overallConfidence))

	return result, nil
}

// checkAccountingPolicy validates invoice against accounting policies
func (a *InvoiceAuditor) checkAccountingPolicy(
	ctx context.Context,
	invoice *models.ExtractedInvoiceData,
	expenseCategory string,
) (*models.AccountingPolicyCheckResult, error) {
	a.logger.Debug("Checking accounting policy",
		zap.String("category", expenseCategory),
		zap.String("buyer_name", invoice.BuyerName))

	result := &models.AccountingPolicyCheckResult{
		IsCompliant: true,
		Violations:  []string{},
		Confidence:  1.0,
	}

	// Check 1: Company name match
	if invoice.BuyerName != "" {
		result.CompanyNameMatch = a.fuzzyMatch(invoice.BuyerName, a.companyName)
		if !result.CompanyNameMatch {
			result.Violations = append(result.Violations,
				fmt.Sprintf("Buyer name '%s' does not match company '%s'", invoice.BuyerName, a.companyName))
		}
	} else {
		result.CompanyNameMatch = false
		result.Violations = append(result.Violations, "Buyer name is missing from invoice")
	}

	// Check 2: Tax ID match (if available)
	if invoice.BuyerTaxID != "" && a.companyTaxID != "" {
		if invoice.BuyerTaxID != a.companyTaxID {
			result.Violations = append(result.Violations,
				fmt.Sprintf("Buyer tax ID '%s' does not match company tax ID", invoice.BuyerTaxID))
		}
	}

	// Check 3: Date validity (invoice should not be in the future, and not too old)
	if invoice.InvoiceDate != "" {
		invoiceDate, err := time.Parse("2006-01-02", invoice.InvoiceDate)
		if err == nil {
			now := time.Now()
			if invoiceDate.After(now) {
				result.Violations = append(result.Violations, "Invoice date is in the future")
				result.DateValid = false
			} else if invoiceDate.Before(now.AddDate(-1, 0, 0)) {
				result.Violations = append(result.Violations, "Invoice is more than 1 year old")
				result.DateValid = false
			} else {
				result.DateValid = true
			}
		}
	}

	// Check 4: VAT type validity
	result.VATTypeValid = true
	if invoice.InvoiceType != "" {
		specialCategories := []string{"EQUIPMENT", "PROCUREMENT", "ASSET"}
		isSpecialCategory := false
		for _, cat := range specialCategories {
			if strings.EqualFold(expenseCategory, cat) {
				isSpecialCategory = true
				break
			}
		}

		// Special invoices (专票) are typically required for equipment/asset purchases
		isSpecialInvoice := strings.Contains(invoice.InvoiceType, "专用")
		if isSpecialCategory && !isSpecialInvoice {
			result.Violations = append(result.Violations,
				fmt.Sprintf("Category '%s' typically requires VAT Special Invoice (增值税专用发票)", expenseCategory))
			result.VATTypeValid = false
		}
	}

	// Check 5: Category match using AI
	categoryResult, err := a.checkCategoryMatch(ctx, invoice, expenseCategory)
	if err != nil {
		a.logger.Warn("Category check via AI failed", zap.Error(err))
		result.CategoryMatch = true // Default to true if AI check fails
	} else {
		result.CategoryMatch = categoryResult.CategoryMatch
		if !categoryResult.CategoryMatch {
			result.Violations = append(result.Violations, categoryResult.Reasoning)
		}
		// Adjust confidence based on AI result
		result.Confidence = (result.Confidence + categoryResult.Confidence) / 2
	}

	// Determine overall compliance
	result.IsCompliant = len(result.Violations) == 0

	// Calculate confidence based on how many checks passed
	checks := []bool{result.CompanyNameMatch, result.DateValid, result.VATTypeValid, result.CategoryMatch}
	passedChecks := 0
	for _, check := range checks {
		if check {
			passedChecks++
		}
	}
	result.Confidence = result.Confidence * float64(passedChecks) / float64(len(checks))

	result.Reasoning = fmt.Sprintf("Policy check: %d/%d checks passed. %d violations found.",
		passedChecks, len(checks), len(result.Violations))

	return result, nil
}

// checkCategoryMatch uses AI to verify expense category matches invoice content
func (a *InvoiceAuditor) checkCategoryMatch(
	ctx context.Context,
	invoice *models.ExtractedInvoiceData,
	expenseCategory string,
) (*models.AccountingPolicyCheckResult, error) {
	// Build items description
	var itemsDesc strings.Builder
	for _, item := range invoice.Items {
		itemsDesc.WriteString(fmt.Sprintf("- %s (规格: %s, 金额: %.2f)\n", item.Name, item.Specification, item.Amount))
	}

	// Detect language from input data
	lang := detectLanguage(invoice.SellerName, invoice.InvoiceType, expenseCategory, itemsDesc.String())
	langInstruction := getLanguageInstruction(lang)

	prompt := fmt.Sprintf(`Analyze if this invoice content matches the claimed expense category.

Claimed Expense Category: %s

Invoice Details:
- Seller: %s
- Invoice Type: %s
- Items:
%s

Question: Does the invoice content reasonably match the claimed expense category "%s"?

%s

Respond with JSON:
{
  "category_match": boolean,
  "confidence": float 0.0-1.0,
  "reasoning": "explanation in the same language as the input"
}`,
		expenseCategory,
		invoice.SellerName,
		invoice.InvoiceType,
		itemsDesc.String(),
		expenseCategory,
		langInstruction)

	systemPrompt := "You are an expert accountant analyzing expense reimbursements. Determine if invoice content matches the claimed expense category. " + langInstruction

	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       a.model,
		Temperature: 0.1,
		MaxTokens:   500,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
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
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	var result models.AccountingPolicyCheckResult
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return &result, nil
}

// verifyPrice verifies the claimed price against invoice and market prices
func (a *InvoiceAuditor) verifyPrice(
	ctx context.Context,
	invoice *models.ExtractedInvoiceData,
	claimedAmount float64,
) (*models.PriceVerificationResult, error) {
	a.logger.Debug("Verifying price",
		zap.Float64("claimed", claimedAmount),
		zap.Float64("invoice_total", invoice.TotalAmount))

	result := &models.PriceVerificationResult{
		ClaimedAmount: claimedAmount,
		InvoiceAmount: invoice.TotalAmount,
		Confidence:    1.0,
	}

	// Check 1: Amount match between claim and invoice
	if invoice.TotalAmount > 0 {
		deviation := math.Abs(claimedAmount - invoice.TotalAmount)
		result.DeviationPercent = (deviation / invoice.TotalAmount) * 100

		// Allow small deviation (e.g., rounding differences)
		if result.DeviationPercent <= 1.0 {
			result.AmountMatch = true
		} else {
			result.AmountMatch = false
			result.Confidence = result.Confidence * 0.5
		}
	}

	// Check 2: Market price reasonableness via AI
	if len(invoice.Items) > 0 {
		marketResult, err := a.checkMarketPrice(ctx, invoice)
		if err != nil {
			a.logger.Warn("Market price check failed", zap.Error(err))
			// Default to reasonable if market check fails
			result.IsReasonable = true
		} else {
			result.IsReasonable = marketResult.IsReasonable
			result.MarketPriceMin = marketResult.MarketPriceMin
			result.MarketPriceMax = marketResult.MarketPriceMax
			result.Confidence = (result.Confidence + marketResult.Confidence) / 2
			result.Reasoning = marketResult.Reasoning
		}
	} else {
		// No line items to verify
		result.IsReasonable = true
		result.Reasoning = "No line items to verify against market prices"
	}

	// Final determination
	if !result.AmountMatch {
		result.Reasoning = fmt.Sprintf("Claimed amount (%.2f) differs from invoice total (%.2f) by %.1f%%. %s",
			claimedAmount, invoice.TotalAmount, result.DeviationPercent, result.Reasoning)
	}

	return result, nil
}

// checkMarketPrice uses AI to verify prices against market expectations
func (a *InvoiceAuditor) checkMarketPrice(
	ctx context.Context,
	invoice *models.ExtractedInvoiceData,
) (*models.PriceVerificationResult, error) {
	var itemsDesc strings.Builder
	for _, item := range invoice.Items {
		itemsDesc.WriteString(fmt.Sprintf("- %s: 数量=%v, 单价=%.2f, 金额=%.2f\n",
			item.Name, item.Quantity, item.UnitPrice, item.Amount))
	}

	// Detect language from input data
	lang := detectLanguage(invoice.SellerName, itemsDesc.String())
	langInstruction := getLanguageInstruction(lang)

	prompt := fmt.Sprintf(`Analyze if these prices are reasonable based on typical market prices in China:

Seller: %s
Items:
%s

Total Amount: %.2f

Question: Are these prices reasonable for business expenses in China?

%s

Respond with JSON:
{
  "is_reasonable": boolean,
  "market_price_min": float (estimated minimum market price for total),
  "market_price_max": float (estimated maximum market price for total),
  "confidence": float 0.0-1.0,
  "reasoning": "explanation in the same language as the input"
}`,
		invoice.SellerName,
		itemsDesc.String(),
		invoice.TotalAmount,
		langInstruction)

	systemPrompt := "You are a market price analyst for business expenses in China. Estimate if prices are reasonable. " + langInstruction

	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       a.model,
		Temperature: 0.2,
		MaxTokens:   500,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
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
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	var result models.PriceVerificationResult
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return &result, nil
}

// checkCompleteness verifies the completeness of invoice data
func (a *InvoiceAuditor) checkCompleteness(invoice *models.ExtractedInvoiceData) *models.CompletenessResult {
	requiredFields := []string{
		"invoice_code",
		"invoice_number",
		"invoice_date",
		"total_amount",
		"seller_name",
		"seller_tax_id",
		"buyer_name",
		"buyer_tax_id",
	}

	result := &models.CompletenessResult{
		RequiredFields: requiredFields,
		PresentFields:  []string{},
		MissingFields:  []string{},
	}

	// Check each required field
	fieldChecks := map[string]bool{
		"invoice_code":   invoice.InvoiceCode != "",
		"invoice_number": invoice.InvoiceNumber != "",
		"invoice_date":   invoice.InvoiceDate != "",
		"total_amount":   invoice.TotalAmount > 0,
		"seller_name":    invoice.SellerName != "",
		"seller_tax_id":  invoice.SellerTaxID != "",
		"buyer_name":     invoice.BuyerName != "",
		"buyer_tax_id":   invoice.BuyerTaxID != "",
	}

	for field, present := range fieldChecks {
		if present {
			result.PresentFields = append(result.PresentFields, field)
		} else {
			result.MissingFields = append(result.MissingFields, field)
		}
	}

	// Calculate score
	result.Score = float64(len(result.PresentFields)) / float64(len(requiredFields))

	// Verify total matches sum of items (if items present)
	if len(invoice.Items) > 0 {
		var itemSum float64
		for _, item := range invoice.Items {
			itemSum += item.Amount + item.TaxAmount
		}

		// Allow 1% tolerance for rounding
		tolerance := invoice.TotalAmount * 0.01
		result.TotalMatchesSum = math.Abs(invoice.TotalAmount-itemSum) <= tolerance
	} else {
		result.TotalMatchesSum = true // No items to verify
	}

	result.Reasoning = fmt.Sprintf("Completeness: %d/%d required fields present. Score: %.0f%%",
		len(result.PresentFields), len(requiredFields), result.Score*100)

	return result
}

// determineDecision determines the overall audit decision
func (a *InvoiceAuditor) determineDecision(
	policy *models.AccountingPolicyCheckResult,
	price *models.PriceVerificationResult,
	completeness *models.CompletenessResult,
	overallConfidence float64,
) string {
	// FAIL conditions
	if !policy.IsCompliant && len(policy.Violations) > 2 {
		return "FAIL"
	}
	if !price.AmountMatch && price.DeviationPercent > 10 {
		return "FAIL"
	}
	if completeness.Score < 0.5 {
		return "FAIL"
	}

	// NEEDS_REVIEW conditions
	if !policy.IsCompliant {
		return "NEEDS_REVIEW"
	}
	if !price.IsReasonable {
		return "NEEDS_REVIEW"
	}
	if overallConfidence < 0.7 {
		return "NEEDS_REVIEW"
	}
	if completeness.Score < 0.8 {
		return "NEEDS_REVIEW"
	}

	// PASS
	return "PASS"
}

// buildOverallReasoning builds a comprehensive reasoning summary
func (a *InvoiceAuditor) buildOverallReasoning(
	policy *models.AccountingPolicyCheckResult,
	price *models.PriceVerificationResult,
	completeness *models.CompletenessResult,
) string {
	var parts []string

	parts = append(parts, policy.Reasoning)
	parts = append(parts, price.Reasoning)
	parts = append(parts, completeness.Reasoning)

	if len(policy.Violations) > 0 {
		parts = append(parts, fmt.Sprintf("Violations: %s", strings.Join(policy.Violations, "; ")))
	}

	return strings.Join(parts, " | ")
}

// fuzzyMatch performs fuzzy string matching for company names
func (a *InvoiceAuditor) fuzzyMatch(s1, s2 string) bool {
	// Normalize strings
	s1 = strings.TrimSpace(strings.ToLower(s1))
	s2 = strings.TrimSpace(strings.ToLower(s2))

	// Exact match
	if s1 == s2 {
		return true
	}

	// Contains match
	if strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		return true
	}

	// Remove common suffixes and compare
	suffixes := []string{"有限公司", "有限责任公司", "股份有限公司", "集团", "公司", "ltd", "inc", "co."}
	clean1 := s1
	clean2 := s2
	for _, suffix := range suffixes {
		clean1 = strings.TrimSuffix(clean1, suffix)
		clean2 = strings.TrimSuffix(clean2, suffix)
	}
	clean1 = strings.TrimSpace(clean1)
	clean2 = strings.TrimSpace(clean2)

	return clean1 == clean2 || strings.Contains(clean1, clean2) || strings.Contains(clean2, clean1)
}

// detectLanguage detects whether the input text is primarily Chinese or English
// Returns LangChinese if Chinese characters are detected, LangEnglish otherwise
func detectLanguage(texts ...string) string {
	var chineseCount, totalCount int

	for _, text := range texts {
		for _, r := range text {
			if unicode.IsLetter(r) {
				totalCount++
				// Check if the character is in CJK Unified Ideographs range
				if unicode.Is(unicode.Han, r) {
					chineseCount++
				}
			}
		}
	}

	// If more than 20% of letters are Chinese, consider it Chinese
	if totalCount > 0 && float64(chineseCount)/float64(totalCount) > 0.2 {
		return LangChinese
	}
	return LangEnglish
}

// getLanguageInstruction returns a language instruction for the GPT prompt
func getLanguageInstruction(lang string) string {
	if lang == LangChinese {
		return "请用中文回复。(Please respond in Chinese.)"
	}
	return "Please respond in English."
}
