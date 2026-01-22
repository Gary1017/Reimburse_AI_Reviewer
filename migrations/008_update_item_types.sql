-- Migration 008: Update item_type CHECK constraint to include all expense categories
-- ARCH-011-F, ARCH-011-G: Add new expense categories (TRANSPORTATION, ENTERTAINMENT, TEAM_BUILDING, COMMUNICATION)

-- SQLite doesn't support ALTER TABLE to modify CHECK constraints
-- We need to recreate the table with the new constraint

-- Step 1: Create new table with updated constraint
CREATE TABLE reimbursement_items_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL,
    item_type TEXT NOT NULL CHECK(item_type IN ('TRAVEL', 'MEAL', 'ACCOMMODATION', 'EQUIPMENT', 'TRANSPORTATION', 'ENTERTAINMENT', 'TEAM_BUILDING', 'COMMUNICATION', 'OTHER')),
    description TEXT NOT NULL,
    amount REAL NOT NULL,
    currency TEXT DEFAULT 'CNY',
    receipt_attachment TEXT,
    ai_price_check TEXT,
    ai_policy_check TEXT,
    expense_date TEXT,
    vendor TEXT,
    business_purpose TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id)
);

-- Step 2: Copy existing data
INSERT INTO reimbursement_items_new
SELECT id, instance_id, item_type, description, amount, currency, receipt_attachment,
       ai_price_check, ai_policy_check, expense_date, vendor, business_purpose, created_at
FROM reimbursement_items;

-- Step 3: Drop old table
DROP TABLE reimbursement_items;

-- Step 4: Rename new table
ALTER TABLE reimbursement_items_new RENAME TO reimbursement_items;

-- Step 5: Recreate indexes
CREATE INDEX idx_reimbursement_items_instance ON reimbursement_items(instance_id);
CREATE INDEX idx_reimbursement_items_type ON reimbursement_items(item_type);
