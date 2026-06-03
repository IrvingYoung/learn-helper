package handler

import (
	"bytes"
	"context"
	"encoding/json"
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

// silence unused import warnings
var _ = context.Background
