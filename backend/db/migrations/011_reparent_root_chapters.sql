-- Migration 010_add_plan_stage.sql added the `stage` column.
-- This file repairs dirty data from the original Go learning plan bug:
-- 11 chapter pages were created with parent_id = NULL (at root) instead
-- of under "Go 语言从基础到精通" (id = 32). All 11 pages originally
-- referenced {{action:plan-1780318840946982000-a1.page_id}} which
-- resolves to id 32, so we reparent them all to 32 and recompute paths.

-- SAFETY: Wrap in transaction. To rollback manually:
--   sqlite3 learn-helper.db "ROLLBACK;"  (only if BEGIN TRANSACTION was issued)
BEGIN TRANSACTION;

-- Reparent
UPDATE wiki_pages
   SET parent_id = 32
 WHERE id IN (21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31)
   AND parent_id IS NULL;

-- Recompute paths: parent_path + new_id + "/"
-- Parent 32 has path "13/14/16/32/", so children become "13/14/16/32/<id>/"
UPDATE wiki_pages SET path = '13/14/16/32/21/'  WHERE id = 21;
UPDATE wiki_pages SET path = '13/14/16/32/22/'  WHERE id = 22;
UPDATE wiki_pages SET path = '13/14/16/32/23/'  WHERE id = 23;
UPDATE wiki_pages SET path = '13/14/16/32/24/'  WHERE id = 24;
UPDATE wiki_pages SET path = '13/14/16/32/25/'  WHERE id = 25;
UPDATE wiki_pages SET path = '13/14/16/32/26/'  WHERE id = 26;
UPDATE wiki_pages SET path = '13/14/16/32/27/'  WHERE id = 27;
UPDATE wiki_pages SET path = '13/14/16/32/28/'  WHERE id = 28;
UPDATE wiki_pages SET path = '13/14/16/32/29/'  WHERE id = 29;
UPDATE wiki_pages SET path = '13/14/16/32/30/'  WHERE id = 30;
UPDATE wiki_pages SET path = '13/14/16/32/31/'  WHERE id = 31;

COMMIT;

-- Verify:
-- SELECT COUNT(*) FROM wiki_pages WHERE parent_id IS NULL AND page_type IN ('concept', 'overview');
-- Expected: 0 (down from 11)
-- SELECT id, title, parent_id, path FROM wiki_pages WHERE id BETWEEN 21 AND 31 ORDER BY id;
