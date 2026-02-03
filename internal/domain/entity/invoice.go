package entity

import "time"

// Invoice represents a Chinese invoice (发票)
type Invoice struct {
	ID            int64      `json:"id"`
	InvoiceCode   string     `json:"invoice_code"`
	InvoiceNumber string     `json:"invoice_number"`
	UniqueID      string     `json:"unique_id"`
	InstanceID    int64      `json:"instance_id"`
	FileToken     string     `json:"file_token"`
	FilePath      string     `json:"file_path"`
	InvoiceDate   *time.Time `json:"invoice_date"`
	InvoiceAmount float64    `json:"invoice_amount"`
	SellerName    string     `json:"seller_name"`
	SellerTaxID   string     `json:"seller_tax_id"`
	BuyerName     string     `json:"buyer_name"`
	BuyerTaxID    string     `json:"buyer_tax_id"`
	ExtractedData string     `json:"extracted_data"`
	CreatedAt     time.Time  `json:"created_at"`
}

// InvoiceValidation represents validation results for an invoice
type InvoiceValidation struct {
	ID             int64     `json:"id"`
	InvoiceID      int64     `json:"invoice_id"`
	ValidationType string    `json:"validation_type"`
	IsValid        bool      `json:"is_valid"`
	ErrorMessage   string    `json:"error_message"`
	ValidationData string    `json:"validation_data"`
	ValidatedAt    time.Time `json:"validated_at"`
}

// ExtractedInvoiceData represents all data extracted from an invoice PDF
type ExtractedInvoiceData struct {
	InvoiceCode      string                 `json:"invoice_code"`
	InvoiceNumber    string                 `json:"invoice_number"`
	InvoiceType      string                 `json:"invoice_type"`
	InvoiceDate      string                 `json:"invoice_date"`
	TotalAmount      float64                `json:"total_amount"`
	TaxAmount        float64                `json:"tax_amount"`
	AmountWithoutTax float64                `json:"amount_without_tax"`
	SellerName       string                 `json:"seller_name"`
	SellerTaxID      string                 `json:"seller_tax_id"`
	SellerAddress    string                 `json:"seller_address"`
	SellerBank       string                 `json:"seller_bank"`
	BuyerName        string                 `json:"buyer_name"`
	BuyerTaxID       string                 `json:"buyer_tax_id"`
	BuyerAddress     string                 `json:"buyer_address"`
	BuyerBank        string                 `json:"buyer_bank"`
	Items            []InvoiceItem          `json:"items"`
	Remarks          string                 `json:"remarks"`
	CheckCode        string                 `json:"check_code"`
	AccountingPolicy AccountingPolicyInfo   `json:"accounting_policy"`
	PriceCompleteness PriceCompleteness     `json:"price_completeness"`
	RawData          map[string]interface{} `json:"raw_data"`
}

// AccountingPolicyInfo represents accounting policy related information extracted from invoice
type AccountingPolicyInfo struct {
	DetectedCategory string `json:"detected_category"`
	IsSpecialInvoice bool   `json:"is_special_invoice"`
	HasCompanyTitle  bool   `json:"has_company_title"`
}

// PriceCompleteness represents verification of price information
type PriceCompleteness struct {
	IsTotalMatchSum bool     `json:"is_total_match_sum"`
	ConfidenceScore float64  `json:"confidence_score"`
	MissingFields   []string `json:"missing_fields"`
}

// InvoiceItem represents a line item in an invoice
type InvoiceItem struct {
	Name          string  `json:"name"`
	Specification string  `json:"specification"`
	Unit          string  `json:"unit"`
	Quantity      float64 `json:"quantity"`
	UnitPrice     float64 `json:"unit_price"`
	Amount        float64 `json:"amount"`
	TaxRate       float64 `json:"tax_rate"`
	TaxAmount     float64 `json:"tax_amount"`
}

// UniquenessCheckResult represents the result of checking invoice uniqueness
type UniquenessCheckResult struct {
	IsUnique            bool       `json:"is_unique"`
	DuplicateInvoiceID  int64      `json:"duplicate_invoice_id,omitempty"`
	DuplicateInstanceID int64      `json:"duplicate_instance_id,omitempty"`
	FirstSeenAt         *time.Time `json:"first_seen_at,omitempty"`
	Message             string     `json:"message"`
}

// InvoiceAuditResult represents the complete audit result for an invoice
type InvoiceAuditResult struct {
	ExtractedData     *ExtractedInvoiceData       `json:"extracted_data"`
	PolicyResult      *AccountingPolicyCheckResult `json:"policy_result"`
	PriceVerification *PriceVerificationResult     `json:"price_verification"`
	Completeness      *CompletenessResult          `json:"completeness"`
	OverallConfidence float64                      `json:"overall_confidence"`
	OverallDecision   string                       `json:"overall_decision"`
	Reasoning         string                       `json:"reasoning"`
	ProcessedAt       time.Time                    `json:"processed_at"`
}

// AccountingPolicyCheckResult represents the result of accounting policy validation
type AccountingPolicyCheckResult struct {
	IsCompliant      bool     `json:"is_compliant"`
	CategoryMatch    bool     `json:"category_match"`
	CompanyNameMatch bool     `json:"company_name_match"`
	VATTypeValid     bool     `json:"vat_type_valid"`
	DateValid        bool     `json:"date_valid"`
	Violations       []string `json:"violations"`
	Confidence       float64  `json:"confidence"`
	Reasoning        string   `json:"reasoning"`
}

// PriceVerificationResult represents price verification against invoice
type PriceVerificationResult struct {
	ClaimedAmount    float64 `json:"claimed_amount"`
	InvoiceAmount    float64 `json:"invoice_amount"`
	AmountMatch      bool    `json:"amount_match"`
	DeviationPercent float64 `json:"deviation_percent"`
	IsReasonable     bool    `json:"is_reasonable"`
	MarketPriceMin   float64 `json:"market_price_min"`
	MarketPriceMax   float64 `json:"market_price_max"`
	Confidence       float64 `json:"confidence"`
	Reasoning        string  `json:"reasoning"`
}

// CompletenessResult represents the completeness check of invoice data
type CompletenessResult struct {
	Score           float64  `json:"score"`
	RequiredFields  []string `json:"required_fields"`
	PresentFields   []string `json:"present_fields"`
	MissingFields   []string `json:"missing_fields"`
	TotalMatchesSum bool     `json:"total_matches_sum"`
	Reasoning       string   `json:"reasoning"`
}

// InvoiceV2 represents a Chinese invoice linked to an attachment.
// This is the new structure replacing Invoice for the refactored schema.
// Each InvoiceV2 has a 1:1 relationship with an Attachment.
type InvoiceV2 struct {
	ID           int64 `json:"id"`
	InstanceID   int64 `json:"instance_id"`   // Primary link to approval instance
	AttachmentID int64 `json:"attachment_id"` // 1:1 with attachment
	ItemID       int64 `json:"item_id"`       // Derived from attachment

	// Invoice identification (from GPT-4 extraction)
	InvoiceCode   string `json:"invoice_code"`
	InvoiceNumber string `json:"invoice_number"`
	UniqueID      string `json:"unique_id"`

	// Extracted data (GPT-4 Vision result)
	InvoiceDate         *time.Time `json:"invoice_date,omitempty"`
	InvoiceAmountCents  int64      `json:"invoice_amount_cents"`  // Amount in cents (分)
	InvoiceAmount       float64    `json:"invoice_amount"`        // Deprecated: use InvoiceAmountCents
	SellerName          string     `json:"seller_name"`
	SellerTaxID         string     `json:"seller_tax_id"`
	BuyerName           string     `json:"buyer_name"`
	BuyerTaxID          string     `json:"buyer_tax_id"`
	ExtractedData       string     `json:"extracted_data"`

	CreatedAt time.Time `json:"created_at"`
}

// AmountYuan returns the invoice amount in yuan (元) for display purposes.
func (i *InvoiceV2) AmountYuan() float64 {
	return float64(i.InvoiceAmountCents) / 100.0
}
