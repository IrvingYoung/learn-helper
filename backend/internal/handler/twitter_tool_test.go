package handler

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"learn-helper/internal/ai"
	_ "modernc.org/sqlite"
)

func newToolTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dsn+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	for _, stmt := range []string{
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
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestExecuteListRecentTweets_FilterAndLimit(t *testing.T) {
	db := newToolTestDB(t)
	h := &AIHandler{db: db}
	ctx := context.Background()

	// Seed: 3 tweets, mix of handles and times
	rows := []struct {
		id, handle, text, created string
	}{
		{"1", "karpathy", "old", "2026-06-01T10:00:00Z"},
		{"2", "karpathy", "new", "2026-06-03T10:00:00Z"},
		{"3", "sama", "x", "2026-06-03T11:00:00Z"},
	}
	for _, r := range rows {
		_, err := db.ExecContext(ctx,
			`INSERT INTO tweets (tweet_id, handle, text, created_at, url, raw_json) VALUES (?,?,?,?,?,?)`,
			r.id, r.handle, r.text, r.created, "https://x.com/"+r.id, `{}`,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"no filter returns 3", `{"limit":10}`, 3},
		{"filter by handle=karpathy", `{"handle":"karpathy","limit":10}`, 2},
		{"filter by since", `{"since":"2026-06-03T00:00:00Z","limit":10}`, 2},
		{"limit 1", `{"limit":1}`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := h.executeListRecentTweets(ctx, ai.ToolCall{Name: "list_recent_tweets", Input: tc.input})
			// Output contains "[系统] ..." wrapper; count occurrences of `"tweet_id":`
			got := jsonContainsCount(out, "tweet_id")
			if got != tc.want {
				t.Errorf("got %d tweets in output, want %d\noutput: %s", got, tc.want, out)
			}
		})
	}
}

// jsonContainsCount counts how many times a JSON-stringified key appears
// in the output. We use it as a proxy for tweet count.
func jsonContainsCount(s, key string) int {
	n := 0
	needle := `"` + key + `":`
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			n++
		}
	}
	return n
}
