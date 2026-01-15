# API Documentation

## Overview

The AI Reimbursement Workflow System exposes HTTP endpoints for webhook integration and administrative operations.

## Base URL

```
http://localhost:8080
```

## Authentication

### Webhook Endpoints

Lark webhooks are authenticated via:
- **Signature verification**: SHA256 HMAC with timestamp and nonce
- **Token validation**: Verify token matches configuration

### Admin API (Future)

Currently, admin endpoints are for internal use. Future versions will implement:
- JWT-based authentication
- Role-based access control (RBAC)

## Endpoints

### Health Check

Check system health status.

**Request**:
```http
GET /health
```

**Response**:
```json
{
  "status": "healthy",
  "service": "ai-reimbursement",
  "time": "2026-01-14T10:30:00Z"
}
```

### Webhook Handler

Receive Lark approval events.

**Request**:
```http
POST /webhook/approval
Content-Type: application/json
X-Lark-Request-Timestamp: 1642156800
X-Lark-Request-Nonce: abc123
X-Lark-Signature: sha256signature

{
  "schema": "2.0",
  "header": {
    "event_id": "abc123",
    "event_type": "approval.approval_instance.approved",
    "create_time": "1642156800",
    "token": "verification_token",
    "app_id": "cli_xxxxx",
    "tenant_key": "tenant_xxxxx"
  },
  "event": {
    "instance_code": "6BFE0B02-xxxx",
    "user_id": "ou_xxxxx",
    "status": "APPROVED"
  }
}
```

**Challenge Verification (Initial Setup)**:
```json
{
  "type": "url_verification",
  "challenge": "challenge_string",
  "token": "verification_token"
}
```

**Challenge Response**:
```json
{
  "challenge": "challenge_string"
}
```

**Event Response**:
```json
{
  "message": "Event received"
}
```

## Event Types

### approval.approval_instance.created

Triggered when a new approval instance is created.

**Event Payload**:
```json
{
  "instance_code": "6BFE0B02-xxxx",
  "user_id": "ou_xxxxx",
  "definition_code": "approval_def_code",
  "start_time": "1642156800"
}
```

### approval.approval_instance.approved

Triggered when an approval instance is approved.

**Event Payload**:
```json
{
  "instance_code": "6BFE0B02-xxxx",
  "user_id": "ou_xxxxx",
  "status": "APPROVED",
  "approval_time": "1642156800"
}
```

### approval.approval_instance.rejected

Triggered when an approval instance is rejected.

**Event Payload**:
```json
{
  "instance_code": "6BFE0B02-xxxx",
  "user_id": "ou_xxxxx",
  "status": "REJECTED",
  "rejection_time": "1642156800",
  "rejection_reason": "Reason text"
}
```

### approval.approval_instance.status_changed

Triggered when approval status changes.

**Event Payload**:
```json
{
  "instance_code": "6BFE0B02-xxxx",
  "user_id": "ou_xxxxx",
  "previous_status": "PENDING",
  "new_status": "IN_REVIEW"
}
```

## Error Responses

### 400 Bad Request

```json
{
  "error": "Failed to parse request body",
  "details": "Invalid JSON format"
}
```

### 401 Unauthorized

```json
{
  "error": "Invalid signature",
  "details": "Webhook signature verification failed"
}
```

### 500 Internal Server Error

```json
{
  "error": "Internal server error",
  "details": "Database connection failed"
}
```

## Status Codes

- **200 OK**: Request successful
- **400 Bad Request**: Invalid request format
- **401 Unauthorized**: Authentication failed
- **404 Not Found**: Resource not found
- **500 Internal Server Error**: Server error

## Rate Limiting

Currently, no rate limiting is enforced. Future versions will implement:
- Per-IP rate limiting
- Token bucket algorithm
- Configurable limits

## Webhooks Best Practices

### Retry Logic

Lark implements automatic retry for failed webhooks:
- 3 retry attempts
- Exponential backoff
- 30-second timeout

### Idempotency

The system handles duplicate events using:
- Event ID deduplication
- Instance ID uniqueness checks
- Idempotent operations

### Response Time

- Respond within 3 seconds
- Process events asynchronously
- Return 200 immediately

## Testing Webhooks

### Using cURL

```bash
# Health check
curl http://localhost:8080/health

# Simulate webhook (with signature)
curl -X POST http://localhost:8080/webhook/approval \
  -H "Content-Type: application/json" \
  -H "X-Lark-Request-Timestamp: $(date +%s)" \
  -H "X-Lark-Request-Nonce: test123" \
  -H "X-Lark-Signature: <calculated_signature>" \
  -d '{
    "schema": "2.0",
    "header": {
      "event_id": "test-event-123",
      "event_type": "approval.approval_instance.created",
      "create_time": "1642156800",
      "token": "your_verify_token"
    },
    "event": {
      "instance_code": "TEST-INSTANCE-001",
      "user_id": "ou_test_user"
    }
  }'
```

### Signature Calculation

```python
import hashlib
import hmac

timestamp = "1642156800"
nonce = "test123"
encrypt_key = "your_encrypt_key"
body = '{"schema":"2.0",...}'

content = timestamp + nonce + encrypt_key + body
signature = hashlib.sha256(content.encode()).hexdigest()
```

## Monitoring & Debugging

### Request Logging

All requests are logged with:
- Timestamp
- Method and path
- Status code
- Response time
- Client IP

### Debug Mode

Enable debug logging:
```yaml
# config.yaml
logger:
  level: debug
```

Logs include:
- Incoming webhook payloads
- AI API requests/responses
- Database queries
- Error stack traces

## Support

For API questions and issues:
- Check application logs
- Review architecture documentation
- Contact development team
