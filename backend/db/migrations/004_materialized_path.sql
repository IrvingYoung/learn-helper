-- Migration 004: Add materialized path for wiki_pages tree
-- Uses recursive CTE to handle arbitrarily deep trees.

BEGIN;

ALTER TABLE wiki_pages ADD COLUMN path TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_wiki_pages_path ON wiki_pages(path);

-- Backfill path for existing records using recursive CTE.
-- This handles arbitrarily deep trees in a single pass.
WITH RECURSIVE path_cte(id, path) AS (
    SELECT id, CAST(id AS TEXT) || '/'
    FROM wiki_pages WHERE parent_id IS NULL
    UNION ALL
    SELECT wp.id, path_cte.path || CAST(wp.id AS TEXT) || '/'
    FROM wiki_pages wp, path_cte
    WHERE wp.parent_id = path_cte.id
)
UPDATE wiki_pages SET path = (
    SELECT path FROM path_cte WHERE path_cte.id = wiki_pages.id
)
WHERE wiki_pages.path = '';

COMMIT;
