-- Migration 011: Add approval_tasks table
-- Database Schema Refactoring Phase 1: Unified task table (definition + result)
-- Aligned with Lark's task_list structure, supports both AI and human review tasks

CREATE TABLE IF NOT EXISTS approval_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id INTEGER NOT NULL,

    -- Lark task_list mapping (NULL for AI-generated tasks)
    lark_task_id TEXT,

    -- Task type and workflow position
    task_type TEXT NOT NULL,  -- AI_REVIEW | HUMAN_REVIEW
    sequence_number INTEGER NOT NULL DEFAULT 0,

    -- Lark node information
    node_id TEXT,
    node_name TEXT,
    custom_node_id TEXT,
    approval_type TEXT,

    -- Assignee (for Lark governance accountability)
    assignee_user_id TEXT,
    assignee_open_id TEXT,

    -- Status and timing
    status TEXT NOT NULL DEFAULT 'PENDING',  -- PENDING | IN_PROGRESS | COMPLETED | REJECTED
    start_time TEXT,
    end_time TEXT,

    -- Workflow control
    is_current BOOLEAN DEFAULT FALSE,

    -- AI decision tracking (for technical auditing)
    is_ai_decision BOOLEAN DEFAULT FALSE,

    -- Result fields (merged from task_results + approval_history)
    decision TEXT,       -- PASS | NEEDS_REVIEW | FAIL | APPROVED | REJECTED
    confidence REAL,
    result_data TEXT,    -- JSON blob with detailed results
    violations TEXT,     -- JSON array of violation strings
    completed_by TEXT,

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (instance_id) REFERENCES approval_instances(id) ON DELETE CASCADE
);

-- Create indices for efficient querying
CREATE INDEX IF NOT EXISTS idx_approval_tasks_instance ON approval_tasks(instance_id);
CREATE INDEX IF NOT EXISTS idx_approval_tasks_status ON approval_tasks(status);
CREATE INDEX IF NOT EXISTS idx_approval_tasks_lark_id ON approval_tasks(lark_task_id);
CREATE INDEX IF NOT EXISTS idx_approval_tasks_type ON approval_tasks(task_type);

-- Trigger to update updated_at on approval_tasks
CREATE TRIGGER IF NOT EXISTS update_approval_tasks_timestamp
AFTER UPDATE ON approval_tasks
BEGIN
    UPDATE approval_tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
