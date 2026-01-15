package models

import "time"

// Invoice represents a Chinese invoice (发票)
type Invoice struct {
	ID             int64     `json:"id"`
	InvoiceCode    string    `json:"invoice_code"`    // 发票代码
	InvoiceNumber  string    `json:"invoice_number"`  // 发票号码
	UniqueID       string    `json:"unique_id"`       // Combination: code + number
	InstanceID     int64     `json:"instance_id"`
	FileToken      string    `json:"file_token"`      // Lark file token
	FilePath       string    `json:"file_path"`       // Local file path
	InvoiceDate    *time.Time `json:"invoice_date"`   // 发票日期
	InvoiceAmount  float64   `json:"invoice_amount"`  // 发票金额
	SellerName     string    `json:"seller_name"`     // 销售方名称
	SellerTaxID    string    `json:"seller_tax_id"`   // 销售方税号
	BuyerName      string    `json:"buyer_name"`      // 购买方名称
	BuyerTaxID     string    `json:"buyer_tax_id"`    // 购买方税号
	ExtractedData  string    `json:"extracted_data"`  // Full JSON of extracted data
	CreatedAt      time.Time `json:"created_at"`
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
	InvoiceCode    string                 `json:"invoice_code"`    // 发票代码
	InvoiceNumber  string                 `json:"invoice_number"`  // 发票号码
	InvoiceType    string                 `json:"invoice_type"`    // 发票类型
	InvoiceDate    string                 `json:"invoice_date"`    // 开票日期
	TotalAmount    float64                `json:"total_amount"`    // 价税合计
	TaxAmount      float64                `json:"tax_amount"`      // 税额
	AmountWithoutTax float64              `json:"amount_without_tax"` // 金额
	SellerName     string                 `json:"seller_name"`     // 销售方名称
	SellerTaxID    string                 `json:"seller_tax_id"`   // 销售方税号
	SellerAddress  string                 `json:"seller_address"`  // 销售方地址电话
	SellerBank     string                 `json:"seller_bank"`     // 销售方开户行及账号
	BuyerName      string                 `json:"buyer_name"`      // 购买方名称
	BuyerTaxID     string                 `json:"buyer_tax_id"`    // 购买方税号
	BuyerAddress   string                 `json:"buyer_address"`   // 购买方地址电话
	BuyerBank      string                 `json:"buyer_bank"`      // 购买方开户行及账号
	Items          []InvoiceItem          `json:"items"`           // 明细项目
	Remarks        string                 `json:"remarks"`         // 备注
	CheckCode      string                 `json:"check_code"`      // 校验码
	RawData        map[string]interface{} `json:"raw_data"`        // Raw OCR data
}

// InvoiceItem represents a line item in an invoice
type InvoiceItem struct {
	Name        string  `json:"name"`        // 项目名称
	Specification string `json:"specification"` // 规格型号
	Unit        string  `json:"unit"`        // 单位
	Quantity    float64 `json:"quantity"`    // 数量
	UnitPrice   float64 `json:"unit_price"`  // 单价
	Amount      float64 `json:"amount"`      // 金额
	TaxRate     float64 `json:"tax_rate"`    // 税率
	TaxAmount   float64 `json:"tax_amount"`  // 税额
}

// GenerateUniqueID creates a unique identifier from invoice code and number
func GenerateUniqueID(code, number string) string {
	return code + "-" + number
}

// UniquenessCheckResult represents the result of checking invoice uniqueness
type UniquenessCheckResult struct {
	IsUnique          bool      `json:"is_unique"`
	DuplicateInvoiceID int64    `json:"duplicate_invoice_id,omitempty"`
	DuplicateInstanceID int64   `json:"duplicate_instance_id,omitempty"`
	FirstSeenAt       *time.Time `json:"first_seen_at,omitempty"`
	Message           string    `json:"message"`
}
