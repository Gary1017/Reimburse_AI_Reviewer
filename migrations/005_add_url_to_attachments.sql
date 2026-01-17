-- Migration 005: Add URL column to attachments table for Phase 4 downloads

-- Add url column to attachments table to store Lark Drive download URLs
ALTER TABLE attachments ADD COLUMN url TEXT;

-- Create index on url for efficient lookup if needed
CREATE INDEX IF NOT EXISTS idx_attachments_url ON attachments(url) WHERE url IS NOT NULL;
