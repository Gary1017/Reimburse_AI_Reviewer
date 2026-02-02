# Database Schema Refactoring Design

**Date**: 2026-02-02
**Status**: Approved
**Scope**: Task-based approval workflow with invoice list management and Lark task_list integration

## 1. Overview

Refactor the database schema to:
1. Support task-based approval workflow aligned with Lark's `task_list` structure
2. Introduce invoice list management at instance level
3. Add AI decision tracking for technical auditing
4. Simplify entity relationships by merging task results into tasks

## 2. Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Item list structure | Direct relationship (no intermediate table) | Current `reimbursement_items` → `approval_instances` is sufficient |
| Invoice list structure | Instance-level (Option B) | 1:1 with instance, invoices link back to items via `item_id` |
| Task model | Unified (Option A) | AI review is "just another task" with `is_ai_decision` flag |
| Task + Result | Merged (Option B) | `approval_tasks` contains both definition and result |
| Item-Invoice constraint | Nullable with TODO | Business logic validates, DB allows NULL for async workflow |

## 3. New Tables

### 3.1 invoice_lists
One invoice list per approval instance (1:1 relationship).

```sql
CREATE TABLE invoice_lists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER UNIQUE NOT NULL,
    total_invoice_count INTEGER DEFAULT 0,
    total_invoice_amount DECIMAL(12,2) DEFAULT 0,
    status TEXT DEFAULT 'PENDING',  -- PENDING, PROCESSING, COMPLETED, FAILED
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

CREATE INDEX idx_invoice_lists_instance ON invoice_lists(instance_id);
CREATE INDEX idx_invoice_lists_status ON invoice_lists(status);
```

### 3.2 invoices_v2
Individual invoices linked to invoice_list AND reimbursement_item.

```sql
CREATE TABLE invoices_v2 (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_list_id INTEGER NOT NULL,
    item_id INTEGER,
    -- TODO: Business rule - each reimbursement_item should have exactly one
    -- corresponding invoice. Validated by application logic in
    -- internal/application/service/invoice_service.go

    -- Invoice identification (from GPT-4 extraction)
    invoice_code TEXT NOT NULL,
    invoice_number TEXT NOT NULL,
    unique_id TEXT UNIQUE NOT NULL,

    -- File references
    file_token TEXT,
    file_path TEXT,

    -- Extracted data
    invoice_date DATE,
    invoice_amount DECIMAL(12,2),
    seller_name TEXT,
    seller_tax_id TEXT,
    buyer_name TEXT,
    buyer_tax_id TEXT,
    extracted_data TEXT,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (invoice_list_id) REFERENCES invoice_lists(id) ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES reimbursement_items(id) ON DELETE SET NULL
);

CREATE INDEX idx_invoices_v2_list ON invoices_v2(invoice_list_id);
CREATE INDEX idx_invoices_v2_item ON invoices_v2(item_id);
CREATE INDEX idx_invoices_v2_unique ON invoices_v2(unique_id);
```

### 3.3 approval_tasks
Unified task table (definition + result) aligned with Lark's task_list.

```sql
CREATE TABLE approval_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL,

    -- Lark task_list mapping (NULL for AI-generated tasks)
    lark_task_id TEXT,

    -- Task type and workflow position
    task_type TEXT NOT NULL,  -- AI_REVIEW | HUMAN_REVIEW
    sequence_number INTEGER NOT NULL DEFAULT 0,

    -- Lark node information
    node_id TEXT,
    node_name TEXT,
    custom_node_id TEXT,
    approval_type TEXT,

    -- Assignee (for Lark governance accountability)
    assignee_user_id TEXT,
    assignee_open_id TEXT,

    -- Status and timing
    status TEXT NOT NULL DEFAULT 'PENDING',  -- PENDING | IN_PROGRESS | COMPLETED | REJECTED
    start_time TEXT,
    end_time TEXT,

    -- Workflow control
    is_current BOOLEAN DEFAULT FALSE,

    -- AI decision tracking (for technical auditing)
    is_ai_decision BOOLEAN DEFAULT FALSE,

    -- Result fields (merged from task_results + approval_history)
    decision TEXT,  -- PASS | NEEDS_REVIEW | FAIL | APPROVED | REJECTED
    confidence REAL,
    result_data TEXT,
    violations TEXT,
    completed_by TEXT,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

CREATE INDEX idx_approval_tasks_instance ON approval_tasks(instance_id);
CREATE INDEX idx_approval_tasks_status ON approval_tasks(status);
CREATE INDEX idx_approval_tasks_lark_id ON approval_tasks(lark_task_id);
CREATE INDEX idx_approval_tasks_type ON approval_tasks(task_type);
```

### 3.4 review_notifications
Notification record linked to AI_REVIEW task (1:1).

```sql
CREATE TABLE review_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER UNIQUE NOT NULL,
    lark_instance_id TEXT NOT NULL,

    status TEXT NOT NULL DEFAULT 'PENDING',  -- PENDING | SENT | FAILED
    approver_count INTEGER NOT NULL DEFAULT 0,

    sent_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (task_id) REFERENCES approval_tasks(id) ON DELETE CASCADE
);

CREATE INDEX idx_review_notifications_task ON review_notifications(task_id);
CREATE INDEX idx_review_notifications_status ON review_notifications(status);
```

## 4. Modified Tables

### 4.1 approval_instances
- Remove `ai_audit_result` column (moved to `approval_tasks.result_data`)

### 4.2 reimbursement_items
- Remove `invoice_id` column (invoices now link via `item_id` in `invoices_v2`)

## 5. Deprecated Tables

| Table | Action | Reason |
|-------|--------|--------|
| `invoices` | Rename to `invoices_deprecated` | Migrated to `invoices_v2` |
| `invoice_validations` | Keep, update FK | Update FK to `invoices_v2.id` |
| `approval_history` | Drop | Merged into `approval_tasks` |
| `audit_notifications` | Drop | Replaced by `review_notifications` |

## 6. Entity Relationship Diagram

```
approval_instances (1) ──────┬────── (1) invoice_lists
                             │              │
                             │              └──── (N) invoices_v2
                             │                         │
                             │                         │ item_id FK
                             │                         ▼
                             ├────── (N) reimbursement_items
                             │                         ▲
                             │                         │ TODO: Business rule
                             │                         │ (1 item : 1 invoice)
                             │
                             ├────── (N) attachments
                             │
                             ├────── (N) approval_tasks ◄─────┐
                             │       (definition + result)    │
                             │              │                 │
                             │              └── (1) review_notifications
                             │                   (only for AI_REVIEW task)
                             │
                             ├────── (1) generated_vouchers
                             │
                             └────── system_config (standalone)
```

## 7. Lark API Alignment

Based on Lark SDK `InstanceTask` structure:

| Lark Field | DB Column | Notes |
|------------|-----------|-------|
| `id` | `lark_task_id` | NULL for AI tasks |
| `user_id` | `assignee_user_id` | |
| `open_id` | `assignee_open_id` | |
| `status` | `status` | |
| `node_id` | `node_id` | |
| `node_name` | `node_name` | |
| `custom_node_id` | `custom_node_id` | |
| `type` | `approval_type` | |
| `start_time` | `start_time` | TEXT format |
| `end_time` | `end_time` | TEXT format |

## 8. Migration Strategy

1. Create new tables first (additive)
2. Migrate data from old tables
3. Update application code
4. Drop deprecated tables (after verification)

## 9. References

- Lark API: [Get Instance Details](https://open.larksuite.com/document/server-docs/approval-v4/instance/get)
- Lark SDK: `github.com/larksuite/oapi-sdk-go/v3/service/approval/v4.InstanceTask`
