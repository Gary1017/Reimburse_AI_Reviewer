-- Add invoice tracking tables for uniqueness checking

-- Invoices table to track all submitted invoices
CREATE TABLE IF NOT EXISTS invoices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_code TEXT NOT NULL,             -- 发票代码
    invoice_number TEXT NOT NULL,           -- 发票号码
    unique_id TEXT UNIQUE NOT NULL,         -- Combination of code + number
    instance_id INTEGER NOT NULL,           -- Reference to approval instance
    file_token TEXT,                        -- Lark file token
    file_path TEXT,                         -- Local file path after download
    invoice_date DATE,                      -- 发票日期
    invoice_amount DECIMAL(10,2),           -- 发票金额
    seller_name TEXT,                       -- 销售方名称
    seller_tax_id TEXT,                     -- 销售方税号
    buyer_name TEXT,                        -- 购买方名称
    buyer_tax_id TEXT,                      -- 购买方税号
    extracted_data TEXT,                    -- Full JSON of extracted data
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

CREATE INDEX idx_invoices_unique_id ON invoices(unique_id);
CREATE INDEX idx_invoices_instance ON invoices(instance_id);
CREATE INDEX idx_invoices_code_number ON invoices(invoice_code, invoice_number);

-- Invoice validation results table
CREATE TABLE IF NOT EXISTS invoice_validations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id INTEGER NOT NULL,
    validation_type TEXT NOT NULL CHECK(validation_type IN ('UNIQUENESS', 'FORMAT', 'AMOUNT', 'AI_CHECK')),
    is_valid BOOLEAN NOT NULL,
    error_message TEXT,
    validation_data TEXT,               -- JSON blob with validation details
    validated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (invoice_id) REFERENCES invoices(id) ON DELETE CASCADE
);

CREATE INDEX idx_invoice_validations_invoice ON invoice_validations(invoice_id);
CREATE INDEX idx_invoice_validations_type ON invoice_validations(validation_type);

-- Add invoice-related fields to reimbursement_items table
ALTER TABLE reimbursement_items ADD COLUMN invoice_id INTEGER REFERENCES invoices(id);

-- Create index for faster lookups
CREATE INDEX idx_reimbursement_items_invoice ON reimbursement_items(invoice_id);
