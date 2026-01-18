-- Migration 007: Add audit_notifications table
-- ARCH-012: AI Audit Result Notification via Lark Approval Bot

-- Create audit_notifications table to track notification delivery
CREATE TABLE IF NOT EXISTS audit_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL,
    lark_instance_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',  -- PENDING, SENT, FAILED
    audit_decision TEXT NOT NULL,             -- PASS, NEEDS_REVIEW, FAIL
    confidence REAL NOT NULL DEFAULT 0,
    total_amount REAL NOT NULL DEFAULT 0,
    approver_count INTEGER NOT NULL DEFAULT 0,
    violations TEXT,                          -- JSON array of violation strings
    sent_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Foreign key to approval_instances
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id),

    -- Unique constraint for idempotency: one notification per instance
    UNIQUE(instance_id)
);

-- Create index for status queries
CREATE INDEX IF NOT EXISTS idx_audit_notifications_status ON audit_notifications(status);

-- Create index for instance lookup
CREATE INDEX IF NOT EXISTS idx_audit_notifications_instance ON audit_notifications(instance_id);
