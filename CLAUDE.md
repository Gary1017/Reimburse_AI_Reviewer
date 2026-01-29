# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AI-Driven Reimbursement Workflow System - A production-grade Go service that automates enterprise reimbursement processing by integrating Lark (Feishu) approval workflows with GPT-4 powered auditing. Generates accounting vouchers compliant with Mainland China regulations.

**Architecture**: Clean Architecture (Hexagonal/Ports & Adapters) with strict dependency inversion.

## Build & Development Commands

```bash
# Build
make build              # Build to bin/server

# Run
make run                # Run server (go run cmd/server/main.go)
go run cmd/server/main.go  # Alternative direct run

# Test
make test               # Run all tests with verbose + coverage
go test -v ./internal/application/...  # Run tests for specific package
go test -v -run TestFunctionName ./...  # Run single test

# Code Quality
make lint               # Run golangci-lint
make fmt                # Format code (go fmt + goimports)
make security           # Run gosec security scan

# Database
make migrate            # Run SQLite migrations
```

---

## Clean Architecture Structure

**CRITICAL**: This project follows Clean Architecture with strict dependency rules. **All new code MUST respect these boundaries.**

### Directory Structure

```
AI_Reimbursement/
├── cmd/server/main.go              # Entry point: config + container lifecycle ONLY
│
├── internal/
│   ├── domain/                     # ═══ DOMAIN LAYER (innermost) ═══
│   │   │                           # ** NO EXTERNAL DEPENDENCIES **
│   │   ├── entity/                 # Pure data structures
│   │   │   ├── instance.go         # ApprovalInstance entity
│   │   │   ├── attachment.go       # Attachment entity
│   │   │   ├── invoice.go          # Invoice entity
│   │   │   ├── voucher.go          # Voucher entity
│   │   │   ├── history.go          # StatusHistory entity
│   │   │   └── notification.go     # Notification entity
│   │   │
│   │   ├── workflow/               # State machine (pure logic)
│   │   │   ├── state.go            # State definitions
│   │   │   ├── trigger.go          # Trigger definitions
│   │   │   ├── machine.go          # State machine implementation
│   │   │   ├── builder.go          # Fluent builder for state machine
│   │   │   └── errors.go           # Domain-specific errors
│   │   │
│   │   └── event/                  # Domain events
│   │       ├── type.go             # Event type constants
│   │       └── event.go            # Event structures
│   │
│   ├── application/                # ═══ APPLICATION LAYER ═══
│   │   │                           # Depends ONLY on Domain
│   │   ├── port/                   # Interfaces (contracts)
│   │   │   ├── repository.go       # Repository interfaces
│   │   │   ├── external.go         # External service interfaces
│   │   │   └── storage.go          # Storage interfaces
│   │   │
│   │   ├── dispatcher/             # Event dispatcher
│   │   │   ├── dispatcher.go       # Dispatcher implementation
│   │   │   └── handler.go          # Handler registration
│   │   │
│   │   ├── workflow/               # Workflow engine
│   │   │   ├── engine.go           # Engine interface
│   │   │   ├── factory.go          # Engine factory
│   │   │   └── impl.go             # Engine implementation
│   │   │
│   │   └── service/                # Application services
│   │       ├── approval_service.go # Approval orchestration
│   │       ├── audit_service.go    # AI audit orchestration
│   │       ├── voucher_service.go  # Voucher generation
│   │       └── notification_service.go # Notification orchestration
│   │
│   ├── infrastructure/             # ═══ INFRASTRUCTURE LAYER ═══
│   │   │                           # Implements Application ports
│   │   ├── persistence/            # Database layer
│   │   │   ├── sqlite/
│   │   │   │   └── db.go           # SQLite connection & transactions
│   │   │   └── repository/         # Repository implementations
│   │   │       ├── instance_repo.go
│   │   │       ├── item_repo.go
│   │   │       ├── attachment_repo.go
│   │   │       ├── invoice_repo.go
│   │   │       ├── voucher_repo.go
│   │   │       ├── history_repo.go
│   │   │       └── notification_repo.go
│   │   │
│   │   ├── external/               # External service adapters
│   │   │   ├── lark/               # Lark SDK wrapper + adapters
│   │   │   │   ├── sdk_client.go   # Core SDK client wrapper
│   │   │   │   ├── approval_api.go # Approval API operations
│   │   │   │   ├── message_api.go  # IM messaging
│   │   │   │   ├── attachment_handler.go # Attachment downloads
│   │   │   │   ├── form_parser.go  # Form data parsing
│   │   │   │   ├── event_processor.go # Event processing
│   │   │   │   ├── event_dispatcher.go # Event dispatcher creation
│   │   │   │   ├── approval_bot_api.go # Audit notification bot
│   │   │   │   ├── client.go       # LarkClient adapter
│   │   │   │   ├── downloader.go   # LarkAttachmentDownloader adapter
│   │   │   │   └── messenger.go    # LarkMessageSender adapter
│   │   │   │
│   │   │   └── openai/             # OpenAI wrapper + adapters
│   │   │       └── auditor.go      # AIAuditor adapter
│   │   │
│   │   ├── storage/                # File storage
│   │   │   ├── file_storage.go     # FileStorage implementation
│   │   │   └── folder_manager.go   # FolderManager implementation
│   │   │
│   │   └── worker/                 # Background workers
│   │       ├── manager.go          # Worker lifecycle manager
│   │       ├── download_worker.go  # Async attachment downloads
│   │       └── invoice_worker.go   # Invoice AI processing
│   │
│   ├── container/                  # ═══ DEPENDENCY INJECTION ═══
│   │   ├── container.go            # Main container with lifecycle
│   │   ├── config.go               # Container configuration
│   │   └── providers.go            # Factory functions for components
│   │
│   ├── interfaces/                 # ═══ INTERFACE ADAPTERS ═══
│   │   ├── http/                   # HTTP server
│   │   │   ├── server.go           # HTTP server setup
│   │   │   └── handlers.go         # Request handlers
│   │   │
│   │   └── websocket/              # Lark WebSocket
│   │       └── lark_adapter.go     # WebSocket event adapter
│   │
│   └── config/                     # Configuration loading
│       └── config.go               # YAML config parsing
│
├── configs/
│   ├── config.yaml                 # Main configuration (gitignored)
│   ├── config.example.yaml         # Example configuration
│   └── prompts.yaml                # AI prompts & model parameters (temperature, max_tokens)
│
└── docs/
    ├── ARCHITECTURE.md             # Detailed architecture documentation
    ├── DEVELOPMENT_PLAN.md         # Development roadmap
    └── PROMPT_CONFIGURATION.md     # AI prompt configuration guide
```

---

## Architectural Principles

### The Dependency Rule

**Dependencies flow inward ONLY. Outer layers can depend on inner layers, but NEVER the reverse.**

```
┌─────────────────────────────────────────────────────┐
│             External World (Lark, OpenAI)           │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│          Interface Adapters (HTTP, WebSocket)       │
│                  Can depend on ↓                    │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│      Infrastructure (Repositories, External APIs)   │
│                  Can depend on ↓                    │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│  Application (Services, Workflow Engine, Ports)     │
│                  Can depend on ↓                    │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│          Domain (Entities, State Machine)           │
│          ** NO EXTERNAL DEPENDENCIES **             │
└─────────────────────────────────────────────────────┘
```

### Allowed Dependencies

✅ **ALLOWED:**
- `application/service/` → `domain/entity/`, `domain/workflow/`, `application/port/`
- `infrastructure/repository/` → `application/port/`, `domain/entity/`
- `infrastructure/external/lark/` → `application/port/`, `domain/entity/`
- `interfaces/websocket/` → `application/workflow/`, `infrastructure/external/lark/`
- `container/` → ALL layers (it assembles everything)

❌ **FORBIDDEN:**
- `domain/` → ANYTHING outside `domain/` (domain must be pure)
- `application/` → `infrastructure/` (application defines interfaces, infrastructure implements them)
- `domain/entity/` → `domain/workflow/` (entities are more fundamental than workflows)

### Violation Examples

```go
// ❌ WRONG - Domain importing infrastructure
package entity

import "github.com/garyjia/ai-reimbursement/internal/infrastructure/persistence/sqlite"

type ApprovalInstance struct {
    db *sqlite.DB  // VIOLATION: Domain depends on Infrastructure
}

// ✅ CORRECT - Domain is pure
package entity

type ApprovalInstance struct {
    ID          string
    Status      string
    TotalAmount float64
    // Pure data, no external dependencies
}
```

```go
// ❌ WRONG - Application importing infrastructure implementation
package service

import "github.com/garyjia/ai-reimbursement/internal/infrastructure/external/lark"

type ApprovalService struct {
    larkClient *lark.Client  // VIOLATION: Application depends on concrete implementation
}

// ✅ CORRECT - Application depends on port (interface)
package service

import "github.com/garyjia/ai-reimbursement/internal/application/port"

type ApprovalService struct {
    larkClient port.LarkClient  // Correct: Application depends on interface
}
```

---

## Layer Responsibilities

### Domain Layer (`internal/domain/`)

**What belongs here:**
- Pure business entities (ApprovalInstance, Invoice, etc.)
- State machine logic (approval workflow states and transitions)
- Domain events (InstanceCreated, StatusChanged)
- Business validation rules

**What NEVER belongs here:**
- Database queries
- API calls
- File I/O
- JSON marshaling
- Framework dependencies (even zap logger)

**Rule**: If it references ANYTHING outside `internal/domain/`, it's wrong.

### Application Layer (`internal/application/`)

**What belongs here:**
- **Ports** (`port/`): Interfaces defining contracts for external dependencies
- **Services** (`service/`): Orchestration logic combining domain + ports
- **Workflow Engine** (`workflow/`): Approval lifecycle orchestration
- **Event Dispatcher** (`dispatcher/`): Event pub/sub system

**What NEVER belongs here:**
- Concrete implementations (those go in `infrastructure/`)
- Direct SDK imports (Lark SDK, OpenAI SDK)
- SQL queries (those go in `infrastructure/persistence/repository/`)

**Rule**: Application defines WHAT needs to be done (interfaces), not HOW (implementations).

### Infrastructure Layer (`internal/infrastructure/`)

**What belongs here:**
- **Repository implementations** (`persistence/repository/`): SQL queries, transactions
- **External API adapters** (`external/lark/`, `external/openai/`): SDK wrappers implementing ports
- **Storage implementations** (`storage/`): File I/O, folder management
- **Workers** (`worker/`): Background goroutines with retry logic

**What NEVER belongs here:**
- Business logic (that goes in `domain/` or `application/service/`)
- Direct domain entity modification (use services)

**Rule**: Infrastructure implements ports defined by application.

### Interface Adapters (`internal/interfaces/`)

**What belongs here:**
- **HTTP handlers** (`http/`): REST API endpoints converting HTTP to service calls
- **WebSocket adapters** (`websocket/`): Lark event ingress converting to workflow engine calls

**What NEVER belongs here:**
- Business logic (delegate to `application/service/`)
- Direct database access (use `application/service/`)

**Rule**: Adapters convert external protocols to application calls.

### Container (`internal/container/`)

**What belongs here:**
- Dependency injection setup
- Component lifecycle management (Start/Close)
- Factory functions (providers.go)

**Rule**: Container is the ONLY place that wires everything together. It can import from all layers.

---

## Key Patterns

### 1. Repository Pattern

All database access goes through repository interfaces defined in `application/port/`:

```go
// application/port/repository.go
type InstanceRepository interface {
    Create(ctx context.Context, instance *entity.ApprovalInstance) error
    FindByID(ctx context.Context, id string) (*entity.ApprovalInstance, error)
    // ...
}

// infrastructure/persistence/repository/instance_repo.go
type instanceRepository struct {
    db *sql.DB
}

func (r *instanceRepository) Create(ctx context.Context, instance *entity.ApprovalInstance) error {
    // SQL implementation here
}
```

### 2. Port-Adapter Pattern

External services are accessed via interfaces:

```go
// application/port/external.go
type LarkClient interface {
    GetInstanceDetail(ctx context.Context, instanceID string) (*LarkInstanceDetail, error)
    GetApprovers(ctx context.Context, instanceID string) ([]ApproverInfo, error)
}

// infrastructure/external/lark/client.go
type Client struct {
    sdkClient *SDKClient
    logger    *zap.Logger
}

func (c *Client) GetInstanceDetail(ctx context.Context, instanceID string) (*port.LarkInstanceDetail, error) {
    // Lark SDK implementation
}
```

### 3. Event-Driven State Machine

State transitions emit domain events, which are handled asynchronously:

```go
// domain/workflow/machine.go - Pure state transition logic
func (m *StateMachine) Transition(trigger Trigger) error {
    // Pure state transition validation
}

// application/workflow/impl.go - Orchestration with side effects
func (e *engine) HandleEvent(ctx context.Context, event event.Event) error {
    // 1. Validate state transition (using domain state machine)
    // 2. Execute transition
    // 3. Publish new domain event
    // 4. Trigger side effects (async via event handlers)
}
```

### 4. Idempotency

All external event handlers check for existing state before creating:

```go
func (e *engine) HandleInstanceCreated(ctx context.Context, eventData map[string]interface{}) error {
    // Check if instance already exists (idempotency)
    existing, _ := e.instanceRepo.FindByInstanceCode(ctx, instanceCode)
    if existing != nil {
        return nil // Already processed
    }

    // Create new instance
}
```

### 5. Transaction Safety

Multi-table operations use transaction manager:

```go
err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
    // All operations in this block are in same transaction
    if err := s.instanceRepo.Update(txCtx, instance); err != nil {
        return err
    }
    if err := s.historyRepo.Create(txCtx, history); err != nil {
        return err
    }
    return nil // Commit
})
```

---

## Lark Event Subscription

**CRITICAL: This project uses WebSocket-based event subscription, NOT HTTP webhooks.**

### Implementation Pattern (CORRECT WAY)

The system subscribes to Lark approval events using WebSocket:

```go
// internal/interfaces/websocket/lark_adapter.go

import (
    "github.com/larksuite/oapi-sdk-go/v3/ws"
    "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
    infraLark "github.com/garyjia/ai-reimbursement/internal/infrastructure/external/lark"
)

// 1. Create SDK client
sdkClient := infraLark.NewSDKClient(cfg, logger)

// 2. Create event dispatcher with custom handler
eventProcessor := infraLark.NewEventProcessor(workflowEngine, logger)
dispatcher := infraLark.NewEventDispatcher(eventProcessor, logger)

// 3. Create WebSocket client with dispatcher
wsClient := ws.NewClient(
    appID,
    appSecret,
    ws.WithEventHandler(dispatcher),
)

// 4. Start WebSocket connection
wsClient.Start(ctx)
```

### Event Flow

```
Lark Platform → WebSocket → EventDispatcher → EventProcessor → WorkflowEngine → Services
```

### Why WebSocket, Not HTTP Webhooks?

1. **No intranet penetration needed** during development
2. **Automatic authentication** at connection establishment
3. **Plaintext event data** (no decryption needed)
4. **Real-time events** with persistent connection
5. **Only requires public network access**

### DO NOT

❌ **DO NOT create HTTP webhook endpoints** like `/webhook/event`
❌ **DO NOT use verification tokens or encrypt keys** (not needed for WebSocket)
❌ **DO NOT use `dispatcher.Do()` method** (that's for HTTP webhooks)
❌ **DO NOT read webhook documentation** when implementing event subscription

### Implementation Locations

- **WebSocket Client Setup**: `internal/interfaces/websocket/lark_adapter.go`
- **SDK Client Wrapper**: `internal/infrastructure/external/lark/sdk_client.go`
- **Event Dispatcher**: `internal/infrastructure/external/lark/event_dispatcher.go`
- **Event Processor**: `internal/infrastructure/external/lark/event_processor.go`
- **Workflow Engine**: `internal/application/workflow/impl.go`
- **Container Wiring**: `internal/container/providers.go`

---

## Configuration

Configuration is loaded from `configs/config.yaml` with environment variable overrides:

**Required environment variables:**
- `LARK_APP_ID` - Lark application ID
- `LARK_APP_SECRET` - Lark application secret
- `LARK_APPROVAL_CODE` - Approval workflow code
- `OPENAI_API_KEY` - OpenAI API key
- `ACCOUNTANT_EMAIL` - Email for voucher notifications
- `COMPANY_NAME` - Company name for vouchers
- `COMPANY_TAX_ID` - Company tax ID for vouchers

**Configuration loading:**
```go
// internal/config/config.go
cfg, err := config.Load("configs/config.yaml")
```

---

## AI Prompt Configuration

**All AI prompts and model parameters are externalized to `configs/prompts.yaml`.**

This allows you to tune AI behavior without changing code:

```yaml
# configs/prompts.yaml
policy_audit:
  temperature: 0.3      # Controls randomness (0.0-2.0)
  max_tokens: 1000      # Maximum response length
  system: |
    System prompt defining AI role
  user_template: |
    User prompt with {{.Variables}}

price_audit:
  temperature: 0.3
  max_tokens: 1000
  # ... prompts

invoice_extraction:
  temperature: 0.1      # Lower = more accurate
  max_tokens: 4096      # Higher for detailed extraction
  # ... prompts
```

**To update prompts or parameters:**
1. Edit `configs/prompts.yaml`
2. Restart the server

**See [docs/PROMPT_CONFIGURATION.md](docs/PROMPT_CONFIGURATION.md) for:**
- Available template variables
- Temperature tuning guidelines
- Optimization tips
- Troubleshooting

---

## Testing Guidelines

### Unit Tests

- **Domain**: Test state machine transitions, entity validation
- **Application**: Test service orchestration with mock ports
- **Infrastructure**: Test repository queries against real SQLite (use transactions + rollback)

### Test Locations

```
internal/domain/workflow/machine_test.go        # State machine tests
internal/application/service/*_test.go          # Service tests with mocks
internal/infrastructure/persistence/repository/*_test.go  # Repository integration tests
```

### Running Tests

```bash
# All tests
make test

# Specific package
go test -v ./internal/application/service/

# Single test
go test -v -run TestApprovalService_CreateInstance ./internal/application/service/

# With coverage
go test -v -cover ./...
```

---

## Common Tasks

### Adding a New Entity

1. **Define entity**: `internal/domain/entity/new_entity.go`
2. **Define repository interface**: Add to `internal/application/port/repository.go`
3. **Implement repository**: `internal/infrastructure/persistence/repository/new_entity_repo.go`
4. **Add to container**: Update `internal/container/providers.go` to instantiate repository
5. **Use in service**: Inject repository into `internal/application/service/`

### Adding a New External Service

1. **Define port interface**: Add to `internal/application/port/external.go`
2. **Implement adapter**: `internal/infrastructure/external/service_name/adapter.go`
3. **Add to container**: Update `internal/container/providers.go` to instantiate adapter
4. **Use in service**: Inject port interface into `internal/application/service/`

### Adding a New State

1. **Define state constant**: `internal/domain/workflow/state.go`
2. **Define trigger**: `internal/domain/workflow/trigger.go`
3. **Add transition rule**: `internal/domain/workflow/builder.go`
4. **Handle in workflow engine**: `internal/application/workflow/impl.go`
5. **Add domain event**: `internal/domain/event/type.go` and `internal/domain/event/event.go`

---

## Known Issues & Workarounds

### CJK Font Error in Excel Generation

**Error**: `syntax error: cannot find builtin CJK font`

**Cause**: The excelize library requires CJK fonts for Chinese characters, which may not be available in all environments.

**Workaround**: The system gracefully degrades:
1. Form generation is optional and non-blocking
2. If FormFiller fails, system continues without Excel generation
3. Attachments are still downloaded and organized
4. Error is logged but doesn't crash workflow

**Impact**: Users can access downloaded invoices; Excel form generation may fail.

**Future Fix**:
- Bundle fonts with application
- Use alternative Excel library
- Generate PDF reports instead

---

## Code Review Checklist

Before committing code, verify:

- [ ] **Dependency Rule**: Does new code respect layer boundaries?
- [ ] **Interface Usage**: Does application layer use ports (interfaces), not concrete implementations?
- [ ] **Domain Purity**: Does domain layer have zero external dependencies?
- [ ] **Transaction Safety**: Do multi-table operations use `txManager.WithTransaction()`?
- [ ] **Idempotency**: Do event handlers check for existing state?
- [ ] **Error Handling**: Are errors wrapped with context (`fmt.Errorf("context: %w", err)`)?
- [ ] **Logging**: Are operations logged with structured fields (zap)?
- [ ] **Tests**: Are there unit tests for new business logic?

---

## Reference Documentation

- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**: Complete architectural design with diagrams
- **[docs/DEVELOPMENT_PLAN.md](docs/DEVELOPMENT_PLAN.md)**: Development roadmap and phase breakdown
- **[docs/REFACTORING_PLAN.md](docs/REFACTORING_PLAN.md)**: Clean Architecture migration plan

---

**When in doubt, follow the Dependency Rule: Dependencies flow inward only.**
