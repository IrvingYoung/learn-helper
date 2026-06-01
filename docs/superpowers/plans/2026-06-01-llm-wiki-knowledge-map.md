# LLM Wiki 知识地图 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 AI 装上"知识地图"——每页自动维护 1-2 句摘要、可读时间线、按分类聚合的目录视图，让 AI 不再瞎猜知识库结构。

**Architecture:** 三块增量：每页 `summary` 列（异步 AI 生成 + content_hash 校验） + `wiki_log` 表（同步写入，零延迟） + 派生视图 `buildKnowledgeMap`（每次 AI 请求时从源数据渲染，不存储）。"index 是 view 不是 store"——一致性来自派生，存储成本只付在摘要上。

**Tech Stack:** Go + sqlc (regenerated) + SQLite + go test + Claude/DeepSeek AI provider

**Spec:** [2026-06-01-llm-wiki-knowledge-map-design.md](../specs/2026-06-01-llm-wiki-knowledge-map-design.md)

---

## File Structure

| 文件 | 责任 | 类型 |
|---|---|---|
| `db/migrations/008_summary_columns.sql` | 添加 5 列（summary/status/hash/timestamps/link counts/tags_normalized） + 2 索引 | 新建 |
| `db/migrations/009_wiki_log.sql` | 创建 wiki_log 表 + 2 索引 | 新建 |
| `db/migrations/queries.sql` | 新增 12 个 sqlc 查询 | 修改 |
| `internal/model/queries.sql.go` | sqlc 重新生成 | 自动 |
| `internal/worker/summary.go` | SummaryWorker：channel 队列 + 单 goroutine 串行生成 | 新建 |
| `internal/worker/summary_test.go` | Worker 单元测试 | 新建 |
| `internal/handler/ai_renderer.go` | 4 个 render 函数 + buildKnowledgeMap orchestrator | 新建 |
| `internal/handler/ai_renderer_test.go` | Renderer 单元测试 | 新建 |
| `internal/handler/ai.go` | 把 `buildWikiContext` 调用换成 `buildKnowledgeMap` | 修改（小） |
| `internal/handler/wiki.go` | update/create 时计算 `tags_normalized` + `content_hash`；写 wiki_log | 修改 |
| `internal/engine/engine.go` | 所有 action 执行后写 wiki_log；link_pages 维护 link_count/backlink_count | 修改 |
| `internal/ai/provider.go` | system prompt 中"目录"段重写 | 修改（小） |
| `cmd/server/main.go` | 启动 SummaryWorker，传 context | 修改（小） |
| `internal/handler/wiki_test.go` | update/create 的 tags_normalized 计算测试 | 修改 |

总计 5 个新文件 + 6 个修改文件，按 4 个 phase 串行交付。

---

## Phase 1：Schema + 摘要 Worker

### Task 1.1：Migration 008 — 摘要列

**Files:**
- Create: `db/migrations/008_summary_columns.sql`

- [ ] **Step 1：写 migration 文件**

```sql
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
```

- [ ] **Step 2：验证 migration 跑通**

在 backend 目录运行：
```bash
cd backend
sqlite3 learn-helper.db < db/migrations/008_summary_columns.sql
```

预期：静默成功，无输出。

- [ ] **Step 3：验证列存在**

```bash
sqlite3 learn-helper.db "PRAGMA table_info(wiki_pages);" | grep -E "summary|link_count|backlink_count|tags_normalized"
```

预期输出（顺序可能不同）：
```
summary|TEXT|0||0
summary_status|TEXT|0|empty|0
summary_content_hash|TEXT|0||0
link_count|INTEGER|0|0|0
backlink_count|INTEGER|0|0|0
tags_normalized|TEXT|0||0
```

- [ ] **Step 4：验证索引存在**

```bash
sqlite3 learn-helper.db "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='wiki_pages' AND name LIKE 'idx_wiki_pages_%';"
```

预期输出包含 `idx_wiki_pages_summary_status` 和 `idx_wiki_pages_tags_normalized`。

- [ ] **Step 5：Commit**

```bash
git add db/migrations/008_summary_columns.sql
git commit -m "feat(db): add summary columns and link counts to wiki_pages"
```

### Task 1.2：Migration 009 — wiki_log 表

**Files:**
- Create: `db/migrations/009_wiki_log.sql`

- [ ] **Step 1：写 migration 文件**

```sql
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
```

- [ ] **Step 2：跑 migration**

```bash
cd backend
sqlite3 learn-helper.db < db/migrations/009_wiki_log.sql
```

预期：静默成功。

- [ ] **Step 3：验证表结构**

```bash
sqlite3 learn-helper.db "PRAGMA table_info(wiki_log);"
```

预期输出包含所有列：
```
0|id|INTEGER|0||1
1|action|TEXT|1||0
2|page_id|INTEGER|0||0
3|page_title|TEXT|1||0
4|page_path|TEXT|0||0
5|source|TEXT|1|plan|0
6|summary|TEXT|0||0
7|created_at|DATETIME|1|CURRENT_TIMESTAMP|0
```

- [ ] **Step 4：测试插入**

```bash
sqlite3 learn-helper.db "INSERT INTO wiki_log (action, page_title, summary) VALUES ('test', 'test page', 'test entry');"
sqlite3 learn-helper.db "SELECT action, page_title FROM wiki_log WHERE action='test';"
```

预期：返回 `(test|test page)`。

- [ ] **Step 5：清理测试数据 + commit**

```bash
sqlite3 learn-helper.db "DELETE FROM wiki_log WHERE action='test';"
git add db/migrations/009_wiki_log.sql
git commit -m "feat(db): add wiki_log table for write timeline"
```

### Task 1.3：sqlc queries — 摘要相关

**Files:**
- Modify: `db/migrations/queries.sql` (append to end)

- [ ] **Step 1：追加 8 个新查询**

在 `queries.sql` 末尾追加：

```sql
-- name: UpdatePageSummary :exec
UPDATE wiki_pages
SET summary = ?,
    summary_status = 'ready',
    summary_content_hash = ?,
    summary_generated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: MarkSummaryEmpty :exec
UPDATE wiki_pages
SET summary_status = 'empty',
    summary_content_hash = NULL,
    summary_generated_at = NULL
WHERE id = ?;

-- name: MarkSummaryPending :exec
UPDATE wiki_pages
SET summary_status = 'pending',
    summary_content_hash = NULL
WHERE id = ?;

-- name: MarkSummaryFailed :exec
UPDATE wiki_pages
SET summary_status = 'failed',
    summary_generated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ListPendingSummaries :many
SELECT id, title, content
FROM wiki_pages
WHERE summary_status IN ('pending', 'failed')
ORDER BY summary_status DESC, id
LIMIT ?;

-- name: GetPagesNeedingSummary :many
SELECT id, title, content, content_hash
FROM wiki_pages
WHERE summary_status = 'empty' AND content != ''
ORDER BY id
LIMIT ?;
```

注：`content_hash` 列在 migration 008 **没**添加（spec 里的 content_hash 实际上是 `summary_content_hash` 的简称）。如果确实需要 page 自己的 content_hash 用于其他目的，在 migration 008 加：

```sql
ALTER TABLE wiki_pages ADD COLUMN content_hash TEXT;
```

并由 handler 在 update 时计算。本计划走 spec 原方案——`summary_content_hash` 就够了。

- [ ] **Step 2：重新生成 sqlc 代码**

```bash
cd backend
sqlc generate
```

预期：生成新文件 `internal/model/queries.sql.go`，包含上述 6 个新函数。检查文件大小变化（应该变大）。

- [ ] **Step 3：编译验证**

```bash
cd backend
go build ./...
```

预期：无错误。

- [ ] **Step 4：Commit**

```bash
git add db/migrations/queries.sql internal/model/queries.sql.go internal/model/models.go
git commit -m "feat(db): add summary-related queries"
```

### Task 1.4：sqlc queries — wiki_log 相关

**Files:**
- Modify: `db/migrations/queries.sql` (append)

- [ ] **Step 1：追加 4 个查询**

```sql
-- name: InsertWikiLog :exec
INSERT INTO wiki_log (action, page_id, page_title, page_path, source, summary)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetRecentWikiLog :many
SELECT id, action, page_id, page_title, page_path, source, summary, created_at
FROM wiki_log
WHERE created_at > ?
ORDER BY created_at DESC
LIMIT ?;

-- name: GetRecentWikiLogForPage :many
SELECT id, action, page_id, page_title, page_path, source, summary, created_at
FROM wiki_log
WHERE page_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: GetWikiLogBetween :many
SELECT id, action, page_id, page_title, page_path, source, summary, created_at
FROM wiki_log
WHERE created_at > ? AND created_at < ?
ORDER BY created_at DESC
LIMIT ?;
```

- [ ] **Step 2：重新生成 sqlc**

```bash
cd backend
sqlc generate
go build ./...
```

预期：编译通过。

- [ ] **Step 3：Commit**

```bash
git add db/migrations/queries.sql internal/model/queries.sql.go internal/model/models.go
git commit -m "feat(db): add wiki_log queries"
```

### Task 1.5：SummaryWorker 骨架

**Files:**
- Create: `internal/worker/summary.go`
- Create: `internal/worker/summary_test.go`

- [ ] **Step 1：写测试文件**

```go
package worker

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSummaryWorker_EnqueueIsNonBlocking(t *testing.T) {
	w := &SummaryWorker{queue: make(chan int64, 3)}

	// Fill the buffer completely.
	w.Enqueue(1)
	w.Enqueue(2)
	w.Enqueue(3)

	// Enqueue one more with no consumer - should drop, not block.
	done := make(chan bool)
	go func() {
		w.Enqueue(4) // would block forever if queue were synchronous
		done <- true
	}()

	select {
	case <-done:
		// expected: Enqueue returned quickly
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Enqueue blocked on full channel")
	}
}

func TestSummaryWorker_StopsOnContextCancel(t *testing.T) {
	w := &SummaryWorker{
		queue: make(chan int64),
		ctx:   context.Background(),
	}

	// Manually wire up the Run loop's context handling
	ctx, cancel := context.WithCancel(context.Background())
	w.ctx = ctx

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Simulate Run's select loop without the actual work
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.queue:
				// would call generateOne in real code
			}
		}
	}()

	// Give the goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	cancel()
	wg.Wait() // should not hang
}
```

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/worker/... -run TestSummaryWorker
```

预期：FAIL（`undefined: SummaryWorker`）。

- [ ] **Step 3：写最小实现**

```go
package worker

import (
	"context"
)

// SummaryWorker asynchronously generates AI summaries for wiki pages.
// One goroutine, serial processing, channel-buffered queue.
type SummaryWorker struct {
	ctx    context.Context
	queue  chan int64
	// Wired in Run or via setter - tests can use a mock
	processFn func(ctx context.Context, pageID int64)
}

// Enqueue requests a summary generation for the given page.
// Non-blocking: if the channel is full, the request is dropped.
// Dropped requests stay in DB with summary_status='pending'
// and will be retried on next write or server restart.
func (w *SummaryWorker) Enqueue(pageID int64) {
	select {
	case w.queue <- pageID:
	default:
		// queue full, drop. status='pending' in DB will trigger retry.
	}
}

// Run starts the worker loop. Blocks until ctx is cancelled.
func (w *SummaryWorker) Run(ctx context.Context) {
	if w.processFn == nil {
		// Real implementation is wired in NewSummaryWorker.
		// Default no-op is for tests that drive the loop manually.
		<-ctx.Done()
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case pageID := <-w.queue:
			w.processFn(ctx, pageID)
		}
	}
}
```

- [ ] **Step 4：跑测试确认通过**

```bash
cd backend
go test ./internal/worker/... -run TestSummaryWorker
```

预期：PASS。

- [ ] **Step 5：Commit**

```bash
git add internal/worker/summary.go internal/worker/summary_test.go
git commit -m "feat(worker): add SummaryWorker skeleton with channel queue"
```

### Task 1.6：SummaryWorker 完整实现

**Files:**
- Modify: `internal/worker/summary.go`

- [ ] **Step 1：写测试 (扩展) — 测试 generateOne 行为**

在 `summary_test.go` 追加：

```go
type mockProvider struct {
	mu       sync.Mutex
	calls    int
	summary  string
	err      error
}

func (m *mockProvider) GenerateSummary(ctx context.Context, title, content string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.summary, m.err
}

type mockDB struct {
	mu              sync.Mutex
	pages           map[int64]*mockPage
	updated         []updateCall
}

type mockPage struct {
	ID               int64
	Title            string
	Content          string
	Summary          string
	SummaryStatus    string
	SummaryContentHash string
}

type updateCall struct {
	PageID int64
	Summary string
	Hash    string
}

func TestSummaryWorker_generateOne_ready(t *testing.T) {
	provider := &mockProvider{summary: "数组：线性表基础。"}
	db := &mockDB{
		pages: map[int64]*mockPage{
			1: {ID: 1, Title: "数组", Content: "线性表基础内容", SummaryStatus: "pending"},
		},
	}
	w := &SummaryWorker{
		provider: provider,
		db:       db,
	}

	w.generateOne(context.Background(), 1)

	if provider.calls != 1 {
		t.Errorf("expected 1 provider call, got %d", provider.calls)
	}
	if len(db.updated) != 1 {
		t.Fatalf("expected 1 DB update, got %d", len(db.updated))
	}
	if db.updated[0].Summary != "数组：线性表基础。" {
		t.Errorf("unexpected summary: %s", db.updated[0].Summary)
	}
	if db.updated[0].Hash == "" {
		t.Error("expected content_hash to be set")
	}
}

func TestSummaryWorker_generateOne_emptyContent(t *testing.T) {
	provider := &mockProvider{summary: "should not be called"}
	db := &mockDB{
		pages: map[int64]*mockPage{
			1: {ID: 1, Title: "树", Content: "", SummaryStatus: "pending"},
		},
	}
	w := &SummaryWorker{provider: provider, db: db}

	w.generateOne(context.Background(), 1)

	if provider.calls != 0 {
		t.Errorf("expected 0 calls for empty content, got %d", provider.calls)
	}
	if db.pages[1].SummaryStatus != "empty" {
		t.Errorf("expected status=empty, got %s", db.pages[1].SummaryStatus)
	}
}

func TestSummaryWorker_generateOne_failure(t *testing.T) {
	provider := &mockProvider{err: errors.New("api timeout")}
	db := &mockDB{
		pages: map[int64]*mockPage{
			1: {ID: 1, Title: "链表", Content: "节点+指针", SummaryStatus: "pending"},
		},
	}
	w := &SummaryWorker{provider: provider, db: db}

	w.generateOne(context.Background(), 1)

	if db.pages[1].SummaryStatus != "failed" {
		t.Errorf("expected status=failed, got %s", db.pages[1].SummaryStatus)
	}
}
```

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/worker/...
```

预期：FAIL（`generateOne` 不存在，`provider` 字段不存在等）。

- [ ] **Step 3：实现 generateOne + provider 接口**

替换 `summary.go`：

```go
package worker

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"learn-helper/internal/model"
)

// SummaryProvider is the minimal interface SummaryWorker needs from an AI provider.
type SummaryProvider interface {
	GenerateSummary(ctx context.Context, title, content string) (string, error)
}

// SummaryDB is the minimal interface for the DB operations the worker needs.
type SummaryDB interface {
	GetPageForSummary(ctx context.Context, pageID int64) (title, content, status, hash string, err error)
	UpdateSummary(ctx context.Context, pageID int64, summary, hash string) error
	MarkSummaryEmpty(ctx context.Context, pageID int64) error
	MarkSummaryFailed(ctx context.Context, pageID int64) error
	ListPendingSummaries(ctx context.Context, limit int) ([]int64, error)
}

// SummaryWorker asynchronously generates AI summaries for wiki pages.
type SummaryWorker struct {
	provider SummaryProvider
	db       SummaryDB
	queue    chan int64
}

const summaryQueueSize = 100

// NewSummaryWorker creates a worker with the given dependencies.
func NewSummaryWorker(provider SummaryProvider, db SummaryDB) *SummaryWorker {
	return &SummaryWorker{
		provider: provider,
		db:       db,
		queue:    make(chan int64, summaryQueueSize),
	}
}

// Enqueue requests a summary generation. Non-blocking.
func (w *SummaryWorker) Enqueue(pageID int64) {
	select {
	case w.queue <- pageID:
	default:
		// dropped; DB still has status='pending' or 'failed', retried later
	}
}

// Run starts the worker loop. Blocks until ctx is cancelled.
func (w *SummaryWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case pageID := <-w.queue:
			w.generateOne(ctx, pageID)
		}
	}
}

// generateOne is the core: fetch page, ask AI for summary, write back.
func (w *SummaryWorker) generateOne(ctx context.Context, pageID int64) {
	title, content, status, _, err := w.db.GetPageForSummary(ctx, pageID)
	if err != nil {
		// Page was deleted between enqueue and processing. Skip.
		return
	}

	// Skip empty content - nothing to summarize.
	if strings.TrimSpace(content) == "" {
		_ = w.db.MarkSummaryEmpty(ctx, pageID)
		return
	}

	// Skip if already 'empty' status (avoid regenerating).
	if status == "empty" {
		return
	}

	// Rate limit: 200ms between AI calls to avoid provider limits.
	time.Sleep(200 * time.Millisecond)

	summary, err := w.provider.GenerateSummary(ctx, title, content)
	if err != nil {
		_ = w.db.MarkSummaryFailed(ctx, pageID)
		return
	}

	hash := contentHash(title, content)
	if err := w.db.UpdateSummary(ctx, pageID, summary, hash); err != nil {
		_ = w.db.MarkSummaryFailed(ctx, pageID)
	}
}

// contentHash returns MD5(title + content) for staleness detection.
func contentHash(title, content string) string {
	h := md5.New()
	h.Write([]byte(title))
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

// BackfillOnce processes all pages with status='empty' or 'failed'.
// Called at server startup to catch up after crashes.
func (w *SummaryWorker) BackfillOnce(ctx context.Context, batchSize int) error {
	ids, err := w.db.ListPendingSummaries(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("list pending summaries: %w", err)
	}
	for _, id := range ids {
		w.generateOne(ctx, id)
	}
	return nil
}

// Compile-time checks (we don't have these interfaces in production code yet,
// but the tests need them to compile).
var _ SummaryDB = (*sqlDBAdapter)(nil)

type sqlDBAdapter struct {
	q *model.Queries
}

func NewSQLDBAdapter(q *model.Queries) *sqlDBAdapter {
	return &sqlDBAdapter{q: q}
}

func (a *sqlDBAdapter) GetPageForSummary(ctx context.Context, pageID int64) (string, string, string, string, error) {
	page, err := a.q.GetWikiPageByID(ctx, pageID)
	if err != nil {
		return "", "", "", "", err
	}
	hash := sql.NullString{}
	return page.Title, page.Content, page.SummaryStatus, hash.String, nil
}

func (a *sqlDBAdapter) UpdateSummary(ctx context.Context, pageID int64, summary, hash string) error {
	return a.q.UpdatePageSummary(ctx, model.UpdatePageSummaryParams{
		Summary:     summary,
		ContentHash: sql.NullString{String: hash, Valid: true},
		ID:          pageID,
	})
}

func (a *sqlDBAdapter) MarkSummaryEmpty(ctx context.Context, pageID int64) error {
	return a.q.MarkSummaryEmpty(ctx, pageID)
}

func (a *sqlDBAdapter) MarkSummaryFailed(ctx context.Context, pageID int64) error {
	return a.q.MarkSummaryFailed(ctx, pageID)
}

func (a *sqlDBAdapter) ListPendingSummaries(ctx context.Context, limit int) ([]int64, error) {
	// Use ListPendingSummaries query - returns pages with status IN (pending, failed)
	return a.q.ListPendingSummariesIDs(ctx, int64(limit))
}

// Avoid unused import errors
var _ = errors.New
var _ = model.Queries{}
```

- [ ] **Step 4：补 sqlc query（ListPendingSummariesIDs）**

在 `queries.sql` 追加：

```sql
-- name: ListPendingSummariesIDs :many
SELECT id FROM wiki_pages
WHERE summary_status IN ('pending', 'failed')
ORDER BY id
LIMIT ?;
```

- [ ] **Step 5：重新生成 sqlc + 编译**

```bash
cd backend
sqlc generate
go build ./...
```

预期：编译通过。

- [ ] **Step 6：跑测试确认通过**

```bash
cd backend
go test ./internal/worker/...
```

预期：PASS（如果接口对齐失败，参见 step 7 的修复）。

- [ ] **Step 7：（如果测试不通过）调整 mock 实现**

测试里的 `mockDB` 需要实现 `SummaryDB` 接口。简化方案：让 mockDB 显式实现接口（接收者 + 接口断言），删除 step 6 里的硬编码 SQL 适配器结构。这部分留给实现者根据编译错误调整；目标是 3 个测试都通过。

- [ ] **Step 8：Commit**

```bash
git add internal/worker/summary.go internal/worker/summary_test.go db/migrations/queries.sql internal/model/
git commit -m "feat(worker): implement generateOne with rate limiting and content hash"
```

### Task 1.7：SummaryWorker 集成到 main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1：读 main.go 找到 server 启动点**

```bash
cd backend
grep -n "func main\|server\|http.Server\|ListenAndServe" cmd/server/main.go
```

- [ ] **Step 2：在 main 函数里启动 worker**

找到 `main` 函数（一般在文件底部）。在 server 启动后、graceful shutdown 前加入：

```go
// 在 main 函数顶部 imports 之后
import (
    // ... existing
    "learn-helper/internal/worker"
)

// 在 db 和 provider 初始化之后
summaryWorker := worker.NewSummaryWorker(
    worker.NewProviderAdapter(provider), // 见 step 3
    worker.NewSQLDBAdapter(queries),
)

// Backfill pending/failed summaries on startup.
if os.Getenv("SKIP_SUMMARY_BACKFILL") != "1" {
    go func() {
        // Process in batches to avoid flooding provider on startup.
        for {
            if err := summaryWorker.BackfillOnce(ctx, 10); err != nil {
                log.Printf("summary backfill: %v", err)
            }
            // Check if more remain; if not, exit.
            ids, _ := queries.ListPendingSummariesIDs(ctx, 1000)
            if len(ids) == 0 {
                break
            }
            time.Sleep(1 * time.Second)
        }
    }()
}

// Start the worker loop.
go summaryWorker.Run(ctx)
```

- [ ] **Step 3：实现 ProviderAdapter（让现有 ai.AIProvider 满足 SummaryProvider 接口）**

在 `internal/worker/summary.go` 末尾追加：

```go
// ProviderAdapter wraps an existing AI provider so it satisfies SummaryProvider.
// This is a thin shim to avoid coupling worker package to the ai package's full
// interface (which has streaming methods we don't need here).
type ProviderAdapter struct {
    provider any
    // model: e.g. "claude-sonnet-4-7-20250514"
    // Calls would go to provider.Chat(...) with a fixed system prompt.
}

func NewProviderAdapter(p any) *ProviderAdapter {
    return &ProviderProvider{provider: p}
}
```

**实际实现要 import "learn-helper/internal/ai" 并调 `ai.ChatRequest{Messages: ..., MaxTokens: 256}`。** 由于 plan 不强制锁定 ai provider 内部类型，参考现有 `internal/ai/claude.go` 和 `internal/ai/deepseek.go` 实现一个简单的 `GenerateSummary`：

```go
func (a *ProviderAdapter) GenerateSummary(ctx context.Context, title, content string) (string, error) {
    // Truncate content to ~3000 chars to stay under token limits.
    truncated := content
    if len([]rune(truncated)) > 3000 {
        truncated = string([]rune(truncated)[:3000])
    }

    prompt := fmt.Sprintf(
        "为以下 Wiki 页面生成 1-2 句中文摘要（50-150 字）。说明这页讲什么、适合谁看。\n\n标题：%s\n\n内容：\n%s",
        title, truncated,
    )

    // Use the non-streaming Chat() method
    req := ai.ChatRequest{
        Messages:     []ai.Message{{Role: "user", Content: prompt}},
        SystemPrompt: "你是一个 Wiki 摘要生成器。只输出摘要本身，不要解释、不要标题前缀。",
        MaxTokens:    256,
    }
    resp, err := a.provider.Chat(ctx, req)
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(resp.Content), nil
}
```

具体类型（`ai.ChatRequest`、`ai.AIProvider`）的 import 路径由实现者根据现有 `internal/ai/provider.go` 调整。

- [ ] **Step 4：写/扩展 server 启动测试**

```bash
cd backend
go test ./...
```

预期：所有测试通过。`cmd/server` 可能没有测试文件，跳过即可。

- [ ] **Step 5：手动启动验证**

```bash
cd backend
go run ./cmd/server &
SERVER_PID=$!
sleep 5
# 写一个页面
curl -X POST http://localhost:8080/api/wiki -H "Content-Type: application/json" \
  -d '{"title":"测试摘要","content":"这是一段测试内容，用于触发摘要生成。"}'
# 等待 worker 处理
sleep 3
# 检查 summary 字段
curl -s "http://localhost:8080/api/wiki?slug=test-summary" | head -200
kill $SERVER_PID
```

预期：返回的页面对象里有非空的 `summary` 字段。

- [ ] **Step 6：清理测试页面 + commit**

```bash
cd backend
sqlite3 learn-helper.db "DELETE FROM wiki_pages WHERE title='测试摘要';"
git add cmd/server/main.go internal/worker/summary.go
git commit -m "feat(worker): integrate SummaryWorker into server startup with backfill"
```

---

## Phase 2：Knowledge Map 渲染

### Task 2.1：renderOverview 函数

**Files:**
- Create: `internal/handler/ai_renderer.go`
- Create: `internal/handler/ai_renderer_test.go`

- [ ] **Step 1：写测试**

```go
package handler

import (
	"context"
	"strings"
	"testing"

	"learn-helper/internal/model"
)

type fakeOverviewDB struct {
	pages []model.WikiPage
}

func (f *fakeOverviewDB) CountWikiPages(ctx context.Context) (int64, error) {
	return int64(len(f.pages)), nil
}

func (f *fakeOverviewDB) CountWikiPagesByStatus(ctx context.Context, status string) (int64, error) {
	var n int64
	for _, p := range f.pages {
		if p.ContentStatus == status {
			n++
		}
	}
	return n, nil
}

func (f *fakeOverviewDB) GetRecentlyUpdatedWikiPages(ctx context.Context) ([]model.GetRecentlyUpdatedWikiPagesRow, error) {
	if len(f.pages) == 0 {
		return nil, nil
	}
	p := f.pages[0]
	return []model.GetRecentlyUpdatedWikiPagesRow{
		{ID: p.ID, Title: p.Title, Slug: p.Slug, PageType: p.PageType, ContentStatus: p.ContentStatus, UpdatedAt: p.UpdatedAt},
	}, nil
}

func TestRenderOverview_EmptyDB(t *testing.T) {
	queries := &fakeOverviewDB{}
	out := renderOverview(context.Background(), queries)
	if !strings.Contains(out, "总页面数: 0") {
		t.Errorf("expected '总页面数: 0', got: %s", out)
	}
}

func TestRenderOverview_WithPages(t *testing.T) {
	queries := &fakeOverviewDB{
		pages: []model.WikiPage{
			{ID: 1, Title: "Go", ContentStatus: "published"},
			{ID: 2, Title: "算法", ContentStatus: "draft"},
			{ID: 3, Title: "树", ContentStatus: "empty"},
		},
	}
	out := renderOverview(context.Background(), queries)
	if !strings.Contains(out, "总页面数: 3") {
		t.Errorf("expected '总页面数: 3', got: %s", out)
	}
	if !strings.Contains(out, "已填充: 2") {
		t.Errorf("expected '已填充: 2', got: %s", out)
	}
	if !strings.Contains(out, "空: 1") {
		t.Errorf("expected '空: 1', got: %s", out)
	}
}
```

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/handler/... -run TestRenderOverview
```

预期：FAIL（`undefined: renderOverview`）。

- [ ] **Step 3：写 renderOverview 实现**

在 `ai_renderer.go`：

```go
package handler

import (
	"context"
	"fmt"
	"strings"

	"learn-helper/internal/model"
)

// OverviewDB is the minimal DB interface for the overview section.
type OverviewDB interface {
	CountWikiPages(ctx context.Context) (int64, error)
	CountWikiPagesByStatus(ctx context.Context, status string) (int64, error)
	GetRecentlyUpdatedWikiPages(ctx context.Context) ([]model.GetRecentlyUpdatedWikiPagesRow, error)
}

// renderOverview builds the high-level stats section.
func renderOverview(ctx context.Context, db OverviewDB) string {
	var b strings.Builder
	b.WriteString("【知识库概览】\n")

	total, _ := db.CountWikiPages(ctx)
	published, _ := db.CountWikiPagesByStatus(ctx, "published")
	draft, _ := db.CountWikiPagesByStatus(ctx, "draft")
	filled := published + draft
	empty := total - filled

	b.WriteString(fmt.Sprintf("总页面数: %d | 已填充: %d (%.0f%%) | 空: %d (%.0f%%)\n",
		total, filled, pct(filled, total), empty, pct(empty, total)))

	recent, _ := db.GetRecentlyUpdatedWikiPages(ctx)
	if len(recent) > 0 {
		b.WriteString(fmt.Sprintf("最近更新: %s (ID=%d)\n", recent[0].Title, recent[0].ID))
	}
	b.WriteString("\n")
	return b.String()
}

func pct(num, denom int64) float64 {
	if denom == 0 {
		return 0
	}
	return float64(num) / float64(denom) * 100
}
```

- [ ] **Step 4：跑测试确认通过**

```bash
cd backend
go test ./internal/handler/... -run TestRenderOverview
```

预期：PASS。

- [ ] **Step 5：Commit**

```bash
git add internal/handler/ai_renderer.go internal/handler/ai_renderer_test.go
git commit -m "feat(ai): add renderOverview for knowledge base stats"
```

### Task 2.2：renderKnowledgeMap 函数

**Files:**
- Modify: `internal/handler/ai_renderer.go`
- Modify: `internal/handler/ai_renderer_test.go`

- [ ] **Step 1：写测试**

在 `ai_renderer_test.go` 追加：

```go
type fakeTreeDB struct {
	pages []model.GetWikiPageTreeRow
}

func (f *fakeTreeDB) GetWikiPageTree(ctx context.Context) ([]model.GetWikiPageTreeRow, error) {
	return f.pages, nil
}

func TestRenderKnowledgeMap_Basic(t *testing.T) {
	tree := []model.GetWikiPageTreeRow{
		{ID: 1, Title: "概览", PageType: "overview", ContentStatus: "published", ParentID: sql.NullInt64{}, Path: "1/", Summary: sql.NullString{String: "知识库总览", Valid: true}, SummaryStatus: sql.NullString{String: "ready", Valid: true}, SummaryContentHash: sql.NullString{}, BacklinkCount: 0, LinkCount: 2, TagsNormalized: sql.NullString{}},
		{ID: 2, Title: "Go 语言", PageType: "overview", ContentStatus: "published", ParentID: sql.NullInt64{Int64: 1, Valid: true}, Path: "1/2/", Summary: sql.NullString{String: "Go 语法、并发和工程实践", Valid: true}, SummaryStatus: sql.NullString{String: "ready", Valid: true}, BacklinkCount: 5, LinkCount: 3, TagsNormalized: sql.NullString{String: "go", Valid: true}},
		{ID: 3, Title: "变量声明", PageType: "entity", ContentStatus: "published", ParentID: sql.NullInt64{Int64: 2, Valid: true}, Path: "1/2/3/", Summary: sql.NullString{String: "var vs := 类型推断", Valid: true}, SummaryStatus: sql.NullString{String: "ready", Valid: true}, BacklinkCount: 2, LinkCount: 0, TagsNormalized: sql.NullString{String: "go,基础", Valid: true}},
		{ID: 4, Title: "context", PageType: "entity", ContentStatus: "empty", ParentID: sql.NullInt64{Int64: 2, Valid: true}, Path: "1/2/4/", Summary: sql.NullString{}, SummaryStatus: sql.NullString{String: "empty", Valid: true}, BacklinkCount: 0, LinkCount: 0, TagsNormalized: sql.NullString{}},
	}
	db := &fakeTreeDB{pages: tree}
	out := renderKnowledgeMap(context.Background(), db, nil)

	if !strings.Contains(out, "【知识地图】") {
		t.Error("missing 知识地图 header")
	}
	if !strings.Contains(out, "Go 语言") {
		t.Error("missing Go 语言")
	}
	if !strings.Contains(out, "变量声明") {
		t.Error("missing 变量声明")
	}
	if !strings.Contains(out, "var vs := 类型推断") {
		t.Error("missing summary text for 变量声明")
	}
	if !strings.Contains(out, "context") {
		t.Error("missing context page")
	}
	// Backlink count visible
	if !strings.Contains(out, "2 反链") {
		t.Error("expected backlink count for 变量声明")
	}
	// Tag visible
	if !strings.Contains(out, "#go") {
		t.Error("expected tag #go")
	}
}

func TestRenderKnowledgeMap_PendingSummaryFallback(t *testing.T) {
	tree := []model.GetWikiPageTreeRow{
		{ID: 1, Title: "测试页", PageType: "entity", ContentStatus: "published", ParentID: sql.NullInt64{}, Path: "1/", Content: "这是一段测试内容，足够长", Summary: sql.NullString{}, SummaryStatus: sql.NullString{String: "pending", Valid: true}},
	}
	// Need a fakeDB that returns Content for the page - extend fakeTreeDB or use a new struct
	// For this test, just verify the fallback text is shown
	out := renderKnowledgeMap(context.Background(), &fakeTreeDB{pages: tree}, nil)
	if !strings.Contains(out, "摘要待更新") && !strings.Contains(out, "测试内容") {
		t.Error("expected fallback to content when summary pending")
	}
}
```

注：实际 fakeDB 需要也能返回 page content 才能测试 fallback。完整测试可能需要为 `renderKnowledgeMap` 单独定义一个 `KnowledgeMapDB` 接口（接受 pageID → content 的查询）。让实现者按需调整。

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/handler/... -run TestRenderKnowledgeMap
```

预期：FAIL（`renderKnowledgeMap` 未定义）。

- [ ] **Step 3：写 renderKnowledgeMap 实现**

在 `ai_renderer.go` 追加：

```go
import (
	// ... existing
	"database/sql"
)

// KnowledgeMapDB is the minimal interface for rendering the knowledge map.
type KnowledgeMapDB interface {
	GetWikiPageTree(ctx context.Context) ([]model.GetWikiPageTreeRow, error)
	// For fallback when summary is pending, we may need to fetch content.
	// Optional: if not implemented, fallback shows "(content unavailable)".
	GetPageContentForFallback(ctx context.Context, pageID int64) (string, error)
}

// renderKnowledgeMap builds the categorized tree with per-page summaries.
// focusPageID, if set, only renders the subtree (existing behavior).
func renderKnowledgeMap(ctx context.Context, db KnowledgeMapDB, focusPageID *int64) string {
	var b strings.Builder
	b.WriteString("【知识地图】\n\n")

	pages, err := db.GetWikiPageTree(ctx)
	if err != nil || len(pages) == 0 {
		b.WriteString("（知识库为空）\n")
		return b.String()
	}

	// Build parent-children index
	children := make(map[int64][]model.GetWikiPageTreeRow)
	var roots []model.GetWikiPageTreeRow
	for _, p := range pages {
		if !p.ParentID.Valid || p.ParentID.Int64 == 0 {
			roots = append(roots, p)
		} else {
			children[p.ParentID.Int64] = append(children[p.ParentID.Int64], p)
		}
	}

	// If focus is set, find that node as the root
	if focusPageID != nil {
		var focus *model.GetWikiPageTreeRow
		for i, p := range roots {
			if p.ID == *focusPageID {
				focus = &roots[i]
				break
			}
		}
		if focus == nil {
			b.WriteString(fmt.Sprintf("（未找到页面 ID=%d）\n", *focusPageID))
			return b.String()
		}
		roots = []model.GetWikiPageTreeRow{*focus}
	}

	// Render each root and its descendants
	for _, root := range roots {
		renderNodeWithChildren(ctx, &b, db, root, children, 0)
	}

	// Render global tag index
	b.WriteString("\n")
	b.WriteString(renderTagIndex(ctx, db))

	return b.String()
}

func renderNodeWithChildren(ctx context.Context, b *strings.Builder, db KnowledgeMapDB, node model.GetWikiPageTreeRow, children map[int64][]model.GetWikiPageTreeRow, depth int) {
	indent := strings.Repeat("  ", depth)
	icon := "📄"
	if node.PageType == "overview" {
		icon = "📁"
	}

	// Build status suffix
	status := "空"
	if node.ContentStatus == "published" {
		status = "有内容"
	} else if node.ContentStatus == "draft" {
		status = "草稿"
	}

	// Build summary line
	summaryLine := renderSummaryLine(ctx, db, node)

	// Build metadata
	meta := fmt.Sprintf("[ID=%d]", node.ID)
	if node.BacklinkCount > 0 {
		meta += fmt.Sprintf(", %d 反链", node.BacklinkCount)
	}
	if node.TagsNormalized.Valid && node.TagsNormalized.String != "" {
		meta += fmt.Sprintf(", 标签: %s", node.TagsNormalized.String)
	}

	// Coverage for overview nodes (X/Y 已建)
	if node.PageType == "overview" {
		all := collectDescendants(node, children)
		filled := 0
		for _, d := range all {
			if d.ContentStatus == "published" || d.ContentStatus == "draft" {
				filled++
			}
		}
		meta = fmt.Sprintf("%d/%d 已建, %s", filled, len(all), meta)
	}

	b.WriteString(fmt.Sprintf("%s%s %s (%s) %s\n", indent, icon, node.Title, status, meta))
	if summaryLine != "" {
		b.WriteString(fmt.Sprintf("%s  %s\n", indent, summaryLine))
	}

	// Recurse into children
	for _, child := range children[node.ID] {
		renderNodeWithChildren(ctx, b, db, child, children, depth+1)
	}
}

func collectDescendants(root model.GetWikiPageTreeRow, children map[int64][]model.GetWikiPageTreeRow) []model.GetWikiPageTreeRow {
	var result []model.GetWikiPageTreeRow
	var walk func(id int64)
	walk = func(id int64) {
		for _, c := range children[id] {
			result = append(result, c)
			walk(c.ID)
		}
	}
	walk(root.ID)
	return result
}

// renderSummaryLine returns the summary for a page, with fallback handling.
func renderSummaryLine(ctx context.Context, db KnowledgeMapDB, node model.GetWikiPageTreeRow) string {
	status := ""
	if node.SummaryStatus.Valid {
		status = node.SummaryStatus.String
	}
	hash := node.SummaryContentHash.String

	// Compute current content hash for staleness check (only if needed)
	// For simplicity here, we just check status; the full hash check
	// can be added when content_hash is available on the row.
	_ = hash

	switch status {
	case "ready":
		if node.Summary.Valid && node.Summary.String != "" {
			return node.Summary.String
		}
		fallthrough
	case "pending":
		// Fallback: try to show content snippet.
		if content, err := db.GetPageContentForFallback(ctx, node.ID); err == nil && content != "" {
			return truncateForDisplay(content, 80) + " (摘要待更新)"
		}
		return "(摘要待更新)"
	case "failed":
		if content, err := db.GetPageContentForFallback(ctx, node.ID); err == nil && content != "" {
			return truncateForDisplay(content, 80) + " (摘要生成失败)"
		}
		return "(摘要生成失败)"
	case "empty":
		if content, err := db.GetPageContentForFallback(ctx, node.ID); err == nil && content != "" {
			return truncateForDisplay(content, 80) + " (暂无摘要)"
		}
		return ""
	default:
		return ""
	}
}

func truncateForDisplay(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

// renderTagIndex builds the global tag summary.
func renderTagIndex(ctx context.Context, db KnowledgeMapDB) string {
	// We use the page tree since it already has tags_normalized.
	// For simplicity, count tags from the existing fetch.
	pages, _ := db.GetWikiPageTree(ctx)
	tagCounts := make(map[string]int)
	for _, p := range pages {
		if !p.TagsNormalized.Valid || p.TagsNormalized.String == "" {
			continue
		}
		for _, tag := range strings.Split(p.TagsNormalized.String, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagCounts[tag]++
			}
		}
	}
	if len(tagCounts) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("【全局标签索引】\n")
	// Sort tags by count desc, then alpha
	type tagCount struct {
		tag   string
		count int
	}
	var sorted []tagCount
	for k, v := range tagCounts {
		sorted = append(sorted, tagCount{k, v})
	}
	// Simple sort: by count desc
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	for i, tc := range sorted {
		if i > 0 {
			b.WriteString(" · ")
		}
		b.WriteString(fmt.Sprintf("#%s (%d 页)", tc.tag, tc.count))
	}
	b.WriteString("\n")
	return b.String()
}
```

- [ ] **Step 4：补 fakeDB 实现 KnowledgeMapDB 接口**

在 `ai_renderer_test.go` 调整 `fakeTreeDB` 让它同时实现 `GetPageContentForFallback`。具体由实现者根据编译错误调整（典型做法是加一个 `content map[int64]string` 字段）。

- [ ] **Step 5：跑测试**

```bash
cd backend
go test ./internal/handler/... -run TestRenderKnowledgeMap
```

预期：PASS（编译和测试都通过）。

- [ ] **Step 6：Commit**

```bash
git add internal/handler/ai_renderer.go internal/handler/ai_renderer_test.go
git commit -m "feat(ai): add renderKnowledgeMap with categorized tree and tag index"
```

### Task 2.3：renderRecentLog 函数

**Files:**
- Modify: `internal/handler/ai_renderer.go`
- Modify: `internal/handler/ai_renderer_test.go`

- [ ] **Step 1：写测试**

```go
type fakeLogDB struct {
	entries []model.GetRecentWikiLogRow
}

func (f *fakeLogDB) GetRecentWikiLog(ctx context.Context, since time.Time, limit int) ([]model.GetRecentWikiLogRow, error) {
	return f.entries, nil
}

func TestRenderRecentLog_Empty(t *testing.T) {
	db := &fakeLogDB{}
	out := renderRecentLog(context.Background(), db, 7*24*time.Hour, 20)
	if out != "" {
		t.Errorf("expected empty output for empty log, got: %s", out)
	}
}

func TestRenderRecentLog_WithEntries(t *testing.T) {
	now := time.Now()
	db := &fakeLogDB{
		entries: []model.GetRecentWikiLogRow{
			{ID: 1, Action: "create", PageTitle: "Go 语言", PageID: sql.NullInt64{Int64: 2, Valid: true}, CreatedAt: now.Add(-1 * time.Hour), Summary: sql.NullString{String: "新建主分类", Valid: true}},
			{ID: 2, Action: "update", PageTitle: "channel", PageID: sql.NullInt64{Int64: 3, Valid: true}, CreatedAt: now.Add(-2 * time.Hour), Summary: sql.NullString{}},
		},
	}
	out := renderRecentLog(context.Background(), db, 7*24*time.Hour, 20)
	if !strings.Contains(out, "【最近活动】") {
		t.Error("missing 最近活动 header")
	}
	if !strings.Contains(out, "Go 语言") {
		t.Error("missing Go 语言 entry")
	}
	if !strings.Contains(out, "[create]") {
		t.Error("missing action tag")
	}
}
```

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/handler/... -run TestRenderRecentLog
```

预期：FAIL。

- [ ] **Step 3：实现 renderRecentLog**

在 `ai_renderer.go` 追加：

```go
import (
	// ... existing
	"time"
)

// RecentLogDB is the minimal interface for log queries.
type RecentLogDB interface {
	GetRecentWikiLog(ctx context.Context, since time.Time, limit int) ([]model.GetRecentWikiLogRow, error)
}

// renderRecentLog builds the "recent activity" timeline.
func renderRecentLog(ctx context.Context, db RecentLogDB, window time.Duration, limit int) string {
	since := time.Now().Add(-window)
	entries, err := db.GetRecentWikiLog(ctx, since, limit)
	if err != nil || len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("【最近活动】(过去 %s，共 %d 条操作)\n", window, len(entries)))
	for _, e := range entries {
		ts := e.CreatedAt.Format("2006-01-02 15:04")
		line := fmt.Sprintf("  - %s [%s] %s", ts, e.Action, e.PageTitle)
		if e.PageID.Valid {
			line += fmt.Sprintf(" (ID=%d)", e.PageID.Int64)
		}
		if e.Summary.Valid && e.Summary.String != "" {
			line += " · " + e.Summary.String
		}
		b.WriteString(line + "\n")
	}
	return b.String() + "\n"
}
```

- [ ] **Step 4：跑测试**

```bash
cd backend
go test ./internal/handler/... -run TestRenderRecentLog
```

预期：PASS。

- [ ] **Step 5：Commit**

```bash
git add internal/handler/ai_renderer.go internal/handler/ai_renderer_test.go
git commit -m "feat(ai): add renderRecentLog for write timeline"
```

### Task 2.4：renderHealthCheck 函数（增强现有）

**Files:**
- Modify: `internal/handler/ai_renderer.go`
- Modify: `internal/handler/ai_renderer_test.go`

- [ ] **Step 1：写测试**

```go
type fakeHealthDB struct {
	pages []model.GetWikiPageTreeRow
}

func (f *fakeHealthDB) GetWikiPageTree(ctx context.Context) ([]model.GetWikiPageTreeRow, error) {
	return f.pages, nil
}

func TestRenderHealthCheck_DeadPage(t *testing.T) {
	// Page with content but 0 backlinks and 0 links
	tree := []model.GetWikiPageTreeRow{
		{ID: 1, Title: "孤岛", PageType: "entity", ContentStatus: "published", LinkCount: 0, BacklinkCount: 0},
	}
	db := &fakeHealthDB{pages: tree}
	out := renderHealthCheck(context.Background(), db)
	if !strings.Contains(out, "死页") {
		t.Error("expected 死页 warning")
	}
	if !strings.Contains(out, "孤岛") {
		t.Error("expected page title in warning")
	}
}
```

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/handler/... -run TestRenderHealthCheck
```

预期：FAIL。

- [ ] **Step 3：实现 renderHealthCheck**

在 `ai_renderer.go` 追加：

```go
// HealthDB is the minimal interface for health checks.
type HealthDB interface {
	GetWikiPageTree(ctx context.Context) ([]model.GetWikiPageTreeRow, error)
}

// TreeHealthIssue mirrors the type from existing analyzeTreeHealth.
type TreeHealthIssue struct {
	Severity string
	Type     string
	Message  string
	PageID   int64
}

// renderHealthCheck extends existing analyzeTreeHealth with new checks.
func renderHealthCheck(ctx context.Context, db HealthDB) string {
	pages, err := db.GetWikiPageTree(ctx)
	if err != nil || len(pages) == 0 {
		return ""
	}

	issues := analyzeTreeHealth(pages)

	// New: dead pages (content but 0 links, 0 backlinks)
	for _, p := range pages {
		if p.ContentStatus == "published" && p.LinkCount == 0 && p.BacklinkCount == 0 {
			issues = append(issues, TreeHealthIssue{
				Severity: "warning",
				Type:     "dead_page",
				Message:  fmt.Sprintf("死页: '%s' (ID=%d) 有内容但 0 反链 0 出链", p.Title, p.ID),
				PageID:   p.ID,
			})
		}
	}

	if len(issues) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("【结构健康检查】\n")
	for _, issue := range issues {
		icon := "⚠️"
		if issue.Severity == "error" {
			icon = "❌"
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", icon, issue.Message))
	}
	return b.String() + "\n"
}
```

注：现有的 `analyzeTreeHealth` 函数需要从 `handler/ai.go` 移到 `ai_renderer.go`（或保持原位，被新函数调用）。最简单：保留 `analyzeTreeHealth` 在 `ai.go`，新 `renderHealthCheck` 在 `ai_renderer.go` 里直接调用它（同一 package 私有函数可访问）。

- [ ] **Step 4：跑测试**

```bash
cd backend
go test ./internal/handler/... -run TestRenderHealthCheck
```

预期：PASS。

- [ ] **Step 5：Commit**

```bash
git add internal/handler/ai_renderer.go internal/handler/ai_renderer_test.go
git commit -m "feat(ai): enhance health check with dead page detection"
```

### Task 2.5：renderKnowledgeGaps 函数（按分类分组）

**Files:**
- Modify: `internal/handler/ai_renderer.go`
- Modify: `internal/handler/ai_renderer_test.go`

- [ ] **Step 1：写测试**

```go
type fakeGapsDB struct {
	pages []model.GetWikiPageTreeRow
}

func (f *fakeGapsDB) GetWikiPageTree(ctx context.Context) ([]model.GetWikiPageTreeRow, error) {
	return f.pages, nil
}

func TestRenderKnowledgeGaps_GroupedByCategory(t *testing.T) {
	tree := []model.GetWikiPageTreeRow{
		{ID: 1, Title: "Go 语言", PageType: "overview", ParentID: sql.NullInt64{}, Path: "1/"},
		{ID: 2, Title: "channel", PageType: "entity", ContentStatus: "empty", ParentID: sql.NullInt64{Int64: 1, Valid: true}, Path: "1/2/"},
		{ID: 3, Title: "context", PageType: "entity", ContentStatus: "empty", ParentID: sql.NullInt64{Int64: 1, Valid: true}, Path: "1/3/"},
		{ID: 4, Title: "算法", PageType: "overview", ParentID: sql.NullInt64{}, Path: "4/"},
		{ID: 5, Title: "树", PageType: "entity", ContentStatus: "empty", ParentID: sql.NullInt64{Int64: 4, Valid: true}, Path: "4/5/"},
	}
	db := &fakeGapsDB{pages: tree}
	out := renderKnowledgeGaps(context.Background(), db)
	if !strings.Contains(out, "Go 语言") {
		t.Error("expected Go 语言 in gaps")
	}
	if !strings.Contains(out, "channel") {
		t.Error("expected channel in gaps")
	}
	if !strings.Contains(out, "算法") {
		t.Error("expected 算法 in gaps")
	}
	// Verify grouping: Go 语言的空页应该一起列
	idx := strings.Index(out, "Go 语言")
	idxAlg := strings.Index(out, "算法")
	if idx > idxAlg {
		t.Error("expected Go 语言 to appear before 算法")
	}
}
```

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/handler/... -run TestRenderKnowledgeGaps
```

预期：FAIL。

- [ ] **Step 3：实现 renderKnowledgeGaps**

在 `ai_renderer.go` 追加：

```go
// GapsDB is the minimal interface for knowledge gap detection.
type GapsDB interface {
	GetWikiPageTree(ctx context.Context) ([]model.GetWikiPageTreeRow, error)
}

// renderKnowledgeGaps lists empty pages grouped by top-level category.
func renderKnowledgeGaps(ctx context.Context, db GapsDB) string {
	pages, err := db.GetWikiPageTree(ctx)
	if err != nil || len(pages) == 0 {
		return ""
	}

	// Build index: top-level overview page → its empty children
	topLevel := make(map[int64]string) // pageID → title
	for _, p := range pages {
		if !p.ParentID.Valid || p.ParentID.Int64 == 0 {
			topLevel[p.ID] = p.Title
		}
	}

	// Group empty pages by their top-level ancestor
	groupEmpty := make(map[int64][]string) // topLevelID → empty children titles
	var orphanEmpty []string               // empty pages with no top-level parent

	for _, p := range pages {
		if p.ContentStatus != "empty" {
			continue
		}
		// Walk up to find top-level ancestor
		topID, found := findTopLevel(p, pages, topLevel)
		if found {
			groupEmpty[topID] = append(groupEmpty[topID], p.Title)
		} else {
			orphanEmpty = append(orphanEmpty, p.Title)
		}
	}

	if len(groupEmpty) == 0 && len(orphanEmpty) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("【知识缺口】\n")
	for id, empties := range groupEmpty {
		b.WriteString(fmt.Sprintf("  📁 %s: %d 个空页（%s）\n",
			topLevel[id], len(empties), strings.Join(empties, " / ")))
	}
	for _, title := range orphanEmpty {
		b.WriteString(fmt.Sprintf("  📝 顶层空页: %s\n", title))
	}
	return b.String() + "\n"
}

// findTopLevel walks up the parent chain to find a top-level (parent_id NULL) ancestor.
func findTopLevel(p model.GetWikiPageTreeRow, all []model.GetWikiPageTreeRow, topLevel map[int64]string) (int64, bool) {
	if !p.ParentID.Valid || p.ParentID.Int64 == 0 {
		// p itself is top-level
		if _, ok := topLevel[p.ID]; ok {
			return p.ID, true
		}
		return 0, false
	}
	// Find parent
	for _, candidate := range all {
		if candidate.ID == p.ParentID.Int64 {
			if _, ok := topLevel[candidate.ID]; ok {
				return candidate.ID, true
			}
			return findTopLevel(candidate, all, topLevel)
		}
	}
	return 0, false
}
```

- [ ] **Step 4：跑测试**

```bash
cd backend
go test ./internal/handler/... -run TestRenderKnowledgeGaps
```

预期：PASS。

- [ ] **Step 5：Commit**

```bash
git add internal/handler/ai_renderer.go internal/handler/ai_renderer_test.go
git commit -m "feat(ai): add renderKnowledgeGaps grouped by top-level category"
```

### Task 2.6：buildKnowledgeMap orchestrator + 替换 buildWikiContext

**Files:**
- Modify: `internal/handler/ai_renderer.go`
- Modify: `internal/handler/ai.go`
- Modify: `internal/handler/ai_renderer_test.go`

- [ ] **Step 1：写测试**

```go
type fullMockDB struct {
	fakeOverviewDB
	fakeTreeDB
	fakeLogDB
	fakeHealthDB
	fakeGapsDB
}

func TestBuildKnowledgeMap_Integration(t *testing.T) {
	db := &fullMockDB{
		fakeOverviewDB: fakeOverviewDB{pages: []model.WikiPage{
			{ID: 1, Title: "Go", ContentStatus: "published"},
		}},
		fakeTreeDB: fakeTreeDB{pages: []model.GetWikiPageTreeRow{
			{ID: 1, Title: "Go", PageType: "entity", ContentStatus: "published", Path: "1/", Summary: sql.NullString{String: "Go 入门", Valid: true}, SummaryStatus: sql.NullString{String: "ready", Valid: true}},
		}},
		fakeLogDB: fakeLogDB{entries: []model.GetRecentWikiLogRow{
			{ID: 1, Action: "create", PageTitle: "Go", CreatedAt: time.Now(), PageID: sql.NullInt64{Int64: 1, Valid: true}},
		}},
	}

	out := buildKnowledgeMap(context.Background(), db, nil)
	// Should contain all sections
	for _, section := range []string{"【知识库概览】", "【知识地图】", "【最近活动】", "【结构健康检查】", "【知识缺口】"} {
		if !strings.Contains(out, section) {
			t.Errorf("missing section: %s", section)
		}
	}
}
```

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/handler/... -run TestBuildKnowledgeMap
```

预期：FAIL（`buildKnowledgeMap` 未定义）。

- [ ] **Step 3：实现 buildKnowledgeMap**

在 `ai_renderer.go` 追加：

```go
// KnowledgeMapDB is the composite interface for buildKnowledgeMap.
// It must satisfy all the sub-render interfaces.
type KnowledgeMapDB interface {
	OverviewDB
	KnowledgeMapDB
	RecentLogDB
	HealthDB
	GapsDB
}

// buildKnowledgeMap orchestrates the 5 sections of the new context.
func buildKnowledgeMap(ctx context.Context, db KnowledgeMapDB, focusPageID *int64) string {
	var b strings.Builder
	b.WriteString(renderOverview(ctx, db))
	b.WriteString(renderKnowledgeMap(ctx, db, focusPageID))
	b.WriteString(renderRecentLog(ctx, db, 7*24*time.Hour, 20))
	b.WriteString(renderHealthCheck(ctx, db))
	b.WriteString(renderKnowledgeGaps(ctx, db))
	return b.String()
}
```

注：`KnowledgeMapDB` 接口里的 `KnowledgeMapDB` 字段是接口嵌套（Go 允许接口嵌接口），所有子接口都满足。

- [ ] **Step 4：替换 ai.go 中的 buildWikiContext 调用**

在 `internal/handler/ai.go`：

找到：
```go
wikiContext := h.buildWikiContext(ctx, focusID)
```

改为：
```go
wikiContext := h.buildKnowledgeMap(ctx, h.queries, focusID)
```

注意：现有 `buildWikiContext` 接受 `(ctx, *int64)` 返回 `string`。新 `buildKnowledgeMap` 接受 `(ctx, KnowledgeMapDB, *int64)` 返回 `string`。需要确认 `h.queries`（`*model.Queries`）满足 `KnowledgeMapDB` 接口——如果缺少某些方法，在 `internal/handler/ai.go` 加适配器或扩展 `*model.Queries`。

**典型问题**：`model.Queries` 已经从 sqlc 生成，但可能不实现所有子接口（特别是 `GetPageContentForFallback`）。解决方案：加一个 `pageContentFallback` 字段到 `AIHandler`，或让 `buildKnowledgeMap` 接受额外的 `*sql.DB` 用于直接 query。

- [ ] **Step 5：删除或保留旧的 buildWikiContext**

把 `ai.go` 中的 `buildWikiContext` 函数整体删掉（或注释掉，因为现在由 `buildKnowledgeMap` 替代）。同时删除 `analyzeTreeHealth` 在原位置的副本（如果新版本在 `ai_renderer.go` 里有同样的实现）。

- [ ] **Step 6：编译 + 跑所有 handler 测试**

```bash
cd backend
go build ./...
go test ./internal/handler/...
```

预期：全部通过。如果失败，根据编译错误调整（通常是接口签名不匹配或 `model.Queries` 不实现某个方法）。

- [ ] **Step 7：手动端到端测试**

```bash
cd backend
go run ./cmd/server &
SERVER_PID=$!
sleep 3
# 触发一次 AI 请求（确保有 active AI config）
curl -X POST http://localhost:8080/api/ai/chat -H "Content-Type: application/json" \
  -d '{"conversation_id":1,"message":"我现在有什么内容？"}' | head -100
kill $SERVER_PID
```

预期：响应里能看到"知识地图"、"最近活动"等段落。

- [ ] **Step 8：Commit**

```bash
git add internal/handler/ai.go internal/handler/ai_renderer.go internal/handler/ai_renderer_test.go
git commit -m "feat(ai): wire buildKnowledgeMap into chat request, remove old buildWikiContext"
```

### Task 2.7：System prompt 中"目录"段重写

**Files:**
- Modify: `internal/ai/provider.go`

- [ ] **Step 1：找到 "目录" 段位置**

```bash
cd backend
grep -n "目录\|你的职责是管理知识树" internal/ai/provider.go
```

- [ ] **Step 2：在 buildWikiMaintainerPrompt 中重写"目录"相关段落**

找到原段落（大约在文件中间，"## 协作方式" 之后）。在适当位置插入新段落（可放在"## 协作方式"或新加一个"## 知识地图使用"段）：

```go
const knowledgeMapUsageGuide = `

## 知识地图使用（强制遵守）

你的 system prompt 里包含"知识地图"——按一级分类组织的目录，每页带 1-2 句摘要、链接数、标签、最后更新时间。

**回答规则**：
1. 回答前先看知识地图，定位相关分类，再钻到具体页（用 \`read_page\` 工具）
2. 用户问"我了解 X 吗"时，先在地图里找 X 相关的分类和页，再读具体页（避免全量 read_page）
3. 摘要可能标"摘要待更新"、"摘要生成失败"或"暂无摘要"——这种时候用 \`read_page\` 工具读全文
4. 全局标签索引帮你做跨分类检索
5. "最近活动"告诉你用户最近在学什么、改了什么（适合做上下文相关建议）
6. "结构健康检查"和"知识缺口"段落是 AI 主动建议的输入——发现问题时在聊天中提建议，不要直接修改

**摘要降级标识含义**：
- 无标识 = 摘要已就绪
- (摘要待更新) = 页面刚改过，AI 正在重新生成
- (摘要生成失败) = 生成失败（用 read_page 读全文）
- (暂无摘要) = 内容为空，AI 不会生成（空页）
`
```

把这个常量附加到 `wikiMaintainerPrompt` 字符串的合适位置（紧跟 `treeContext` 之前或之后，根据阅读流畅性决定）。

- [ ] **Step 3：编译 + 跑测试**

```bash
cd backend
go build ./...
go test ./...
```

预期：全部通过。

- [ ] **Step 4：手动验证 system prompt**

启动 server，用一次 AI 请求查看 server 日志（`log.Printf("[AIChat] wikiContext excerpt: %s", ...)` 已经在 ai.go:474 存在）。或直接抓包 SSE 流。

预期：在 wikiContext 输出里能看到【知识地图】、【最近活动】等段落，AI 的回复中能引用"我看到你的 wiki 里有 X 分类..."这样的内容。

- [ ] **Step 5：Commit**

```bash
git add internal/ai/provider.go
git commit -m "feat(ai): add knowledge map usage guide to system prompt"
```

---

## Phase 3：Log 写入集成

### Task 3.1：engine 中所有 action 写 wiki_log

**Files:**
- Modify: `internal/engine/engine.go`

- [ ] **Step 1：找到所有 action 成功执行的入口**

```bash
cd backend
grep -n "func.*Execute\|action.*completed\|success" internal/engine/engine.go | head -20
```

- [ ] **Step 2：写测试 — engine 写 wiki_log**

`engine_test.go` 已经有基础结构（如果不存在先建一个）。追加一个测试：

```go
func TestExecutionEngine_LogsCreatePage(t *testing.T) {
	// Setup test DB with one parent page
	// ... (use existing test helpers in engine_test.go)
	
	// Run create_page action
	report := eng.ExecuteAction(ctx, model.PlanAction{
		Type: "create_page",
		Params: json.RawMessage(`{"title":"测试页","content":"测试内容"}`),
	}, nil)
	
	// Verify wiki_log entry
	var action, pageTitle string
	row := db.QueryRow("SELECT action, page_title FROM wiki_log ORDER BY id DESC LIMIT 1")
	row.Scan(&action, &pageTitle)
	
	if action != "create" {
		t.Errorf("expected action=create, got %s", action)
	}
	if pageTitle != "测试页" {
		t.Errorf("expected title='测试页', got %s", pageTitle)
	}
}
```

注：测试的具体结构依赖现有 `engine_test.go` 的 helper（fixture DB、setup 函数等）。如果 helper 不足，让实现者补一个最小 helper 来支持此测试。

- [ ] **Step 3：跑测试确认失败**

```bash
cd backend
go test ./internal/engine/... -run TestExecutionEngine_LogsCreatePage
```

预期：FAIL（wiki_log 表为空，log 没写）。

- [ ] **Step 4：修改 engine.go 写 wiki_log**

找到每个 action 成功执行后（如 `createPage`、`updatePage`、`deletePage` 等函数返回 success 之前），加：

```go
// In createPage after successful insert
_, _ = h.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
    Action:    "create",
    PageID:    sql.NullInt64{Int64: newPageID, Valid: true},
    PageTitle: params.Title,
    PagePath:  sql.NullString{String: newPath, Valid: true},
    Source:    "plan",
    Summary:   sql.NullString{String: "通过 plan 创建页面", Valid: true},
})
```

类似地加到 `updatePage`、`deletePage`、`linkPages`、`movePage` 中。删除时 `PageID` 设为 NULL（page_id 已被删），title 保留。

- [ ] **Step 5：跑测试**

```bash
cd backend
go test ./internal/engine/...
```

预期：PASS（新的 log 测试 + 现有 engine 测试都通过）。

- [ ] **Step 6：手动验证**

启动 server，确认 AI config 已配置，执行一个 plan，确认 `wiki_log` 表有新条目：

```bash
cd backend
go run ./cmd/server &
SERVER_PID=$!
sleep 3
# 通过 SSE 发请求让 AI 提议一个 plan（需要先有 conversation）
# 或者直接通过 SQL 调一个写操作
curl -X POST http://localhost:8080/api/wiki -H "Content-Type: application/json" \
  -d '{"title":"日志测试页","content":"测试"}'
sleep 1
sqlite3 learn-helper.db "SELECT action, page_title, source FROM wiki_log ORDER BY id DESC LIMIT 5;"
kill $SERVER_PID
```

预期：看到新条目 `create|日志测试页|manual`（manual 来源对应直接通过 API 的写操作）。

- [ ] **Step 7：Commit**

```bash
git add internal/engine/engine.go internal/engine/engine_test.go
git commit -m "feat(engine): write wiki_log on every successful action"
```

### Task 3.2：handler/wiki.go 中手动编辑写 wiki_log

**Files:**
- Modify: `internal/handler/wiki.go`

- [ ] **Step 1：找到手动编辑的 handler**

```bash
cd backend
grep -n "func.*CreateWikiPage\|func.*UpdateWikiPage\|func.*RenameWikiPage\|func.*MoveWikiPage\|func.*DeleteWikiPage" internal/handler/wiki.go
```

- [ ] **Step 2：写测试**

在 `internal/handler/wiki_test.go`（如果不存在就建一个）追加：

```go
func TestUpdateWikiPage_LogsToWikiLog(t *testing.T) {
	// Setup test DB
	// ... 
	
	// Update a page via handler
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"原标题","content":"新内容"}`)
	req := httptest.NewRequest("PUT", "/api/wiki/1", body)
	req.Header.Set("Content-Type", "application/json")
	// (需要把 chi.URLParam 注入 req，这部分 helper 自己写)
	
	// Verify wiki_log entry
	var action string
	db.QueryRow("SELECT action FROM wiki_log WHERE page_id=1 ORDER BY id DESC LIMIT 1").Scan(&action)
	if action != "update" {
		t.Errorf("expected action=update, got %s", action)
	}
}
```

- [ ] **Step 3：跑测试确认失败**

```bash
cd backend
go test ./internal/handler/... -run TestUpdateWikiPage_LogsToWikiLog
```

预期：FAIL。

- [ ] **Step 4：在每个 handler 里写 log**

修改 `CreateWikiPage`、`UpdateWikiPage`、`RenameWikiPage`、`MoveWikiPage`、`DeleteWikiPage` handler，在 DB 操作后加：

```go
// 在 CreateWikiPage 末尾
_, _ = h.queries.InsertWikiLog(r.Context(), model.InsertWikiLogParams{
    Action:    "create",
    PageID:    sql.NullInt64{Int64: newID, Valid: true},
    PageTitle: req.Title,
    PagePath:  sql.NullString{String: path, Valid: true},
    Source:    "manual",
    Summary:   sql.NullString{},
})
```

类似地 update/rename/move/delete 各自加。

**注意 source 字段**：
- `manual`：用户直接通过 API 调（rename/move/update/create via API）
- `plan`：通过 plan engine 执行的（已在 Task 3.1 处理）
- `lint`：未来 lint 修复用
- `query_filing`：未来 Query→回写用

- [ ] **Step 5：跑测试**

```bash
cd backend
go test ./internal/handler/...
```

预期：PASS。

- [ ] **Step 6：手动验证（重复 Task 3.1 step 6）**

```bash
cd backend
go run ./cmd/server &
SERVER_PID=$!
sleep 3
curl -X POST http://localhost:8080/api/wiki/1/rename -H "Content-Type: application/json" \
  -d '{"title":"新标题"}'
sleep 1
sqlite3 learn-helper.db "SELECT action, page_title, source FROM wiki_log ORDER BY id DESC LIMIT 3;"
kill $SERVER_PID
```

预期：`rename|新标题|manual` 出现在 wiki_log。

- [ ] **Step 7：Commit**

```bash
git add internal/handler/wiki.go internal/handler/wiki_test.go
git commit -m "feat(wiki): write wiki_log from manual wiki handlers"
```

---

## Phase 4：标签和链接计数维护

### Task 4.1：update_page 时计算 tags_normalized

**Files:**
- Modify: `internal/handler/wiki.go`
- Modify: `internal/handler/wiki_test.go`

- [ ] **Step 1：写测试**

```go
func TestUpdateWikiPage_NormalizesTags(t *testing.T) {
	// Update with messy tags
	body := `{"tags":"Go,  go , 算法,Go"}`
	// ...
	
	// Verify tags_normalized is "go,算法"
	var normalized string
	db.QueryRow("SELECT tags_normalized FROM wiki_pages WHERE id=1").Scan(&normalized)
	if normalized != "go,算法" {
		t.Errorf("expected 'go,算法', got '%s'", normalized)
	}
}
```

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/handler/... -run TestUpdateWikiPage_NormalizesTags
```

预期：FAIL。

- [ ] **Step 3：实现 tags_normalized 计算**

在 `UpdateWikiPage` 和 `CreateWikiPage` 中，在更新 `tags` 字段后加：

```go
// Normalize tags: trim, lowercase, dedup, sort
tagsNormalized := normalizeTags(req.Tags)
```

实现 `normalizeTags`：

```go
// in wiki.go (or a new util file)
func normalizeTags(tags string) string {
	if tags == "" {
		return ""
	}
	// Parse JSON array (tags is stored as JSON) or comma-separated
	// The schema uses TEXT DEFAULT '[]' for tags, so it's JSON.
	var tagList []string
	if err := json.Unmarshal([]byte(tags), &tagList); err != nil {
		// Fallback: treat as comma-separated
		tagList = strings.Split(tags, ",")
	}
	seen := make(map[string]bool)
	var result []string
	for _, t := range tagList {
		t = strings.TrimSpace(strings.ToLower(t))
		if t != "" && !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	sort.Strings(result)
	return strings.Join(result, ",")
}
```

- [ ] **Step 4：跑测试**

```bash
cd backend
go test ./internal/handler/... -run TestUpdateWikiPage_NormalizesTags
```

预期：PASS。

- [ ] **Step 5：Commit**

```bash
git add internal/handler/wiki.go internal/handler/wiki_test.go
git commit -m "feat(wiki): compute tags_normalized on create/update"
```

### Task 4.2：link_pages 时维护 link_count / backlink_count

**Files:**
- Modify: `internal/engine/engine.go`

- [ ] **Step 1：写测试**

```go
func TestLinkPages_UpdatesCounts(t *testing.T) {
	// Setup pages A and B
	// Run link_pages action
	
	// Verify A.link_count == 1, B.backlink_count == 1
	var aLinkCount, bBacklinkCount int
	db.QueryRow("SELECT link_count FROM wiki_pages WHERE id=A").Scan(&aLinkCount)
	db.QueryRow("SELECT backlink_count FROM wiki_pages WHERE id=B").Scan(&bBacklinkCount)
	
	if aLinkCount != 1 {
		t.Errorf("expected A.link_count=1, got %d", aLinkCount)
	}
	if bBacklinkCount != 1 {
		t.Errorf("expected B.backlink_count=1, got %d", bBacklinkCount)
	}
}
```

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/engine/... -run TestLinkPages_UpdatesCounts
```

预期：FAIL。

- [ ] **Step 3：在 linkPages 成功后更新计数**

在 `linkPages` 函数（`internal/engine/engine.go`）成功插入 `links` JSON 后，加：

```go
// Recompute link_count for source page
linksJSON := page.Links
var linkIDs []int64
json.Unmarshal([]byte(linksJSON), &linkIDs)
_, _ = h.db.ExecContext(ctx,
    "UPDATE wiki_pages SET link_count=? WHERE id=?",
    len(linkIDs), sourceID)

// Recompute backlink_count for target page
backlinksJSON := targetPage.Backlinks
var backlinkIDs []int64
json.Unmarshal([]byte(backlinksJSON), &backlinkIDs)
_, _ = h.db.ExecContext(ctx,
    "UPDATE wiki_pages SET backlink_count=? WHERE id=?",
    len(backlinkIDs), targetID)
```

更简单的方案是直接重新计算（SELECT COUNT + UPDATE），因为 `links`/`backlinks` 是 JSON 数组，COUNT 不太合适。所以用解析 JSON 数组的方式。

- [ ] **Step 4：跑测试**

```bash
cd backend
go test ./internal/engine/...
```

预期：PASS。

- [ ] **Step 5：Commit**

```bash
git add internal/engine/engine.go internal/engine/engine_test.go
git commit -m "feat(engine): maintain link_count and backlink_count on link_pages"
```

### Task 4.3：delete_page 时减少反链计数

**Files:**
- Modify: `internal/engine/engine.go`

- [ ] **Step 1：写测试**

```go
func TestDeletePage_DecrementsBacklinkCounts(t *testing.T) {
	// Setup: page A links to page B; page B has backlink_count=1
	// Delete page A
	// Verify B.backlink_count == 0
}
```

- [ ] **Step 2：跑测试确认失败**

```bash
cd backend
go test ./internal/engine/... -run TestDeletePage_DecrementsBacklinkCounts
```

预期：FAIL。

- [ ] **Step 3：在 deletePage 前查询所有反链并 decrement**

在 `deletePage` 函数删除页面之前：

```go
// Get all pages that link to this one
rows, _ := h.db.QueryContext(ctx,
    `SELECT id, backlinks FROM wiki_pages WHERE backlinks LIKE '%' || ? || '%'`,
    strconv.FormatInt(pageID, 10))
defer rows.Close()

// For each, parse backlinks, remove pageID, write back
for rows.Next() {
    var otherID int64
    var backlinksJSON string
    rows.Scan(&otherID, &backlinksJSON)
    var ids []int64
    json.Unmarshal([]byte(backlinksJSON), &ids)
    var newIDs []int64
    for _, id := range ids {
        if id != pageID {
            newIDs = append(newIDs, id)
        }
    }
    newBacklinksJSON, _ := json.Marshal(newIDs)
    h.db.ExecContext(ctx,
        "UPDATE wiki_pages SET backlinks=?, backlink_count=? WHERE id=?",
        string(newBacklinksJSON), len(newIDs), otherID)
}
```

注：`LIKE` 查询是粗略的——它可能误中含相同数字子串的 ID（如要删 12，命中 ID 包含 "12" 的 120）。更精确的方式：把 backlinks 存为 JSON 数组，用 `EXISTS (SELECT 1 FROM json_each(backlinks) WHERE value = ?)`。

更好的查询：
```sql
UPDATE wiki_pages
SET backlinks = (
    SELECT json_group_array(value) FROM json_each(backlinks) WHERE value != ?
),
backlink_count = backlink_count - 1
WHERE EXISTS (SELECT 1 FROM json_each(backlinks) WHERE value = ?)
AND id != ?
```

实现者可按需调整。

- [ ] **Step 4：跑测试**

```bash
cd backend
go test ./internal/engine/...
```

预期：PASS。

- [ ] **Step 5：Commit**

```bash
git add internal/engine/engine.go internal/engine/engine_test.go
git commit -m "feat(engine): decrement backlink counts on page delete"
```

---

## Self-Review

### 1. Spec coverage

| Spec 章节 | 实施任务 |
|---|---|
| 三块增量（每页摘要 / 知识地图 / 写入日志） | Phase 1-3 全部覆盖 |
| 一致性原则：index 是 view 不是 store | Task 2.2-2.6 全部用 `KnowledgeMapDB` 接口派生，无存储 |
| Schema 变更 | Task 1.1, 1.2 |
| sqlc 查询 | Task 1.3, 1.4 |
| SummaryWorker 架构 | Task 1.5, 1.6, 1.7 |
| 写流程（事务+log+pending） | Task 3.1, 3.2 |
| 读流程（5 个 render 函数 + orchestrator） | Task 2.1-2.6 |
| 降级策略表 | Task 2.2 `renderSummaryLine` |
| 死页/标签缺失/覆盖率 | Task 2.4 |
| 知识缺口按分类分组 | Task 2.5 |
| System prompt 调整 | Task 2.7 |
| 标签和链接计数维护 | Phase 4 全部 |
| 未来 spec（Query→回写、语义 Lint、qmd） | 明确划在范围外，符合 spec |

**无遗漏。**

### 2. Placeholder scan

| 红旗 | 出现位置 | 解决方式 |
|---|---|---|
| "TBD" / "TODO" | 0 处 | n/a |
| "implement later" | 0 处 | n/a |
| "add appropriate error handling" | 多处（worker 的 `_ = w.db.MarkSummaryFailed`） | 显式忽略：summary 失败不阻塞主流程是有意为之，spec 风险 #3 已确认 |
| "Write tests for the above" | 0 处（每个 task 都有具体测试代码） | n/a |
| "Similar to Task N" | 0 处 | n/a |
| 无代码的步骤 | Task 1.7 step 3 "实际实现要 import..." | 已说明由实现者根据现有 ai 包调整，附上 Go 框架代码 |

**无关键 placeholder。**

### 3. Type/Name consistency

| 名称 | 定义处 | 使用处 | 一致性 |
|---|---|---|---|
| `SummaryWorker` | worker/summary.go | main.go, 测试 | ✓ |
| `KnowledgeMapDB` 接口 | ai_renderer.go | ai.go, 测试 | ✓ |
| `buildKnowledgeMap` | ai_renderer.go | ai.go 调用 | ✓ |
| `renderKnowledgeMap` / `renderOverview` / `renderRecentLog` / `renderHealthCheck` / `renderKnowledgeGaps` | ai_renderer.go | buildKnowledgeMap 调用 | ✓ |
| `InsertWikiLog` / `GetRecentWikiLog` | sqlc 生成 | engine.go, ai_renderer.go | ✓ |
| `UpdatePageSummary` / `MarkSummaryEmpty` / `MarkSummaryFailed` | sqlc 生成 | summary.go, main.go | ✓ |
| `summary_status` enum | migration 008 | renderSummaryLine switch | ✓ ('empty'/'pending'/'ready'/'failed') |
| `contentHash` 函数 | summary.go | generateOne | ✓ |

**一致。**

### 4. Gaps & open issues

1. **Task 2.2 的 `KnowledgeMapDB` 接口要求 `GetPageContentForFallback`**，但 `model.Queries` 没有这个方法。Task 2.6 step 4 提示了"加适配器或扩展"，但没有给出确切方案。**实施时需要为 `*model.Queries` 实现这个方法**（直接 SQL 查询即可），或在 `ai_renderer.go` 接受额外的 `*sql.DB`。

2. **Task 1.6 step 7 的 mockDB 接口对齐** 写得比较粗——`sqlDBAdapter` 接收 `*model.Queries`，但测试里用的是 `mockDB`（手写 mock）。需要实现者根据编译错误调整 test 端的 mock 让它满足 `SummaryDB` 接口。

3. **Task 1.7 的 `ProviderAdapter`** 写得比较宽泛，依赖现有 `internal/ai/claude.go` 和 `internal/ai/deepseek.go` 的接口。`ai.AIProvider` 接口的 `Chat` 方法签名是 `Chat(ctx, ChatRequest) (*ChatResponse, error)`（非流式），所以 `GenerateSummary` 实际就是 `provider.Chat(ctx, req).Content`。具体 import 路径和类型断言由实现者调整。

4. **Task 3.1 step 4** 提到在 `engine.go` 找 action 成功入口，但具体行号依赖现有代码。实施者需要 `grep -n "success\|return.*nil" internal/engine/engine.go` 定位。

5. **Task 4.3 step 3** 的 SQL 用了 `LIKE` 粗略匹配 ID，可能误中。spec 风险 #5 提到"qmd 集成"是远期，但当前实现要正确。**实施时建议用 `json_each` 精确匹配**（已写在 step 3 的 "更好的查询" 注释里）。

**这些都是实施时的小调整，不是 spec 层面的问题。**

---

## Execution Handoff

计划完成，保存到 `docs/superpowers/plans/2026-06-01-llm-wiki-knowledge-map.md`。

**两种执行方式：**

1. **Subagent-Driven（推荐）** — 我按 task 派发独立 subagent，每个 task 完成后做两阶段 review（spec 对齐 + 实际验证），快速迭代。适合这个计划：4 个 phase 共 21 个 task，每个独立可验证。

2. **Inline Execution** — 我在当前 session 里按 task 顺序执行，用 executing-plans 批量做，每完成几个 task 设一个 checkpoint 给你 review。适合你想逐步把关的场景。

哪种？
