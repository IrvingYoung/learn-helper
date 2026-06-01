package handler

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/go-chi/chi/v5"
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
	t.Fatalf("could not locate db/migrations/schema.sql")
	return ""
}

// seedPage inserts a wiki page directly via SQL and returns its id.
// Path is fixed to "<id>/" (matching engine.execCreatePage convention).
func seedPage(t *testing.T, db *sql.DB, title string) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO wiki_pages (title, slug, page_type, content, content_status, sort_order, path, links, backlinks)
		 VALUES (?, ?, 'entity', '', 'empty', 0, '0/', '[]', '[]')`,
		title, title,
	)
	if err != nil {
		t.Fatalf("seed page: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	if _, err := db.Exec(`UPDATE wiki_pages SET path = ? WHERE id = ?`, pathForID(id), id); err != nil {
		t.Fatalf("update path: %v", err)
	}
	return id
}

func pathForID(id int64) string {
	return strconv.FormatInt(id, 10) + "/"
}

// latestLogEntry returns the most recent wiki_log row.
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

// doRequest runs the given handler through a chi router so that
// chi.URLParam can extract the {id} path parameter.
func doRequest(h http.HandlerFunc, method, target, routePattern string, body []byte) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}

	r := chi.NewRouter()
	r.Method(method, routePattern, h)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// newTestHandler builds a WikiHandler backed by an in-memory DB.
func newTestHandler(t *testing.T) (*WikiHandler, *sql.DB) {
	t.Helper()
	db := newTestDB(t)
	return NewWikiHandler(db), db
}

func TestCreateWikiPage_LogsToWikiLog(t *testing.T) {
	h, db := newTestHandler(t)

	rec := doRequest(h.CreateWikiPage, "POST", "/api/wiki", "/api/wiki",
		[]byte(`{"title":"新页面","slug":"new-page","page_type":"entity","content":"hello","content_status":"published"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateWikiPage: expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	action, title, source, hasPageID, pageID := latestLogEntry(t, db)
	if action != "create" {
		t.Errorf("action = %q, want %q", action, "create")
	}
	if title != "新页面" {
		t.Errorf("page_title = %q, want %q", title, "新页面")
	}
	if source != "manual" {
		t.Errorf("source = %q, want %q", source, "manual")
	}
	if !hasPageID {
		t.Errorf("expected page_id to be set on create log entry")
	}
	if pageID <= 0 {
		t.Errorf("expected positive page_id, got %d", pageID)
	}
}

func TestUpdateWikiPage_LogsToWikiLog(t *testing.T) {
	h, db := newTestHandler(t)
	pageID := seedPage(t, db, "原标题")

	target := "/api/wiki/" + pathInt(pageID)
	route := "/api/wiki/{id}"
	rec := doRequest(h.UpdateWikiPage, "PUT", target, route,
		[]byte(`{"title":"新标题","slug":"new-slug","content":"新内容","content_status":"published"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("UpdateWikiPage: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	action, title, source, hasPageID, loggedID := latestLogEntry(t, db)
	if action != "update" {
		t.Errorf("action = %q, want %q", action, "update")
	}
	if title != "新标题" {
		t.Errorf("page_title = %q, want %q", title, "新标题")
	}
	if source != "manual" {
		t.Errorf("source = %q, want %q", source, "manual")
	}
	if !hasPageID {
		t.Errorf("expected page_id to be set on update log entry")
	}
	if loggedID != pageID {
		t.Errorf("logged page_id = %d, want %d", loggedID, pageID)
	}
}

func TestRenameWikiPage_LogsToWikiLog(t *testing.T) {
	h, db := newTestHandler(t)
	pageID := seedPage(t, db, "原标题")

	target := "/api/wiki/" + pathInt(pageID) + "/rename"
	route := "/api/wiki/{id}/rename"
	rec := doRequest(h.RenameWikiPage, "PATCH", target, route,
		[]byte(`{"title":"新标题"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("RenameWikiPage: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	action, title, source, hasPageID, loggedID := latestLogEntry(t, db)
	if action != "rename" {
		t.Errorf("action = %q, want %q", action, "rename")
	}
	if title != "新标题" {
		t.Errorf("page_title = %q, want %q", title, "新标题")
	}
	if source != "manual" {
		t.Errorf("source = %q, want %q", source, "manual")
	}
	if !hasPageID {
		t.Errorf("expected page_id to be set on rename log entry")
	}
	if loggedID != pageID {
		t.Errorf("logged page_id = %d, want %d", loggedID, pageID)
	}
}

func TestMoveWikiPage_LogsToWikiLog(t *testing.T) {
	h, db := newTestHandler(t)
	parentID := seedPage(t, db, "父页")
	childID := seedPage(t, db, "子页")

	target := "/api/wiki/" + pathInt(childID) + "/move"
	route := "/api/wiki/{id}/move"
	body := []byte(`{"parent_id":` + pathInt(parentID) + `}`)
	rec := doRequest(h.MoveWikiPage, "PATCH", target, route, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("MoveWikiPage: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	action, title, source, hasPageID, loggedID := latestLogEntry(t, db)
	if action != "move" {
		t.Errorf("action = %q, want %q", action, "move")
	}
	if title != "子页" {
		t.Errorf("page_title = %q, want %q", title, "子页")
	}
	if source != "manual" {
		t.Errorf("source = %q, want %q", source, "manual")
	}
	if !hasPageID {
		t.Errorf("expected page_id to be set on move log entry")
	}
	if loggedID != childID {
		t.Errorf("logged page_id = %d, want %d", loggedID, childID)
	}
}

func TestDeleteWikiPage_LogsToWikiLog(t *testing.T) {
	h, db := newTestHandler(t)
	pageID := seedPage(t, db, "要删除的页")

	if n := countLogEntries(t, db); n != 0 {
		t.Fatalf("expected 0 log entries before delete, got %d", n)
	}

	target := "/api/wiki/" + pathInt(pageID)
	route := "/api/wiki/{id}"
	rec := doRequest(h.DeleteWikiPage, "DELETE", target, route, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DeleteWikiPage: expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}

	action, title, source, hasPageID, _ := latestLogEntry(t, db)
	if action != "delete" {
		t.Errorf("action = %q, want %q", action, "delete")
	}
	if title != "要删除的页" {
		t.Errorf("page_title = %q, want %q (title must be preserved on delete)", title, "要删除的页")
	}
	if source != "manual" {
		t.Errorf("source = %q, want %q", source, "manual")
	}
	if hasPageID {
		t.Errorf("page_id must be NULL on delete log entry, but was set")
	}
}

// pathInt formats an int64 for use inside a URL path segment.
func pathInt(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`["Go", "算法", "Go"]`, "go,算法"},
		{`Go,  go , 算法,Go`, "go,算法"},
		{``, ""},
		{`[]`, ""},
		{`[""]`, ""},
		{`[ ]`, ""},
		{`Go,算法`, "go,算法"},
		{`["a","B","A","b"]`, "a,b"},
	}
	for _, tt := range tests {
		got := normalizeTags(tt.in)
		if got != tt.want {
			t.Errorf("normalizeTags(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// onPageWritten callback wiring (C1/C3 fix)
// ---------------------------------------------------------------------------

// recordingCallback appends every pageID it receives.
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

func TestCreateWikiPage_InvokesOnPageWritten(t *testing.T) {
	h, db := newTestHandler(t)

	rec := &recordingCallback{}
	h.SetOnPageWritten(rec.fn())

	recResp := doRequest(h.CreateWikiPage, "POST", "/api/wiki", "/api/wiki",
		[]byte(`{"title":"回调测试","slug":"cb-test","page_type":"entity","content":"hello","content_status":"published"}`))
	if recResp.Code != http.StatusCreated {
		t.Fatalf("CreateWikiPage: expected 201, got %d body=%s", recResp.Code, recResp.Body.String())
	}

	calls := rec.snapshot()
	if len(calls) != 1 {
		t.Fatalf("onPageWritten called %d times, want 1", len(calls))
	}
	if calls[0] <= 0 {
		t.Errorf("onPageWritten received non-positive pageID %d", calls[0])
	}

	if status := readSummaryStatus(t, db, calls[0]); status != "pending" {
		t.Errorf("summary_status after create = %q, want %q", status, "pending")
	}
}

func TestUpdateWikiPage_InvokesOnPageWritten(t *testing.T) {
	h, db := newTestHandler(t)
	pageID := seedPage(t, db, "原标题")

	rec := &recordingCallback{}
	h.SetOnPageWritten(rec.fn())

	target := "/api/wiki/" + pathInt(pageID)
	route := "/api/wiki/{id}"
	recResp := doRequest(h.UpdateWikiPage, "PUT", target, route,
		[]byte(`{"title":"新标题","slug":"new-slug","content":"新内容","content_status":"published"}`))
	if recResp.Code != http.StatusOK {
		t.Fatalf("UpdateWikiPage: expected 200, got %d body=%s", recResp.Code, recResp.Body.String())
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

func TestRenameWikiPage_DoesNotInvokeOnPageWritten(t *testing.T) {
	h, db := newTestHandler(t)
	pageID := seedPage(t, db, "原标题")

	rec := &recordingCallback{}
	h.SetOnPageWritten(rec.fn())

	target := "/api/wiki/" + pathInt(pageID) + "/rename"
	route := "/api/wiki/{id}/rename"
	recResp := doRequest(h.RenameWikiPage, "PATCH", target, route,
		[]byte(`{"title":"新标题"}`))
	if recResp.Code != http.StatusOK {
		t.Fatalf("RenameWikiPage: expected 200, got %d body=%s", recResp.Code, recResp.Body.String())
	}

	// Rename is title-only; summary is preserved. Callback should not fire.
	if calls := rec.snapshot(); len(calls) != 0 {
		t.Errorf("onPageWritten called %d times on rename, want 0", len(calls))
	}
}

func TestWikiHandler_OnPageWrittenNilSafe(t *testing.T) {
	h, db := newTestHandler(t)

	// No callback set — should not panic.
	recResp := doRequest(h.CreateWikiPage, "POST", "/api/wiki", "/api/wiki",
		[]byte(`{"title":"no-cb","slug":"no-cb","content":"hi","content_status":"published"}`))
	if recResp.Code != http.StatusCreated {
		t.Fatalf("CreateWikiPage with nil callback: expected 201, got %d body=%s", recResp.Code, recResp.Body.String())
	}

	pageID := seedPage(t, db, "no-cb-2")
	target := "/api/wiki/" + pathInt(pageID)
	route := "/api/wiki/{id}"
	recResp = doRequest(h.UpdateWikiPage, "PUT", target, route,
		[]byte(`{"title":"no-cb-2b","slug":"no-cb-2b","content":"hi","content_status":"published"}`))
	if recResp.Code != http.StatusOK {
		t.Fatalf("UpdateWikiPage with nil callback: expected 200, got %d body=%s", recResp.Code, recResp.Body.String())
	}
}
