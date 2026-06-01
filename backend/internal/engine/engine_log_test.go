package engine

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"learn-helper/internal/model"
)

// newTestDB returns an in-memory SQLite database with the project schema applied.
// The DB is closed via t.Cleanup.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	schemaPath := findSchemaSQL(t)
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}

	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	// SQLite serializes writes; limiting to one connection avoids SQLITE_BUSY in tests.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(string(schemaBytes)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })
	return db
}

func findSchemaSQL(t *testing.T) string {
	t.Helper()
	// Walk up from the test's working dir to find db/migrations/schema.sql.
	// `go test` runs the package in its directory, so the relative path is stable.
	candidates := []string{
		"db/migrations/schema.sql",
		"../../db/migrations/schema.sql",
		"../../../db/migrations/schema.sql",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	t.Fatalf("could not locate db/migrations/schema.sql from cwd=%s", mustGetwd(t))
	return ""
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}

// seedParentPage creates a simple parent page directly via SQL so that
// child-creation actions have a valid parent_id to reference.
func seedParentPage(t *testing.T, db *sql.DB, title string) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO wiki_pages (title, slug, page_type, content, content_status, sort_order, path, links, backlinks)
		 VALUES (?, ?, 'overview', '', 'empty', 0, '1/', '[]', '[]')`,
		title, title,
	)
	if err != nil {
		t.Fatalf("seed parent: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	// Fix path to include the actual id.
	if _, err := db.Exec(`UPDATE wiki_pages SET path = ? WHERE id = ?`, pathFor(id), id); err != nil {
		t.Fatalf("update path: %v", err)
	}
	return id
}

func pathFor(id int64) string {
	// Matches engine.execCreatePage convention.
	return strconv.FormatInt(id, 10) + "/"
}

func latestLogEntry(t *testing.T, db *sql.DB) (action, pageTitle, source string, pageIDValid bool, pageID int64) {
	t.Helper()
	row := db.QueryRow(`SELECT action, page_id, page_title, source FROM wiki_log ORDER BY id DESC LIMIT 1`)
	var pid sql.NullInt64
	if err := row.Scan(&action, &pid, &pageTitle, &source); err != nil {
		t.Fatalf("scan wiki_log: %v", err)
	}
	if pid.Valid {
		pageIDValid = true
		pageID = pid.Int64
	}
	return
}

func countLogEntries(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM wiki_log`).Scan(&n); err != nil {
		t.Fatalf("count wiki_log: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestExecutionEngine_LogsCreatePage(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	params := map[string]any{
		"title":   "测试页",
		"content": "测试内容",
	}
	if _, err := eng.execCreatePage(ctx, params); err != nil {
		t.Fatalf("execCreatePage: %v", err)
	}

	action, title, source, hasPageID, pageID := latestLogEntry(t, db)
	if action != "create" {
		t.Errorf("action = %q, want %q", action, "create")
	}
	if title != "测试页" {
		t.Errorf("page_title = %q, want %q", title, "测试页")
	}
	if source != "plan" {
		t.Errorf("source = %q, want %q", source, "plan")
	}
	if !hasPageID {
		t.Errorf("expected page_id to be set on create log entry")
	}
	if pageID <= 0 {
		t.Errorf("expected positive page_id, got %d", pageID)
	}
}

func TestExecutionEngine_LogsUpdatePage(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	pageID := seedParentPage(t, db, "原标题")

	params := map[string]any{
		"page_id": pageID,
		"title":   "新标题",
		"content": "更新内容",
	}
	if _, err := eng.execUpdatePage(ctx, params); err != nil {
		t.Fatalf("execUpdatePage: %v", err)
	}

	action, title, _, hasPageID, loggedID := latestLogEntry(t, db)
	if action != "update" {
		t.Errorf("action = %q, want %q", action, "update")
	}
	if title != "新标题" {
		t.Errorf("page_title = %q, want %q", title, "新标题")
	}
	if !hasPageID {
		t.Errorf("expected page_id to be set on update log entry")
	}
	if loggedID != pageID {
		t.Errorf("logged page_id = %d, want %d", loggedID, pageID)
	}
}

func TestExecutionEngine_LogsDeletePage(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	pageID := seedParentPage(t, db, "要删除的页")

	// Sanity: log is empty before delete.
	if n := countLogEntries(t, db); n != 0 {
		t.Fatalf("expected 0 log entries before delete, got %d", n)
	}

	params := map[string]any{"page_id": pageID}
	if _, err := eng.execDeletePage(ctx, params); err != nil {
		t.Fatalf("execDeletePage: %v", err)
	}

	action, title, _, hasPageID, _ := latestLogEntry(t, db)
	if action != "delete" {
		t.Errorf("action = %q, want %q", action, "delete")
	}
	if title != "要删除的页" {
		t.Errorf("page_title = %q, want %q (title must be preserved on delete)", title, "要删除的页")
	}
	if hasPageID {
		t.Errorf("page_id must be NULL on delete log entry, but was set")
	}
}

func TestExecutionEngine_LogsLinkPages(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	sourceID := seedParentPage(t, db, "源页")
	targetID := seedParentPage(t, db, "目标页")

	params := map[string]any{
		"source_page_id": sourceID,
		"target_page_id": targetID,
		"link_text":      "目标页",
	}
	if _, err := eng.execLinkPages(ctx, params); err != nil {
		t.Fatalf("execLinkPages: %v", err)
	}

	action, title, _, hasPageID, loggedID := latestLogEntry(t, db)
	if action != "link" {
		t.Errorf("action = %q, want %q", action, "link")
	}
	if title != "源页" {
		t.Errorf("page_title = %q, want %q (source page title)", title, "源页")
	}
	if !hasPageID {
		t.Errorf("expected page_id to be set on link log entry")
	}
	if loggedID != sourceID {
		t.Errorf("logged page_id = %d, want source id %d", loggedID, sourceID)
	}
}

func TestExecutionEngine_LogsMovePage(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	parentID := seedParentPage(t, db, "父页")
	childID := seedParentPage(t, db, "子页")

	params := map[string]any{
		"page_id":       childID,
		"new_parent_id": parentID,
	}
	if _, err := eng.execMovePage(ctx, params); err != nil {
		t.Fatalf("execMovePage: %v", err)
	}

	action, title, _, hasPageID, loggedID := latestLogEntry(t, db)
	if action != "move" {
		t.Errorf("action = %q, want %q", action, "move")
	}
	if title != "子页" {
		t.Errorf("page_title = %q, want %q", title, "子页")
	}
	if !hasPageID {
		t.Errorf("expected page_id to be set on move log entry")
	}
	if loggedID != childID {
		t.Errorf("logged page_id = %d, want %d", loggedID, childID)
	}
}

func TestExecutionEngine_LinkPagesUpdatesCounts(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	sourceID := seedParentPage(t, db, "源页")
	targetID := seedParentPage(t, db, "目标页")

	params := map[string]any{
		"source_page_id": sourceID,
		"target_page_id": targetID,
		"link_text":      "目标页",
	}
	if _, err := eng.execLinkPages(ctx, params); err != nil {
		t.Fatalf("execLinkPages: %v", err)
	}

	var aLinkCount, bBacklinkCount int
	if err := db.QueryRow("SELECT link_count FROM wiki_pages WHERE id=?", sourceID).Scan(&aLinkCount); err != nil {
		t.Fatalf("read source link_count: %v", err)
	}
	if err := db.QueryRow("SELECT backlink_count FROM wiki_pages WHERE id=?", targetID).Scan(&bBacklinkCount); err != nil {
		t.Fatalf("read target backlink_count: %v", err)
	}
	if aLinkCount != 1 {
		t.Errorf("source link_count = %d, want 1", aLinkCount)
	}
	if bBacklinkCount != 1 {
		t.Errorf("target backlink_count = %d, want 1", bBacklinkCount)
	}
}

func TestExecutionEngine_DeletePageDecrementsBacklinkCounts(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	// Page A links to page B (so B has backlink_count=1).
	resA, err := db.Exec(
		`INSERT INTO wiki_pages (title, slug, page_type, content, path, links, backlinks, content_status, sort_order, created_at, updated_at)
		 VALUES (?, ?, 'entity', '', ?, ?, ?, 'empty', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"A", "a", "1/", "[2]", "[]")
	if err != nil {
		t.Fatalf("insert A: %v", err)
	}
	idA, err := resA.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id A: %v", err)
	}

	resB, err := db.Exec(
		`INSERT INTO wiki_pages (title, slug, page_type, content, path, links, backlinks, content_status, backlink_count, sort_order, created_at, updated_at)
		 VALUES (?, ?, 'entity', '', ?, '[]', ?, 'empty', 1, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"B", "b", "2/", "[1]")
	if err != nil {
		t.Fatalf("insert B: %v", err)
	}
	idB, err := resB.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id B: %v", err)
	}

	// Delete page A.
	params := map[string]any{"page_id": idA}
	if _, err := eng.execDeletePage(ctx, params); err != nil {
		t.Fatalf("execDeletePage: %v", err)
	}

	// Verify B.backlink_count == 0 and B.backlinks no longer contains A's id.
	var bBacklinkCount int
	if err := db.QueryRow("SELECT backlink_count FROM wiki_pages WHERE id=?", idB).Scan(&bBacklinkCount); err != nil {
		t.Fatalf("read B.backlink_count: %v", err)
	}
	if bBacklinkCount != 0 {
		t.Errorf("expected B.backlink_count=0 after deleting A, got %d", bBacklinkCount)
	}

	var bBacklinks string
	if err := db.QueryRow("SELECT backlinks FROM wiki_pages WHERE id=?", idB).Scan(&bBacklinks); err != nil {
		t.Fatalf("read B.backlinks: %v", err)
	}
	if strings.Contains(bBacklinks, fmt.Sprintf("%d", idA)) {
		t.Errorf("expected B.backlinks to no longer contain id %d, got %s", idA, bBacklinks)
	}
}

func TestExecutionEngine_FailedActionDoesNotLog(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	// Try to create a page with no title — should fail and not log.
	if _, err := eng.execCreatePage(ctx, map[string]any{}); err == nil {
		t.Fatalf("expected error for missing title")
	}
	if n := countLogEntries(t, db); n != 0 {
		t.Errorf("failed action should not log; got %d entries", n)
	}
}

// ---------------------------------------------------------------------------
// onPageWritten callback wiring (C1/C3 fix)
// ---------------------------------------------------------------------------

// recordingCallback returns a callback that appends every pageID it receives.
type recordingCallback struct {
	mu    sync.Mutex
	calls []int64
}

func (r *recordingCallback) fn() func(pageID int64) {
	return func(pageID int64) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.calls = append(r.calls, pageID)
	}
}

func (r *recordingCallback) snapshot() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int64, len(r.calls))
	copy(out, r.calls)
	return out
}

func readSummaryStatus(t *testing.T, db *sql.DB, pageID int64) string {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT summary_status FROM wiki_pages WHERE id = ?`, pageID).Scan(&status); err != nil {
		t.Fatalf("read summary_status for page %d: %v", pageID, err)
	}
	return status
}

func TestExecutionEngine_ExecCreatePageInvokesOnPageWritten(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	rec := &recordingCallback{}
	eng.SetOnPageWritten(rec.fn())

	params := map[string]any{
		"title":   "回调测试页",
		"content": "一些内容",
	}
	res, err := eng.execCreatePage(ctx, params)
	if err != nil {
		t.Fatalf("execCreatePage: %v", err)
	}
	resultMap, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("execCreatePage returned non-map result: %T", res)
	}
	pageID, ok := resultMap["page_id"].(int64)
	if !ok {
		t.Fatalf("execCreatePage result missing page_id: %v", resultMap)
	}

	calls := rec.snapshot()
	if len(calls) != 1 {
		t.Fatalf("onPageWritten called %d times, want 1", len(calls))
	}
	if calls[0] != pageID {
		t.Errorf("onPageWritten received pageID=%d, want %d", calls[0], pageID)
	}

	if status := readSummaryStatus(t, db, pageID); status != "pending" {
		t.Errorf("summary_status after create = %q, want %q", status, "pending")
	}
}

func TestExecutionEngine_ExecUpdatePageInvokesOnPageWritten(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	pageID := seedParentPage(t, db, "原标题")

	rec := &recordingCallback{}
	eng.SetOnPageWritten(rec.fn())

	params := map[string]any{
		"page_id": pageID,
		"title":   "新标题",
		"content": "更新内容",
	}
	if _, err := eng.execUpdatePage(ctx, params); err != nil {
		t.Fatalf("execUpdatePage: %v", err)
	}

	calls := rec.snapshot()
	if len(calls) != 1 {
		t.Fatalf("onPageWritten called %d times, want 1", len(calls))
	}
	if calls[0] != pageID {
		t.Errorf("onPageWritten received pageID=%d, want %d", calls[0], pageID)
	}

	if status := readSummaryStatus(t, db, pageID); status != "pending" {
		t.Errorf("summary_status after update = %q, want %q", status, "pending")
	}
}

func TestExecutionEngine_OnPageWrittenNilSafe(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	// No callback set — should not panic on create or update.
	if _, err := eng.execCreatePage(ctx, map[string]any{"title": "X"}); err != nil {
		t.Fatalf("execCreatePage with nil callback: %v", err)
	}
	pageID := seedParentPage(t, db, "Y")
	if _, err := eng.execUpdatePage(ctx, map[string]any{"page_id": pageID, "title": "Y2"}); err != nil {
		t.Fatalf("execUpdatePage with nil callback: %v", err)
	}
}

func TestExecutionEngine_SetOnPageWrittenClearWithNil(t *testing.T) {
	db := newTestDB(t)
	q := model.New(db)
	eng := NewExecutionEngine(db, q)
	ctx := context.Background()

	rec := &recordingCallback{}
	eng.SetOnPageWritten(rec.fn())
	if _, err := eng.execCreatePage(ctx, map[string]any{"title": "first"}); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// Clear the callback.
	eng.SetOnPageWritten(nil)
	if _, err := eng.execCreatePage(ctx, map[string]any{"title": "second"}); err != nil {
		t.Fatalf("second create: %v", err)
	}

	if calls := rec.snapshot(); len(calls) != 1 {
		t.Errorf("onPageWritten called %d times after clear, want 1", len(calls))
	}
}
