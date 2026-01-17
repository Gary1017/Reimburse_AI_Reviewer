-- Migration 006: Add invoice audit fields to attachments table
-- ARCH-011-E: Support for AI invoice processing and audit results

-- Add new columns for invoice processing status and audit results
ALTER TABLE attachments ADD COLUMN processed_at TIMESTAMP;
ALTER TABLE attachments ADD COLUMN audit_result TEXT;

-- Create index for processed attachments
CREATE INDEX IF NOT EXISTS idx_attachments_processed ON attachments(processed_at);

-- Update download_status comment: PENDING, COMPLETED, FAILED, PROCESSING, PROCESSED, AUDIT_FAILED
