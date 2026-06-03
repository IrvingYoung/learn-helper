package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"database/sql"
	_ "modernc.org/sqlite"

	"github.com/go-chi/chi/v5"
)

func newAccountTestDB(t *testing.T) (*sql.DB, *chi.Mux) {
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
	_, _ = db.Exec(`CREATE TABLE ai_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider TEXT NOT NULL,
		model_name TEXT NOT NULL,
		api_key TEXT NOT NULL,
		is_active INTEGER NOT NULL DEFAULT 1,
		config TEXT
	)`)

	h := NewTwitterAccountHandler(db)
	r := chi.NewRouter()
	r.Get("/api/twitter/accounts", h.List)
	r.Post("/api/twitter/accounts", h.Create)
	r.Put("/api/twitter/accounts/{id}", h.Update)
	r.Delete("/api/twitter/accounts/{id}", h.Delete)
	r.Get("/api/twitter/config", h.GetConfig)
	r.Put("/api/twitter/config", h.PutConfig)
	return db, r
}

func TestTwitterAccountHandler_List(t *testing.T) {
	db, r := newAccountTestDB(t)
	_, _ = db.Exec(`INSERT INTO tracked_twitter_accounts (handle) VALUES ('karpathy')`)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/twitter/accounts", nil))
	if w.Code != 200 {
		t.Fatalf("status: got %d want 200, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"karpathy"`) {
		t.Errorf("body should contain handle: %s", w.Body.String())
	}
}

func TestTwitterAccountHandler_Create(t *testing.T) {
	_, r := newAccountTestDB(t)
	body := `{"handle":"sama","notes":"ceo"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/twitter/accounts", strings.NewReader(body)))
	if w.Code != 201 {
		t.Fatalf("status: got %d want 201, body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["handle"] != "sama" {
		t.Errorf("handle: got %v", got["handle"])
	}
}

func TestTwitterAccountHandler_Create_RejectsBadHandle(t *testing.T) {
	_, r := newAccountTestDB(t)
	for _, handle := range []string{"@sama", "sama spaces", ""} {
		body, _ := json.Marshal(map[string]string{"handle": handle})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/twitter/accounts", bytes.NewReader(body)))
		if w.Code != 400 {
			t.Errorf("handle=%q: expected 400, got %d", handle, w.Code)
		}
	}
}

func TestTwitterAccountHandler_Update(t *testing.T) {
	db, r := newAccountTestDB(t)
	res, err := db.Exec(`INSERT INTO tracked_twitter_accounts (handle, notes) VALUES ('karpathy', 'old')`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()

	// Disable and update notes in one call
	body := `{"enabled":false,"notes":"paused"}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT",
		fmt.Sprintf("/api/twitter/accounts/%d", id), strings.NewReader(body)))
	if w.Code != 200 {
		t.Fatalf("status: got %d want 200, body=%s", w.Code, w.Body.String())
	}

	var enabled int
	var notes string
	if err := db.QueryRow(`SELECT enabled, notes FROM tracked_twitter_accounts WHERE id=?`, id).
		Scan(&enabled, &notes); err != nil {
		t.Fatal(err)
	}
	if enabled != 0 {
		t.Errorf("expected enabled=0, got %d", enabled)
	}
	if notes != "paused" {
		t.Errorf("expected notes=%q, got %q", "paused", notes)
	}

	// Re-enable to confirm second patch works
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT",
		fmt.Sprintf("/api/twitter/accounts/%d", id), strings.NewReader(`{"enabled":true}`)))
	if w.Code != 200 {
		t.Fatalf("re-enable status: got %d want 200", w.Code)
	}
	if err := db.QueryRow(`SELECT enabled FROM tracked_twitter_accounts WHERE id=?`, id).
		Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Errorf("expected enabled=1 after re-enable, got %d", enabled)
	}
}

func TestTwitterAccountHandler_Update_RejectsBadHandle(t *testing.T) {
	db, r := newAccountTestDB(t)
	res, _ := db.Exec(`INSERT INTO tracked_twitter_accounts (handle) VALUES ('karpathy')`)
	id, _ := res.LastInsertId()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT",
		fmt.Sprintf("/api/twitter/accounts/%d", id), strings.NewReader(`{"handle":"bad space"}`)))
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTwitterAccountHandler_Delete(t *testing.T) {
	db, r := newAccountTestDB(t)
	res, err := db.Exec(`INSERT INTO tracked_twitter_accounts (handle) VALUES ('sama')`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", fmt.Sprintf("/api/twitter/accounts/%d", id), nil))
	if w.Code != 200 {
		t.Fatalf("status: got %d want 200, body=%s", w.Code, w.Body.String())
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tracked_twitter_accounts WHERE id=?`, id).
		Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected row deleted, count=%d", n)
	}
}

func TestTwitterAccountHandler_Delete_NotFoundIsOK(t *testing.T) {
	// DELETE is idempotent in our handler — a missing id should still
	// return 200 because the post-condition (row gone) holds.
	_, r := newAccountTestDB(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/twitter/accounts/999", nil))
	if w.Code != 200 {
		t.Errorf("expected 200 even for missing id, got %d", w.Code)
	}
}
