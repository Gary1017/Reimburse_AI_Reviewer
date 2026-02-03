-- Migration: fix_amount_precision
-- Description: Convert float amounts to INTEGER (stored in cents/分) to avoid precision loss
-- In financial scenarios, using float is dangerous due to precision loss.
-- Storing as INTEGER in cents (分) eliminates this risk: 100.50 CNY = 10050 分

-- ============================================================================
-- Step 1: Add new INTEGER columns for amounts in cents
-- ============================================================================

-- reimbursement_items: amount -> amount_cents
ALTER TABLE reimbursement_items ADD COLUMN amount_cents INTEGER;

-- invoice_lists: total_invoice_amount -> total_invoice_amount_cents
ALTER TABLE invoice_lists ADD COLUMN total_invoice_amount_cents INTEGER DEFAULT 0;

-- invoices_v2: invoice_amount -> invoice_amount_cents
ALTER TABLE invoices_v2 ADD COLUMN invoice_amount_cents INTEGER;

-- ============================================================================
-- Step 2: Migrate existing data (multiply by 100 and round)
-- ============================================================================

UPDATE reimbursement_items
SET amount_cents = CAST(ROUND(amount * 100) AS INTEGER)
WHERE amount IS NOT NULL;

UPDATE invoice_lists
SET total_invoice_amount_cents = CAST(ROUND(total_invoice_amount * 100) AS INTEGER)
WHERE total_invoice_amount IS NOT NULL;

UPDATE invoices_v2
SET invoice_amount_cents = CAST(ROUND(invoice_amount * 100) AS INTEGER)
WHERE invoice_amount IS NOT NULL;

-- ============================================================================
-- Note: SQLite does not support DROP COLUMN in older versions.
-- The old float columns (amount, total_invoice_amount, invoice_amount) are kept
-- for backwards compatibility but should NOT be used in new code.
-- All new code should use the *_cents columns.
-- ============================================================================

-- Add comments via table recreation is complex in SQLite, so we document here:
-- DEPRECATED: reimbursement_items.amount - use amount_cents instead
-- DEPRECATED: invoice_lists.total_invoice_amount - use total_invoice_amount_cents instead
-- DEPRECATED: invoices_v2.invoice_amount - use invoice_amount_cents instead
