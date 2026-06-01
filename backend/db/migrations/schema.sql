-- Learn Helper Database Schema
-- SQLite

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS topics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id INTEGER REFERENCES topics(id),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    key_points TEXT,
    content TEXT,
    code_examples TEXT,
    common_mistakes TEXT,
    difficulty TEXT DEFAULT 'beginner' CHECK(difficulty IN ('beginner', 'intermediate', 'advanced')),
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS exercises (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER NOT NULL REFERENCES topics(id),
    type TEXT DEFAULT 'algorithm' CHECK(type IN ('algorithm', 'system_design', 'knowledge')),
    title TEXT NOT NULL,
    description TEXT,
    difficulty TEXT DEFAULT 'medium' CHECK(difficulty IN ('easy', 'medium', 'hard')),
    sort_order INTEGER DEFAULT 0,
    tags TEXT,
    hints TEXT,
    solution_outline TEXT,
    solution_detail TEXT,
    common_errors TEXT,
    time_complexity_expected TEXT,
    space_complexity_expected TEXT,
    sample_code TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS learning_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER REFERENCES topics(id),
    exercise_id INTEGER REFERENCES exercises(id),
    status TEXT DEFAULT 'not_started' CHECK(status IN ('not_started', 'in_progress', 'completed')),
    mastery_level INTEGER CHECK(mastery_level >= 1 AND mastery_level <= 5),
    notes TEXT,
    last_reviewed_at DATETIME,
    review_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER REFERENCES topics(id),
    exercise_id INTEGER REFERENCES exercises(id),
    context_type TEXT CHECK(context_type IN ('topic', 'exercise', 'dashboard')),
    title TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    model_provider TEXT,
    token_count INTEGER,
    tool_calls TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ai_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    model_name TEXT NOT NULL,
    api_key TEXT NOT NULL,
    is_active INTEGER DEFAULT 0,
    config TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_topics_parent ON topics(parent_id);
CREATE INDEX IF NOT EXISTS idx_topics_slug ON topics(slug);
CREATE INDEX IF NOT EXISTS idx_exercises_topic ON exercises(topic_id);
CREATE INDEX IF NOT EXISTS idx_learning_records_topic ON learning_records(topic_id);
CREATE INDEX IF NOT EXISTS idx_learning_records_exercise ON learning_records(exercise_id);
CREATE TABLE IF NOT EXISTS wiki_pages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    title           TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    page_type       TEXT NOT NULL DEFAULT 'entity',
    content         TEXT NOT NULL DEFAULT '',
    tags            TEXT DEFAULT '[]',
    parent_id       INTEGER REFERENCES wiki_pages(id),
    path            TEXT NOT NULL DEFAULT '',
    links           TEXT NOT NULL DEFAULT '[]',
    backlinks       TEXT NOT NULL DEFAULT '[]',
    content_status  TEXT NOT NULL DEFAULT 'empty',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Migration 008: Per-page summary columns
ALTER TABLE wiki_pages ADD COLUMN summary TEXT NOT NULL DEFAULT '';
ALTER TABLE wiki_pages ADD COLUMN summary_status TEXT NOT NULL DEFAULT 'empty';
ALTER TABLE wiki_pages ADD COLUMN summary_generated_at DATETIME;
ALTER TABLE wiki_pages ADD COLUMN summary_content_hash TEXT;
ALTER TABLE wiki_pages ADD COLUMN link_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE wiki_pages ADD COLUMN backlink_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE wiki_pages ADD COLUMN tags_normalized TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_wiki_pages_summary_status ON wiki_pages(summary_status);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_tags_normalized ON wiki_pages(tags_normalized);

-- Migration 009: Wiki write log
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

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_parent ON wiki_pages(parent_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_slug ON wiki_pages(slug);

CREATE TABLE IF NOT EXISTS plans (
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

CREATE TABLE IF NOT EXISTS plan_actions (
    id TEXT PRIMARY KEY,
    plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK(type IN ('create_page', 'update_page', 'delete_page', 'link_pages', 'move_page')),
    params TEXT NOT NULL DEFAULT '{}',
    depends_on TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'in_progress', 'completed', 'failed', 'skipped')),
    result TEXT,
    error TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_plans_conversation ON plans(conversation_id);
CREATE INDEX IF NOT EXISTS idx_plan_actions_plan ON plan_actions(plan_id);