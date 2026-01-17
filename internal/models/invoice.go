package models

import "time"

// Invoice represents a Chinese invoice (发票)
type Invoice struct {
	ID            int64      `json:"id"`
	InvoiceCode   string     `json:"invoice_code"`   // 发票代码
	InvoiceNumber string     `json:"invoice_number"` // 发票号码
	UniqueID      string     `json:"unique_id"`      // Combination: code + number
	InstanceID    int64      `json:"instance_id"`
	FileToken     string     `json:"file_token"`     // Lark file token
	FilePath      string     `json:"file_path"`      // Local file path
	InvoiceDate   *time.Time `json:"invoice_date"`   // 发票日期
	InvoiceAmount float64    `json:"invoice_amount"` // 发票金额
	SellerName    string     `json:"seller_name"`    // 销售方名称
	SellerTaxID   string     `json:"seller_tax_id"`  // 销售方税号
	BuyerName     string     `json:"buyer_name"`     // 购买方名称
	BuyerTaxID    string     `json:"buyer_tax_id"`   // 购买方税号
	ExtractedData string     `json:"extracted_data"` // Full JSON of extracted data
	CreatedAt     time.Time  `json:"created_at"`
}

// InvoiceValidation represents validation results for an invoice
type InvoiceValidation struct {
	ID             int64     `json:"id"`
	InvoiceID      int64     `json:"invoice_id"`
	ValidationType string    `json:"validation_type"` // UNIQUENESS, FORMAT, AMOUNT, AI_CHECK
	IsValid        bool      `json:"is_valid"`
	ErrorMessage   string    `json:"error_message"`
	ValidationData string    `json:"validation_data"` // JSON blob
	ValidatedAt    time.Time `json:"validated_at"`
}

// Validation type constants
const (
	ValidationTypeUniqueness = "UNIQUENESS"
	ValidationTypeFormat     = "FORMAT"
	ValidationTypeAmount     = "AMOUNT"
	ValidationTypeAICheck    = "AI_CHECK"
)

// ExtractedInvoiceData represents all data extracted from an invoice PDF
type ExtractedInvoiceData struct {
	InvoiceCode      string                 `json:"invoice_code"`       // 发票代码
	InvoiceNumber    string                 `json:"invoice_number"`     // 发票号码
	InvoiceType      string                 `json:"invoice_type"`       // 发票类型
	InvoiceDate      string                 `json:"invoice_date"`       // 开票日期
	TotalAmount      float64                `json:"total_amount"`       // 价税合计
	TaxAmount        float64                `json:"tax_amount"`         // 税额
	AmountWithoutTax float64                `json:"amount_without_tax"` // 金额
	SellerName       string                 `json:"seller_name"`        // 销售方名称
	SellerTaxID      string                 `json:"seller_tax_id"`      // 销售方税号
	SellerAddress    string                 `json:"seller_address"`     // 销售方地址电话
	SellerBank       string                 `json:"seller_bank"`        // 销售方开户行及账号
	BuyerName        string                 `json:"buyer_name"`         // 购买方名称
	BuyerTaxID       string                 `json:"buyer_tax_id"`       // 购买方税号
	BuyerAddress     string                 `json:"buyer_address"`      // 购买方地址电话
	BuyerBank        string                 `json:"buyer_bank"`         // 购买方开户行及账号
	Items            []InvoiceItem          `json:"items"`              // 明细项目
	Remarks          string                 `json:"remarks"`            // 备注
	CheckCode        string                 `json:"check_code"`         // 校验码
	AccountingPolicy AccountingPolicyInfo   `json:"accounting_policy"`  // Accounting policy info
	PriceCompleteness PriceCompleteness     `json:"price_completeness"` // Price completeness verification
	RawData          map[string]interface{} `json:"raw_data"`           // Raw OCR data
}

// AccountingPolicyInfo represents accounting policy related information extracted from invoice
type AccountingPolicyInfo struct {
	DetectedCategory string `json:"detected_category"` // e.g. Travel, Meal, Office
	IsSpecialInvoice bool   `json:"is_special_invoice"` // VAT Special Invoice vs General
	HasCompanyTitle  bool   `json:"has_company_title"` // Does it match configured company name?
}

// PriceCompleteness represents verification of price information
type PriceCompleteness struct {
	IsTotalMatchSum bool     `json:"is_total_match_sum"` // Calculation verification
	ConfidenceScore float64  `json:"confidence_score"`   // 0.0 - 1.0
	MissingFields   []string `json:"missing_fields"`     // List of required fields not found
}

// InvoiceItem represents a line item in an invoice
type InvoiceItem struct {
	Name          string  `json:"name"`          // 项目名称
	Specification string  `json:"specification"` // 规格型号
	Unit          string  `json:"unit"`          // 单位
	Quantity      float64 `json:"quantity"`      // 数量
	UnitPrice     float64 `json:"unit_price"`    // 单价
	Amount        float64 `json:"amount"`        // 金额
	TaxRate       float64 `json:"tax_rate"`      // 税率
	TaxAmount     float64 `json:"tax_amount"`    // 税额
}

// GenerateUniqueID creates a unique identifier from invoice code and number
func GenerateUniqueID(code, number string) string {
	return code + "-" + number
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
// ARCH-011-C/D: Invoice auditing with policy check and completeness verification
type InvoiceAuditResult struct {
	ExtractedData     *ExtractedInvoiceData       `json:"extracted_data"`
	PolicyResult      *AccountingPolicyCheckResult `json:"policy_result"`
	PriceVerification *PriceVerificationResult     `json:"price_verification"`
	Completeness      *CompletenessResult          `json:"completeness"`
	OverallConfidence float64                      `json:"overall_confidence"` // 0.0-1.0
	OverallDecision   string                       `json:"overall_decision"`   // PASS, NEEDS_REVIEW, FAIL
	Reasoning         string                       `json:"reasoning"`
	ProcessedAt       time.Time                    `json:"processed_at"`
}

// AccountingPolicyCheckResult represents the result of accounting policy validation
type AccountingPolicyCheckResult struct {
	IsCompliant      bool     `json:"is_compliant"`
	CategoryMatch    bool     `json:"category_match"`    // Does expense category match invoice content?
	CompanyNameMatch bool     `json:"company_name_match"` // Does buyer name match company?
	VATTypeValid     bool     `json:"vat_type_valid"`     // Is VAT type appropriate for this expense?
	DateValid        bool     `json:"date_valid"`         // Is invoice date within acceptable range?
	Violations       []string `json:"violations"`
	Confidence       float64  `json:"confidence"`
	Reasoning        string   `json:"reasoning"`
}

// PriceVerificationResult represents price verification against invoice
type PriceVerificationResult struct {
	ClaimedAmount     float64 `json:"claimed_amount"`
	InvoiceAmount     float64 `json:"invoice_amount"`
	AmountMatch       bool    `json:"amount_match"`       // Does claimed amount match invoice?
	DeviationPercent  float64 `json:"deviation_percent"`  // Percentage difference
	IsReasonable      bool    `json:"is_reasonable"`      // Is the price reasonable for the item?
	MarketPriceMin    float64 `json:"market_price_min"`
	MarketPriceMax    float64 `json:"market_price_max"`
	Confidence        float64 `json:"confidence"`
	Reasoning         string  `json:"reasoning"`
}

// CompletenessResult represents the completeness check of invoice data
type CompletenessResult struct {
	Score           float64  `json:"score"`            // 0.0-1.0 completeness score
	RequiredFields  []string `json:"required_fields"`  // List of required fields
	PresentFields   []string `json:"present_fields"`   // Fields that are present
	MissingFields   []string `json:"missing_fields"`   // Fields that are missing
	TotalMatchesSum bool     `json:"total_matches_sum"` // Invoice total = sum of line items
	Reasoning       string   `json:"reasoning"`
}
