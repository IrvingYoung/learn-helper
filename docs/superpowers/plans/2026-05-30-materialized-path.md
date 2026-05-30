# Materialized Path Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor wiki_pages from adjacency list to Materialized Path, enabling efficient subtree queries and reducing frontend tree-building overhead.

**Architecture:** Add `path TEXT` column + index to wiki_pages. Maintain `parent_id` for direct parent lookups. All writes (create/move/delete) atomically update path. GetWikiPageTree query includes path. A new `GetSubtreePages` query enables AI to request only relevant subtree content instead of the full forest.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), sqlc, React 19

---

### Task 1: Add migration SQL for path column + index + data backfill

**Files:**
- Create: `backend/db/migrations/004_materialized_path.sql`
- Modify: (none, standalone migration script)

- [ ] **Step 1: Write the migration SQL**

```sql
-- Migration 004: Add materialized path for wiki_pages tree

-- Add path column
ALTER TABLE wiki_pages ADD COLUMN path TEXT NOT NULL DEFAULT '';

-- Create index for subtree lookups
CREATE INDEX IF NOT EXISTS idx_wiki_pages_path ON wiki_pages(path);

-- Backfill path for existing records
-- We need to do this in multiple passes since SQLite doesn't support recursive CTEs easily
-- Pass 1: root nodes (parent_id IS NULL)
UPDATE wiki_pages SET path = CAST(id AS TEXT) || '/'
WHERE parent_id IS NULL AND path = '';

-- Pass 2: depth-1 children
UPDATE wiki_pages SET path = (SELECT path FROM wiki_pages WHERE id = wiki_pages.parent_id) || CAST(wiki_pages.id AS TEXT) || '/'
WHERE parent_id IS NOT NULL AND path = ''
AND EXISTS (SELECT 1 FROM wiki_pages p2 WHERE p2.id = wiki_pages.parent_id AND p2.path != '');

-- Pass 3: depth-2 children (repeat until no more updates)
UPDATE wiki_pages SET path = (SELECT path FROM wiki_pages WHERE id = wiki_pages.parent_id) || CAST(wiki_pages.id AS TEXT) || '/'
WHERE parent_id IS NOT NULL AND path = ''
AND EXISTS (SELECT 1 FROM wiki_pages p2 WHERE p2.id = wiki_pages.parent_id AND p2.path != '');
```

For a personal wiki the tree depth rarely exceeds 3-4, so 4 passes in the migration script is sufficient. The full Go-based BFS is overkill for a migration that runs once.

- [ ] **Step 2: Commit**

```bash
git add backend/db/migrations/004_materialized_path.sql
git commit -m "feat: add migration 004 for materialized path column + backfill"
```

---

### Task 2: Update sqlc queries

**Files:**
- Modify: `backend/db/migrations/queries.sql`
- Modify: `backend/db/migrations/schema.sql`
- Regenerates: `backend/internal/model/queries.sql.go`

- [ ] **Step 0: Update schema.sql for sqlc**

sqlc reads `schema.sql` to determine column types and generate Go structs. Add the `path` column:

```sql
-- In backend/db/migrations/schema.sql, update wiki_pages table:
CREATE TABLE IF NOT EXISTS wiki_pages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    title           TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    page_type       TEXT NOT NULL DEFAULT 'entity',
    content         TEXT NOT NULL DEFAULT '',
    tags            TEXT DEFAULT '[]',
    parent_id       INTEGER REFERENCES wiki_pages(id),
    path            TEXT NOT NULL DEFAULT '',
    content_status  TEXT NOT NULL DEFAULT 'empty',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Also add the index:
```sql
CREATE INDEX IF NOT EXISTS idx_wiki_pages_path ON wiki_pages(path);
```

- [ ] **Step 1: Add path to GetWikiPageTree**

Update the existing query to include path:

```sql
-- name: GetWikiPageTree :many
SELECT id, title, slug, page_type, content_status, parent_id, sort_order, path
FROM wiki_pages
ORDER BY sort_order, id;
```

- [ ] **Step 2: Add path to GetAllWikiPages**

```sql
-- name: GetAllWikiPages :many
SELECT id, title, slug, page_type, content, tags, parent_id, content_status, sort_order, created_at, updated_at, path
FROM wiki_pages
ORDER BY sort_order, id;
```

- [ ] **Step 3: Add path to GetWikiPageBySlug**

```sql
-- name: GetWikiPageBySlug :one
SELECT id, title, slug, page_type, content, tags, parent_id, content_status, sort_order, created_at, updated_at, path
FROM wiki_pages
WHERE slug = ?;
```

- [ ] **Step 4: Add new GetSubtreePages query**

```sql
-- name: GetSubtreePages :many
SELECT id, title, slug, page_type, content_status, parent_id, sort_order, path
FROM wiki_pages
WHERE path LIKE ? || '%'
ORDER BY path, sort_order, id;
```

- [ ] **Step 5: Add new GetPathByID query (for move/delete operations)**

```sql
-- name: GetWikiPagePathByID :one
SELECT path FROM wiki_pages WHERE id = ?;
```

- [ ] **Step 6: Add GetWikiPageByID path column**

The existing query `SELECT * FROM wiki_pages WHERE id = ?` already returns all columns. Since we added the `path` column to the table, the existing `GetWikiPageByID` query that uses `SELECT *` should automatically include `path` when regenerated. No change needed.

Actually this uses `SELECT *` so sqlc will auto-include path. Verify after regeneration.

- [ ] **Step 7: Add UpdateWikiPagePath query (for move and delete)**

```sql
-- name: UpdateWikiPagePath :exec
UPDATE wiki_pages SET path = ? WHERE id = ?;
```

- [ ] **Step 8: Add BatchUpdateWikiPagePath query (for subtree moves)**

```sql
-- name: BatchUpdateWikiPagePath :exec
UPDATE wiki_pages SET path = REPLACE(path, ?, ?) WHERE path LIKE ? || '%';
```

- [ ] **Step 9: Regenerate Go code**

```bash
cd backend && sqlc generate
```

Verify `queries.sql.go` now includes `Path string` in `GetWikiPageTreeRow`, `CreateWikiPageParams`, `UpdateWikiPageParams`, and new `GetSubtreePages` / `GetWikiPagePathByID` / `UpdateWikiPagePath` / `BatchUpdateWikiPagePath` functions.

- [ ] **Step 10: Commit**

```bash
git add backend/db/migrations/queries.sql backend/db/migrations/schema.sql backend/internal/model/queries.sql.go
git commit -m "feat: add path column to wiki queries, add subtree and path update queries"
```

---

### Task 3: Update WikiPage model struct + add Path field

**Files:**
- Modify: `backend/internal/model/models.go`

- [ ] **Step 1: Add Path field to WikiPage struct**

```go
type WikiPage struct {
    ID            int64
    Title         string
    Slug          string
    PageType      string
    Content       string
    Tags          sql.NullString
    ParentID      sql.NullInt64
    Path          string
    ContentStatus string
    SortOrder     int64
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/model/models.go
git commit -m "feat: add Path field to WikiPage model"
```

---

### Task 4: Update wiki handler — tree, create, move, delete

**Files:**
- Modify: `backend/internal/handler/wiki.go`

- [ ] **Step 1: Update WikiTreeNode struct to include Path**

```go
type WikiTreeNode struct {
    ID            int64          `json:"id"`
    Title         string         `json:"title"`
    Slug          string         `json:"slug"`
    PageType      string         `json:"page_type"`
    ContentStatus string         `json:"content_status"`
    ParentID      *int64         `json:"parent_id"`
    Path          string         `json:"path"`
    SortOrder     int64          `json:"sort_order"`
    Children      []WikiTreeNode `json:"children,omitempty"`
}
```

- [ ] **Step 2: Update GetWikiTree to populate Path**

In the tree building loop, add `Path: p.Path` when creating WikiTreeNode.

- [ ] **Step 3: Update CreateWikiPage to set path after insert**

After `h.queries.CreateWikiPage(...)` returns, compute path and update:

```go
// After successfully creating a page:
id, _ := result.LastInsertId()
if id > 0 {
    var path string
    if req.ParentID != nil && *req.ParentID > 0 {
        // Fetch parent's path and append this ID
        parentPath, err := h.queries.GetWikiPagePathByID(ctx, *req.ParentID)
        if err == nil {
            path = parentPath + fmt.Sprintf("%d/", id)
        }
    } else {
        path = fmt.Sprintf("%d/", id)
    }
    if path != "" {
        h.queries.UpdateWikiPagePath(ctx, model.UpdateWikiPagePathParams{
            Path: path,
            ID:   id,
        })
    }
}
```

- [ ] **Step 4: Update MoveWikiPage to batch-update subtree paths**

After the existing parent_id update succeeds, add path migration:

```go
// After updating parent_id successfully:
// Fetch the moved page's new path (based on new parent)
var newPath string
if parentID.Valid && parentID.Int64 > 0 {
    parentPath, err := h.queries.GetWikiPagePathByID(ctx, parentID.Int64)
    if err == nil {
        newPath = parentPath + fmt.Sprintf("%d/", id)
    }
} else {
    newPath = fmt.Sprintf("%d/", id)
}

// Fetch old path before updating
oldPath := page.Path  // need to fetch this earlier — add page.Path to the GetWikiPageByID result

// Update the moved page's own path
h.queries.UpdateWikiPagePath(ctx, model.UpdateWikiPagePathParams{
    Path: newPath,
    ID:   id,
})

// Batch update descendants
if oldPath != "" && newPath != "" {
    h.queries.BatchUpdateWikiPagePath(ctx, model.BatchUpdateWikiPagePathParams{
        OldPrefix: oldPath,
        NewPrefix: newPath,
        LikePath:  oldPath,
    })
}
```

**Note:** To get `page.Path`, the existing GetWikiPageByID should now return Path since it uses `SELECT *`. Need to access it after the path column was added to the table and model.

Alternatively, since MoveWikiPage already calls `GetWikiPageByID`, the returned `page.Path` should be available if the model was updated.

- [ ] **Step 5: Update DeleteWikiPage to reparent children's paths**

After deletion, orphaned children need their paths fixed:

```go
// After deleting page, reparent children's paths
// Fetch deleted page's path before deletion
// (store it from the initial GetWikiPageByID call that should exist)

// Remove the deleted node's segment from descendants' paths
if deletedPath != "" {
    h.queries.BatchUpdateWikiPagePath(ctx, model.BatchUpdateWikiPagePathParams{
        OldPrefix: deletedPath,
        NewPrefix: "",   // remove the segment entirely
        LikePath:  deletedPath,
    })
}
```

Also add an explicit path fetch before deletion. The current DeleteWikiPage only gets the id from the URL, not the full page. Need to add:

```go
// Before deleting, fetch page to get its path for cascade update
page, err := h.queries.GetWikiPageByID(ctx, id)
if err != nil {
    http.Error(w, "Page not found", http.StatusNotFound)
    return
}
deletedPath := page.Path
// ... then delete, then batch-update children paths
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler/wiki.go
git commit -m "feat: add path management to wiki handler CRUD operations"
```

---

### Task 5: Update AI handler — subtree context support

**Files:**
- Modify: `backend/internal/handler/ai.go`

- [ ] **Step 1: Update buildWikiContext to accept optional targetPageID**

Change signature from no-args to accept optional page ID for subtree mode:

```go
func (h *AIHandler) buildWikiContext(ctx context.Context, targetPageID *int64) string {
    if targetPageID != nil {
        // Subtree mode: only load the target page and its descendants
        parentPage, err := h.queries.GetWikiPageByID(ctx, *targetPageID)
        if err != nil {
            return fmt.Sprintf("（未找到目标页面 ID=%d）", *targetPageID)
        }
        pages, err := h.queries.GetSubtreePages(ctx, parentPage.Path)
        if err != nil || len(pages) == 0 {
            return fmt.Sprintf("（页面「%s」下没有子节点）", parentPage.Title)
        }
        // Build subtree context (same rendering logic as full tree, but scoped)
        return h.renderTreeContext(pages, parentPage.Title)
    }

    // Existing full-tree behavior unchanged
    pages, err := h.queries.GetWikiPageTree(ctx)
    if err != nil || len(pages) == 0 {
        return "（知识库为空）"
    }
    // ... existing tree building and rendering code ...
}
```

- [ ] **Step 2: Extract renderTreeContext helper**

The existing rendering logic (building tree from flat list → rendering markdown) should be extracted to:

```go
func (h *AIHandler) renderTreeContext(pages []model.GetWikiPageTreeRow, header string) string {
    // Same tree-building + markdown rendering as existing buildWikiContext body
    // Accepts header string for the root label
}
```

This keeps the rendering code in one place, used by both full-tree and subtree modes.

- [ ] **Step 3: Update executeLookupPage to pass page ID for subtree context**

Modify the existing `lookup_page` tool so that when the AI queries a page, the response includes not just the page metadata but also its subtree context. This is where the actual performance win lands — the AI can explore a topic and only see relevant content.

```go
// In executeLookupPage:
func (h *AIHandler) executeLookupPage(ctx context.Context, tc ai.ToolCall) string {
    // ... existing title parsing + GetWikiPageByTitle call ...
    
    // Build subtree context for this page
    subtreeContext := h.buildWikiContext(ctx, &page.ID)
    
    return fmt.Sprintf(
        "[系统] 工具 lookup_page 已执行完毕，查询「%s」结果。\n页面信息：%s\n\n下级内容：\n%s\n请根据以上信息继续操作。",
        details.Title, string(result), subtreeContext,
    )
}
```

- [ ] **Step 4: Update all existing callers of buildWikiContext**

Search for `h.buildWikiContext(ctx)` in ai.go and update to `h.buildWikiContext(ctx, nil)`:

```go
// In executeConfirmedActions or wherever buildWikiContext is called
wikiContext := h.buildWikiContext(ctx, nil)
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/ai.go
git commit -m "feat: add subtree context for AI lookup, extract renderTreeContext"
```

---

### Task 6: Update handler's init code to run the new migration

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/handler/wiki.go` (if CreateEmptyWikiPage also needs path)

- [ ] **Step 1: Add migration SQL to server init**

In `backend/cmd/server/main.go`, add the ALTER TABLE and CREATE INDEX statements to the schema SQL block, wrapped with IF NOT EXISTS for idempotency:

```go
// Add after the existing schema SQL
ALTER TABLE wiki_pages ADD COLUMN path TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_wiki_pages_path ON wiki_pages(path);
```

- [ ] **Step 2: Update CreateEmptyWikiPage to set path**

```go
// After successful insert in CreateEmptyWikiPage:
if id > 0 {
    var path string
    if req.ParentID != nil && *req.ParentID > 0 {
        parentPath, err := h.queries.GetWikiPagePathByID(ctx, *req.ParentID)
        if err == nil {
            path = parentPath + fmt.Sprintf("%d/", id)
        }
    } else {
        path = fmt.Sprintf("%d/", id)
    }
    if path != "" {
        h.queries.UpdateWikiPagePath(ctx, model.UpdateWikiPagePathParams{
            Path: path,
            ID:   id,
        })
    }
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat: add path migration to server init and CreateEmptyWikiPage path handling"
```

---

### Task 7: Update frontend types

**Files:**
- Modify: `frontend/src/types/index.ts`

- [ ] **Step 1: Add path to WikiTreeNode**

```typescript
export interface WikiTreeNode {
  id: number;
  title: string;
  slug: string;
  page_type: string;
  content_status: string;
  parent_id: number | null;
  path: string;
  sort_order: number;
  children?: WikiTreeNode[];
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/types/index.ts
git commit -m "feat: add path field to WikiTreeNode type"
```

---

### Task 8: Verify end-to-end

- [ ] **Step 1: Rebuild backend**

```bash
cd backend && go build ./...
```

Expected: builds without errors.

- [ ] **Step 2: Start backend and verify migration runs**

```bash
cd backend && rm -f learn-helper.db && go run ./cmd/server
```

Expected: server starts, schema initializes, overview page created with path field populated.

- [ ] **Step 3: Start frontend**

```bash
cd frontend && npm run dev
```

Expected: dev server starts, tree renders without errors (path field is present in JSON but not used for rendering logic).

- [ ] **Step 4: Test subtree query via SQLite**

```sql
-- Insert test data
INSERT INTO wiki_pages (title, slug, page_type, content_status, sort_order) VALUES ('Go', 'go', 'entity', 'published', 1);
-- Get ID (e.g. 2), then:
UPDATE wiki_pages SET path = '2/' WHERE id = 2;

INSERT INTO wiki_pages (title, slug, parent_id, page_type, content_status, sort_order) VALUES ('基础语法', 'basics', 2, 'entity', 'empty', 1);
-- Get ID (e.g. 3), then:
UPDATE wiki_pages SET path = '2/3/' WHERE id = 3;

-- Test subtree query:
SELECT * FROM wiki_pages WHERE path LIKE '2/%';
```

Expected: returns both Go (id=2) and 基础语法 (id=3).

- [ ] **Step 5: Commit any final fixes**

```bash
git add -A && git commit -m "chore: fix issues found during verification"
```
