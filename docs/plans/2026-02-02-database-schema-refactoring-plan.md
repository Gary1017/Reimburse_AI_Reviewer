# Database Schema Refactoring Implementation Plan

**Design Document**: [2026-02-02-database-schema-refactoring-design.md](./2026-02-02-database-schema-refactoring-design.md)
**Date**: 2026-02-02
**Estimated Phases**: 5

---

## Phase 1: Database Migrations (Foundation)

**Goal**: Create new tables and modify existing schema without breaking current functionality.

### Task 1.1: Create Migration Files
**File**: `migrations/009_add_invoice_lists.sql`
- [ ] Create `invoice_lists` table with all columns and indices
- [ ] Verify FK constraint to `approval_instances`

**Measurable**: Migration file exists and passes `make migrate`

### Task 1.2: Create Invoice V2 Migration
**File**: `migrations/010_add_invoices_v2.sql`
- [ ] Create `invoices_v2` table with all columns
- [ ] Add TODO comment for business rule validation
- [ ] Create indices for `invoice_list_id`, `item_id`, `unique_id`

**Measurable**: Migration file exists and passes `make migrate`

### Task 1.3: Create Approval Tasks Migration
**File**: `migrations/011_add_approval_tasks.sql`
- [ ] Create `approval_tasks` table with task definition + result fields
- [ ] Add `is_ai_decision` flag for technical auditing
- [ ] Create indices for `instance_id`, `status`, `lark_task_id`, `task_type`

**Measurable**: Migration file exists and passes `make migrate`

### Task 1.4: Create Review Notifications Migration
**File**: `migrations/012_add_review_notifications.sql`
- [ ] Create `review_notifications` table with `task_id` FK
- [ ] Add UNIQUE constraint on `task_id`

**Measurable**: Migration file exists and passes `make migrate`

### Task 1.5: Deprecate Old Tables Migration
**File**: `migrations/013_deprecate_old_tables.sql`
- [ ] Rename `invoices` to `invoices_deprecated`
- [ ] Rename `audit_notifications` to `audit_notifications_deprecated`
- [ ] Drop `approval_history` table (data loss acceptable per design)
- [ ] Remove `invoice_id` from `reimbursement_items`
- [ ] Remove `ai_audit_result` from `approval_instances`

**Measurable**: Migration file exists and passes `make migrate`

**Parallel Execution**: Tasks 1.1-1.4 can run in parallel (additive changes)

---

## Phase 2: Domain Layer (Entities)

**Goal**: Update domain entities to reflect new schema. No external dependencies.

### Task 2.1: Create InvoiceList Entity
**File**: `internal/domain/entity/invoice_list.go`
- [ ] Define `InvoiceList` struct with all fields
- [ ] Add status constants (PENDING, PROCESSING, COMPLETED, FAILED)

**Measurable**: File compiles, `go build ./internal/domain/...` passes

### Task 2.2: Update Invoice Entity
**File**: `internal/domain/entity/invoice.go`
- [ ] Create `InvoiceV2` struct (or update existing `Invoice`)
- [ ] Add `InvoiceListID` and `ItemID` fields
- [ ] Keep backward-compatible `Invoice` if needed for migration

**Measurable**: File compiles, `go build ./internal/domain/...` passes

### Task 2.3: Create ApprovalTask Entity
**File**: `internal/domain/entity/task.go`
- [ ] Define `ApprovalTask` struct with all Lark-aligned fields
- [ ] Add result fields (decision, confidence, result_data, violations)
- [ ] Define task type constants (AI_REVIEW, HUMAN_REVIEW)
- [ ] Define status constants (PENDING, IN_PROGRESS, COMPLETED, REJECTED)

**Measurable**: File compiles, `go build ./internal/domain/...` passes

### Task 2.4: Update Notification Entity
**File**: `internal/domain/entity/notification.go`
- [ ] Create `ReviewNotification` struct with `TaskID` instead of `InstanceID`
- [ ] Remove duplicated fields (decision, confidence read from task)

**Measurable**: File compiles, `go build ./internal/domain/...` passes

### Task 2.5: Update Instance Entity
**File**: `internal/domain/entity/instance.go`
- [ ] Remove `AIAuditResult` field from `ApprovalInstance`

**Measurable**: File compiles, `go build ./internal/domain/...` passes

**Parallel Execution**: All tasks in Phase 2 can run in parallel

---

## Phase 3: Application Layer (Ports & Interfaces)

**Goal**: Define repository interfaces for new entities.

### Task 3.1: Add InvoiceList Repository Interface
**File**: `internal/application/port/repository.go`
- [ ] Add `InvoiceListRepository` interface
- [ ] Methods: Create, GetByID, GetByInstanceID, Update, UpdateStatus

**Measurable**: File compiles, `go build ./internal/application/...` passes

### Task 3.2: Update Invoice Repository Interface
**File**: `internal/application/port/repository.go`
- [ ] Update `InvoiceRepository` for new schema
- [ ] Add `GetByInvoiceListID` method
- [ ] Update `Create` to accept `InvoiceListID`

**Measurable**: File compiles, `go build ./internal/application/...` passes

### Task 3.3: Add ApprovalTask Repository Interface
**File**: `internal/application/port/repository.go`
- [ ] Add `ApprovalTaskRepository` interface
- [ ] Methods: Create, GetByID, GetByInstanceID, GetByLarkTaskID
- [ ] Methods: UpdateStatus, UpdateResult, GetCurrentTask

**Measurable**: File compiles, `go build ./internal/application/...` passes

### Task 3.4: Update Notification Repository Interface
**File**: `internal/application/port/repository.go`
- [ ] Update to `ReviewNotificationRepository`
- [ ] Change `GetByInstanceID` to `GetByTaskID`

**Measurable**: File compiles, `go build ./internal/application/...` passes

### Task 3.5: Remove History Repository Interface
**File**: `internal/application/port/repository.go`
- [ ] Remove `HistoryRepository` interface (merged into tasks)

**Measurable**: File compiles, `go build ./internal/application/...` passes

**Parallel Execution**: Tasks 3.1-3.4 can run in parallel

---

## Phase 4: Infrastructure Layer (Repositories)

**Goal**: Implement repository interfaces with SQL queries.

### Task 4.1: Implement InvoiceListRepository
**File**: `internal/infrastructure/persistence/repository/invoice_list_repo.go`
- [ ] Implement all methods from interface
- [ ] Add transaction support via context
- [ ] Write unit tests

**Measurable**: `go test -v ./internal/infrastructure/persistence/repository/... -run InvoiceList` passes

### Task 4.2: Update InvoiceRepository
**File**: `internal/infrastructure/persistence/repository/invoice_repo.go`
- [ ] Update SQL queries for `invoices_v2` table
- [ ] Implement `GetByInvoiceListID` method
- [ ] Update tests

**Measurable**: `go test -v ./internal/infrastructure/persistence/repository/... -run Invoice` passes

### Task 4.3: Implement ApprovalTaskRepository
**File**: `internal/infrastructure/persistence/repository/task_repo.go`
- [ ] Implement all methods from interface
- [ ] Add Lark task_list field mapping
- [ ] Write unit tests

**Measurable**: `go test -v ./internal/infrastructure/persistence/repository/... -run Task` passes

### Task 4.4: Update NotificationRepository
**File**: `internal/infrastructure/persistence/repository/notification_repo.go`
- [ ] Rename to `review_notification_repo.go`
- [ ] Update SQL for `review_notifications` table
- [ ] Change FK from `instance_id` to `task_id`
- [ ] Update tests

**Measurable**: `go test -v ./internal/infrastructure/persistence/repository/... -run Notification` passes

### Task 4.5: Remove HistoryRepository
**File**: `internal/infrastructure/persistence/repository/history_repo.go`
- [ ] Delete file (functionality merged into task repo)
- [ ] Remove from container wiring

**Measurable**: `go build ./...` passes without history_repo.go

**Parallel Execution**: Tasks 4.1-4.4 can run in parallel

---

## Phase 5: Application Services & Integration

**Goal**: Update services to use new repositories and entities.

### Task 5.1: Create InvoiceListService
**File**: `internal/application/service/invoice_list_service.go`
- [ ] Create service for invoice list management
- [ ] Implement CreateForInstance, AddInvoice, UpdateTotals
- [ ] Add TODO for item-invoice validation logic

**Measurable**: `go test -v ./internal/application/service/... -run InvoiceList` passes

### Task 5.2: Create ApprovalTaskService
**File**: `internal/application/service/task_service.go`
- [ ] Create service for task management
- [ ] Implement CreateAIReviewTask, SyncLarkTasks
- [ ] Implement CompleteTask (records decision, updates status)

**Measurable**: `go test -v ./internal/application/service/... -run Task` passes

### Task 5.3: Update AuditService → ReviewService
**File**: `internal/application/service/audit_service.go`
- [ ] Rename to `review_service.go`
- [ ] Update to create AI_REVIEW task first
- [ ] Store result in task instead of instance

**Measurable**: `go test -v ./internal/application/service/... -run Review` passes

### Task 5.4: Update NotificationService
**File**: `internal/application/service/notification_service.go`
- [ ] Update to link notification to AI_REVIEW task
- [ ] Read decision/confidence from task, not duplicated fields

**Measurable**: `go test -v ./internal/application/service/... -run Notification` passes

### Task 5.5: Update Container Wiring
**File**: `internal/container/providers.go`
- [ ] Add new repository providers
- [ ] Add new service providers
- [ ] Remove deprecated providers (history_repo)

**Measurable**: `go build ./cmd/server` passes

### Task 5.6: Update Workflow Engine
**File**: `internal/application/workflow/impl.go`
- [ ] Update to use ApprovalTaskService
- [ ] Create AI_REVIEW task on instance creation
- [ ] Update state transitions to work with tasks

**Measurable**: `go test -v ./internal/application/workflow/...` passes

**Sequential Execution**: Task 5.5 depends on 5.1-5.4. Task 5.6 depends on 5.2.

---

## Phase 6: Data Migration (Optional)

**Goal**: Migrate existing data from old tables to new schema.

### Task 6.1: Write Data Migration Script
**File**: `scripts/migrate_to_v2_schema.go`
- [ ] Migrate `invoices` → `invoice_lists` + `invoices_v2`
- [ ] Migrate `ai_audit_result` → `approval_tasks` (AI_REVIEW)
- [ ] Migrate `audit_notifications` → `review_notifications`

**Measurable**: Script runs without errors on test database

### Task 6.2: Verify Data Integrity
- [ ] Count records match between old and new tables
- [ ] Spot check critical fields

**Measurable**: Verification script passes

---

## Execution Strategy

### Parallel Execution Groups

| Group | Tasks | Dependencies |
|-------|-------|--------------|
| A | 1.1, 1.2, 1.3, 1.4 | None (additive migrations) |
| B | 2.1, 2.2, 2.3, 2.4, 2.5 | None (domain layer) |
| C | 3.1, 3.2, 3.3, 3.4 | Group B (needs entities) |
| D | 4.1, 4.2, 4.3, 4.4 | Group A + C (needs schema + interfaces) |
| E | 5.1, 5.2, 5.3, 5.4 | Group D (needs repos) |
| F | 5.5, 5.6 | Group E (needs services) |

### Recommended Agent Assignment

```
Agent 1: Phase 1 (Migrations)           ← Can start immediately
Agent 2: Phase 2 (Domain Entities)      ← Can start immediately
Agent 3: Phase 3 (Ports)                ← After Phase 2
Agent 4: Phase 4 (Repositories)         ← After Phase 1 + 3
Agent 5: Phase 5 (Services)             ← After Phase 4
```

---

## Success Criteria

- [ ] All migrations run successfully: `make migrate`
- [ ] All tests pass: `make test`
- [ ] No lint errors: `make lint`
- [ ] Server starts without errors: `make run`
- [ ] Clean Architecture boundaries maintained (no layer violations)

---

## Rollback Plan

1. Keep deprecated tables for 1 release cycle
2. Migration scripts are idempotent (can re-run safely)
3. Feature flag for new vs old code paths (if needed)
