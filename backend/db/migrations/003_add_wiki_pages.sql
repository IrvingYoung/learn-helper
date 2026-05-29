-- Migration 003: Add wiki_pages table for LLM Wiki

CREATE TABLE IF NOT EXISTS wiki_pages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    title           TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    page_type       TEXT NOT NULL DEFAULT 'entity',
    content         TEXT NOT NULL DEFAULT '',
    tags            TEXT DEFAULT '[]',
    parent_id       INTEGER REFERENCES wiki_pages(id),
    content_status  TEXT NOT NULL DEFAULT 'empty',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_parent ON wiki_pages(parent_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_slug ON wiki_pages(slug);

INSERT OR IGNORE INTO wiki_pages (title, slug, page_type, content_status, sort_order)
VALUES ('概览', 'overview', 'overview', 'published', 0);
