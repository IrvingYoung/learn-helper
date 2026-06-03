package cron

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"learn-helper/internal/twitter"
)

type stubClient struct {
	calls []stubCall
	out   []twitter.Tweet
	err   error
}

type stubCall struct {
	Handle string
	Since  time.Time
	Limit  int
}

func (s *stubClient) FetchUserTweets(ctx context.Context, handle string, since time.Time, limit int) ([]twitter.Tweet, error) {
	s.calls = append(s.calls, stubCall{handle, since, limit})
	out := s.out
	s.out = nil
	return out, s.err
}

func newDigestTestDB(t *testing.T) *sql.DB {
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

func TestFetchAndPersist_FetchesAllEnabledAccounts(t *testing.T) {
	db := newDigestTestDB(t)
	store := twitter.NewStore(db)
	_, _ = db.Exec(`INSERT INTO tracked_twitter_accounts (handle) VALUES ('a')`)
	_, _ = db.Exec(`INSERT INTO tracked_twitter_accounts (handle, enabled) VALUES ('b', 0)`)
	_, _ = db.Exec(`INSERT INTO tracked_twitter_accounts (handle) VALUES ('c')`)

	client := &stubClient{out: []twitter.Tweet{
		{TweetID: "t1", Handle: "a", Text: "x", CreatedAt: time.Now(), URL: "u", Raw: []byte("{}")},
	}}
	runner := &DigestRunner{Store: store, Client: client}
	cfg := DigestConfig{SinceHours: 24, MaxTweetsPerAccount: 50, MaxTotalTweets: 200}
	n, err := runner.fetchAndPersist(context.Background(), "run-1", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 tweet persisted, got %d", n)
	}
	if len(client.calls) != 2 {
		t.Errorf("expected 2 client calls (skipping disabled), got %d", len(client.calls))
	}
}

func TestFetchAndPersist_OneAccountFailureDoesNotStopOthers(t *testing.T) {
	db := newDigestTestDB(t)
	store := twitter.NewStore(db)
	_, _ = db.Exec(`INSERT INTO tracked_twitter_accounts (handle) VALUES ('a')`)
	_, _ = db.Exec(`INSERT INTO tracked_twitter_accounts (handle) VALUES ('b')`)

	good := &stubClient{out: []twitter.Tweet{
		{TweetID: "b1", Handle: "b", Text: "ok", CreatedAt: time.Now(), URL: "u", Raw: []byte("{}")},
	}}
	bad := &stubClient{err: context.DeadlineExceeded}
	multi := &multiClient{impls: []twitter.Client{good, bad}}
	runner := &DigestRunner{Store: store, Client: multi}
	cfg := DigestConfig{SinceHours: 24, MaxTweetsPerAccount: 50, MaxTotalTweets: 200}
	n, err := runner.fetchAndPersist(context.Background(), "run-1", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 tweet from the good account, got %d", n)
	}
}

type multiClient struct{ impls []twitter.Client }

func (m *multiClient) FetchUserTweets(ctx context.Context, h string, s time.Time, l int) ([]twitter.Tweet, error) {
	// dispatch by hash of handle so 'a' is bad and 'b' is good
	if h == "a" {
		return m.impls[1].FetchUserTweets(ctx, h, s, l)
	}
	return m.impls[0].FetchUserTweets(ctx, h, s, l)
}

func TestDigestRunner_Run_NoTweetsMarksFailed(t *testing.T) {
	db := newDigestTestDB(t)
	store := twitter.NewStore(db)
	_, _ = db.Exec(`INSERT INTO tracked_twitter_accounts (handle) VALUES ('a')`)

	runner := &DigestRunner{
		Store:  store,
		Client: &stubClient{out: nil}, // no tweets
		AI:     nil,
	}
	_, fetched, err := runner.Run(context.Background(), nil, DigestConfig{
		SinceHours: 24, MaxTweetsPerAccount: 50, MaxTotalTweets: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fetched != 0 {
		t.Errorf("expected 0 fetched, got %d", fetched)
	}
	var status, errStr string
	if err := db.QueryRow(`SELECT status, COALESCE(error,'') FROM twitter_digest_runs`).Scan(&status, &errStr); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Errorf("status: got %q want failed", status)
	}
	if errStr != "no_new_tweets" {
		t.Errorf("error: got %q want no_new_tweets", errStr)
	}
}
