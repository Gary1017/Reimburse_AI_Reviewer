-- Migration: add_attachment_file_type
-- Description: Add file_type enum to distinguish between invoices and supporting documents
-- Not all attachments are tax invoices (发票) - some may be supporting documents.

-- ============================================================================
-- Add file_type column with enum constraint
-- ============================================================================

ALTER TABLE attachments ADD COLUMN file_type TEXT DEFAULT 'INVOICE'
    CHECK(file_type IN ('INVOICE', 'OTHER'));

-- File types:
-- INVOICE - Tax invoice (发票) - can be extracted and validated
-- OTHER   - Other supporting documents (receipts, contracts, etc.)

-- ============================================================================
-- Create index for filtering by file type
-- ============================================================================

CREATE INDEX idx_attachments_file_type ON attachments(file_type);

-- ============================================================================
-- Update existing records
-- Assume all existing attachments are invoices (since that was the previous behavior)
-- ============================================================================

UPDATE attachments SET file_type = 'INVOICE' WHERE file_type IS NULL;
