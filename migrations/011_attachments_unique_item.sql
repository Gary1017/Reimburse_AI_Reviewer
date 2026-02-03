-- Migration 010b: Add UNIQUE constraint on attachments.item_id
-- Database Schema Refactoring Phase 1: Enforce 1:1 relationship between attachments and reimbursement_items
-- Each reimbursement item has exactly ONE attachment (the invoice file)

-- Note: The existing UNIQUE(item_id, file_name) constraint from 004_add_attachments.sql
-- is more permissive. This adds a stricter 1:1 constraint.
-- We create this as a unique index rather than altering the table constraint.

CREATE UNIQUE INDEX IF NOT EXISTS idx_attachments_item_unique ON attachments(item_id);
