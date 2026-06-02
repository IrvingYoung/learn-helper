-- Migration 013: Cron tasks
-- User-configurable scheduled tasks that trigger the AI to autonomously
-- execute a user-defined prompt at cron-scheduled times.

CREATE TABLE IF NOT EXISTS cron_tasks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  cron_expr TEXT NOT NULL,
  prompt TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  auto_approve INTEGER NOT NULL DEFAULT 1,
  max_steps INTEGER NOT NULL DEFAULT 10,
  timeout_sec INTEGER NOT NULL DEFAULT 300,
  next_run_at DATETIME,
  last_run_at DATETIME,
  last_status TEXT,
  last_error TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cron_tasks_enabled_next_run
  ON cron_tasks(enabled, next_run_at);

CREATE TABLE IF NOT EXISTS cron_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id INTEGER NOT NULL REFERENCES cron_tasks(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at DATETIME,
  duration_ms INTEGER,
  output_summary TEXT,
  error TEXT,
  write_count INTEGER NOT NULL DEFAULT 0,
  steps_used INTEGER NOT NULL DEFAULT 0,
  conversation_id INTEGER REFERENCES conversations(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_cron_runs_task_id
  ON cron_runs(task_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_cron_runs_status
  ON cron_runs(status);
