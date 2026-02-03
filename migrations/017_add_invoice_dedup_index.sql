-- Migration: add_invoice_dedup_index
-- Description: Add composite unique index on (invoice_code, invoice_number) for deduplication
-- This is CRITICAL for preventing double-reimbursement of the same invoice.
-- The combination of 发票代码 + 发票号码 uniquely identifies a Chinese invoice.

-- ============================================================================
-- Composite Unique Index for Invoice Deduplication
-- ============================================================================

-- This index prevents the same invoice from being submitted twice across
-- ANY reimbursement request in the system (not just within the same instance).
-- If someone tries to submit an invoice that was already reimbursed, the
-- INSERT will fail with a UNIQUE constraint violation.

CREATE UNIQUE INDEX idx_invoices_v2_code_number
ON invoices_v2(invoice_code, invoice_number);

-- Note: The existing unique_id column (which is invoice_code + invoice_number concatenated)
-- also has a UNIQUE constraint, but having the composite index directly on the
-- source columns is:
-- 1. More explicit and self-documenting
-- 2. Allows efficient duplicate checking without string concatenation
-- 3. Supports queries like: WHERE invoice_code = ? AND invoice_number = ?
