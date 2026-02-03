-- Migration: add_instance_id_to_invoices
-- Description: Add redundant instance_id to invoices_v2 for query efficiency
-- Without this, finding an invoice's approval instance requires joining 3 tables:
-- invoices_v2 -> invoice_lists -> approval_instances
-- With instance_id directly on invoices_v2, reports and queries are much faster.

-- ============================================================================
-- Step 1: Add instance_id column with foreign key
-- ============================================================================

ALTER TABLE invoices_v2 ADD COLUMN instance_id INTEGER REFERENCES approval_instances(id) ON DELETE CASCADE;

-- ============================================================================
-- Step 2: Populate from invoice_lists join
-- ============================================================================

UPDATE invoices_v2
SET instance_id = (
    SELECT il.instance_id
    FROM invoice_lists il
    WHERE il.id = invoices_v2.invoice_list_id
);

-- ============================================================================
-- Step 3: Create index for efficient querying
-- ============================================================================

CREATE INDEX idx_invoices_v2_instance ON invoices_v2(instance_id);
