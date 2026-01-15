-- Initial database schema for AI Reimbursement Workflow System

-- Approval instances table
CREATE TABLE IF NOT EXISTS approval_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    lark_instance_id TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('CREATED', 'PENDING', 'AI_AUDITING', 'AI_AUDITED', 'IN_REVIEW', 'AUTO_APPROVED', 'APPROVED', 'REJECTED', 'VOUCHER_GENERATING', 'COMPLETED')),
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

-- Approval history table for audit trail
CREATE TABLE IF NOT EXISTS approval_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL,
    reviewer_user_id TEXT,
    previous_status TEXT NOT NULL,
    new_status TEXT NOT NULL,
    action_type TEXT NOT NULL CHECK(action_type IN ('STATUS_CHANGE', 'HUMAN_REVIEW', 'AI_AUDIT', 'WEBHOOK_EVENT')),
    action_data TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

CREATE INDEX idx_approval_history_instance ON approval_history(instance_id);
CREATE INDEX idx_approval_history_timestamp ON approval_history(timestamp);

-- Reimbursement items table
CREATE TABLE IF NOT EXISTS reimbursement_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL,
    item_type TEXT NOT NULL CHECK(item_type IN ('TRAVEL', 'MEAL', 'ACCOMMODATION', 'EQUIPMENT', 'OTHER')),
    description TEXT NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    currency TEXT DEFAULT 'CNY',
    receipt_attachment TEXT,
    ai_price_check TEXT,
    ai_policy_check TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

CREATE INDEX idx_reimbursement_items_instance ON reimbursement_items(instance_id);
CREATE INDEX idx_reimbursement_items_type ON reimbursement_items(item_type);

-- Generated vouchers table
CREATE TABLE IF NOT EXISTS generated_vouchers (
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

-- System configuration table
CREATE TABLE IF NOT EXISTS system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Trigger to update updated_at on approval_instances
CREATE TRIGGER IF NOT EXISTS update_approval_instances_timestamp 
AFTER UPDATE ON approval_instances
BEGIN
    UPDATE approval_instances SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Trigger to update updated_at on system_config
CREATE TRIGGER IF NOT EXISTS update_system_config_timestamp 
AFTER UPDATE ON system_config
BEGIN
    UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
END;
