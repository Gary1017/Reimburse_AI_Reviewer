# AI Reimbursement Workflow System - Architecture Documentation

## Table of Contents

1. [System Overview](#system-overview)
2. [Architecture Layers](#architecture-layers)
3. [Component Details](#component-details)
4. [Data Flow](#data-flow)
5. [Security Architecture](#security-architecture)
6. [Scalability](#scalability)
7. [Maintenance](#maintenance)

## System Overview

The AI-Driven Reimbursement Workflow System automates the entire reimbursement lifecycle from approval tracking to voucher generation, integrating Lark's approval workflow with AI-powered auditing and seamless external accountant collaboration.

### Key Features

- **Automated Approval Tracking**: Real-time synchronization with Lark approval instances
- **AI-Driven Auditing**: Semantic policy validation and market-price benchmarking using OpenAI GPT-4
- **Exception-Based Management**: Intelligent routing of approvals requiring manual review  
- **Regulatory Compliance**: Automatic generation of accounting vouchers compliant with Mainland China standards
- **Zero-Error Processing**: ACID transactions with complete audit trail (10-year retention)
- **Seamless Collaboration**: Direct integration with external accountants via Lark messaging

## Architecture Layers

### 1. Integration Layer

**Purpose**: Handle all external system communications

**Components**:
- **Lark Webhook Receiver**: RESTful endpoint receiving approval instance events
  - Challenge-response verification
  - Signature validation (AES + SHA256)
  - Event routing
  
- **Lark API Client**: Official Lark Go SDK wrapper
  - Approval instance queries
  - Attachment downloads
  - Message sending
  - Token management
  
- **OpenAI Client**: AI service integration
  - Policy validation requests
  - Price benchmarking requests
  - Response parsing
  - Error handling

### 2. Business Logic Layer

**Purpose**: Core workflow orchestration and business rules

**Components**:
- **Workflow Engine**: State machine managing instance lifecycle
  - Status transitions: Created → Pending → AI_Auditing → In_Review/Auto_Approved → Approved → Voucher_Generating → Completed
  - Event handlers for webhook events
  - Idempotency handling
  
- **AI Auditing Orchestrator**: Coordinates AI-driven validation
  - Policy validation using OpenAI
  - Price benchmarking using OpenAI
  - Confidence scoring
  - Aggregation of results
  
- **Exception Manager**: Flags items requiring manual review
  - Confidence threshold evaluation
  - Violation detection
  - Priority assignment
  - Escalation rules
  
- **Voucher Generator**: Transforms approved instances into compliant forms
  - Excel template population
  - Chinese number capitalization
  - Regulatory field mapping
  - Attachment bundling

### 3. Data Persistence Layer

**Purpose**: Zero-error transaction processing and audit trails

**Components**:
- **Repository Pattern**: Abstraction over SQLite operations
  - Instance repository
  - History repository
  - Voucher repository
  
- **Transaction Manager**: ACID-compliant operations
  - SQLite WAL mode for concurrency
  - Connection pooling
  - Rollback handling
  
- **Audit Logger**: Immutable event log
  - Status change tracking
  - User action logging
  - 10-year retention
  - Integrity verification

## Component Details

### Workflow Engine

**File**: `internal/workflow/engine.go`

**State Machine**:
```
CREATED → PENDING → AI_AUDITING → AI_AUDITED 
  ↓                                    ↓
REJECTED                    IN_REVIEW / AUTO_APPROVED
                                      ↓
                                  APPROVED
                                      ↓
                            VOUCHER_GENERATING
                                      ↓
                                  COMPLETED
```

**Key Methods**:
- `HandleInstanceCreated()`: Creates database entry for new approvals
- `HandleStatusChanged()`: Updates status with audit trail
- `HandleInstanceApproved()`: Triggers voucher generation
- `HandleInstanceRejected()`: Terminates workflow

### AI Auditing Layer

**Files**: 
- `internal/ai/auditor.go` - Orchestration
- `internal/ai/policy_validator.go` - Policy checking
- `internal/ai/price_benchmarker.go` - Price validation

**Policy Validation Process**:
1. Load company policies from `configs/policies.json`
2. Build structured prompt with item details
3. Call OpenAI GPT-4 with JSON response format
4. Parse validation result (compliant, violations, confidence)

**Price Benchmarking Process**:
1. Extract item description, amount, context
2. Build market analysis prompt
3. Call OpenAI GPT-4 for price estimation
4. Calculate deviation from submitted amount
5. Flag if deviation > threshold (default 30%)

**Decision Logic**:
- **PASS**: Policy compliant, price reasonable, high confidence (>80%)
- **NEEDS_REVIEW**: Price unreasonable or confidence < 80%
- **FAIL**: Policy violations detected

### Voucher Generator

**Files**:
- `internal/voucher/generator.go` - Orchestration
- `internal/voucher/excel_filler.go` - Template population
- `internal/voucher/attachment_handler.go` - File downloads

**Excel Template Mapping**:
- Company information (name, tax ID)
- Voucher number (format: RB-YYYYMMDD-NNNN)
- Accounting period (会计期间)
- Applicant details (报销人、部门、工号)
- Itemized expenses table
- Total amount with Chinese capitalization (大写金额)
- Approval chain (审核人、复核人、批准人)
- Original receipt count (原始凭证张数)

**Chinese Number Capitalization Algorithm**:
```
123.56 CNY → 壹佰贰拾叁元伍角陆分
```

## Data Flow

### End-to-End Flow

```
1. User submits reimbursement in Lark
   ↓
2. Lark sends webhook event to system
   ↓
3. System verifies webhook signature
   ↓
4. Creates database entry (status: CREATED)
   ↓
5. Fetches instance details from Lark API
   ↓
6. Updates status to PENDING
   ↓
7. Triggers AI auditing (status: AI_AUDITING)
   ↓
8. Policy Validator checks compliance
   ↓
9. Price Benchmarker estimates reasonable price
   ↓
10. Exception Manager evaluates results
    ↓
11a. High confidence → AUTO_APPROVED
11b. Low confidence/violations → IN_REVIEW (manual)
    ↓
12. Human approver (if IN_REVIEW) or system (if AUTO_APPROVED)
    ↓
13. Status: APPROVED
    ↓
14. Voucher Generator fills Excel template
    ↓
15. Downloads attachments from Lark
    ↓
16. Generates voucher number
    ↓
17. Saves Excel file to output directory
    ↓
18. Email Sender sends to accountant via Lark
    ↓
19. Updates status to COMPLETED
    ↓
20. Full audit trail preserved in database
```

### Database Schema

**approval_instances**
- Primary tracking table
- Stores instance metadata and status
- JSON blob for form data
- Links to history and vouchers

**approval_history**
- Immutable audit trail
- Records every status transition
- Includes actor, timestamp, action data

**reimbursement_items**
- Individual expense line items
- AI audit results per item
- Links to receipt attachments

**generated_vouchers**
- Voucher metadata
- File paths
- Email delivery status

**system_config**
- Key-value configuration store
- Runtime settings

## Security Architecture

### Authentication & Authorization

1. **Webhook Verification**
   - Challenge-response handshake
   - SHA256 signature validation
   - Token verification
   - Timestamp validation (replay attack prevention)

2. **API Credentials**
   - Environment-based storage
   - No hardcoded secrets
   - Automatic token refresh (Lark)
   - Rate limit awareness

### Data Protection

1. **At Rest**
   - SQLite file permissions (chmod 600)
   - Optional encryption extension
   - Secure backup procedures

2. **In Transit**
   - HTTPS/TLS for all external APIs
   - Webhook requires HTTPS
   - Certificate validation

3. **Audit Trail**
   - Append-only logs
   - Checksumming for integrity
   - 10-year retention (regulatory)
   - No PII in logs

### Input Validation

- Parameterized SQL queries (no injection)
- String sanitization
- Type checking
- Business rule validation
- File type restrictions

## Scalability

### Current Architecture (SQLite)

**Suitable for**:
- Single-instance deployments
- Up to 1000 approvals/day
- Read-heavy workloads
- Small to medium enterprises

**Limitations**:
- Single writer (write serialization)
- No horizontal scaling
- File-based replication complexity

### Scaling Options

**Vertical Scaling**:
- Increase server CPU/RAM
- Optimize database queries
- Enable query caching
- Use connection pooling (already implemented)

**Horizontal Scaling** (requires migration):
- Switch to PostgreSQL for multi-writer support
- Implement request queuing (Redis)
- Load balancer for webhook endpoints
- Stateless application design (already achieved)

**Performance Optimizations**:
- Database indices on frequently queried fields
- Batch AI API calls where possible
- Async webhook processing (already implemented)
- CDN for static assets

## Maintenance

### Regular Tasks

**Daily**:
- Monitor error rates in logs
- Check AI API quota usage
- Verify webhook delivery

**Weekly**:
- Review AI audit accuracy
- Analyze exception routing effectiveness
- Database health check

**Monthly**:
- SQLite VACUUM operation
- Dependency security scan
- Backup verification

**Quarterly**:
- Security audit
- Policy configuration review
- Performance tuning
- Cost optimization

### Monitoring Metrics

**System Health**:
- HTTP response times (p50, p95, p99)
- Database connection pool usage
- Disk space utilization
- Memory consumption

**Business Metrics**:
- Approval processing rate
- AI auto-approval percentage
- Exception routing accuracy
- Voucher generation success rate
- Email delivery success rate

**AI Metrics**:
- OpenAI API latency
- Policy validation confidence distribution
- Price benchmark accuracy
- False positive/negative rates

### Troubleshooting

**Common Issues**:

1. **Webhook not receiving events**
   - Check Lark webhook configuration
   - Verify firewall rules
   - Review signature verification logs

2. **AI API failures**
   - Check API key validity
   - Review rate limits
   - Implement fallback to manual review

3. **Database lock errors**
   - Verify WAL mode enabled
   - Check connection pool settings
   - Review transaction timeout

4. **Email delivery failures**
   - Verify Lark message API permissions
   - Check accountant email configuration
   - Review attachment file sizes

## Deployment Architecture

```
Internet
   ↓
HTTPS/TLS (Nginx)
   ↓
Application Server (Go)
   ├── SQLite DB
   ├── File Storage (vouchers, attachments)
   └── Logs
   ↓
External APIs
   ├── Lark Open Platform
   └── OpenAI API
```

## Future Enhancements

1. **Machine Learning Improvements**
   - Train custom models on historical data
   - Improve price estimation accuracy
   - Anomaly detection

2. **Integration Expansions**
   - ERP system integration
   - Bank reconciliation
   - Invoice OCR

3. **Feature Additions**
   - Web dashboard for reviewers
   - Mobile app integration
   - Real-time notifications
   - Analytics and reporting

4. **Performance**
   - PostgreSQL migration for scale
   - Redis caching layer
   - Microservices architecture

## References

- [Lark Open Platform Documentation](https://open.larksuite.com/)
- [OpenAI API Documentation](https://platform.openai.com/docs)
- [China Accounting Standards](http://www.mof.gov.cn/)
- [SQLite Documentation](https://www.sqlite.org/docs.html)
- [Go Best Practices](https://golang.org/doc/effective_go)
