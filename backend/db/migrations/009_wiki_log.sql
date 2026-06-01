-- Migration 009: Wiki write log
-- Append-only record of every page write, for both human browsing
-- and AI's "recent activity" context.
-- Synchronous writes (same transaction as page write) - always consistent.

CREATE TABLE IF NOT EXISTS wiki_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  action TEXT NOT NULL,
  page_id INTEGER,
  page_title TEXT NOT NULL,
  page_path TEXT,
  source TEXT NOT NULL DEFAULT 'plan',
  summary TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_wiki_log_created_at ON wiki_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_wiki_log_page_id ON wiki_log(page_id);
