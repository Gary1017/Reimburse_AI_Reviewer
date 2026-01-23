# Clean Architecture Refactoring Plan for AI_Reimbursement

## 1. Problem Understanding

The current AI_Reimbursement codebase has evolved organically and exhibits several architectural issues that hinder maintainability, testability, and extensibility:

**Current Issues Identified:**

1. **Mixed Responsibilities in main.go**: The entry point handles configuration, infrastructure setup, service wiring, worker creation, and HTTP routing - violating single responsibility principle.

2. **Implicit State Machine**: Status transitions are embedded in `status_tracker.go` as a map of valid transitions, but there is no explicit state machine abstraction.

3. **Scattered Interface Definitions**: Interfaces are defined where they are implemented (e.g., `voucher/interfaces.go`), not where they are consumed - violating the demand-driven interface principle.

4. **Coupled Infrastructure and Application Logic**: The `services/container.go` mixes service creation with infrastructure concerns. The `workflow/engine.go` couples event handling with business logic.

5. **No Central Event Dispatcher**: Events from Lark are processed directly by `event_processor.go` which calls workflow handlers. There is no unified dispatcher pattern.

6. **Container Lacks Lifecycle Management**: The current container is a struct holder, not a lifecycle manager with proper Start/Close semantics.

---

## 2. Architecture Overview

The target architecture follows Clean Architecture principles with four distinct layers:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              main.go                                     │
│                         container.Start/Close                            │
└────────────────────────────────┬────────────────────────────────────────┘
                                 │
┌────────────────────────────────▼────────────────────────────────────────┐
│                            Container                                     │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                    Interface Adapters                             │   │
│  │         HTTP Server  │  WebSocket Client (Lark Events)           │   │
│  └──────────────────────────────┬───────────────────────────────────┘   │
│                                 │                                        │
│  ┌──────────────────────────────▼───────────────────────────────────┐   │
│  │                     Application Layer                             │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌──────────────────────────┐  │   │
│  │  │ Dispatcher  │◄─┤   Workflow  │  │       Services           │  │   │
│  │  │             │  │   Engine    │  │ Approval │ Audit         │  │   │
│  │  │  ↓ Events   │  │     ↓↑      │  │ Voucher  │ Notification  │  │   │
│  │  │  Handlers ──┼──┼─────┘       │  └──────────┼───────────────┘  │   │
│  │  └─────────────┘  └─────────────┘             │                  │   │
│  │                                               ▼                  │   │
│  │  ┌────────────────────────────────────────────────────────────┐  │   │
│  │  │                    Ports (Interfaces)                       │  │   │
│  │  │  Repository Ports │ External Ports │ Storage Ports         │  │   │
│  │  └─────────────────────────────┬──────────────────────────────┘  │   │
│  └────────────────────────────────┼─────────────────────────────────┘   │
│                                   │                                      │
│  ┌────────────────────────────────▼─────────────────────────────────┐   │
│  │                      Domain Layer                                 │   │
│  │    Entities  │  State Machine  │  Domain Events                  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                   ▲                                      │
│  ┌────────────────────────────────┼─────────────────────────────────┐   │
│  │                   Infrastructure Layer                            │   │
│  │  ┌────────────────────┐  ┌────────────────────────────────────┐  │   │
│  │  │    Data Infra      │  │      Functionality Infra           │  │   │
│  │  │  SQLite + Repos    │  │  Lark │ OpenAI │ Email │ Storage  │  │   │
│  │  └────────────────────┘  └────────────────────────────────────┘  │   │
│  │  ┌────────────────────────────────────────────────────────────┐  │   │
│  │  │                      Workers                                │  │   │
│  │  │         DownloadWorker  │  InvoiceWorker                   │  │   │
│  │  └────────────────────────────────────────────────────────────┘  │   │
└─────────────────────────────────────────────────────────────────────────┘
```

**Dependency Rule**: Dependencies point inward. Domain has no dependencies. Application depends on Domain. Infrastructure depends on Application (implements ports). Container assembles everything.

---

## 3. Architectural Requirements Table

| traceability_id | description | affected_components | constraints / invariants |
|-----------------|-------------|---------------------|-------------------------|
| **ARCH-100** | Create domain/entity package with pure data structures (no business logic) | `internal/domain/entity/` | Entities must have no external dependencies; Only data + validation methods |
| **ARCH-101** | Create explicit state machine for workflow states | `internal/domain/workflow/` | State machine must be immutable; All transitions must be declarative |
| **ARCH-102** | Create domain events for workflow communication | `internal/domain/event/` | Events are immutable value objects; Must include correlation ID |
| **ARCH-110** | Create application/port interfaces (demand-driven) | `internal/application/port/` | Interfaces defined by consumers, not implementers; No implementation details in ports |
| **ARCH-111** | Implement dispatcher pattern for event routing | `internal/application/dispatcher/` | Single point of event distribution; Must support sync and async dispatch |
| **ARCH-112** | Create WorkflowEngine using StateMachine | `internal/application/workflow/` | Engine orchestrates services based on state; State changes are transactional |
| **ARCH-113** | Refactor application services with clear interfaces | `internal/application/service/` | Services are stateless; Depend only on ports, not concrete implementations |
| **ARCH-120** | Migrate repository implementations to infrastructure | `internal/infrastructure/persistence/repository/` | Must implement port interfaces; Transaction support required |
| **ARCH-121** | Migrate Lark client to infrastructure layer | `internal/infrastructure/external/lark/` | Implement port.LarkClient interface; Handle rate limiting and retries |
| **ARCH-122** | Migrate OpenAI client to infrastructure layer | `internal/infrastructure/external/openai/` | Implement port.OpenAIAuditor interface; Handle API errors gracefully |
| **ARCH-123** | Migrate storage to infrastructure layer | `internal/infrastructure/storage/` | Implement port.FileStorage and port.FolderManager interfaces |
| **ARCH-124** | Migrate workers to infrastructure layer | `internal/infrastructure/worker/` | Workers use ports, not concrete types; Must support graceful shutdown |
| **ARCH-130** | Create Container with full lifecycle management | `internal/container/` | Start() initializes in order; Close() tears down in reverse order |
| **ARCH-131** | Create dependency providers for container | `internal/container/providers.go` | Provider functions return interfaces; Lazy initialization supported |
| **ARCH-132** | Simplify main.go to container.Start/Close only | `cmd/server/main.go` | main.go has no business logic; Only config load + container lifecycle |
| **ARCH-140** | Create HTTP server interface adapter | `internal/interfaces/http/` | Adapter translates HTTP to application calls; Decoupled from business logic |
| **ARCH-141** | Create WebSocket adapter for Lark events | `internal/interfaces/websocket/` | Adapter translates Lark events to domain events; Passes to dispatcher |
| **ARCH-150** | Remove deprecated code paths | All packages | No orphaned code; All imports updated |
| **ARCH-151** | Update documentation and imports | `docs/`, all `.go` files | README updated; ARCHITECTURE.md reflects new structure |
| **ARCH-152** | Integration testing for refactored architecture | `tests/integration/` | All existing functionality preserved; No regression |

---

## 4. Interface and Class Definitions

### 4.1 Domain Layer

**File: `internal/domain/entity/instance.go`**
```go
package entity

import "time"

// ApprovalInstance represents a reimbursement approval request
type ApprovalInstance struct {
    ID              int64
    LarkInstanceID  string
    Status          string
    ApplicantUserID string
    Department      string
    SubmissionTime  time.Time
    ApprovalTime    *time.Time
    FormData        string
    AIAuditResult   string
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

**File: `internal/domain/workflow/state.go`**
```go
package workflow

// State represents a workflow state
type State string

const (
    StateCreated           State = "CREATED"
    StatePending           State = "PENDING"
    StateAIAuditing        State = "AI_AUDITING"
    StateAIAudited         State = "AI_AUDITED"
    StateInReview          State = "IN_REVIEW"
    StateAutoApproved      State = "AUTO_APPROVED"
    StateApproved          State = "APPROVED"
    StateRejected          State = "REJECTED"
    StateVoucherGenerating State = "VOUCHER_GENERATING"
    StateCompleted         State = "COMPLETED"
)

// Trigger represents an event that causes state transition
type Trigger string

const (
    TriggerSubmit          Trigger = "SUBMIT"
    TriggerStartAudit      Trigger = "START_AUDIT"
    TriggerCompleteAudit   Trigger = "COMPLETE_AUDIT"
    TriggerRequestReview   Trigger = "REQUEST_REVIEW"
    TriggerAutoApprove     Trigger = "AUTO_APPROVE"
    TriggerApprove         Trigger = "APPROVE"
    TriggerReject          Trigger = "REJECT"
    TriggerStartVoucher    Trigger = "START_VOUCHER"
    TriggerCompleteVoucher Trigger = "COMPLETE_VOUCHER"
)

// IsTerminal returns true if state is terminal (no further transitions)
func (s State) IsTerminal() bool {
    return s == StateRejected || s == StateCompleted
}
```

**File: `internal/domain/workflow/machine.go`**
```go
package workflow

import "context"

// TransitionRule defines a valid state transition
type TransitionRule struct {
    FromState State
    Trigger   Trigger
    ToState   State
    Guard     func(ctx context.Context) bool // Optional condition check
}

// StateMachine defines workflow state management
type StateMachine interface {
    // State returns current state
    State() State

    // CanFire checks if trigger is valid from current state
    CanFire(trigger Trigger) bool

    // Fire executes state transition
    Fire(ctx context.Context, trigger Trigger) error

    // PermittedTriggers returns valid triggers from current state
    PermittedTriggers() []Trigger
}

// StateMachineBuilder constructs a state machine with rules
type StateMachineBuilder interface {
    Configure(state State) StateConfiguration
    Build(initialState State) StateMachine
}

// StateConfiguration configures transitions for a state
type StateConfiguration interface {
    Permit(trigger Trigger, toState State) StateConfiguration
    PermitIf(trigger Trigger, toState State, guard func(ctx context.Context) bool) StateConfiguration
}
```

**File: `internal/domain/event/event.go`**
```go
package event

import "time"

// Type identifies the type of domain event
type Type string

const (
    TypeInstanceCreated     Type = "instance.created"
    TypeInstanceApproved    Type = "instance.approved"
    TypeInstanceRejected    Type = "instance.rejected"
    TypeStatusChanged       Type = "instance.status_changed"
    TypeAttachmentReady     Type = "attachment.ready"
    TypeAuditCompleted      Type = "audit.completed"
    TypeVoucherGenerated    Type = "voucher.generated"
)

// Event represents a domain event
type Event struct {
    ID             string
    Type           Type
    InstanceID     int64
    LarkInstanceID string
    Payload        map[string]interface{}
    Timestamp      time.Time
    CorrelationID  string
}

// NewEvent creates a new domain event
func NewEvent(eventType Type, instanceID int64, larkInstanceID string, payload map[string]interface{}) *Event
```

### 4.2 Application Layer

**File: `internal/application/port/repository.go`**
```go
package port

import (
    "context"
    "time"
    "github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// InstanceRepository defines persistence operations for ApprovalInstance
type InstanceRepository interface {
    Create(ctx context.Context, instance *entity.ApprovalInstance) error
    GetByID(ctx context.Context, id int64) (*entity.ApprovalInstance, error)
    GetByLarkInstanceID(ctx context.Context, larkID string) (*entity.ApprovalInstance, error)
    UpdateStatus(ctx context.Context, id int64, status string) error
    SetApprovalTime(ctx context.Context, id int64, t time.Time) error
    List(ctx context.Context, limit, offset int) ([]*entity.ApprovalInstance, error)
}

// ItemRepository defines persistence operations for ReimbursementItem
type ItemRepository interface {
    Create(ctx context.Context, item *entity.ReimbursementItem) error
    GetByID(ctx context.Context, id int64) (*entity.ReimbursementItem, error)
    GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error)
    Update(ctx context.Context, item *entity.ReimbursementItem) error
}

// AttachmentRepository defines persistence operations for Attachment
type AttachmentRepository interface {
    Create(ctx context.Context, att *entity.Attachment) error
    GetByID(ctx context.Context, id int64) (*entity.Attachment, error)
    GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Attachment, error)
    GetPending(ctx context.Context, limit int) ([]*entity.Attachment, error)
    MarkCompleted(ctx context.Context, id int64, filePath string, fileSize int64) error
    UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error
}

// HistoryRepository defines persistence operations for ApprovalHistory
type HistoryRepository interface {
    Create(ctx context.Context, history *entity.ApprovalHistory) error
    GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ApprovalHistory, error)
}

// InvoiceRepository defines persistence operations for Invoice
type InvoiceRepository interface {
    Create(ctx context.Context, invoice *entity.Invoice) error
    GetByID(ctx context.Context, id int64) (*entity.Invoice, error)
    GetByAttachmentID(ctx context.Context, attachmentID int64) (*entity.Invoice, error)
    GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Invoice, error)
    Update(ctx context.Context, invoice *entity.Invoice) error
}

// TransactionManager handles database transactions
type TransactionManager interface {
    WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
```

**File: `internal/application/port/external.go`**
```go
package port

import (
    "context"
    "github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// LarkInstanceDetail represents details fetched from Lark API
type LarkInstanceDetail struct {
    InstanceCode    string
    ApprovalCode    string
    UserID          string
    Status          string
    FormData        string
}

// ApproverInfo represents an approver from Lark
type ApproverInfo struct {
    UserID string
    OpenID string
    Name   string
}

// LarkClient defines Lark API operations
type LarkClient interface {
    GetInstanceDetail(ctx context.Context, instanceID string) (*LarkInstanceDetail, error)
    GetApprovers(ctx context.Context, instanceID string) ([]ApproverInfo, error)
}

// LarkAttachmentDownloader defines attachment download operations
type LarkAttachmentDownloader interface {
    Download(ctx context.Context, url string) ([]byte, int64, error)
    DownloadWithRetry(ctx context.Context, url string, maxAttempts int) ([]byte, int64, error)
}

// LarkMessageSender defines message sending operations
type LarkMessageSender interface {
    SendMessage(ctx context.Context, openID string, content string) error
    SendAuditNotification(ctx context.Context, openID string, result *AuditNotificationPayload) error
}

// AuditNotificationPayload for notification
type AuditNotificationPayload struct {
    InstanceID string
    Status     string
    Message    string
}

// PolicyAuditResult represents policy validation result
type PolicyAuditResult struct {
    Compliant   bool
    Violations  []string
    Confidence  float64
    Reasoning   string
}

// PriceAuditResult represents price benchmarking result
type PriceAuditResult struct {
    Reasonable          bool
    DeviationPercentage float64
    MarketPrice         float64
    Confidence          float64
    Reasoning           string
}

// InvoiceData represents extracted invoice data
type InvoiceData struct {
    InvoiceCode   string
    InvoiceNumber string
    Amount        float64
    TaxAmount     float64
    Date          string
    Seller        string
    Buyer         string
}

// AIAuditor defines AI auditing operations
type AIAuditor interface {
    AuditPolicy(ctx context.Context, item *entity.ReimbursementItem) (*PolicyAuditResult, error)
    AuditPrice(ctx context.Context, item *entity.ReimbursementItem) (*PriceAuditResult, error)
    ExtractInvoice(ctx context.Context, imageData []byte) (*InvoiceData, error)
}
```

**File: `internal/application/port/storage.go`**
```go
package port

import "context"

// FileStorage defines file storage operations
type FileStorage interface {
    Save(ctx context.Context, path string, content []byte) error
    Read(ctx context.Context, path string) ([]byte, error)
    Exists(ctx context.Context, path string) bool
    Delete(ctx context.Context, path string) error
}

// FolderManager defines folder management operations
type FolderManager interface {
    CreateFolder(ctx context.Context, name string) (string, error)
    GetPath(name string) string
    Exists(name string) bool
    Delete(ctx context.Context, name string) error
    SanitizeName(name string) string
}
```

**File: `internal/application/dispatcher/dispatcher.go`**
```go
package dispatcher

import (
    "context"
    "github.com/garyjia/ai-reimbursement/internal/domain/event"
)

// Handler processes domain events
type Handler func(ctx context.Context, evt *event.Event) error

// Dispatcher routes events to registered handlers
type Dispatcher interface {
    // Subscribe registers a handler for an event type
    Subscribe(eventType event.Type, handler Handler)

    // Unsubscribe removes a handler for an event type
    Unsubscribe(eventType event.Type, handler Handler)

    // Dispatch sends event to all registered handlers synchronously
    Dispatch(ctx context.Context, evt *event.Event) error

    // DispatchAsync sends event to handlers asynchronously
    DispatchAsync(ctx context.Context, evt *event.Event)
}

// WorkflowDispatcher extends Dispatcher with workflow coordination
type WorkflowDispatcher interface {
    Dispatcher

    // SetWorkflowEngine links dispatcher to workflow engine
    SetWorkflowEngine(engine WorkflowEngine)
}
```

**File: `internal/application/workflow/engine.go`**
```go
package workflow

import (
    "context"
    "github.com/garyjia/ai-reimbursement/internal/domain/event"
    domainwf "github.com/garyjia/ai-reimbursement/internal/domain/workflow"
)

// WorkflowEngine orchestrates the approval workflow
type WorkflowEngine interface {
    // HandleEvent processes a domain event through the workflow
    HandleEvent(ctx context.Context, evt *event.Event) error

    // GetStateMachine returns state machine for an instance
    GetStateMachine(ctx context.Context, instanceID int64) (domainwf.StateMachine, error)

    // TransitionState triggers a state transition for an instance
    TransitionState(ctx context.Context, instanceID int64, trigger domainwf.Trigger) error
}

// WorkflowHandler defines handlers for specific workflow events
type WorkflowHandler interface {
    HandleInstanceCreated(ctx context.Context, larkInstanceID string, payload map[string]interface{}) error
    HandleInstanceApproved(ctx context.Context, larkInstanceID string, payload map[string]interface{}) error
    HandleInstanceRejected(ctx context.Context, larkInstanceID string, payload map[string]interface{}) error
    HandleStatusChanged(ctx context.Context, larkInstanceID string, status string, payload map[string]interface{}) error
}
```

**File: `internal/application/service/approval_service.go`**
```go
package service

import (
    "context"
    "github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// ApprovalService manages approval instances
type ApprovalService interface {
    // CreateInstance creates a new approval instance from Lark event
    CreateInstance(ctx context.Context, larkInstanceID string, formData map[string]interface{}) (*entity.ApprovalInstance, error)

    // GetInstance retrieves an instance by ID
    GetInstance(ctx context.Context, id int64) (*entity.ApprovalInstance, error)

    // GetInstanceByLarkID retrieves an instance by Lark instance ID
    GetInstanceByLarkID(ctx context.Context, larkInstanceID string) (*entity.ApprovalInstance, error)

    // UpdateStatus updates instance status with audit trail
    UpdateStatus(ctx context.Context, id int64, status string, actionData interface{}) error

    // SetApprovalTime marks instance as approved with timestamp
    SetApprovalTime(ctx context.Context, id int64) error
}
```

**File: `internal/application/service/audit_service.go`**
```go
package service

import (
    "context"
    "github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// AuditResult represents combined audit result
type AuditResult struct {
    PolicyResult *PolicyAuditResult
    PriceResult  *PriceAuditResult
    OverallPass  bool
    Confidence   float64
    Reasoning    string
}

// PolicyAuditResult from port package
type PolicyAuditResult struct {
    Compliant  bool
    Violations []string
    Confidence float64
    Reasoning  string
}

// PriceAuditResult from port package
type PriceAuditResult struct {
    Reasonable          bool
    DeviationPercentage float64
    MarketPrice         float64
    Confidence          float64
    Reasoning           string
}

// AuditService manages AI auditing
type AuditService interface {
    // AuditInstance performs full audit on an instance
    AuditInstance(ctx context.Context, instanceID int64) (*AuditResult, error)

    // AuditItem audits a single reimbursement item
    AuditItem(ctx context.Context, item *entity.ReimbursementItem) (*AuditResult, error)

    // ExtractInvoice extracts data from invoice image
    ExtractInvoice(ctx context.Context, imageData []byte) (*InvoiceData, error)
}

// InvoiceData from port package
type InvoiceData struct {
    InvoiceCode   string
    InvoiceNumber string
    Amount        float64
    TaxAmount     float64
    Date          string
    Seller        string
    Buyer         string
}
```

**File: `internal/application/service/voucher_service.go`**
```go
package service

import (
    "context"
)

// VoucherResult represents the result of voucher generation
type VoucherResult struct {
    Success         bool
    FolderPath      string
    VoucherFilePath string
    AttachmentPaths []string
    IncompleteCount int
    Error           error
}

// VoucherService manages voucher generation
type VoucherService interface {
    // GenerateVoucher creates voucher package for an instance
    GenerateVoucher(ctx context.Context, instanceID int64) (*VoucherResult, error)

    // GenerateVoucherAsync generates voucher in background
    GenerateVoucherAsync(ctx context.Context, instanceID int64)

    // IsInstanceReady checks if all attachments are downloaded
    IsInstanceReady(ctx context.Context, instanceID int64) (bool, error)
}
```

**File: `internal/application/service/notification_service.go`**
```go
package service

import (
    "context"
)

// NotificationService manages notifications
type NotificationService interface {
    // NotifyApplicant sends notification to applicant
    NotifyApplicant(ctx context.Context, instanceID int64, message string) error

    // NotifyAuditResult sends audit result notification
    NotifyAuditResult(ctx context.Context, instanceID int64, result *AuditResult) error

    // NotifyVoucherReady sends voucher ready notification
    NotifyVoucherReady(ctx context.Context, instanceID int64, voucherPath string) error
}
```

### 4.3 Container

**File: `internal/container/container.go`**
```go
package container

import (
    "context"
    "github.com/garyjia/ai-reimbursement/internal/application/dispatcher"
    "github.com/garyjia/ai-reimbursement/internal/application/port"
    "github.com/garyjia/ai-reimbursement/internal/application/service"
    "github.com/garyjia/ai-reimbursement/internal/application/workflow"
    "go.uber.org/zap"
)

// Container manages all application dependencies and lifecycle
type Container struct {
    config *Config
    logger *zap.Logger

    // Infrastructure - Data
    db           port.TransactionManager
    repositories *RepositoryBundle

    // Infrastructure - External
    larkClient     port.LarkClient
    larkDownloader port.LarkAttachmentDownloader
    larkMessenger  port.LarkMessageSender
    aiAuditor      port.AIAuditor

    // Infrastructure - Storage
    fileStorage   port.FileStorage
    folderManager port.FolderManager

    // Application
    dispatcher dispatcher.Dispatcher
    workflow   workflow.WorkflowEngine
    services   *ServiceBundle

    // Workers
    workers *WorkerManager

    // Interface Adapters
    httpServer *HTTPServer
    wsClient   *WebSocketClient

    // Lifecycle
    ctx    context.Context
    cancel context.CancelFunc
}

// RepositoryBundle groups all repositories
type RepositoryBundle struct {
    Instance   port.InstanceRepository
    Item       port.ItemRepository
    Attachment port.AttachmentRepository
    History    port.HistoryRepository
    Invoice    port.InvoiceRepository
}

// ServiceBundle groups all application services
type ServiceBundle struct {
    Approval     service.ApprovalService
    Audit        service.AuditService
    Voucher      service.VoucherService
    Notification service.NotificationService
}

// NewContainer creates a new container from configuration
func NewContainer(cfg *Config, logger *zap.Logger) (*Container, error)

// Start initializes all components and begins processing
func (c *Container) Start(ctx context.Context) error

// Close gracefully shuts down all components in reverse order
func (c *Container) Close() error

// Ready returns true when all components are initialized
func (c *Container) Ready() bool

// Health returns health status of all components
func (c *Container) Health() *HealthStatus
```

### 4.4 Simplified main.go

**File: `cmd/server/main.go`**
```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/garyjia/ai-reimbursement/internal/config"
    "github.com/garyjia/ai-reimbursement/internal/container"
    "github.com/garyjia/ai-reimbursement/pkg/utils"
    "go.uber.org/zap"
)

func main() {
    // Load configuration
    cfg, err := config.Load("configs/config.yaml")
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }

    // Initialize logger
    logger, err := utils.NewLogger(cfg.Logger)
    if err != nil {
        log.Fatalf("Failed to initialize logger: %v", err)
    }
    defer logger.Sync()

    // Create container
    c, err := container.NewContainer(cfg, logger)
    if err != nil {
        logger.Fatal("Failed to create container", zap.Error(err))
    }

    // Start container
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := c.Start(ctx); err != nil {
        logger.Fatal("Failed to start container", zap.Error(err))
    }

    logger.Info("Application started successfully")

    // Wait for shutdown signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down...")

    // Close container
    if err := c.Close(); err != nil {
        logger.Error("Container shutdown error", zap.Error(err))
    }

    logger.Info("Application stopped")
}
```

---

## 5. Risks and Assumptions

### 5.1 Risks

| Risk | Severity | Probability | Mitigation |
|------|----------|-------------|------------|
| **Breaking Changes** | HIGH | HIGH | Incremental migration with backward compatibility aliases; keep old paths working during transition |
| **Testing Regression** | HIGH | MEDIUM | Run existing tests continuously; create interface-based test doubles; parallel test migration |
| **Circular Dependencies** | MEDIUM | MEDIUM | Strict layer dependency rules; all interfaces in application/port; use dependency injection |
| **Performance Degradation** | LOW | LOW | Benchmark before/after; optimize hot paths; minimize abstraction overhead |
| **WebSocket Integration** | MEDIUM | MEDIUM | Create adapter layer; maintain Lark SDK compatibility; extensive integration testing |
| **Feature Freeze Violation** | MEDIUM | MEDIUM | Clear communication; separate refactoring branch; merge conflicts resolution strategy |

### 5.2 Assumptions

1. **Stable External APIs**: Lark and OpenAI API contracts remain unchanged during refactoring
2. **Database Schema Unchanged**: Only code restructuring; no database migrations required
3. **Full Test Coverage Exists**: Current functionality can be validated through existing tests
4. **Clean Architecture Understanding**: Development team familiar with Clean Architecture principles
5. **Feature Freeze**: No new features added during refactoring period

### 5.3 Success Criteria

- [ ] main.go contains only configuration loading, container creation, Start, and Close
- [ ] No business logic in infrastructure layer
- [ ] All interfaces defined in `application/port/`
- [ ] State machine is explicit, declarative, and independently testable
- [ ] Dispatcher handles all event routing through a single entry point
- [ ] All existing tests pass without modification (or with minimal changes)
- [ ] No import cycles between layers
- [ ] Container manages complete lifecycle with proper shutdown ordering

---

## 6. Phased Implementation Plan

### Phase 1: Foundation (Domain Layer)

| ID | Task | Dependencies | Deliverables |
|----|------|--------------|--------------|
| ARCH-100 | Create domain/entity package | None | Pure entity structs without business logic |
| ARCH-101 | Create domain/workflow state machine | ARCH-100 | State enum, Trigger enum, StateMachine interface |
| ARCH-102 | Create domain/event types | ARCH-100 | Event struct, EventType constants |

### Phase 2: Application Layer

| ID | Task | Dependencies | Deliverables |
|----|------|--------------|--------------|
| ARCH-110 | Create application/port interfaces | ARCH-100 | Repository, External, Storage port interfaces |
| ARCH-111 | Create application/dispatcher | ARCH-102 | Dispatcher interface and implementation |
| ARCH-112 | Create application/workflow engine | ARCH-101, ARCH-111 | WorkflowEngine using StateMachine |
| ARCH-113 | Create application services | ARCH-110 | ApprovalService, AuditService, VoucherService, NotificationService |

### Phase 3: Infrastructure Layer

| ID | Task | Dependencies | Deliverables |
|----|------|--------------|--------------|
| ARCH-120 | Migrate repositories | ARCH-110 | Repository implementations in infrastructure |
| ARCH-121 | Migrate Lark client | ARCH-110 | LarkClient implementation |
| ARCH-122 | Migrate OpenAI client | ARCH-110 | AIAuditor implementation |
| ARCH-123 | Migrate storage | ARCH-110 | FileStorage, FolderManager implementations |
| ARCH-124 | Migrate workers | ARCH-120, ARCH-121, ARCH-123 | Workers using port interfaces |

### Phase 4: Container & Lifecycle

| ID | Task | Dependencies | Deliverables |
|----|------|--------------|--------------|
| ARCH-130 | Create Container | All ARCH-11x, ARCH-12x | Container with Start/Close |
| ARCH-131 | Create dependency providers | ARCH-130 | Provider functions |
| ARCH-132 | Simplify main.go | ARCH-130 | main.go with only container lifecycle |

### Phase 5: Interface Adapters

| ID | Task | Dependencies | Deliverables |
|----|------|--------------|--------------|
| ARCH-140 | Create HTTP server adapter | ARCH-130 | HTTP adapter translating to application calls |
| ARCH-141 | Create WebSocket adapter | ARCH-111, ARCH-130 | Lark event adapter to dispatcher |

### Phase 6: Cleanup & Validation

| ID | Task | Dependencies | Deliverables |
|----|------|--------------|--------------|
| ARCH-150 | Remove deprecated code | All previous | Clean codebase |
| ARCH-151 | Update documentation | ARCH-150 | Updated ARCHITECTURE.md, README |
| ARCH-152 | Integration testing | ARCH-150 | Passing integration tests |

---

## 7. Target Package Structure

```
AI_Reimbursement/
├── cmd/
│   └── server/
│       └── main.go                          # Simplified: config + container.Start/Close
├── internal/
│   ├── domain/                              # Domain Layer (pure business)
│   │   ├── entity/
│   │   │   ├── instance.go
│   │   │   ├── item.go
│   │   │   ├── attachment.go
│   │   │   ├── invoice.go
│   │   │   ├── voucher.go
│   │   │   └── notification.go
│   │   ├── workflow/
│   │   │   ├── state.go                     # State, Trigger enums
│   │   │   ├── machine.go                   # StateMachine interface
│   │   │   └── builder.go                   # StateMachineBuilder
│   │   └── event/
│   │       └── event.go                     # Domain event types
│   ├── application/                         # Application Layer (use cases)
│   │   ├── port/
│   │   │   ├── repository.go                # Repository interfaces
│   │   │   ├── external.go                  # External service interfaces
│   │   │   └── storage.go                   # Storage interfaces
│   │   ├── dispatcher/
│   │   │   ├── dispatcher.go                # Dispatcher interface
│   │   │   └── impl.go                      # Dispatcher implementation
│   │   ├── workflow/
│   │   │   ├── engine.go                    # WorkflowEngine interface
│   │   │   └── impl.go                      # WorkflowEngine implementation
│   │   └── service/
│   │       ├── approval_service.go
│   │       ├── audit_service.go
│   │       ├── voucher_service.go
│   │       └── notification_service.go
│   ├── infrastructure/                      # Infrastructure Layer
│   │   ├── persistence/
│   │   │   ├── sqlite/
│   │   │   │   ├── db.go
│   │   │   │   └── migrations.go
│   │   │   └── repository/
│   │   │       ├── instance_repo.go
│   │   │       ├── item_repo.go
│   │   │       ├── attachment_repo.go
│   │   │       └── ...
│   │   ├── external/
│   │   │   ├── lark/
│   │   │   │   ├── client.go
│   │   │   │   ├── approval_api.go
│   │   │   │   ├── downloader.go
│   │   │   │   └── messenger.go
│   │   │   └── openai/
│   │   │       ├── client.go
│   │   │       └── auditor.go
│   │   ├── storage/
│   │   │   ├── file_storage.go
│   │   │   └── folder_manager.go
│   │   └── worker/
│   │       ├── download_worker.go
│   │       ├── invoice_worker.go
│   │       └── manager.go
│   ├── container/                           # Dependency Injection
│   │   ├── container.go
│   │   ├── providers.go
│   │   └── config.go
│   ├── interfaces/                          # Interface Adapters
│   │   ├── http/
│   │   │   ├── server.go
│   │   │   └── handlers.go
│   │   └── websocket/
│   │       └── lark_adapter.go
│   └── config/
│       └── config.go
├── pkg/
│   ├── database/                            # Generic database utilities
│   └── utils/
│       ├── logger.go
│       └── validator.go
└── docs/
    └── ARCHITECTURE.md                      # Updated architecture documentation
```

This architecture ensures:
1. **Clear Layer Separation**: Domain has no dependencies, Application defines ports, Infrastructure implements them
2. **Demand-Driven Interfaces**: All interfaces in `application/port/` defined by consumers
3. **Explicit State Machine**: Workflow state transitions are declarative and testable
4. **Central Dispatcher**: All events flow through a single dispatcher
5. **Lifecycle Management**: Container handles complete Start/Close lifecycle
6. **Simplified Entry Point**: main.go only manages container lifecycle
