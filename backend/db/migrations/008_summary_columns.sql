-- Migration 008: Per-page summary columns
-- Each page gets a 1-2 sentence AI-generated summary, async.
-- summary_status: 'empty' | 'pending' | 'ready' | 'failed'
-- summary_content_hash: MD5(content+title), used to detect staleness

ALTER TABLE wiki_pages ADD COLUMN summary TEXT NOT NULL DEFAULT '';
ALTER TABLE wiki_pages ADD COLUMN summary_status TEXT NOT NULL DEFAULT 'empty';
ALTER TABLE wiki_pages ADD COLUMN summary_generated_at DATETIME;
ALTER TABLE wiki_pages ADD COLUMN summary_content_hash TEXT;

-- Denormalized link/backlink counts (avoid JOIN on every context render)
ALTER TABLE wiki_pages ADD COLUMN link_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE wiki_pages ADD COLUMN backlink_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE wiki_pages ADD COLUMN tags_normalized TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_wiki_pages_summary_status ON wiki_pages(summary_status);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_tags_normalized ON wiki_pages(tags_normalized);
