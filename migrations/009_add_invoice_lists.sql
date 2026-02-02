-- Migration 009: Add invoice_lists table
-- Database Schema Refactoring Phase 1: Invoice list management at instance level
-- 1:1 relationship with approval_instances for aggregating all invoices per instance

CREATE TABLE IF NOT EXISTS invoice_lists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER UNIQUE NOT NULL,
    total_invoice_count INTEGER DEFAULT 0,
    total_invoice_amount DECIMAL(12,2) DEFAULT 0,
    status TEXT DEFAULT 'PENDING',  -- PENDING, PROCESSING, COMPLETED, FAILED
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

-- Create indices for efficient querying
CREATE INDEX IF NOT EXISTS idx_invoice_lists_instance ON invoice_lists(instance_id);
CREATE INDEX IF NOT EXISTS idx_invoice_lists_status ON invoice_lists(status);

-- Trigger to update updated_at on invoice_lists
CREATE TRIGGER IF NOT EXISTS update_invoice_lists_timestamp
AFTER UPDATE ON invoice_lists
BEGIN
    UPDATE invoice_lists SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
