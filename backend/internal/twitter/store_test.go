package twitter

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dsn+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	for _, stmt := range []string{
		`CREATE TABLE tracked_twitter_accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			handle TEXT NOT NULL UNIQUE,
			display_name TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			notes TEXT
		)`,
		`CREATE TABLE tweets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tweet_id TEXT NOT NULL UNIQUE,
			handle TEXT NOT NULL,
			author_name TEXT,
			text TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			url TEXT NOT NULL,
			metrics_json TEXT,
			raw_json TEXT NOT NULL,
			fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			digest_run_id TEXT
		)`,
		`CREATE TABLE twitter_digest_runs (
			id TEXT PRIMARY KEY,
			cron_run_id INTEGER,
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			finished_at DATETIME,
			status TEXT NOT NULL,
			tweets_fetched INTEGER NOT NULL DEFAULT 0,
			wiki_page_id INTEGER,
			error TEXT
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestStore_InsertTweet_Idempotent(t *testing.T) {
	db := newTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	tw := Tweet{
		TweetID:   "tx-1",
		Handle:    "karpathy",
		Text:      "hello",
		CreatedAt: time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC),
		URL:       "https://x.com/karpathy/status/1",
		Raw:       json.RawMessage(`{"id":"tx-1"}`),
	}
	if err := s.InsertTweet(ctx, tw, "run-1"); err != nil {
		t.Fatal(err)
	}
	// Second insert with same tweet_id must be a no-op.
	if err := s.InsertTweet(ctx, tw, "run-1"); err != nil {
		t.Fatalf("second insert should be idempotent: %v", err)
	}
	n, _ := s.CountTweetsByRun(ctx, "run-1")
	if n != 1 {
		t.Errorf("expected 1 row, got %d", n)
	}
}

func TestStore_ListEnabledAccounts(t *testing.T) {
	db := newTestDB(t)
	s := NewStore(db)
	ctx := context.Background()

	must(t, db, `INSERT INTO tracked_twitter_accounts (handle, enabled) VALUES ('a', 1)`)
	must(t, db, `INSERT INTO tracked_twitter_accounts (handle, enabled) VALUES ('b', 0)`)
	must(t, db, `INSERT INTO tracked_twitter_accounts (handle, enabled) VALUES ('c', 1)`)

	accounts, err := s.ListEnabledAccounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 enabled, got %d", len(accounts))
	}
	got := []string{accounts[0].Handle, accounts[1].Handle}
	if got[0] != "a" || got[1] != "c" {
		t.Errorf("unexpected handles: %v", got)
	}
}

func TestStore_UpdateAccountDisplayName(t *testing.T) {
	db := newTestDB(t)
	s := NewStore(db)
	ctx := context.Background()
	must(t, db, `INSERT INTO tracked_twitter_accounts (handle) VALUES ('a')`)

	if err := s.UpdateAccountDisplayName(ctx, "a", "Alice"); err != nil {
		t.Fatal(err)
	}
	var name sql.NullString
	mustQuery(t, db, `SELECT display_name FROM tracked_twitter_accounts WHERE handle='a'`, &name)
	if !name.Valid || name.String != "Alice" {
		t.Errorf("expected Alice, got %v", name)
	}
}

func must(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.Exec(stmt); err != nil {
		t.Fatal(err)
	}
}

func mustQuery(t *testing.T, db *sql.DB, q string, dst any) {
	t.Helper()
	if err := db.QueryRow(q).Scan(dst); err != nil {
		t.Fatal(err)
	}
}
