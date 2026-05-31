-- Allow NULL conversation_id for user-initiated plans
-- User plans (created from tree context menu) are not associated with any conversation

-- SQLite doesn't support ALTER COLUMN, so we recreate the table
CREATE TABLE plans_new (
    id TEXT PRIMARY KEY,
    conversation_id INTEGER REFERENCES conversations(id) ON DELETE CASCADE,
    reasoning TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'confirmed', 'executing', 'completed', 'rejected', 'completed_with_failures')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    executed_at TEXT,
    outline TEXT,
    phase_index INTEGER,
    total_phases INTEGER
);

INSERT INTO plans_new SELECT * FROM plans;
DROP TABLE plans;
ALTER TABLE plans_new RENAME TO plans;

CREATE INDEX idx_plans_conversation ON plans(conversation_id);
