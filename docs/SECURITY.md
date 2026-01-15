# Security Documentation

## Overview

This document outlines the security measures implemented in the AI Reimbursement Workflow System and best practices for maintaining security.

## Security Architecture

### Defense in Depth

The system implements multiple layers of security:

1. **Network Layer**: HTTPS/TLS encryption
2. **Application Layer**: Webhook signature verification, input validation
3. **Data Layer**: SQLite with file permissions, audit trails
4. **Access Control**: Environment-based credential management

## Threat Model

### Identified Threats

1. **Webhook Spoofing**: Malicious actors sending fake approval events
2. **Data Injection**: SQL injection, XSS, command injection
3. **Data Breach**: Unauthorized access to financial data
4. **Denial of Service**: Resource exhaustion attacks
5. **Credential Exposure**: API keys and secrets leaked

### Mitigations

| Threat | Mitigation |
|--------|-----------|
| Webhook Spoofing | SHA256 signature verification |
| SQL Injection | Parameterized queries, prepared statements |
| XSS | Input sanitization, output encoding |
| Data Breach | File permissions, encryption at rest |
| DoS | Rate limiting, request timeouts |
| Credential Exposure | Environment variables, never in code |

## Authentication & Authorization

### Lark Webhook Verification

All incoming webhooks are verified using:

1. **Challenge-Response**: Initial handshake verification
2. **Signature Verification**: AES + SHA256 validation
3. **Token Validation**: Verify token matches configuration

```go
// Signature calculation
content := timestamp + nonce + encryptKey + body
signature := sha256(content)
```

### API Credentials

- Stored in environment variables
- Never committed to version control
- Rotated regularly (recommended: every 90 days)

## Data Security

### Sensitive Data Handling

**Sensitive fields**:
- Applicant personal information
- Reimbursement amounts
- Receipt attachments
- Bank account information (if applicable)

**Protection measures**:
- Database file permissions: `chmod 600`
- Audit trail logging
- Retention policy enforcement

### Encryption

**At Rest**:
- SQLite database file permissions
- Optional: SQLite encryption extension (SEE)

**In Transit**:
- HTTPS/TLS for all external communications
- Lark API uses HTTPS
- OpenAI API uses HTTPS

### Audit Trail

Every action is logged with:
- Timestamp
- User ID
- Action type
- Previous state
- New state
- Request metadata

Audit logs are:
- Immutable (append-only)
- Retained for 10 years (regulatory requirement)
- Checksummed for integrity verification

## Input Validation

### Webhook Payloads

```go
// Validation steps
1. Verify signature
2. Validate JSON structure
3. Sanitize string inputs
4. Validate data types
5. Check business logic constraints
```

### User Inputs

All user inputs are:
- Type-checked
- Length-limited
- Sanitized for special characters
- Validated against business rules

### File Uploads

Attachments from Lark:
- Size limits enforced
- File type validation
- Virus scanning (recommended)
- Stored in isolated directory

## API Security

### OpenAI API

- API key stored in environment
- Rate limiting implemented
- Timeout configuration
- Error handling (no sensitive data in logs)

### Lark API

- OAuth 2.0 token management
- Token refresh handling
- Permission scoping
- API rate limit awareness

## Logging & Monitoring

### Security Events

Log these security-relevant events:
- Failed webhook verifications
- Invalid authentication attempts
- Unusual activity patterns
- API errors and rate limits
- Database transaction failures

### Log Security

- Structured logging (JSON)
- No sensitive data in logs
- Log rotation and retention
- Centralized log aggregation

### Alerts

Configure alerts for:
- Multiple failed webhook verifications
- Unusual spike in requests
- Database errors
- AI API failures
- Email delivery failures

## Compliance

### Mainland China Regulations

1. **Data Localization**: All data stored within China
2. **Accounting Standards**: GB/T standards compliance
3. **Tax Regulations**: 10-year data retention
4. **Privacy Laws**: PIPL compliance for personal data

### Best Practices

- Regular security audits
- Penetration testing
- Dependency vulnerability scanning
- Security training for developers

## Incident Response

### Incident Types

1. **Data Breach**: Unauthorized access to financial data
2. **Service Disruption**: DoS or system failure
3. **Credential Compromise**: API keys or secrets exposed
4. **Audit Failure**: Loss of audit trail integrity

### Response Procedure

1. **Detection**: Monitor logs and alerts
2. **Containment**: Isolate affected systems
3. **Investigation**: Analyze logs and audit trails
4. **Remediation**: Fix vulnerabilities
5. **Recovery**: Restore normal operations
6. **Post-Mortem**: Document and learn

## Security Checklist

### Deployment

- [ ] All credentials in environment variables
- [ ] HTTPS/TLS enabled
- [ ] Webhook signature verification active
- [ ] File permissions set correctly
- [ ] Database backups configured
- [ ] Logging and monitoring active
- [ ] Rate limiting configured
- [ ] Error messages don't expose sensitive data

### Regular Maintenance

- [ ] Rotate API credentials (every 90 days)
- [ ] Update dependencies (weekly scan)
- [ ] Review audit logs (weekly)
- [ ] Security audit (quarterly)
- [ ] Penetration testing (annually)
- [ ] Disaster recovery drill (annually)

## Security Contact

For security issues:
1. Do not create public GitHub issues
2. Contact security team directly
3. Provide detailed information
4. Allow time for remediation before disclosure

## References

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Lark Security Best Practices](https://open.larksuite.com/)
- [OpenAI API Security](https://platform.openai.com/docs/guides/safety-best-practices)
- [China Cybersecurity Law](http://www.cac.gov.cn/)
