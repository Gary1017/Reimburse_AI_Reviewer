-- Migration 010: Add invoices_v2 table
-- Database Schema Refactoring Phase 1: New invoice tracking with 1:1 attachment relationship
-- Replaces old invoices table - file_token and file_path are stored in attachments table

CREATE TABLE IF NOT EXISTS invoices_v2 (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_list_id INTEGER NOT NULL,
    attachment_id INTEGER UNIQUE NOT NULL,  -- 1:1 with attachments (each invoice IS an attachment)
    item_id INTEGER,                         -- Derived from attachment, for convenience

    -- Invoice identification (from GPT-4 extraction)
    invoice_code TEXT NOT NULL,              -- 发票代码
    invoice_number TEXT NOT NULL,            -- 发票号码
    unique_id TEXT UNIQUE NOT NULL,          -- Combination of code + number for uniqueness check

    -- Extracted data (GPT-4 Vision result)
    invoice_date DATE,                       -- 发票日期
    invoice_amount DECIMAL(12,2),            -- 发票金额
    seller_name TEXT,                        -- 销售方名称
    seller_tax_id TEXT,                      -- 销售方税号
    buyer_name TEXT,                         -- 购买方名称
    buyer_tax_id TEXT,                       -- 购买方税号
    extracted_data TEXT,                     -- Full JSON from GPT-4

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (invoice_list_id) REFERENCES invoice_lists(id) ON DELETE CASCADE,
    FOREIGN KEY (attachment_id) REFERENCES attachments(id) ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES reimbursement_items(id) ON DELETE SET NULL
);

-- Create indices for efficient querying
CREATE INDEX IF NOT EXISTS idx_invoices_v2_list ON invoices_v2(invoice_list_id);
CREATE INDEX IF NOT EXISTS idx_invoices_v2_attachment ON invoices_v2(attachment_id);
CREATE INDEX IF NOT EXISTS idx_invoices_v2_item ON invoices_v2(item_id);
CREATE INDEX IF NOT EXISTS idx_invoices_v2_unique ON invoices_v2(unique_id);
