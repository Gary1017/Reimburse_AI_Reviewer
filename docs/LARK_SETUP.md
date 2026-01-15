# Lark Approval Event Subscription Setup Guide

This guide walks you through setting up Lark approval event subscriptions using your `LARK_APPROVAL_CODE`.

## 📋 Prerequisites

- Lark/Feishu Open Platform account with admin access
- Your approval definition code (`LARK_APPROVAL_CODE`)
- Public webhook endpoint (deployed application or ngrok tunnel)

## 🔑 Step 1: Get Your Approval Code

Your approval code is the unique identifier for your reimbursement approval definition.

### Method 1: From Admin Console

1. Go to [Lark Approval Admin Console](https://www.feishu.cn/approval/admin/approvalList?devMode=on)
   - The `devMode=on` parameter enables developer mode
   - This allows you to see and set custom IDs for controls and nodes

2. Find your reimbursement approval workflow

3. Click the **Edit** button (✏️)

4. Look at the browser address bar

5. Find the URL parameter: `definitionCode=E3254848-D172-4169-B03E-744E7CD11F06`

6. Copy the value after `definitionCode=` - this is your `LARK_APPROVAL_CODE`

Example URL:
```
https://www.feishu.cn/approval/admin/approvalList/edit?definitionCode=E3254848-D172-4169-B03E-744E7CD11F06
```

### Method 2: Via API

```bash
curl -X GET \
  'https://open.feishu.cn/open-apis/approval/v4/approvals' \
  -H 'Authorization: Bearer YOUR_TENANT_ACCESS_TOKEN'
```

The response will include `approval_code` for each approval definition.

## 🔐 Step 2: Security Considerations

**⚠️ IMPORTANT**: The approval code grants access to:
- All approval instances under this definition
- All form data submitted
- All approval history and operations

**Security Best Practices**:
1. ✅ Store in environment variables (never in code)
2. ✅ Use GitHub Secrets for CI/CD
3. ✅ Restrict access to DevOps/Admin team only
4. ✅ Rotate if compromised
5. ❌ Never commit to version control
6. ❌ Never share in chat/email

## 📡 Step 3: Subscribe to Approval Events

There are two methods to receive approval events:

### Method A: Webhook Subscription (Recommended)

This method allows Lark to push events to your server in real-time.

#### 1. Register Your Webhook Endpoint

In Lark Open Platform Console:

1. Go to **Event Subscriptions** section
2. Add Request URL: `https://your-domain.com/webhook/approval`
3. Lark will send a verification challenge
4. Your server must respond with the challenge value

The system already handles this verification in `internal/webhook/verifier.go`:

```go
func (v *Verifier) Verify(c *gin.Context) {
    // ... handles url_verification challenge
    if challenge.Type == "url_verification" {
        c.JSON(http.StatusOK, gin.H{"challenge": challenge.Challenge})
        return
    }
}
```

#### 2. Subscribe to Approval Events

Subscribe to these event types:

| Event Type | Description | Use Case |
|------------|-------------|----------|
| `approval_instance` | Any change to instance | Catch all events |
| `approval.approval_instance.created_v4` | New approval created | Initial processing |
| `approval.approval_instance.approved_v4` | Approval approved | Generate voucher |
| `approval.approval_instance.rejected_v4` | Approval rejected | Send notification |
| `approval.approval_instance.cancelled_v4` | Approval cancelled | Cleanup |

#### 3. Configure Event Scopes

Set permissions in Lark Open Platform:
- ✅ `approval:approval` - Read approval data
- ✅ `approval:approval.readonly` - Read-only access
- ✅ `im:message` - Send messages
- ✅ `im:message.file` - Download attachments

### Method B: Polling (Fallback)

If webhooks aren't available, you can poll for changes:

```go
// Poll approval instances periodically
func (c *Client) PollApprovalInstances(ctx context.Context, approvalCode string) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            instances, err := c.ListApprovalInstances(ctx, approvalCode)
            if err != nil {
                log.Error("Failed to list instances", zap.Error(err))
                continue
            }
            // Process new instances
            for _, instance := range instances {
                // Check if already processed
                // Process if new
            }
        case <-ctx.Done():
            return
        }
    }
}
```

## 🧪 Step 4: Test Event Reception

### Using ngrok for Local Testing

```bash
# 1. Start your local server
go run cmd/server/main.go

# 2. Expose via ngrok
ngrok http 8080

# 3. Use the ngrok URL in Lark webhook config
# Example: https://abc123.ngrok.io/webhook/approval
```

### Test Event Payload

When a reimbursement is submitted, you'll receive:

```json
{
  "schema": "2.0",
  "header": {
    "event_id": "5e3702a84e847582be8db7fb73283c02",
    "event_type": "approval.approval_instance.created_v4",
    "create_time": "1608725989000",
    "token": "your_verification_token",
    "app_id": "cli_a1b2c3d4e5f6",
    "tenant_key": "2ca1d211f64f6438808464e82b281a46"
  },
  "event": {
    "approval_code": "E3254848-D172-4169-B03E-744E7CD11F06",
    "instance_code": "EB1678AC-D2FB-4DBF-9E9D-B52D9C64A41E",
    "user_id": "ou_123456789abcdef",
    "open_id": "ou_123456789abcdef",
    "status": "PENDING",
    "start_time": "1608725989",
    "form": [
      {
        "id": "field_expense_type",
        "type": "select",
        "value": "Travel"
      },
      {
        "id": "field_amount",
        "type": "number",
        "value": "1580.00"
      },
      {
        "id": "field_attachments",
        "type": "file",
        "value": [
          {
            "file_token": "boxcnABCDEF1234567890",
            "file_name": "invoice_20240115.pdf"
          }
        ]
      }
    ]
  }
}
```

## 📊 Step 5: Verify Event Processing

### Check Logs

```bash
# Docker
docker-compose logs -f app | grep "approval_instance"

# Local
tail -f logs/app.log | grep "approval_instance"
```

### Verify Database

```bash
# Connect to SQLite
sqlite3 data/reimbursement.db

# Check for new instances
SELECT * FROM approval_instances ORDER BY created_at DESC LIMIT 10;

# Check invoice extraction
SELECT * FROM invoices ORDER BY created_at DESC LIMIT 10;

# Check audit trail
SELECT * FROM approval_history ORDER BY timestamp DESC LIMIT 20;
```

### API Health Check

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "ok",
  "version": "1.0.0"
}
```

## 🐛 Troubleshooting

### Issue: Events Not Received

**Check:**
1. ✅ Webhook URL is publicly accessible
2. ✅ SSL certificate is valid (Lark requires HTTPS)
3. ✅ Firewall allows incoming connections
4. ✅ Event subscription is active in Lark console
5. ✅ Approval code matches your reimbursement workflow

**Debug:**
```bash
# Test webhook endpoint directly
curl -X POST http://localhost:8080/webhook/approval \
  -H "Content-Type: application/json" \
  -d '{"type":"url_verification","challenge":"test123"}'

# Expected: {"challenge":"test123"}
```

### Issue: Verification Token Mismatch

Your `LARK_APPROVAL_CODE` is NOT the verification token. The system no longer uses `LARK_VERIFY_TOKEN` or `LARK_ENCRYPT_KEY`.

**New Authentication Flow:**
1. Lark sends event with header containing `token`
2. System uses `LARK_APPROVAL_CODE` to validate that events are for your approval
3. No additional verification tokens needed

### Issue: Invoice Extraction Fails

**Common causes:**
1. PDF is image-based (needs OCR)
2. Invoice format not recognized
3. OpenAI API rate limit
4. Invalid invoice (missing code or number)

**Solution:**
```bash
# Check OpenAI API status
curl https://status.openai.com/api/v2/status.json

# Review extraction logs
grep "invoice extraction" logs/app.log

# Test with sample invoice
curl -X POST http://localhost:8080/test/extract-invoice \
  -F "file=@sample_invoice.pdf"
```

## 📚 Additional Resources

### Official Documentation

- [Lark Approval API](https://open.feishu.cn/document/server-docs/approval-v4/approval)
- [Event Subscription](https://open.feishu.cn/document/server-docs/event-subscription-guide)
- [Webhook Best Practices](https://open.feishu.cn/document/ukTMukTMukTM/uUTNz4SN1MjL1UzM)

### Code References

- Webhook handler: `internal/webhook/handler.go`
- Lark client: `internal/lark/client.go`
- Event verifier: `internal/webhook/verifier.go`
- Workflow engine: `internal/workflow/engine.go`

## 🔄 Next Steps

After setting up event subscriptions:

1. **Configure AI auditing rules** in `configs/policies.json`
2. **Prepare Excel template** in `templates/reimbursement_form.xlsx`
3. **Set up email** for accountant notifications
4. **Test end-to-end workflow** with a sample reimbursement
5. **Monitor logs** for first few days
6. **Set up alerts** for failed invoices

## 📞 Support

If you encounter issues:

1. Check the logs: `logs/app.log`
2. Verify configuration: `configs/config.yaml`
3. Review audit trail: `SELECT * FROM approval_history`
4. Contact Lark support for platform issues
5. Check GitHub Issues for known problems

---

**Remember**: Your `LARK_APPROVAL_CODE` is sensitive. Treat it like a password! 🔐
