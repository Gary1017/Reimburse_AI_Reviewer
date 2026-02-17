-- Migration: initial_schema
-- Description: Clean consolidated schema matching ER diagram
-- Date: 2026-02-17

-- ============================================================================
-- approval_instances - Core entity for tracking reimbursement approval lifecycle
-- ============================================================================
CREATE TABLE approval_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    lark_instance_id TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL DEFAULT 'CREATED'
        CHECK(status IN (
            'CREATED', 'PENDING', 'DATA_READY',
            'AI_AUDITING', 'AI_AUDITED', 'IN_REVIEW',
            'AUTO_APPROVED', 'APPROVED', 'REJECTED',
            'VOUCHER_GENERATING', 'COMPLETED'
        )),
    applicant_user_id TEXT NOT NULL,
    department TEXT,
    submission_time DATETIME NOT NULL,
    approval_time DATETIME,
    form_data TEXT,
    ai_audit_result TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_approval_instances_lark_id ON approval_instances(lark_instance_id);
CREATE INDEX idx_approval_instances_status ON approval_instances(status);
CREATE INDEX idx_approval_instances_applicant ON approval_instances(applicant_user_id);

CREATE TRIGGER update_approval_instances_timestamp
AFTER UPDATE ON approval_instances
BEGIN
    UPDATE approval_instances SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- ============================================================================
-- approval_history - Audit trail for state transitions
-- ============================================================================
CREATE TABLE approval_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL,
    reviewer_user_id TEXT,
    previous_status TEXT,
    new_status TEXT NOT NULL,
    action_type TEXT NOT NULL,
    action_data TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

CREATE INDEX idx_approval_history_instance ON approval_history(instance_id);
CREATE INDEX idx_approval_history_timestamp ON approval_history(timestamp);

-- ============================================================================
-- reimbursement_items - Line items within an approval instance
-- ============================================================================
CREATE TABLE reimbursement_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL,
    item_type TEXT NOT NULL
        CHECK(item_type IN (
            'TRAVEL', 'MEAL', 'ACCOMMODATION', 'EQUIPMENT',
            'TRANSPORTATION', 'ENTERTAINMENT', 'TEAM_BUILDING',
            'COMMUNICATION', 'OTHER'
        )),
    description TEXT NOT NULL,
    amount DECIMAL(12,2) NOT NULL,
    currency TEXT DEFAULT 'CNY',
    receipt_attachment TEXT,
    ai_price_check TEXT,
    ai_policy_check TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

CREATE INDEX idx_reimbursement_items_instance ON reimbursement_items(instance_id);
CREATE INDEX idx_reimbursement_items_type ON reimbursement_items(item_type);

-- ============================================================================
-- generated_vouchers - Accounting vouchers (1:1 with instance)
-- ============================================================================
CREATE TABLE generated_vouchers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER UNIQUE NOT NULL,
    voucher_number TEXT UNIQUE NOT NULL,
    file_path TEXT NOT NULL,
    email_message_id TEXT,
    sent_at DATETIME,
    accountant_email TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

CREATE INDEX idx_generated_vouchers_instance ON generated_vouchers(instance_id);
CREATE INDEX idx_generated_vouchers_number ON generated_vouchers(voucher_number);

-- ============================================================================
-- attachments - Files attached to reimbursement items
-- ============================================================================
CREATE TABLE attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id INTEGER NOT NULL,
    instance_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    url TEXT,
    file_path TEXT,
    file_size INTEGER DEFAULT 0,
    mime_type TEXT,
    download_status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK(download_status IN ('PENDING', 'COMPLETED', 'FAILED')),
    file_type TEXT DEFAULT 'INVOICE'
        CHECK(file_type IN ('INVOICE', 'OTHER')),
    error_message TEXT,
    downloaded_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (item_id) REFERENCES reimbursement_items(id) ON DELETE CASCADE,
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE,

    UNIQUE(item_id, file_name)
);

CREATE INDEX idx_attachments_item ON attachments(item_id);
CREATE INDEX idx_attachments_instance ON attachments(instance_id);
CREATE INDEX idx_attachments_status ON attachments(download_status);
CREATE INDEX idx_attachments_file_type ON attachments(file_type);
CREATE INDEX idx_attachments_created ON attachments(created_at);

-- ============================================================================
-- invoices_v2 - Invoice data extracted from attachments via GPT-4
-- ============================================================================
CREATE TABLE invoices_v2 (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_list_id INTEGER,
    attachment_id INTEGER UNIQUE NOT NULL,
    item_id INTEGER,
    instance_id INTEGER,

    invoice_code TEXT NOT NULL,
    invoice_number TEXT NOT NULL,
    unique_id TEXT UNIQUE NOT NULL,

    invoice_date DATE,
    invoice_amount DECIMAL(12,2),
    seller_name TEXT,
    seller_tax_id TEXT,
    buyer_name TEXT,
    buyer_tax_id TEXT,
    extracted_data TEXT,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (attachment_id) REFERENCES attachments(id) ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES reimbursement_items(id) ON DELETE SET NULL,
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

CREATE INDEX idx_invoices_v2_attachment ON invoices_v2(attachment_id);
CREATE INDEX idx_invoices_v2_item ON invoices_v2(item_id);
CREATE INDEX idx_invoices_v2_instance ON invoices_v2(instance_id);
CREATE INDEX idx_invoices_v2_unique ON invoices_v2(unique_id);
CREATE UNIQUE INDEX idx_invoices_v2_code_number ON invoices_v2(invoice_code, invoice_number);

-- ============================================================================
-- approval_tasks - AI and human review tasks in the workflow
-- ============================================================================
CREATE TABLE approval_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL,

    lark_task_id TEXT,

    task_type TEXT NOT NULL,
    sequence_number INTEGER NOT NULL DEFAULT 0,

    node_id TEXT,
    node_name TEXT,
    custom_node_id TEXT,
    approval_type TEXT,

    assignee_user_id TEXT,
    assignee_open_id TEXT,

    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK(status IN ('PENDING', 'IN_PROGRESS', 'COMPLETED', 'REJECTED')),
    start_time TEXT,
    end_time TEXT,

    is_current BOOLEAN DEFAULT FALSE,
    is_ai_decision BOOLEAN DEFAULT FALSE,

    decision TEXT,
    confidence REAL,
    result_data TEXT,
    violations TEXT,
    completed_by TEXT,

    notification_sent_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

CREATE INDEX idx_approval_tasks_instance ON approval_tasks(instance_id);
CREATE INDEX idx_approval_tasks_status ON approval_tasks(status);
CREATE INDEX idx_approval_tasks_lark_id ON approval_tasks(lark_task_id);
CREATE INDEX idx_approval_tasks_type ON approval_tasks(task_type);

CREATE TRIGGER update_approval_tasks_timestamp
AFTER UPDATE ON approval_tasks
BEGIN
    UPDATE approval_tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- ============================================================================
-- system_config - Key-value configuration store
-- ============================================================================
CREATE TABLE system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER update_system_config_timestamp
AFTER UPDATE ON system_config
BEGIN
    UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
END;

-- ============================================================================
-- oauth_tokens - OAuth token storage for Lark API
-- ============================================================================
CREATE TABLE oauth_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL UNIQUE,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    access_token_expires_at DATETIME NOT NULL,
    refresh_token_expires_at DATETIME NOT NULL,
    scope TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_oauth_tokens_user_id ON oauth_tokens(user_id);
