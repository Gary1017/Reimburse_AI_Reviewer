-- Migration 004: Add attachments table for Phase 3 attachment handling

-- Create attachments table to track downloaded files
CREATE TABLE IF NOT EXISTS attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id INTEGER NOT NULL,
    instance_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT,
    file_size INTEGER DEFAULT 0,
    mime_type TEXT,
    download_status TEXT NOT NULL DEFAULT 'PENDING', -- PENDING, COMPLETED, FAILED
    error_message TEXT,
    downloaded_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Foreign keys
    FOREIGN KEY (item_id) REFERENCES reimbursement_items(id) ON DELETE CASCADE,
    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE,
    
    -- Constraints
    UNIQUE(item_id, file_name)
);

-- Create indices for efficient querying
CREATE INDEX IF NOT EXISTS idx_attachments_item ON attachments(item_id);
CREATE INDEX IF NOT EXISTS idx_attachments_instance ON attachments(instance_id);
CREATE INDEX IF NOT EXISTS idx_attachments_status ON attachments(download_status);
CREATE INDEX IF NOT EXISTS idx_attachments_created ON attachments(created_at);
