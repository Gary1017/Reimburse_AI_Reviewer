# AI-Driven Reimbursement Workflow System

An enterprise-grade automated reimbursement workflow system that integrates Lark approval processes with AI-powered auditing, transforming structured form data into legally-binding vouchers compliant with Mainland China accounting regulations.

## 🌟 Key Features

- **Automated Approval Tracking**: Real-time synchronization with Lark approval instances
- **AI-Driven Auditing**: Semantic policy validation and market-price benchmarking using OpenAI GPT-4
- **Invoice Uniqueness Checking**: Automatically extracts and validates invoice codes (发票代码 + 发票号码) to prevent duplicate submissions
- **Exception-Based Management**: Intelligent routing of approvals requiring manual review
- **Regulatory Compliance**: Automatic generation of accounting vouchers compliant with China standards
- **Zero-Error Processing**: ACID transactions with complete audit trail (10-year retention)
- **Seamless Collaboration**: Direct integration with external accountants via Lark messaging

## 🏗️ Architecture

The system is built on three core layers:

1. **Integration Layer**: Lark webhooks, OpenAI API, message delivery
2. **Business Logic Layer**: Workflow engine, AI auditing, invoice verification, voucher generation
3. **Data Persistence Layer**: SQLite with transaction safety and audit trails

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed architecture documentation.

## 📋 Prerequisites

- Go 1.22 or higher
- SQLite 3.42+
- **Lark Open Platform account** with approval workflow access
- **OpenAI API key**
- **Excel template** for reimbursement forms

## 🔧 Configuration

### Required Environment Variables

| Variable | Description | Example | How to Get It |
|----------|-------------|---------|---------------|
| `LARK_APP_ID` | Lark application ID | `cli_a1b2c3d4e5f6` | Lark Open Platform Console → Your App |
| `LARK_APP_SECRET` | Lark application secret | `AbCdEfGhIjKlMn...` | Lark Open Platform Console → Your App |
| `LARK_APPROVAL_CODE` | Approval definition code | `E3254848-D172-4169...` | See "Getting Approval Code" below |
| `OPENAI_API_KEY` | OpenAI API key | `sk-proj-...` | OpenAI Platform → API keys |
| `ACCOUNTANT_EMAIL` | Accountant's email | `accountant@company.com` | Your accountant |
| `COMPANY_NAME` | Company name | `Your Company Ltd.` | Your company |
| `COMPANY_TAX_ID` | Chinese tax ID (18 digits) | `91110000123456789X` | Your tax ID |

### Getting the Lark Approval Code

The approval code is the unique identifier for your approval definition:

1. Go to [Lark Approval Admin Console](https://www.feishu.cn/approval/admin/approvalList?devMode=on) (with `devMode=on`)
2. Find your reimbursement approval and click **Edit**
3. In the browser address bar, find the parameter: `definitionCode=E3254848-D172-4169-B03E-744E7CD11F06`
4. Copy the value after `definitionCode=` - this is your `LARK_APPROVAL_CODE`

**Security Note**: Keep this approval code confidential as it grants access to all approval data under this definition.

## 🚀 Quick Start

### Option 1: Local Development

```bash
# 1. Clone the repository
git clone git@github.com:Gary1017/Reimburse_AI_Reviewer.git
cd Reimburse_AI_Reviewer

# 2. Copy configuration
cp configs/config.example.yaml configs/config.yaml

# 3. Set environment variables
export LARK_APP_ID="your_app_id"
export LARK_APP_SECRET="your_app_secret"
export LARK_APPROVAL_CODE="your_approval_code"
export OPENAI_API_KEY="your_openai_key"
export ACCOUNTANT_EMAIL="accountant@company.com"
export COMPANY_NAME="Your Company Ltd."
export COMPANY_TAX_ID="91110000123456789X"

# 4. Place your Excel template
cp your_template.xlsx templates/reimbursement_form.xlsx

# 5. Install dependencies
go mod download

# 6. Run database migrations
go run cmd/server/main.go

# 7. Start the server
go run cmd/server/main.go
```

### Option 2: Docker

```bash
# Build and run with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f app
```

### Option 3: Deploy to AWS (CI/CD)

See [docs/QUICKSTART_DEVOPS.md](docs/QUICKSTART_DEVOPS.md) for complete deployment guide.

## 📊 Invoice Uniqueness Feature

The system automatically checks invoice uniqueness to prevent duplicate submissions:

### How It Works

1. **PDF Download**: When an approval instance is created, all attachments are downloaded
2. **Invoice Extraction**: AI extracts key invoice fields:
   - **发票代码 (Invoice Code)**: 10-12 digit code
   - **发票号码 (Invoice Number)**: 8-digit number
   - Other fields: amount, date, seller/buyer info
3. **Uniqueness Check**: Combines code + number to create unique ID
4. **Validation**: Checks against all previous invoices in database
5. **Action**:
   - ✅ **Unique**: Approval proceeds normally
   - ❌ **Duplicate**: Approval is flagged and rejected with details of first submission

### Invoice Data Stored

```json
{
  "invoice_code": "1200192130",
  "invoice_number": "00185025",
  "unique_id": "1200192130-00185025",
  "invoice_date": "2024-12-15",
  "total_amount": 1580.00,
  "seller_name": "北京科技有限公司",
  "seller_tax_id": "91110108MA01XXXXX",
  "buyer_name": "Your Company Ltd.",
  "buyer_tax_id": "91110000123456789X"
}
```

## 🔐 Security

- **Webhook Verification**: All incoming webhooks are validated
- **Credential Management**: Environment-based secrets (never in code)
- **Audit Trails**: Complete transaction history with 10-year retention
- **Input Validation**: Sanitization and type checking
- **ACID Transactions**: Zero-error database operations

See [docs/SECURITY.md](docs/SECURITY.md) for complete security documentation.

## 📈 Workflow

```
User submits reimbursement in Lark
    ↓
Lark sends webhook to system
    ↓
System downloads attachments
    ↓
AI extracts invoice data (发票代码 + 发票号码)
    ↓
Check invoice uniqueness
    ├─ Duplicate → Reject with error
    └─ Unique → Continue
        ↓
    AI policy validation
        ↓
    AI price benchmarking
        ↓
    Exception-based routing
    ├─ High confidence → Auto-approve
    └─ Low confidence → Manual review
        ↓
    Generate Excel voucher
        ↓
    Send to accountant via Lark
        ↓
    Mark as COMPLETED
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Test specific package
go test ./internal/invoice/...
```

## 📚 Documentation

- **[ARCHITECTURE.md](docs/ARCHITECTURE.md)**: Complete system design (200+ pages)
- **[DEVOPS_SETUP.md](docs/DEVOPS_SETUP.md)**: Full DevOps guide with AWS deployment
- **[QUICKSTART_DEVOPS.md](docs/QUICKSTART_DEVOPS.md)**: 30-minute quick start for deployment
- **[SECURITY.md](docs/SECURITY.md)**: Security architecture and best practices
- **[DEPLOYMENT.md](docs/DEPLOYMENT.md)**: Production deployment guide
- **[API.md](docs/API.md)**: API and webhook reference

## 🛠️ Technology Stack

- **Language**: Go 1.22
- **Database**: SQLite with WAL mode
- **Lark SDK**: github.com/larksuite/oapi-sdk-go/v3
- **AI Provider**: OpenAI GPT-4
- **Excel**: github.com/xuri/excelize/v2
- **Configuration**: Viper
- **Logging**: Zap
- **HTTP**: Gin

## 📞 Support

For issues and questions:
- Check logs in `logs/` directory
- Review documentation in `docs/` directory
- Verify configuration in `configs/config.yaml`
- Contact development team

## 📝 License

Proprietary - Internal Use Only

---

**Built with ❤️ for enterprise financial automation**
