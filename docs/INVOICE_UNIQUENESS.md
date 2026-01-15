# Invoice Uniqueness Checking Feature

## Overview

The system automatically verifies invoice uniqueness to prevent duplicate reimbursement submissions. This is a critical fraud prevention and compliance feature for Chinese enterprises.

## 🎯 Purpose

**Problem**: Employees might accidentally (or intentionally) submit the same invoice multiple times for reimbursement.

**Solution**: Extract and track unique invoice identifiers (发票代码 + 发票号码) from all submitted invoices, rejecting duplicates automatically.

## 🔍 How It Works

### 1. Invoice Identification

Every Chinese invoice (发票) has two unique identifiers:

- **发票代码 (Invoice Code)**: 10-12 digit code
- **发票号码 (Invoice Number)**: 8-digit number

Together, these create a globally unique identifier for each invoice.

### 2. Processing Flow

```
┌─────────────────────────────────────────────────────────┐
│ User submits reimbursement with invoice PDF             │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ System downloads all attachments from Lark             │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ AI (OpenAI GPT-4) extracts invoice data from PDF       │
│  - Invoice Code (发票代码)                               │
│  - Invoice Number (发票号码)                            │
│  - Amount, date, seller/buyer info                     │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ Generate Unique ID: code + "-" + number                │
│ Example: "1200192130-00185025"                         │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│ Check database for existing invoice with same ID       │
└────────────────┬───────────────────┬────────────────────┘
                 │                   │
        ✅ UNIQUE│                   │❌ DUPLICATE
                 │                   │
                 ▼                   ▼
    ┌────────────────────┐  ┌──────────────────────────┐
    │ Store invoice      │  │ Reject reimbursement     │
    │ Continue workflow  │  │ Notify submitter         │
    │ AI audit → Approve │  │ Log security event       │
    └────────────────────┘  └──────────────────────────┘
```

### 3. Database Schema

**invoices table**:
```sql
CREATE TABLE invoices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_code TEXT NOT NULL,          -- 发票代码
    invoice_number TEXT NOT NULL,        -- 发票号码
    unique_id TEXT UNIQUE NOT NULL,      -- Combined identifier
    instance_id INTEGER NOT NULL,        -- Linked approval
    file_path TEXT,                      -- PDF location
    invoice_date DATE,
    invoice_amount DECIMAL(10,2),
    seller_name TEXT,
    seller_tax_id TEXT,
    buyer_name TEXT,
    buyer_tax_id TEXT,
    extracted_data TEXT,                 -- Full JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id)
);

CREATE INDEX idx_invoices_unique_id ON invoices(unique_id);
```

**invoice_validations table**:
```sql
CREATE TABLE invoice_validations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id INTEGER NOT NULL,
    validation_type TEXT NOT NULL,       -- UNIQUENESS, FORMAT, AMOUNT, AI_CHECK
    is_valid BOOLEAN NOT NULL,
    error_message TEXT,
    validation_data TEXT,
    validated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (invoice_id) REFERENCES invoices(id)
);
```

## 💡 Implementation Details

### AI-Powered Invoice Extraction

**File**: `internal/invoice/extractor.go`

The system uses OpenAI GPT-4 to extract invoice fields from PDF files:

```go
// Extract invoice data from PDF
extractedData, err := invoiceExtractor.ExtractFromPDF(ctx, pdfPath)
// Returns: ExtractedInvoiceData{
//   InvoiceCode:    "1200192130",
//   InvoiceNumber:  "00185025",
//   InvoiceDate:    "2024-12-15",
//   TotalAmount:    1580.00,
//   SellerName:     "北京科技有限公司",
//   SellerTaxID:    "91110108MA01XXXXX",
//   ...
// }
```

**Why AI instead of OCR?**
- Better accuracy for various invoice formats
- Handles handwritten amounts and stamps
- Understands context (e.g., corrected totals)
- Can extract semantic information
- Works with partially damaged/blurry invoices

### Uniqueness Check

**File**: `internal/repository/invoice_repo.go`

```go
func (r *InvoiceRepository) CheckUniqueness(uniqueID string) (*UniquenessCheckResult, error) {
    query := `
        SELECT id, instance_id, created_at
        FROM invoices
        WHERE unique_id = ?
        LIMIT 1
    `
    // If found: return duplicate info
    // If not found: return IsUnique=true
}
```

### Workflow Integration

**File**: `internal/workflow/invoice_checker.go`

The invoice checker is called early in the approval workflow:

```go
// When approval instance is created
func (e *Engine) ProcessApprovalCreated(ctx context.Context, event *LarkEvent) error {
    // 1. Create instance record
    // 2. Download attachments
    // 3. Check invoices (NEW!)
    if err := invoiceChecker.CheckInstanceInvoices(ctx, instance.ID, attachmentPaths); err != nil {
        // Failed uniqueness check
        // Reject approval automatically
        // Notify submitter with details
        return err
    }
    // 4. Continue with AI audit
    // 5. ...
}
```

## 📊 Extracted Invoice Data

### Complete Invoice Information

```json
{
  "invoice_code": "1200192130",
  "invoice_number": "00185025",
  "invoice_type": "增值税普通发票",
  "invoice_date": "2024-12-15",
  "total_amount": 1580.00,
  "tax_amount": 80.00,
  "amount_without_tax": 1500.00,
  "seller_name": "北京科技有限公司",
  "seller_tax_id": "91110108MA01XXXXX",
  "seller_address": "北京市朝阳区XX路XX号",
  "seller_bank": "中国工商银行北京分行 1234567890",
  "buyer_name": "Your Company Ltd.",
  "buyer_tax_id": "91110000123456789X",
  "buyer_address": "上海市浦东新区XX路XX号",
  "buyer_bank": "中国建设银行上海分行 9876543210",
  "items": [
    {
      "name": "技术服务费",
      "specification": "",
      "unit": "次",
      "quantity": 1,
      "unit_price": 1500.00,
      "amount": 1500.00,
      "tax_rate": 0.06,
      "tax_amount": 90.00
    }
  ],
  "remarks": "项目编号: PROJ-2024-001",
  "check_code": "12345678901234567890"
}
```

### Validation Tracking

Every validation attempt is recorded:

```sql
INSERT INTO invoice_validations (
    invoice_id,
    validation_type,
    is_valid,
    error_message,
    validation_data
) VALUES (
    123,
    'UNIQUENESS',
    false,
    'Duplicate invoice found (first seen at instance 45 on 2024-11-20)',
    '{"duplicate_instance_id": 45, "first_seen_at": "2024-11-20T10:30:00Z"}'
);
```

## 🚨 Duplicate Detection Response

### User Notification

When a duplicate is detected:

1. **Immediate Rejection**: Approval is automatically rejected
2. **Detailed Message**: User receives notification with:
   - Duplicate invoice unique ID
   - Original submission date
   - Original approval instance ID
   - Link to original approval

**Example Message**:
```
❌ Reimbursement Rejected: Duplicate Invoice

Invoice: 1200192130-00185025
This invoice was previously submitted on 2024-11-20 in approval #A-2024-000045.

Please review your submission and ensure you haven't already been reimbursed for this expense.

If you believe this is an error, please contact the finance team.
```

3. **Security Logging**: Event logged for audit
4. **Alert to Finance**: Repeated duplicates trigger review

### Admin Dashboard Metrics

Track duplicate attempts:

```sql
-- Count duplicate attempts per user
SELECT 
    ai.applicant_user_id,
    COUNT(*) as duplicate_attempts
FROM invoice_validations iv
JOIN invoices i ON iv.invoice_id = i.id
JOIN approval_instances ai ON i.instance_id = ai.id
WHERE 
    iv.validation_type = 'UNIQUENESS' 
    AND iv.is_valid = false
GROUP BY ai.applicant_user_id
ORDER BY duplicate_attempts DESC;
```

## 🔧 Configuration

### AI Model Selection

In `configs/config.yaml`:

```yaml
openai:
  model: gpt-4  # or gpt-4-vision-preview for better image handling
  temperature: 0.1  # Low for factual extraction
  max_tokens: 2000
  timeout: 60s
```

### Extraction Confidence Thresholds

In `internal/invoice/extractor.go`:

```go
// Require high confidence for invoice fields
if extractedData.InvoiceCode == "" || extractedData.InvoiceNumber == "" {
    // Fall back to manual review
    return nil, fmt.Errorf("failed to extract invoice identifiers")
}
```

## 🧪 Testing

### Unit Tests

```bash
go test ./internal/invoice/... -v
go test ./internal/repository/... -run TestInvoiceUniqueness -v
```

### Integration Test

```go
func TestInvoiceUniquenessEndToEnd(t *testing.T) {
    // 1. Submit first approval with invoice
    // 2. Verify invoice stored
    // 3. Submit second approval with SAME invoice
    // 4. Verify rejection
    // 5. Check validation record
}
```

### Manual Testing

```bash
# 1. Submit reimbursement with invoice
# 2. Wait for processing
# 3. Check database
sqlite3 data/reimbursement.db "SELECT * FROM invoices ORDER BY created_at DESC LIMIT 1;"

# 4. Submit again with same invoice
# 5. Verify rejection
sqlite3 data/reimbursement.db "SELECT * FROM invoice_validations WHERE validation_type='UNIQUENESS' AND is_valid=0;"
```

## 📈 Performance Considerations

### Optimization Strategies

1. **Database Indexes**:
   - Unique index on `invoices.unique_id` for O(1) lookup
   - Index on `invoices.instance_id` for related queries

2. **Caching**:
   - Cache recent invoice IDs in Redis for faster checks
   - Invalidate cache on new invoice creation

3. **Async Processing**:
   - Invoice extraction runs in background
   - User gets immediate acknowledgment
   - Rejection sent later if duplicate found

4. **Rate Limiting**:
   - Limit OpenAI API calls to prevent quota exhaustion
   - Queue invoice processing during high load

### Scaling

For high-volume scenarios:

- **Sharding**: Partition invoices by date range
- **Read Replicas**: Separate DB for uniqueness checks
- **Batch Processing**: Process multiple invoices in parallel
- **CDN**: Cache common invoice templates

## 🔐 Security & Compliance

### Data Retention

- **Invoice PDFs**: Stored for 10 years (accounting requirement)
- **Extracted Data**: Permanent retention in DB
- **Validation Records**: Full audit trail

### Privacy

- Invoice data is sensitive financial information
- Access restricted to authorized personnel
- Encrypted at rest and in transit
- Audit logging for all access

### Fraud Prevention

The system detects:
- ✅ Exact duplicate invoices
- ✅ Similar invoices (future: OCR hash matching)
- ✅ Suspicious patterns (multiple rejections)
- ✅ Temporal anomalies (old invoices suddenly submitted)

## 📚 Additional Resources

- [Chinese Invoice Regulations](https://www.chinatax.gov.cn/)
- [OpenAI Vision API](https://platform.openai.com/docs/guides/vision)
- [PDF Processing in Go](https://github.com/pdfcpu/pdfcpu)
- [SQLite Performance Tuning](https://www.sqlite.org/speed.html)

---

**Questions?** Check logs or contact the development team.
