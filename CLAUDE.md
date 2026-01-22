# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AI-Driven Reimbursement Workflow System - A Go service that automates enterprise reimbursement processing by integrating Lark (Feishu) approval workflows with GPT-4 powered auditing. Generates accounting vouchers compliant with Mainland China regulations.

## Build & Development Commands

```bash
# Build
make build              # Build to bin/server

# Run
make run                # Run server (go run cmd/server/main.go)
go run cmd/server/main.go  # Alternative direct run

# Test
make test               # Run all tests with verbose + coverage
go test -v ./internal/workflow/...  # Run tests for specific package
go test -v -run TestFunctionName ./internal/ai/...  # Run single test

# Code Quality
make lint               # Run golangci-lint
make fmt                # Format code (go fmt + goimports)
make security           # Run gosec security scan

# Database
make migrate            # Run SQLite migrations

# Docker
make docker-build       # Build Docker image
make docker-run         # Run container (requires env vars)
```

## Architecture

### Three-Layer Design

1. **Integration Layer** (`internal/lark/`, `internal/ai/`)
   - `lark.Client`: Lark SDK wrapper for approvals and attachments
   - `lark.EventProcessor`: Event processing adapter (available for webhook integration)
   - `ai.Auditor`: Orchestrates PolicyValidator and PriceBenchmarker

2. **Business Logic Layer** (`internal/workflow/`, `internal/voucher/`, `internal/invoice/`)
   - `workflow.Engine`: State machine orchestrating approval lifecycle (CREATED → PENDING → AI_AUDITING → APPROVED/REJECTED → VOUCHER_GENERATING → COMPLETED)
   - `workflow.StatusTracker`: Status transitions with audit trail
   - `voucher.Generator`: Excel voucher generation with Chinese number formatting
   - `invoice.PDFReader`: GPT-4o Vision API for PDF/image invoice extraction

3. **Persistence Layer** (`internal/repository/`, `pkg/database/`)
   - Repository pattern for all entities (instances, items, attachments, invoices)
   - SQLite with WAL mode, ACID transactions via `db.WithTransaction()`

### Background Workers (`internal/worker/`)

- `AsyncDownloadWorker`: Non-blocking Lark attachment downloads (ARCH-007)
- `InvoiceProcessor`: AI-driven invoice extraction and audit (ARCH-011)
- `StatusPoller`: Fallback polling for approval status changes (every 30s, not actively used)

### Key Data Models (`internal/models/`)

- `ApprovalInstance`: Main reimbursement record with status and form data
- `ReimbursementItem`: Individual expense items (TRAVEL, MEAL, ACCOMMODATION, EQUIPMENT)
- `Attachment`: Downloaded receipts with status tracking (PENDING → DOWNLOADED → PROCESSED)
- `Invoice`: Extracted invoice data with audit results

## Development Workflow (Agents)

This project uses a TDD-oriented agent workflow defined in `.cursor/agents.yaml`:

1. **Architect**: Designs ARCH-XXX requirements with traceability IDs
2. **Test Engineer**: Writes failing-first tests mapped to ARCH-XXX
3. **Implementor**: Implements to pass tests without changing architecture
4. **Documentor**: Creates delivery docs in `docs/` folder

When implementing new features, reference existing ARCH-XXX IDs in `docs/ARCHITECTURE.md` and phase reports in `docs/DEVELOPMENT/`.

## Lark Event Subscription

**CRITICAL: This project uses WebSocket-based event subscription, NOT HTTP webhooks.**

### Implementation Pattern (CORRECT WAY)

The system subscribes to Lark approval events using the official SDK's WebSocket client:

```go
import (
    "github.com/larksuite/oapi-sdk-go/v3/ws"
    "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
)

// 1. Create event dispatcher with custom event handler
dispatcher := dispatcher.NewEventDispatcher("", "")  // Empty strings for WebSocket mode
dispatcher.OnCustomizedEvent("approval_instance", eventProcessor.HandleCustomizedEvent)

// 2. Create WebSocket client
wsClient := ws.NewClient(
    appID,
    appSecret,
    ws.WithEventHandler(dispatcher),
)

// 3. Start WebSocket connection
wsClient.Start(ctx)
```

### Event Flow

```
Lark Platform → WebSocket → EventDispatcher → EventProcessor → WorkflowEngine → VoucherGenerator
```

### Why WebSocket, Not HTTP Webhooks?

1. **No intranet penetration needed** during testing
2. **Automatic authentication** at connection establishment
3. **Plaintext event data** (no decryption needed)
4. **Real-time events** with persistent connection
5. **Only requires public network access**

### DO NOT

❌ **DO NOT create HTTP webhook endpoints** like `/webhook/event`
❌ **DO NOT use verification tokens or encrypt keys** (not needed for WebSocket)
❌ **DO NOT use `dispatcher.Do()` method** (that's for HTTP webhooks)
❌ **DO NOT read webhook documentation** when implementing event subscription

### Implementation Location

- **WebSocket Client**: `internal/services/infrastructure.go` (`InitializeWebSocketClient`, `StartWebSocketClient`)
- **Event Dispatcher**: `internal/lark/event_dispatcher.go` (`NewEventDispatcher`)
- **Event Processor**: `internal/lark/event_processor.go` (`HandleCustomizedEvent`)
- **Workflow Handler**: `internal/workflow/engine.go` (`HandleInstanceApproved`, etc.)

### Reference

See Python SDK example pattern:
```python
event_handler = lark.EventDispatcherHandler.builder("", "") \
    .register_p1_customized_event("approval_instance", do_message_event) \
    .build()

cli = lark.ws.Client("APP_ID", "APP_SECRET", event_handler=event_handler)
cli.start()
```

## Configuration

Copy `configs/config.example.yaml` to `configs/config.yaml`. Sensitive values via environment variables:
- `LARK_APP_ID`, `LARK_APP_SECRET`, `LARK_APPROVAL_CODE`
- `OPENAI_API_KEY`
- `ACCOUNTANT_EMAIL`
- `COMPANY_NAME`, `COMPANY_TAX_ID`

## Key Patterns

- **Idempotency**: `HandleInstanceCreated` checks for existing records before creation
- **Transaction safety**: Use `db.WithTransaction()` for multi-table operations
- **Status mapping**: `mapLarkStatus()` in engine.go translates Lark statuses to internal states
- **Confidence scoring**: AI audit returns confidence 0-1; thresholds determine auto-approval vs review
- **Graceful degradation**: Voucher generation failures don't block the workflow; attachments are still organized

## Known Issues

### CJK Font Error in Excel Generation

**Error:** `syntax error: cannot find builtin CJK font`

**Cause:** The excelize library may fail when saving Excel files with Chinese characters if CJK fonts are not available in the system.

**Workaround:** The system is designed to handle this gracefully:
1. Form generation is **optional** and non-blocking
2. If FormFiller initialization fails, the system continues without Excel generation
3. If SaveAs fails with CJK font error, the error is logged but doesn't crash the workflow
4. Attachments are still downloaded and organized in instance folders

**Impact:** Users can still access downloaded invoices; they just won't get the auto-generated Excel form. The form can be created manually from the downloaded attachments.

**Future Fix:** Consider:
- Using a different Excel library that doesn't require system fonts
- Pre-bundling fonts with the application
- Generating PDF reports instead of Excel files
