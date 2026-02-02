-- Migration 012: Add review_notifications table
-- Database Schema Refactoring Phase 1: Notification record linked to AI_REVIEW task
-- 1:1 relationship with approval_tasks (specifically AI_REVIEW tasks)

CREATE TABLE IF NOT EXISTS review_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER UNIQUE NOT NULL,         -- 1:1 with AI_REVIEW task
    lark_instance_id TEXT NOT NULL,

    status TEXT NOT NULL DEFAULT 'PENDING',  -- PENDING | SENT | FAILED
    approver_count INTEGER NOT NULL DEFAULT 0,

    sent_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (task_id) REFERENCES approval_tasks(id) ON DELETE CASCADE
);

-- Create indices for efficient querying
CREATE INDEX IF NOT EXISTS idx_review_notifications_task ON review_notifications(task_id);
CREATE INDEX IF NOT EXISTS idx_review_notifications_status ON review_notifications(status);

-- Trigger to update updated_at on review_notifications
CREATE TRIGGER IF NOT EXISTS update_review_notifications_timestamp
AFTER UPDATE ON review_notifications
BEGIN
    UPDATE review_notifications SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
