-- Migration 003: Add expense detail fields to reimbursement_items table

-- Add missing columns to support structured data extraction
ALTER TABLE reimbursement_items ADD COLUMN expense_date DATE;
ALTER TABLE reimbursement_items ADD COLUMN vendor TEXT;
ALTER TABLE reimbursement_items ADD COLUMN business_purpose TEXT;

-- Create indices for frequently queried fields
CREATE INDEX IF NOT EXISTS idx_reimbursement_items_date ON reimbursement_items(expense_date);
CREATE INDEX IF NOT EXISTS idx_reimbursement_items_vendor ON reimbursement_items(vendor);
