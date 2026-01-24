# AI Reimbursement Workflow System - Architecture Design

## 1. System Overview

The AI-Driven Reimbursement Workflow System automates the entire reimbursement lifecycle, integrating Lark's approval workflow with AI-powered auditing and seamless external accountant collaboration.

**Core Principles:**
- **Clean Architecture**: Strict dependency inversion with domain at the center
- **Zero-Touch Processing**: Minimize manual intervention via intelligent AI-driven validation
- **Financial Audit Readiness**: Immutable audit trail, compliant vouchers, duplicate prevention
- **High Availability**: Graceful degradation, retry logic, idempotent operations
- **Extensibility**: Port-based abstraction for swappable implementations

## 2. Clean Architecture Layers

The system follows Clean Architecture (also known as Hexagonal/Ports & Adapters architecture) with four distinct layers:

```
                    ┌─────────────────────────────────────────┐
                    │            External World               │
                    │   (Lark, OpenAI, HTTP, File System)     │
                    └──────────────────┬──────────────────────┘
                                       │
                    ┌──────────────────▼──────────────────────┐
                    │         Interface Adapters               │
                    │   (HTTP Server, WebSocket, CLI)          │
                    └──────────────────┬──────────────────────┘
                                       │
┌─────────────────┐  ┌─────────────────▼─────────────────────┐
│  Infrastructure │  │         Application Layer              │
│  (Repositories, │◄─│  (Services, Workflow Engine,           │
│   External APIs, │  │   Event Dispatcher, Ports)            │
│   Storage)       │  └──────────────────┬───────────────────┘
└─────────────────┘                      │
                    ┌────────────────────▼───────────────────┐
                    │           Domain Layer                  │
                    │   (Entities, State Machine, Events)     │
                    │       ** No External Dependencies **    │
                    └─────────────────────────────────────────┘
```

### Dependency Rule

**Dependencies flow inward only:**
- Domain has NO dependencies on any other layer
- Application depends only on Domain
- Infrastructure implements ports defined by Application
- Interfaces adapt external protocols to Application services

## 3. Directory Structure

```
AI_Reimbursement/
├── cmd/server/main.go              # Entry point: config + container lifecycle only
├── internal/
│   ├── domain/                     # DOMAIN LAYER (innermost)
│   │   ├── entity/                 # Pure data structures
│   │   │   ├── instance.go         # ApprovalInstance entity
│   │   │   ├── attachment.go       # Attachment entity
│   │   │   ├── invoice.go          # Invoice entity
│   │   │   ├── voucher.go          # Voucher entity
│   │   │   ├── history.go          # StatusHistory entity
│   │   │   └── notification.go     # Notification entity
│   │   ├── workflow/               # State machine (pure logic)
│   │   │   ├── state.go            # State definitions
│   │   │   ├── trigger.go          # Trigger definitions
│   │   │   ├── machine.go          # State machine implementation
│   │   │   ├── builder.go          # Fluent builder for state machine
│   │   │   └── errors.go           # Domain-specific errors
│   │   └── event/                  # Domain events
│   │       ├── type.go             # Event type constants
│   │       └── event.go            # Event structures
│   │
│   ├── application/                # APPLICATION LAYER
│   │   ├── port/                   # Interfaces (contracts)
│   │   │   ├── repository.go       # Repository interfaces
│   │   │   ├── external.go         # External service interfaces
│   │   │   └── storage.go          # Storage interfaces
│   │   ├── dispatcher/             # Event dispatcher
│   │   │   ├── dispatcher.go       # Dispatcher implementation
│   │   │   └── handler.go          # Handler registration
│   │   ├── workflow/               # Workflow engine
│   │   │   ├── engine.go           # Engine interface
│   │   │   ├── factory.go          # Engine factory
│   │   │   └── impl.go             # Engine implementation
│   │   └── service/                # Application services
│   │       ├── approval_service.go # Approval orchestration
│   │       ├── audit_service.go    # AI audit orchestration
│   │       ├── voucher_service.go  # Voucher generation
│   │       └── notification_service.go
│   │
│   ├── infrastructure/             # INFRASTRUCTURE LAYER
│   │   ├── persistence/            # Database layer
│   │   │   ├── sqlite/
│   │   │   │   └── db.go           # SQLite connection & transactions
│   │   │   └── repository/         # Repository implementations
│   │   │       ├── instance_repo.go
│   │   │       ├── attachment_repo.go
│   │   │       ├── invoice_repo.go
│   │   │       └── ...
│   │   ├── external/               # External service adapters
│   │   │   ├── lark/
│   │   │   │   ├── client.go       # Lark API adapter
│   │   │   │   ├── downloader.go   # Attachment downloader
│   │   │   │   └── messenger.go    # Message sender
│   │   │   └── openai/
│   │   │       └── auditor.go      # AI auditor adapter
│   │   ├── storage/                # File storage
│   │   │   ├── file_storage.go     # File operations
│   │   │   └── folder_manager.go   # Folder management
│   │   └── worker/                 # Background workers
│   │       ├── manager.go          # Worker lifecycle manager
│   │       ├── download_worker.go  # Async attachment downloads
│   │       └── invoice_worker.go   # Invoice processing
│   │
│   ├── container/                  # DEPENDENCY INJECTION
│   │   ├── container.go            # Main container with lifecycle
│   │   ├── config.go               # Container configuration
│   │   └── providers.go            # Factory functions for components
│   │
│   ├── interfaces/                 # INTERFACE ADAPTERS
│   │   ├── http/                   # HTTP server
│   │   │   ├── server.go           # HTTP server setup
│   │   │   └── handlers.go         # Request handlers
│   │   └── websocket/              # Lark WebSocket
│   │       └── lark_adapter.go     # WebSocket event adapter
│   │
│   └── config/                     # Configuration loading
│       └── config.go               # YAML config parsing
```

## 4. Layer Responsibilities

### 4.1 Domain Layer (`internal/domain/`)

The domain layer contains **pure business logic** with zero external dependencies.

**Entities** (`domain/entity/`):
- Pure data structures representing business objects
- No persistence logic, no framework dependencies
- Contains validation rules intrinsic to the entity

**State Machine** (`domain/workflow/`):
- Defines approval workflow states: `CREATED` -> `PENDING` -> `AI_AUDITING` -> `APPROVED/REJECTED` -> `VOUCHER_GENERATING` -> `COMPLETED`
- Pure state transition logic with guards and triggers
- No side effects - just determines valid transitions

**Domain Events** (`domain/event/`):
- Immutable event structures (e.g., `InstanceCreated`, `StatusChanged`)
- Enable loose coupling between components

### 4.2 Application Layer (`internal/application/`)

The application layer **orchestrates** domain logic and **defines contracts** for infrastructure.

**Ports** (`application/port/`):
- Define interfaces for repositories, external services, and storage
- Allow infrastructure to be swapped without changing application logic
- Examples: `InstanceRepository`, `LarkClient`, `AIAuditor`, `FileStorage`

**Event Dispatcher** (`application/dispatcher/`):
- Publishes and routes domain events to handlers
- Enables reactive, event-driven processing

**Workflow Engine** (`application/workflow/`):
- Orchestrates the approval lifecycle
- Uses the domain state machine for transitions
- Triggers side effects via services

**Application Services** (`application/service/`):
- `ApprovalService`: Handles approval instance CRUD
- `AuditService`: Orchestrates AI policy and price validation
- `VoucherService`: Generates accounting vouchers
- `NotificationService`: Manages accountant notifications

### 4.3 Infrastructure Layer (`internal/infrastructure/`)

The infrastructure layer **implements ports** defined by the application layer.

**Persistence** (`infrastructure/persistence/`):
- `sqlite/db.go`: SQLite connection with WAL mode, transaction management
- `repository/*`: Concrete implementations of repository interfaces

**External Services** (`infrastructure/external/`):
- `lark/`: Lark SDK wrapper implementing `LarkClient`, `LarkAttachmentDownloader`, `LarkMessageSender`
- `openai/`: OpenAI adapter implementing `AIAuditor`

**Storage** (`infrastructure/storage/`):
- `FileStorage`: File I/O operations
- `FolderManager`: Directory management for organized attachment storage

**Workers** (`infrastructure/worker/`):
- `WorkerManager`: Lifecycle management for background goroutines
- `DownloadWorker`: Async attachment download (ARCH-007)
- `InvoiceWorker`: Invoice extraction and processing (ARCH-011)

### 4.4 Interface Adapters (`internal/interfaces/`)

Interface adapters convert external protocols to application-layer calls.

**HTTP** (`interfaces/http/`):
- REST API endpoints for health checks and manual triggers
- Adapts HTTP requests to service calls

**WebSocket** (`interfaces/websocket/`):
- Lark WebSocket event subscription (NOT HTTP webhooks)
- Converts Lark events to workflow engine calls

### 4.5 Container (`internal/container/`)

The container manages **dependency injection** and **application lifecycle**.

**Responsibilities:**
- Creates all components in correct dependency order
- Wires dependencies together
- Provides clean `Start()` and `Close()` lifecycle methods
- Exposes health checks and component getters

**Initialization Order:**
1. Database and repositories
2. External clients (Lark, OpenAI)
3. Storage (file system)
4. Application services
5. Event dispatcher and workflow engine
6. Background workers

**Shutdown Order:** Reverse of initialization for graceful cleanup.

## 5. Data Flow

### 5.1 Approval Event Processing

```
┌──────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Lark Platform │───►│ WebSocket       │───►│ Event           │
│ (Event)       │    │ LarkAdapter     │    │ Processor       │
└──────────────┘    └─────────────────┘    └────────┬────────┘
                                                     │
                    ┌────────────────────────────────▼────────┐
                    │           Workflow Engine                │
                    │  - Validates state transition            │
                    │  - Executes transition                   │
                    │  - Publishes domain events               │
                    └────────────────────────────────┬────────┘
                                                     │
     ┌───────────────────────────────────────────────┼───────────────────────────┐
     │                                               │                           │
     ▼                                               ▼                           ▼
┌──────────────┐                           ┌─────────────────┐         ┌─────────────────┐
│ Approval     │                           │ Audit           │         │ Voucher         │
│ Service      │                           │ Service         │         │ Service         │
│ (Persist)    │                           │ (AI Check)      │         │ (Generate)      │
└──────────────┘                           └─────────────────┘         └─────────────────┘
```

### 5.2 Attachment Processing (Async)

```
┌──────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Instance     │───►│ Mark Attachment │───►│ Download        │
│ Created      │    │ as PENDING      │    │ Worker Queue    │
└──────────────┘    └─────────────────┘    └────────┬────────┘
                                                     │
                                                     ▼ (Background)
                    ┌─────────────────┐    ┌─────────────────┐
                    │ Invoice         │◄───│ File Storage    │
                    │ Worker          │    │ (Local/S3)      │
                    └────────┬────────┘    └─────────────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ AI Extraction   │
                    │ & Validation    │
                    └─────────────────┘
```

## 6. Key Architectural Decisions

### 6.1 Why Clean Architecture?

| Benefit | Description |
|---------|-------------|
| **Testability** | Domain logic testable without infrastructure (mocks for ports) |
| **Flexibility** | Swap SQLite for PostgreSQL, local storage for S3, without touching domain |
| **Maintainability** | Clear boundaries prevent spaghetti dependencies |
| **Onboarding** | New developers understand where code belongs |

### 6.2 WebSocket over HTTP Webhooks

The system uses **Lark WebSocket-based event subscription** instead of HTTP webhooks:
- No intranet penetration required
- Automatic authentication at connection establishment
- Plaintext event data (no decryption needed)
- Real-time events with persistent connection

### 6.3 Event-Driven Processing

Domain events enable loose coupling:
- State changes emit events (e.g., `StatusChanged`)
- Services react to events via dispatcher
- Audit trail captured automatically

### 6.4 Background Workers

Long-running tasks (downloads, AI calls) are handled asynchronously:
- Avoids blocking the main event loop
- Provides retry and error isolation
- Managed lifecycle via WorkerManager

## 7. Configuration

```yaml
# configs/config.yaml
lark:
  app_id: ${LARK_APP_ID}
  app_secret: ${LARK_APP_SECRET}
  approval_code: ${LARK_APPROVAL_CODE}

openai:
  api_key: ${OPENAI_API_KEY}

database:
  path: ./data/reimbursement.db

storage:
  base_path: ./attachments

worker:
  download_interval: 5s
  invoice_interval: 10s
```

Environment variables override YAML values for sensitive data.

## 8. Extension Points

### Adding a New External Service

1. Define interface in `application/port/external.go`
2. Implement adapter in `infrastructure/external/<service>/`
3. Add provider in `container/providers.go`
4. Wire in `container/container.go`

### Adding a New Repository

1. Define interface in `application/port/repository.go`
2. Implement in `infrastructure/persistence/repository/`
3. Register in `RepositoryBundle`

### Adding a New Workflow State

1. Add state in `domain/workflow/state.go`
2. Add transition in `domain/workflow/machine.go`
3. Update engine implementation if needed

## 9. Testing Strategy

| Layer | Testing Approach |
|-------|------------------|
| Domain | Unit tests with no mocks (pure logic) |
| Application | Unit tests with mocked ports |
| Infrastructure | Integration tests with real resources |
| Container | Smoke tests for wiring correctness |

## 10. Known Gaps & Roadmap

| Gap | Status | Plan |
|-----|--------|------|
| **Observability** (ARCH-009) | Planned | Prometheus metrics, OpenTelemetry tracing |
| **Adaptive Thresholds** (ARCH-008) | Planned | Configurable confidence-based routing |
| **PostgreSQL Support** (ARCH-010) | Planned | Abstract database layer further |
| **Rate Limiting** | Planned | Add circuit breaker for external APIs |

---

**Last Updated:** January 2025
**Architecture Version:** 2.0 (Clean Architecture)
