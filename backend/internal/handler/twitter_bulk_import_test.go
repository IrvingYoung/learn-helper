package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/go-chi/chi/v5"
)

const followBuildersFixture = `{
  "x_accounts": [
    {"handle": "karpathy", "name": "Andrej Karpathy"},
    {"handle": "swyx", "name": "Swyx"},
    {"handle": "sama", "name": "Sam Altman"}
  ]
}`

func newBulkTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dsn+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, _ = db.Exec(`CREATE TABLE tracked_twitter_accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		handle TEXT NOT NULL UNIQUE,
		display_name TEXT,
		enabled INTEGER NOT NULL DEFAULT 1,
		added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		notes TEXT
	)`)
	return db
}

func newBulkRouter(t *testing.T, h *TwitterAccountHandler) (*sql.DB, *chi.Mux) {
	db := newBulkTestDB(t)
	r := chi.NewRouter()
	r.Post("/api/twitter/accounts/bulk-import", h.BulkImport)
	return db, r
}

func TestBulkImport_FollowBuildersFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(followBuildersFixture))
	}))
	defer srv.Close()

	h := NewTwitterAccountHandler(nil)
	db, r := newBulkRouter(t, h)

	// inject the test DB after construction (since the test wants a clean DB
	// for the handler, but the handler was constructed with nil; we use a
	// helper to set it)
	twitterAccountHandlerDB = db  // see note in step 3
	t.Cleanup(func() { twitterAccountHandlerDB = nil })

	body := `{"url":"` + srv.URL + `"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/twitter/accounts/bulk-import", strings.NewReader(body)))
	if w.Code != 200 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Source          string   `json:"source"`
		TotalFound      int      `json:"total_found"`
		Added           int      `json:"added"`
		SkippedExisting int      `json:"skipped_existing"`
		AddedHandles    []string `json:"added_handles"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.TotalFound != 3 {
		t.Errorf("TotalFound: got %d want 3", resp.TotalFound)
	}
	if resp.Added != 3 {
		t.Errorf("Added: got %d want 3", resp.Added)
	}
	if resp.SkippedExisting != 0 {
		t.Errorf("SkippedExisting: got %d want 0", resp.SkippedExisting)
	}

	// Idempotency: call again
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("POST", "/api/twitter/accounts/bulk-import", strings.NewReader(body)))
	if w2.Code != 200 {
		t.Fatalf("second call status %d", w2.Code)
	}
	var resp2 struct {
		Added           int `json:"added"`
		SkippedExisting int `json:"skipped_existing"`
	}
	_ = json.NewDecoder(w2.Body).Decode(&resp2)
	if resp2.Added != 0 || resp2.SkippedExisting != 3 {
		t.Errorf("idempotent re-import: got added=%d skipped=%d want 0/3", resp2.Added, resp2.SkippedExisting)
	}
}

func TestBulkImport_PlainTextFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`# this is a comment
karpathy
# another comment
swyx
sama
`))
	}))
	defer srv.Close()

	h := NewTwitterAccountHandler(nil)
	db, r := newBulkRouter(t, h)
	twitterAccountHandlerDB = db
	t.Cleanup(func() { twitterAccountHandlerDB = nil })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/twitter/accounts/bulk-import",
		strings.NewReader(`{"url":"`+srv.URL+`"}`)))
	if w.Code != 200 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var resp struct {
		TotalFound int `json:"total_found"`
		Added      int `json:"added"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.TotalFound != 3 || resp.Added != 3 {
		t.Errorf("got TotalFound=%d Added=%d want 3/3", resp.TotalFound, resp.Added)
	}
}

func TestBulkImport_JSONArrayOfStrings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`["a", "b", "c"]`))
	}))
	defer srv.Close()

	h := NewTwitterAccountHandler(nil)
	db, r := newBulkRouter(t, h)
	twitterAccountHandlerDB = db
	t.Cleanup(func() { twitterAccountHandlerDB = nil })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/twitter/accounts/bulk-import",
		strings.NewReader(`{"url":"`+srv.URL+`"}`)))
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	var resp struct {
		Added int `json:"added"`
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Added != 3 {
		t.Errorf("Added: got %d want 3", resp.Added)
	}
}

func TestBulkImport_BuiltInURLWhenEmpty(t *testing.T) {
	// When the request body has empty/no URL, the handler should use the
	// built-in default (the follow-builders list). We can't easily test the
	// real network call here, so we just verify the request reaches the
	// fetch step without panicking on a clearly invalid built-in URL — the
	// real network is exercised in the smoke test.
	//
	// For this unit test, we just verify that an empty body is accepted and
	// that the handler attempts to fetch (which will fail in test env, but
	// should return a 500 with a useful error, not panic).
	h := NewTwitterAccountHandler(nil)
	db, r := newBulkRouter(t, h)
	twitterAccountHandlerDB = db
	t.Cleanup(func() { twitterAccountHandlerDB = nil })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/twitter/accounts/bulk-import",
		strings.NewReader(`{}`)))
	// The real built-in URL may or may not be reachable from the test
	// environment. We accept either 200 (worked) or 500 (network failed)
	// as long as it's not 400/panic.
	if w.Code != 200 && w.Code != 500 {
		t.Errorf("got %d, want 200 or 500. body: %s", w.Code, w.Body.String())
	}
}

func TestBulkImport_RejectsNonHTTP(t *testing.T) {
	h := NewTwitterAccountHandler(nil)
	db, r := newBulkRouter(t, h)
	twitterAccountHandlerDB = db
	t.Cleanup(func() { twitterAccountHandlerDB = nil })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/twitter/accounts/bulk-import",
		strings.NewReader(`{"url":"file:///etc/passwd"}`)))
	if w.Code != 400 {
		t.Errorf("got %d, want 400 for non-http(s) URL. body: %s", w.Code, w.Body.String())
	}
}
