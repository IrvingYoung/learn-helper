-- Migration 015: Twitter digest
-- Adds a new cron task type (twitter_digest) and supporting tables for
-- tracking X/Twitter accounts, storing fetched tweets, and recording
-- each digest run.

-- Extend cron_tasks with task_type and twitter-specific config columns.
-- Existing rows default to 'generic' (preserves backwards compat).
ALTER TABLE cron_tasks ADD COLUMN task_type TEXT NOT NULL DEFAULT 'generic';
ALTER TABLE cron_tasks ADD COLUMN since_hours INTEGER NOT NULL DEFAULT 24;
ALTER TABLE cron_tasks ADD COLUMN max_tweets_per_account INTEGER NOT NULL DEFAULT 50;
ALTER TABLE cron_tasks ADD COLUMN max_total_tweets INTEGER NOT NULL DEFAULT 200;

-- Accounts the user wants to track.
CREATE TABLE IF NOT EXISTS tracked_twitter_accounts (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  handle        TEXT NOT NULL UNIQUE,
  display_name  TEXT,
  enabled       INTEGER NOT NULL DEFAULT 1,
  added_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  notes         TEXT
);

CREATE INDEX IF NOT EXISTS idx_tracked_twitter_enabled
  ON tracked_twitter_accounts(enabled);

-- Raw tweets fetched from RSSHub.
CREATE TABLE IF NOT EXISTS tweets (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  tweet_id      TEXT NOT NULL UNIQUE,
  handle        TEXT NOT NULL,
  author_name   TEXT,
  text          TEXT NOT NULL,
  created_at    DATETIME NOT NULL,
  url           TEXT NOT NULL,
  metrics_json  TEXT,
  raw_json      TEXT NOT NULL,
  fetched_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  digest_run_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_tweets_handle_created
  ON tweets(handle, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tweets_digest_run
  ON tweets(digest_run_id);

-- One row per digest cron run. Linked to the wiki page via plan_id
-- (NULL for now since we don't use propose_plan any more — kept for
-- future debugging) and indirectly via the wiki_log.source.
CREATE TABLE IF NOT EXISTS twitter_digest_runs (
  id              TEXT PRIMARY KEY,
  cron_run_id     INTEGER REFERENCES cron_runs(id) ON DELETE SET NULL,
  started_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at     DATETIME,
  status          TEXT NOT NULL,
  tweets_fetched  INTEGER NOT NULL DEFAULT 0,
  wiki_page_id    INTEGER,
  error           TEXT
);

CREATE INDEX IF NOT EXISTS idx_twitter_digest_runs_started
  ON twitter_digest_runs(started_at DESC);
