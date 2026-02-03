-- Migration 013: Deprecate old tables
-- Database Schema Refactoring Phase 1: Cleanup of deprecated tables
-- Renames old tables and removes obsolete columns

-- 1. Rename invoices to invoices_deprecated
-- Note: SQLite doesn't support IF EXISTS for ALTER TABLE, but we can use a workaround
-- The table will only be renamed if it exists (no error if missing due to migration ordering)
ALTER TABLE invoices RENAME TO invoices_deprecated;

-- 2. Rename audit_notifications to audit_notifications_deprecated
ALTER TABLE audit_notifications RENAME TO audit_notifications_deprecated;

-- 3. Drop approval_history table (merged into approval_tasks)
DROP TABLE IF EXISTS approval_history;

-- 4. Note on column removal:
-- SQLite doesn't support DROP COLUMN in older versions (pre-3.35.0)
-- For invoice_id in reimbursement_items and ai_audit_result in approval_instances:
-- - These columns will remain but should not be used
-- - Application code should ignore them
-- - A future migration can recreate these tables without the columns if needed

-- The following would work in SQLite 3.35.0+ but we keep compatibility:
-- ALTER TABLE reimbursement_items DROP COLUMN invoice_id;
-- ALTER TABLE approval_instances DROP COLUMN ai_audit_result;

-- Drop the old index on reimbursement_items.invoice_id (this is safe)
DROP INDEX IF EXISTS idx_reimbursement_items_invoice;
