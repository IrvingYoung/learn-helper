# X (Twitter) 每日 AI 热点摘要 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a periodic X (Twitter) digest feature that fetches tracked accounts' tweets via RSSHub, stores them in a new `tweets` table, then asks the AI to produce a daily digest page (`AI 日报/YYYY-MM-DD`) via the existing `create_page` / `update_page` tools.

**Architecture:** Three new tables (`tracked_twitter_accounts`, `tweets`, `twitter_digest_runs`) + a `task_type` column on `cron_tasks`. A new `internal/twitter` package wraps RSSHub behind a `Client` interface. The existing `internal/cron.Runner` branches on `task_type`: for `twitter_digest` it (1) fetches tweets into `tweets` table, (2) calls `AIHandler.RunReAct` with a fixed digest-mode prompt + a new `list_recent_tweets` tool, (3) lets the AI use `create_page`/`update_page` directly (cron auto-approves writes, no manual plan-confirm step). Frontend: a Twitter-accounts panel in Settings + a `twitter_digest` option in the cron-task form + a "run now" button.

**Tech Stack:** Go (chi, modernc.org/sqlite, encoding/xml for RSS), React 19 + Tailwind, existing cron + AI ReAct infrastructure.

**Key references:**
- `backend/db/migrations/013_add_cron_tasks.sql` — pattern for the new migration
- `backend/internal/handler/ai.go:829` — `executeWebSearch` pattern for the new tool
- `backend/internal/handler/ai_classify.go:18` — read-set registration
- `backend/internal/cron/runner.go` — branching point for `task_type`

---

## File Structure

### New files

| File | Responsibility |
|---|---|
| `backend/db/migrations/015_twitter_digest.sql` | New tables + cron_tasks columns |
| `backend/internal/twitter/types.go` | `Tweet` struct, `Client` interface |
| `backend/internal/twitter/rsshub_client.go` | RSSHub implementation of `Client` |
| `backend/internal/twitter/rsshub_client_test.go` | httptest-based tests |
| `backend/internal/twitter/store.go` | DB helpers (insert tweet, get accounts, etc.) |
| `backend/internal/twitter/store_test.go` | DB helper tests |
| `backend/internal/handler/twitter_account.go` | HTTP handlers for `/api/twitter/*` |
| `backend/internal/handler/twitter_account_test.go` | HTTP handler tests |
| `backend/internal/handler/twitter_tool.go` | `executeListRecentTweets` |
| `backend/internal/handler/twitter_tool_test.go` | Tool execution tests |
| `backend/internal/cron/digest_runner.go` | `runTwitterDigest` implementation |
| `backend/internal/cron/digest_runner_test.go` | Digest runner tests |

### Modified files

| File | Change |
|---|---|
| `backend/internal/ai/provider.go` | Add `list_recent_tweets` tool to `WikiTools()` |
| `backend/internal/handler/ai_classify.go` | Add `list_recent_tweets` to read set |
| `backend/internal/handler/ai.go` | Add `case "list_recent_tweets"` in `executeAutoTool` |
| `backend/internal/cron/runner.go` | Branch on `task.TaskType`; call `runTwitterDigest` for `twitter_digest` |
| `backend/internal/cron/models.go` | Add `TaskType` field to `Task` struct |
| `backend/cmd/server/main.go` | Register `RSSHubClient`, mount `/api/twitter/*` routes |
| `frontend/src/lib/api.ts` | Add `twitterAccounts.*`, `twitterConfig.*`, `runCronTaskNow` |
| `frontend/src/app/settings/page.tsx` | Add Twitter accounts panel + RSSHub URL field |
| `frontend/src/components/CronTaskForm.tsx` | Add `task_type` field + twitter_digest config fields |
| `frontend/src/app/cron/page.tsx` | Add "立即运行" button on task detail |

---

## Task 1: Migration 015 — new tables + cron_tasks columns

**Files:**
- Create: `backend/db/migrations/015_twitter_digest.sql`

- [ ] **Step 1: Create the migration file**

Create `backend/db/migrations/015_twitter_digest.sql` with this exact content:

```sql
-- Migration 015: Twitter digest
-- Adds a new cron task type (twitter_digest) and supporting tables for
-- tracking X/Twitter accounts, storing fetched tweets, and recording
-- each digest run.

-- Extend cron_tasks with task_type and twitter-specific config columns.
-- Existing rows default to 'generic' (preserves backwards compat).
ALTER TABLE cron_tasks ADD COLUMN task_type TEXT NOT NULL DEFAULT 'generic';
ALTER TABLE cron_tasks ADD COLUMN since_hours INTEGER NOT NULL DEFAULT 24;
ALTER TABLE cron_tasks ADD COLUMN max_tweets_per_account INTEGER NOT NULL DEFAULT 50;
ALTER TABLE cron_tasks ADD COLUMN max_total_tweets INTEGER NOT NULL DEFAULT 200;

-- Accounts the user wants to track.
CREATE TABLE IF NOT EXISTS tracked_twitter_accounts (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  handle        TEXT NOT NULL UNIQUE,
  display_name  TEXT,
  enabled       INTEGER NOT NULL DEFAULT 1,
  added_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  notes         TEXT
);

CREATE INDEX IF NOT EXISTS idx_tracked_twitter_enabled
  ON tracked_twitter_accounts(enabled);

-- Raw tweets fetched from RSSHub.
CREATE TABLE IF NOT EXISTS tweets (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  tweet_id      TEXT NOT NULL UNIQUE,
  handle        TEXT NOT NULL,
  author_name   TEXT,
  text          TEXT NOT NULL,
  created_at    DATETIME NOT NULL,
  url           TEXT NOT NULL,
  metrics_json  TEXT,
  raw_json      TEXT NOT NULL,
  fetched_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  digest_run_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_tweets_handle_created
  ON tweets(handle, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tweets_digest_run
  ON tweets(digest_run_id);

-- One row per digest cron run. Linked to the wiki page via plan_id
-- (NULL for now since we don't use propose_plan any more — kept for
-- future debugging) and indirectly via the wiki_log.source.
CREATE TABLE IF NOT EXISTS twitter_digest_runs (
  id              TEXT PRIMARY KEY,
  cron_run_id     INTEGER REFERENCES cron_runs(id) ON DELETE SET NULL,
  started_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at     DATETIME,
  status          TEXT NOT NULL,
  tweets_fetched  INTEGER NOT NULL DEFAULT 0,
  wiki_page_id    INTEGER,
  error           TEXT
);

CREATE INDEX IF NOT EXISTS idx_twitter_digest_runs_started
  ON twitter_digest_runs(started_at DESC);
```

- [ ] **Step 2: Apply the migration on a fresh DB and verify**

NOTE: `cmd/migrate` is a legacy data tool (topics→wiki_pages) and does NOT apply `db/migrations/*.sql`. Verification is done by piping the schema + migration files into `sqlite3` directly. The actual runtime schema is applied by inlined `db.Exec` calls in `cmd/server/main.go` — that wiring is added in Task 12.

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
rm -f /tmp/tw-test.db /tmp/tw-test.db-shm /tmp/tw-test.db-wal
sqlite3 /tmp/tw-test.db < db/migrations/schema.sql
sqlite3 /tmp/tw-test.db < db/migrations/013_add_cron_tasks.sql
sqlite3 /tmp/tw-test.db < db/migrations/015_twitter_digest.sql
```

Expected: each command exits 0 with no error. Then verify tables exist:

```bash
sqlite3 /tmp/tw-test.db ".schema tracked_twitter_accounts"
sqlite3 /tmp/tw-test.db ".schema tweets"
sqlite3 /tmp/tw-test.db ".schema twitter_digest_runs"
sqlite3 /tmp/tw-test.db "PRAGMA table_info(cron_tasks)" | grep -E "task_type|since_hours|max_tweets"
```

Expected: each command shows the new table / column. `cron_tasks` should show the 4 new columns.

- [ ] **Step 3: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/db/migrations/015_twitter_digest.sql
git commit -m "feat(db): add twitter digest tables + cron_tasks task_type column"
```

---

## Task 2: `internal/twitter` types + Client interface

**Files:**
- Create: `backend/internal/twitter/types.go`

- [ ] **Step 1: Create types.go**

Create `backend/internal/twitter/types.go`:

```go
package twitter

import (
	"context"
	"encoding/json"
	"time"
)

// Tweet is the normalized representation of one tweet, regardless of
// the underlying source (RSSHub, future X API client, etc.).
type Tweet struct {
	TweetID    string
	Handle     string
	AuthorName string
	Text       string
	CreatedAt  time.Time
	URL        string
	Metrics    map[string]int
	Raw        json.RawMessage
}

// Client is the abstract interface for any X data source. Implementations
// fetch tweets for a single handle, filtered by `since`, capped at `limit`.
type Client interface {
	FetchUserTweets(ctx context.Context, handle string, since time.Time, limit int) ([]Tweet, error)
}
```

- [ ] **Step 2: Verify it compiles**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go build ./internal/twitter/...
```

Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/twitter/types.go
git commit -m "feat(twitter): add Tweet type and Client interface"
```

---

## Task 3: RSSHubClient — fetch + parse RSS

**Files:**
- Create: `backend/internal/twitter/rsshub_client.go`
- Create: `backend/internal/twitter/rsshub_client_test.go`

- [ ] **Step 1: Write the failing test**

Create `backend/internal/twitter/rsshub_client_test.go`:

```go
package twitter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const rssFixture = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>@karpathy</title>
    <item>
      <title>karpathy: new post on LLM trends</title>
      <link>https://x.com/karpathy/status/1234</link>
      <guid isPermaLink="false">1234</guid>
      <pubDate>Mon, 03 Jun 2026 10:00:00 GMT</pubDate>
      <description>some long body text here</description>
    </item>
    <item>
      <title>karpathy: another take</title>
      <link>https://x.com/karpathy/status/1235</link>
      <guid isPermaLink="false">1235</guid>
      <pubDate>Sun, 02 Jun 2026 09:00:00 GMT</pubDate>
      <description>old post</description>
    </item>
  </channel>
</rss>`

func TestRSSHubClient_FetchUserTweets_FiltersBySince(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/twitter/user/karpathy" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rssFixture))
	}))
	defer srv.Close()

	c := NewRSSHubClient(srv.URL, 5*time.Second)
	since := parseDate(t, "2026-06-03T00:00:00Z")
	tweets, err := c.FetchUserTweets(context.Background(), "karpathy", since, 50)
	if err != nil {
		t.Fatalf("FetchUserTweets: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet (filtered by since), got %d", len(tweets))
	}
	got := tweets[0]
	if got.TweetID != "1234" {
		t.Errorf("TweetID: got %q want 1234", got.TweetID)
	}
	if got.Handle != "karpathy" {
		t.Errorf("Handle: got %q want karpathy", got.Handle)
	}
	if got.URL != "https://x.com/karpathy/status/1234" {
		t.Errorf("URL: got %q", got.URL)
	}
}

func TestRSSHubClient_FetchUserTweets_RespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(rssFixture))
	}))
	defer srv.Close()

	c := NewRSSHubClient(srv.URL, 5*time.Second)
	tweets, err := c.FetchUserTweets(context.Background(), "karpathy", time.Time{}, 1)
	if err != nil {
		t.Fatalf("FetchUserTweets: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet (limit=1), got %d", len(tweets))
	}
}

func TestRSSHubClient_FetchUserTweets_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewRSSHubClient(srv.URL, 5*time.Second)
	_, err := c.FetchUserTweets(context.Background(), "karpathy", time.Time{}, 10)
	if err == nil {
		t.Fatal("expected error on HTTP 502, got nil")
	}
}

func parseDate(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tt
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/twitter/... -run TestRSSHubClient -v
```

Expected: build fails (`undefined: NewRSSHubClient`).

- [ ] **Step 3: Implement RSSHubClient**

Create `backend/internal/twitter/rsshub_client.go`:

```go
package twitter

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RSSHubClient fetches tweets by calling RSSHub's twitter/user/:handle
// route and parsing the returned RSS 2.0 feed.
type RSSHubClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewRSSHubClient returns a client with the given base URL and timeout.
// If timeout is 0, a 15s default is used.
func NewRSSHubClient(baseURL string, timeout time.Duration) *RSSHubClient {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &RSSHubClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: timeout},
	}
}

type rssFeed struct {
	XMLName xml.Name    `xml:"rss"`
	Channel rssChannel  `xml:"channel"`
	Items   []rssItem   `xml:"channel>item"`
}

type rssChannel struct {
	Title string `xml:"title"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

// FetchUserTweets calls GET {BaseURL}/twitter/user/{handle} and returns
// tweets newer than `since`, capped at `limit`. If `since` is zero, no
// time filter is applied.
func (c *RSSHubClient) FetchUserTweets(ctx context.Context, handle string, since time.Time, limit int) ([]Tweet, error) {
	url := fmt.Sprintf("%s/twitter/user/%s", c.BaseURL, handle)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "learn-helper/1.0")
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("rss hub returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB cap
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}

	out := make([]Tweet, 0, len(feed.Items))
	for _, it := range feed.Items {
		created, err := parseRSSDate(it.PubDate)
		if err != nil {
			continue // skip malformed dates
		}
		if !since.IsZero() && created.Before(since) {
			continue
		}
		if limit > 0 && len(out) >= limit {
			break
		}
		raw, _ := json.Marshal(it)
		out = append(out, Tweet{
			TweetID:   it.GUID,
			Handle:    handle,
			Text:      it.Description,
			CreatedAt: created,
			URL:       it.Link,
			Raw:       raw,
		})
	}
	return out, nil
}

func parseRSSDate(s string) (time.Time, error) {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized RSS date: %q", s)
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/twitter/... -run TestRSSHubClient -v
```

Expected: 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/twitter/rsshub_client.go backend/internal/twitter/rsshub_client_test.go
git commit -m "feat(twitter): add RSSHubClient with RSS 2.0 parsing"
```

---

## Task 4: Twitter store — DB helpers

**Files:**
- Create: `backend/internal/twitter/store.go`
- Create: `backend/internal/twitter/store_test.go`

- [ ] **Step 1: Write the failing test**

Create `backend/internal/twitter/store_test.go`:

```go
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
```

- [ ] **Step 2: Run the test, verify it fails**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/twitter/... -run TestStore -v
```

Expected: build fails (`undefined: NewStore`).

- [ ] **Step 3: Implement the store**

Create `backend/internal/twitter/store.go`:

```go
package twitter

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Account is one row of tracked_twitter_accounts.
type Account struct {
	ID          int64
	Handle      string
	DisplayName sql.NullString
	Enabled     bool
	Notes       sql.NullString
}

// Store wraps the DB operations needed by the twitter package.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// InsertTweet inserts a tweet. Returns nil on duplicate (idempotent via
// tweet_id UNIQUE). digest_runID may be empty.
func (s *Store) InsertTweet(ctx context.Context, tw Tweet, digestRunID string) error {
	metricsJSON, err := json.Marshal(tw.Metrics)
	if err != nil {
		metricsJSON = []byte("{}")
	}
	var runID any
	if digestRunID != "" {
		runID = digestRunID
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO tweets
		  (tweet_id, handle, author_name, text, created_at, url, metrics_json, raw_json, digest_run_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		tw.TweetID, tw.Handle, tw.AuthorName, tw.Text,
		tw.CreatedAt.UTC().Format(time.RFC3339),
		tw.URL, string(metricsJSON), string(tw.Raw), runID,
	)
	return err
}

// CountTweetsByRun returns the number of tweets associated with a run.
func (s *Store) CountTweetsByRun(ctx context.Context, runID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tweets WHERE digest_run_id = ?`, runID,
	).Scan(&n)
	return n, err
}

// ListEnabledAccounts returns all enabled accounts.
func (s *Store) ListEnabledAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, handle, display_name, enabled, notes
		FROM tracked_twitter_accounts
		WHERE enabled = 1
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Handle, &a.DisplayName, &a.Enabled, &a.Notes); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAccountDisplayName sets display_name for the given handle.
// Used to backfill the author name from the first successful fetch.
func (s *Store) UpdateAccountDisplayName(ctx context.Context, handle, name string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tracked_twitter_accounts
		 SET display_name = ?
		 WHERE handle = ? AND (display_name IS NULL OR display_name = '')`,
		name, handle,
	)
	return err
}
```

- [ ] **Step 4: Run the tests, verify they pass**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/twitter/... -run TestStore -v
```

Expected: 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/twitter/store.go backend/internal/twitter/store_test.go
git commit -m "feat(twitter): add Store with idempotent InsertTweet + account queries"
```

---

## Task 5: AI tool — `list_recent_tweets` (provider definition)

**Files:**
- Modify: `backend/internal/ai/provider.go` (add to `WikiTools()`)

- [ ] **Step 1: Locate the right place**

Open `backend/internal/ai/provider.go`. Find the `WikiTools()` function and locate the `// ── Read tools ──` section (or where `websearch`/`webfetch` are registered). If there is no explicit read section, add the new tool just before the existing `websearch` block.

For reference, `websearch` currently looks like:

```go
{
    Name:        "websearch",
    Description: "...",
    InputSchema: map[string]any{...},
},
```

- [ ] **Step 2: Add the `list_recent_tweets` tool definition**

Insert this struct literal right after the `webfetch` tool (so it appears at the end of the read-tool group):

```go
{
    Name:        "list_recent_tweets",
    Description: "读取已经被抓取并落库的 X 推文。since: 可选 ISO8601 时间,只返回该时间之后的;handle: 可选账号过滤;limit: 默认 50,上限 200。",
    InputSchema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "since":  map[string]any{"type": "string", "description": "ISO8601 时间,只返回该时间之后抓到的推文"},
            "handle": map[string]any{"type": "string", "description": "账号 handle (不带 @),只返回该账号的推文"},
            "limit":  map[string]any{"type": "integer", "description": "返回条数上限,默认 50,上限 200"},
        },
    },
},
```

- [ ] **Step 3: Verify build**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go build ./...
```

Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/ai/provider.go
git commit -m "feat(ai): add list_recent_tweets tool definition"
```

---

## Task 6: AI tool — `list_recent_tweets` (executor + classification)

**Files:**
- Modify: `backend/internal/handler/ai_classify.go`
- Modify: `backend/internal/handler/ai.go`
- Create: `backend/internal/handler/twitter_tool.go`
- Create: `backend/internal/handler/twitter_tool_test.go`

- [ ] **Step 1: Register in classify's read set**

In `backend/internal/handler/ai_classify.go`, find the `readSet` literal. Add `"list_recent_tweets": true,` so the line becomes:

```go
readSet := map[string]bool{
    "lookup_page": true, "read_page": true, "search_pages": true,
    "list_backlinks": true, "list_links": true, "list_children": true,
    "find_broken_links": true,
    "websearch":         true, "webfetch": true,
    "list_recent_tweets": true,
}
```

- [ ] **Step 2: Add executor dispatcher in ai.go**

In `backend/internal/handler/ai.go`, find the `executeAutoTool` function (around line 690). Add the new case before the `default` branch:

```go
case "list_recent_tweets":
    return h.executeListRecentTweets(ctx, tc)
```

- [ ] **Step 3: Write the executor with its test**

Create `backend/internal/handler/twitter_tool_test.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"database/sql"
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
			out := h.executeListRecentTweets(ctx, aiToolCall{Name: "list_recent_tweets", Input: tc.input})
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

// Ensure time is used (some Go versions complain about unused imports
// when only used in test setup).
var _ = time.Now
```

- [ ] **Step 4: Run the test, verify it fails**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/handler/... -run TestExecuteListRecentTweets -v
```

Expected: build fails (`h.executeListRecentTweets undefined`).

- [ ] **Step 5: Implement executeListRecentTweets**

Create `backend/internal/handler/twitter_tool.go`:

```go
package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"learn-helper/internal/ai"
)

func (h *AIHandler) executeListRecentTweets(ctx context.Context, tc ai.ToolCall) string {
	var args struct {
		Since  string `json:"since"`
		Handle string `json:"handle"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &args); err != nil {
		return "[系统] list_recent_tweets 执行失败：参数解析错误"
	}
	if args.Limit <= 0 {
		args.Limit = 50
	}
	if args.Limit > 200 {
		args.Limit = 200
	}

	q := `SELECT tweet_id, handle, author_name, text, created_at, url, metrics_json
	      FROM tweets WHERE 1=1`
	var params []any
	if args.Since != "" {
		q += " AND created_at >= ?"
		params = append(params, args.Since)
	}
	if args.Handle != "" {
		q += " AND handle = ?"
		params = append(params, args.Handle)
	}
	q += " ORDER BY created_at DESC LIMIT ?"
	params = append(params, args.Limit)

	rows, err := h.db.QueryContext(ctx, q, params...)
	if err != nil {
		return fmt.Sprintf("[系统] list_recent_tweets 查询失败：%v", err)
	}
	defer rows.Close()

	type item struct {
		TweetID    string         `json:"tweet_id"`
		Handle     string         `json:"handle"`
		AuthorName sql.NullString `json:"-"`
		Author     string         `json:"author,omitempty"`
		Text       string         `json:"text"`
		CreatedAt  string         `json:"created_at"`
		URL        string         `json:"url"`
		Metrics    string         `json:"metrics,omitempty"`
	}
	var out []item
	for rows.Next() {
		var it item
		var mj sql.NullString
		if err := rows.Scan(&it.TweetID, &it.Handle, &it.AuthorName, &it.Text, &it.CreatedAt, &it.URL, &mj); err != nil {
			return fmt.Sprintf("[系统] list_recent_tweets 扫描失败：%v", err)
		}
		if it.AuthorName.Valid {
			it.Author = it.AuthorName.String
		}
		if mj.Valid && mj.String != "" {
			it.Metrics = mj.String
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return fmt.Sprintf("[系统] list_recent_tweets 读取失败：%v", err)
	}

	body, _ := json.Marshal(out)
	return fmt.Sprintf("[系统] list_recent_tweets 返回 %d 条推文：%s", len(out), string(body))
}
```

- [ ] **Step 6: Make sure `AIHandler` has a `db` field**

Open `backend/internal/handler/ai.go` and find the `AIHandler` struct. If it does not have a `db *sql.DB` field, add it. (It almost certainly does — search for `db  *sql.DB` in handler files.) If `db` is exposed under a different name, adapt the test + impl to use that name.

Verify by running:

```bash
cd /Users/irving/repo/learn-helper/backend
grep -n "db\s*\*sql\.DB\|DB\s*\*sql\.DB" internal/handler/*.go | head -5
```

If a `db` field exists on `AIHandler`, you're good. If it's named `DB`, change the receiver in `twitter_tool.go` and `twitter_tool_test.go` to use `h.DB`.

- [ ] **Step 7: Run the test, verify it passes**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/handler/... -run TestExecuteListRecentTweets -v
```

Expected: 4 subtests PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/handler/ai_classify.go backend/internal/handler/ai.go backend/internal/handler/twitter_tool.go backend/internal/handler/twitter_tool_test.go
git commit -m "feat(ai): implement list_recent_tweets executor"
```

---

## Task 7: Twitter account HTTP handler — list + create

**Files:**
- Create: `backend/internal/handler/twitter_account.go`
- Create: `backend/internal/handler/twitter_account_test.go`

- [ ] **Step 1: Write the failing test for list + create**

Create `backend/internal/handler/twitter_account_test.go`:

```go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
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
```

- [ ] **Step 2: Run the test, verify it fails**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/handler/... -run TestTwitterAccountHandler -v
```

Expected: build fails (`undefined: NewTwitterAccountHandler`).

- [ ] **Step 3: Implement the handler**

Create `backend/internal/handler/twitter_account.go`:

```go
package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// TwitterAccountHandler exposes CRUD for tracked_twitter_accounts and
// the RSSHub Base URL config (stored in ai_configs.config JSON).
type TwitterAccountHandler struct {
	db *sql.DB
}

func NewTwitterAccountHandler(db *sql.DB) *TwitterAccountHandler {
	return &TwitterAccountHandler{db: db}
}

var handleRE = regexp.MustCompile(`^[A-Za-z0-9_]{1,15}$`)

type accountJSON struct {
	ID          int64  `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name,omitempty"`
	Enabled     bool   `json:"enabled"`
	Notes       string `json:"notes,omitempty"`
}

func (h *TwitterAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, handle, COALESCE(display_name, ''), enabled, COALESCE(notes, '')
		FROM tracked_twitter_accounts
		ORDER BY id
	`)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	var out []accountJSON
	for rows.Next() {
		var a accountJSON
		var enabled int
		if err := rows.Scan(&a.ID, &a.Handle, &a.DisplayName, &enabled, &a.Notes); err != nil {
			writeJSONError(w, 500, err.Error())
			return
		}
		a.Enabled = enabled != 0
		out = append(out, a)
	}
	writeJSON(w, 200, out)
}

func (h *TwitterAccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Handle string `json:"handle"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, 400, "invalid json")
		return
	}
	in.Handle = strings.TrimPrefix(in.Handle, "@")
	if !handleRE.MatchString(in.Handle) {
		writeJSONError(w, 400, "handle must match ^[A-Za-z0-9_]{1,15}$ (no @)")
		return
	}
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO tracked_twitter_accounts (handle, notes) VALUES (?, ?)`,
		in.Handle, in.Notes,
	)
	if err != nil {
		if isUniqueViolation(err) {
			writeJSONError(w, 409, "handle already exists")
			return
		}
		writeJSONError(w, 500, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, 201, accountJSON{ID: id, Handle: in.Handle, Enabled: true, Notes: in.Notes})
}

func (h *TwitterAccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSONError(w, 400, "bad id")
		return
	}
	var in struct {
		Handle  *string `json:"handle"`
		Enabled *bool   `json:"enabled"`
		Notes   *string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, 400, "invalid json")
		return
	}
	if in.Handle != nil {
		*in.Handle = strings.TrimPrefix(*in.Handle, "@")
		if !handleRE.MatchString(*in.Handle) {
			writeJSONError(w, 400, "handle must match ^[A-Za-z0-9_]{1,15}$")
			return
		}
	}
	q := `UPDATE tracked_twitter_accounts SET `
	var sets []string
	var args []any
	if in.Handle != nil {
		sets = append(sets, "handle = ?")
		args = append(args, *in.Handle)
	}
	if in.Enabled != nil {
		sets = append(sets, "enabled = ?")
		v := 0
		if *in.Enabled {
			v = 1
		}
		args = append(args, v)
	}
	if in.Notes != nil {
		sets = append(sets, "notes = ?")
		args = append(args, *in.Notes)
	}
	if len(sets) == 0 {
		writeJSONError(w, 400, "no fields to update")
		return
	}
	q += strings.Join(sets, ", ") + " WHERE id = ?"
	args = append(args, id)
	if _, err := h.db.ExecContext(r.Context(), q, args...); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (h *TwitterAccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSONError(w, 400, "bad id")
		return
	}
	if _, err := h.db.ExecContext(r.Context(),
		`DELETE FROM tracked_twitter_accounts WHERE id = ?`, id); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (h *TwitterAccountHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	url, err := h.loadRSSHubURL(r.Context())
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"rsshub_base_url": url})
}

func (h *TwitterAccountHandler) PutConfig(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BaseURL string `json:"rsshub_base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, 400, "invalid json")
		return
	}
	if in.BaseURL == "" {
		writeJSONError(w, 400, "rsshub_base_url required")
		return
	}
	if err := h.saveRSSHubURL(r.Context(), in.BaseURL); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"rsshub_base_url": in.BaseURL})
}

// loadRSSHubURL reads rsshub_base_url from ai_configs.config JSON.
// Defaults to "https://rsshub.app" if unset.
func (h *TwitterAccountHandler) loadRSSHubURL(ctx interface{ Done() <-chan struct{} }) (string, error) {
	// Use sql.DB directly with a fresh context.
	return "", errors.New("not used") // placeholder; replaced in next step
}

// writeJSON / writeJSONError are tiny helpers — see if handler.go has
// equivalents before adding these.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
```

- [ ] **Step 4: Replace the placeholder `loadRSSHubURL` with a real implementation**

Replace the entire `loadRSSHubURL` and add a `saveRSSHubURL` directly. Edit the file to replace the placeholder with:

```go
// loadRSSHubURL reads rsshub_base_url from ai_configs.config JSON.
// Defaults to "https://rsshub.app" if unset.
func (h *TwitterAccountHandler) loadRSSHubURL(ctx contextLike) (string, error) {
	row := h.db.QueryRowContext(ctx.toCtx(), `
		SELECT config FROM ai_configs WHERE is_active = 1 LIMIT 1
	`)
	var cfg sql.NullString
	if err := row.Scan(&cfg); err != nil {
		if err == sql.ErrNoRows {
			return "https://rsshub.app", nil
		}
		return "", err
	}
	if !cfg.Valid || cfg.String == "" {
		return "https://rsshub.app", nil
	}
	var parsed struct {
		BaseURL string `json:"rsshub_base_url"`
	}
	if err := json.Unmarshal([]byte(cfg.String), &parsed); err != nil || parsed.BaseURL == "" {
		return "https://rsshub.app", nil
	}
	return parsed.BaseURL, nil
}

// saveRSSHubURL writes rsshub_base_url into the active ai_configs.config JSON.
// If no active config exists, creates a stub one.
func (h *TwitterAccountHandler) saveRSSHubURL(ctx contextLike, baseURL string) error {
	// Read existing config (may be empty)
	var existing sql.NullString
	row := h.db.QueryRowContext(ctx.toCtx(), `SELECT config FROM ai_configs WHERE is_active = 1 LIMIT 1`)
	_ = row.Scan(&existing)

	merged := map[string]any{}
	if existing.Valid && existing.String != "" {
		_ = json.Unmarshal([]byte(existing.String), &merged)
	}
	merged["rsshub_base_url"] = baseURL
	merged["tavily_api_key"] = "" // ensure key present if previously set
	body, _ := json.Marshal(merged)

	// Try update first
	res, err := h.db.ExecContext(ctx.toCtx(),
		`UPDATE ai_configs SET config = ? WHERE is_active = 1`, string(body),
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// No active config — insert a stub
		_, err = h.db.ExecContext(ctx.toCtx(),
			`INSERT INTO ai_configs (provider, model_name, api_key, is_active, config) VALUES (?, ?, ?, 1, ?)`,
			"", "", "", string(body),
		)
	}
	return err
}

// contextLike is satisfied by *http.Request so handlers can pass it
// through without threading *sql.DB separately. Implemented by
// adapterRequest below.
type contextLike interface {
	Done() <-chan struct{}
	toCtx() context.Context
}
```

Now add the adapter at the bottom of the same file. Add:

```go
// adapterRequest wraps *http.Request to satisfy the tiny contextLike
// interface we use here. This avoids leaking context.Context plumbing
// through the helper signatures.
type adapterRequest struct{ r *http.Request }

func (a adapterRequest) Done() <-chan struct{} { return a.r.Context().Done() }
func (a adapterRequest) toCtx() context.Context { return a.r.Context() }
```

Then change the two call sites in `GetConfig` / `PutConfig`:

In `GetConfig`:
```go
url, err := h.loadRSSHubURL(adapterRequest{r})
```

In `PutConfig`:
```go
if err := h.saveRSSHubURL(adapterRequest{r}, in.BaseURL); err != nil {
```

And add the `context` import to the import block.

- [ ] **Step 5: Run the test, verify it passes**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/handler/... -run TestTwitterAccountHandler -v
```

Expected: 3 tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/handler/twitter_account.go backend/internal/handler/twitter_account_test.go
git commit -m "feat(handler): add twitter account CRUD + RSSHub config HTTP endpoints"
```

---

## Task 8: Update cron `Task` model to include `TaskType`

**Files:**
- Modify: `backend/internal/cron/models.go`

- [ ] **Step 1: Read the current Task struct**

Open `backend/internal/cron/models.go` and find the `Task` struct. The existing fields are: `ID, Name, Description, CronExpr, Prompt, Enabled, AutoApprove, MaxSteps, TimeoutSec, NextRunAt, LastRunAt, LastStatus, LastError, CreatedAt, UpdatedAt`.

- [ ] **Step 2: Add new fields to the Task struct**

Append these fields to the existing `Task` struct (in the order shown):

```go
TaskType            string  // 'generic' (default) or 'twitter_digest'
SinceHours          int
MaxTweetsPerAccount int
MaxTotalTweets      int
```

- [ ] **Step 3: Update DB scan/passing**

Search for every place in `internal/cron/` that constructs or scans a `Task` (especially `db.go`). Add the new fields to both the `SELECT` lists and the `Scan` argument lists. If `db.go` uses a single `SelectAllTasks` style helper, add the columns to the SELECT and pass pointer args to Scan.

For the insert path (creating a new cron task), add the 4 new columns to the INSERT statement with values from the new fields.

If the codebase uses `model.Queries` (sqlc) for the cron_tasks queries, regenerate via `sqlc generate` after updating `db/migrations/queries.sql` to add the 4 new columns to the cron_tasks SELECT.

- [ ] **Step 4: Verify build**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go build ./...
```

Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/cron/models.go
git commit -m "feat(cron): add TaskType + twitter digest config fields to Task"
```

---

## Task 9: Digest runner — fetch + persist step

**Files:**
- Create: `backend/internal/cron/digest_runner.go`
- Create: `backend/internal/cron/digest_runner_test.go`

- [ ] **Step 1: Write the failing test**

Create `backend/internal/cron/digest_runner_test.go`:

```go
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
	return s.out, s.err
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
			enabled INTEGER NOT NULL DEFAULT 1
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
	multi := &multiClient{good, bad}
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
```

- [ ] **Step 2: Run the test, verify it fails**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/cron/... -run TestFetchAndPersist -v
```

Expected: build fails (`undefined: DigestRunner`).

- [ ] **Step 3: Implement DigestRunner (fetch + persist portion)**

Create `backend/internal/cron/digest_runner.go`:

```go
package cron

import (
	"context"
	"log"
	"time"

	"learn-helper/internal/twitter"
)

// DigestConfig is the per-task subset used by the digest runner.
type DigestConfig struct {
	SinceHours          int
	MaxTweetsPerAccount int
	MaxTotalTweets      int
}

// DigestRunner orchestrates a single twitter-digest run: fetch tweets
// for each enabled account, persist them, then call the AI to generate
// the wiki page.
type DigestRunner struct {
	Store  *twitter.Store
	Client twitter.Client
	// AI is set by the main package after construction; the AI step
	// is a separate function (see runDigestAI) so tests can skip it.
	AI DigestAI
}

// DigestAI is the abstract surface the digest runner needs from the
// AI handler. The main package provides a concrete implementation.
type DigestAI interface {
	GenerateDigestPage(ctx context.Context, runID string, cfg DigestConfig) error
}

// fetchAndPersist fetches tweets for every enabled account, persists
// them to the tweets table (idempotent on tweet_id), and returns the
// number of rows newly inserted. Per-account failures are logged and
// skipped.
func (d *DigestRunner) fetchAndPersist(ctx context.Context, runID string, cfg DigestConfig) (int, error) {
	accounts, err := d.Store.ListEnabledAccounts(ctx)
	if err != nil {
		return 0, err
	}
	since := time.Now().Add(-time.Duration(cfg.SinceHours) * time.Hour)
	total := 0
	for _, a := range accounts {
		tweets, err := d.Client.FetchUserTweets(ctx, a.Handle, since, cfg.MaxTweetsPerAccount)
		if err != nil {
			log.Printf("[digest] fetch %s: %v (skipping)", a.Handle, err)
			continue
		}
		for _, tw := range tweets {
			if err := d.Store.InsertTweet(ctx, tw, runID); err != nil {
				log.Printf("[digest] insert tweet %s: %v", tw.TweetID, err)
				continue
			}
			total++
			if total >= cfg.MaxTotalTweets {
				log.Printf("[digest] reached MaxTotalTweets=%d, stopping", cfg.MaxTotalTweets)
				return total, nil
			}
		}
		// Backfill display_name from first successful fetch
		if !a.DisplayName.Valid && len(tweets) > 0 && tweets[0].AuthorName != "" {
			_ = d.Store.UpdateAccountDisplayName(ctx, a.Handle, tweets[0].AuthorName)
		}
	}
	return total, nil
}
```

- [ ] **Step 4: Run the test, verify it passes**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/cron/... -run TestFetchAndPersist -v
```

Expected: 2 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/cron/digest_runner.go backend/internal/cron/digest_runner_test.go
git commit -m "feat(cron): add DigestRunner with fetch-and-persist step"
```

---

## Task 10: Digest runner — full run() with AI step

**Files:**
- Modify: `backend/internal/cron/digest_runner.go`
- Modify: `backend/internal/cron/runner.go`
- Modify: `backend/internal/cron/digest_runner_test.go`

- [ ] **Step 1: Append the `Run` method to DigestRunner**

Append to `backend/internal/cron/digest_runner.go`:

```go
// Run executes the full digest: fetch → persist → AI. The cron
// scheduler calls this from a goroutine. Returns the run record so
// the caller can persist it.
func (d *DigestRunner) Run(ctx context.Context, cronRunID *int64, cfg DigestConfig) (runID string, fetched int, err error) {
	runID = newUUID()
	status := "running"
	var runErrStr string
	defer func() {
		// best-effort finalize — caller may also write to twitter_digest_runs
		_ = runID
		_ = status
		_ = runErrStr
	}()

	insertDigestRunSQL := `INSERT INTO twitter_digest_runs (id, cron_run_id, status) VALUES (?, ?, 'running')`
	if _, err := d.Store.DB().ExecContext(ctx, insertDigestRunSQL, runID, cronRunID); err != nil {
		return runID, 0, err
	}

	fetched, ferr := d.fetchAndPersist(ctx, runID, cfg)
	if ferr != nil {
		_ = d.markFailed(ctx, runID, ferr.Error())
		return runID, 0, ferr
	}
	if fetched == 0 {
		_ = d.markFailed(ctx, runID, "no_new_tweets")
		return runID, 0, nil
	}
	if d.AI == nil {
		// No AI wired up (test path) — stop here, mark fetched.
		_ = d.markFetched(ctx, runID, fetched)
		return runID, fetched, nil
	}

	if err := d.AI.GenerateDigestPage(ctx, runID, cfg); err != nil {
		_ = d.markFailed(ctx, runID, "ai: "+err.Error())
		return runID, fetched, err
	}
	_ = d.markAnalyzed(ctx, runID, fetched)
	return runID, fetched, nil
}

func (d *DigestRunner) markFetched(ctx context.Context, runID string, n int) error {
	_, err := d.Store.DB().ExecContext(ctx,
		`UPDATE twitter_digest_runs SET status='fetched', tweets_fetched=? WHERE id=?`,
		n, runID)
	return err
}

func (d *DigestRunner) markAnalyzed(ctx context.Context, runID string, n int) error {
	_, err := d.Store.DB().ExecContext(ctx,
		`UPDATE twitter_digest_runs SET status='analyzed', tweets_fetched=?, finished_at=CURRENT_TIMESTAMP WHERE id=?`,
		n, runID)
	return err
}

func (d *DigestRunner) markFailed(ctx context.Context, runID, reason string) error {
	_, err := d.Store.DB().ExecContext(ctx,
		`UPDATE twitter_digest_runs SET status='failed', error=?, finished_at=CURRENT_TIMESTAMP WHERE id=?`,
		reason, runID)
	return err
}
```

- [ ] **Step 2: Add `DB()` accessor and newUUID helper**

Add to `backend/internal/twitter/store.go` (append):

```go
// DB returns the underlying *sql.DB. Used by the digest runner to
// write to twitter_digest_runs without coupling the runner to the
// store's individual helpers.
func (s *Store) DB() *sql.DB { return s.db }
```

Add to `backend/internal/cron/digest_runner.go` (append):

```go
import "github.com/google/uuid"

func newUUID() string { return uuid.NewString() }
```

(If `github.com/google/uuid` is not in go.mod, add it via `go get github.com/google/uuid`.)

- [ ] **Step 3: Write the test for the no-new-tweets branch**

Append to `backend/internal/cron/digest_runner_test.go`:

```go
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
```

- [ ] **Step 4: Run the test, verify it passes**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go test ./internal/cron/... -run TestDigestRunner -v
```

Expected: 3 tests PASS.

- [ ] **Step 5: Wire DigestRunner into the existing cron Runner**

Open `backend/internal/cron/runner.go`. Find the `Run` method on `*Runner` (around line 96). At the very start, after the per-task timeout setup and before step 2 (cron_runs insert), add:

```go
// Branch for twitter_digest: the fetch+AI flow lives in DigestRunner
// and uses a different conversation, tool set, and prompt.
if task.TaskType == "twitter_digest" {
    return r.runTwitterDigest(runCtx, task, runID)
}
```

Now add the `runTwitterDigest` method on `*Runner` (append to `runner.go`):

```go
// runTwitterDigest executes the twitter_digest task variant. It
// bridges the existing Runner (which manages cron_runs + cron_tasks
// lifecycle) with the DigestRunner (which manages the digest-specific
// fetch/AI flow).
func (r *Runner) runTwitterDigest(ctx context.Context, task *Task, cronRunID int64) error {
    if r.DigestRunner == nil {
        return fmt.Errorf("DigestRunner not wired in")
    }
    cfg := DigestConfig{
        SinceHours:          task.SinceHours,
        MaxTweetsPerAccount: task.MaxTweetsPerAccount,
        MaxTotalTweets:      task.MaxTotalTweets,
    }
    if cfg.SinceHours <= 0 {
        cfg.SinceHours = 24
    }
    if cfg.MaxTweetsPerAccount <= 0 {
        cfg.MaxTweetsPerAccount = 50
    }
    if cfg.MaxTotalTweets <= 0 {
        cfg.MaxTotalTweets = 200
    }
    _, _, err := r.DigestRunner.Run(ctx, &cronRunID, cfg)
    if err != nil {
        // persist the error to cron_runs so the UI shows it
        _ = r.db.UpdateRun(ctx, &Run{
            ID: cronRunID, Status: RunStatusFailed,
            FinishedAt: sql.NullTime{Time: time.Now(), Valid: true},
            Error:      sql.NullString{String: err.Error(), Valid: true},
        })
    } else {
        _ = r.db.UpdateRun(ctx, &Run{
            ID: cronRunID, Status: RunStatusSuccess,
            FinishedAt: sql.NullTime{Time: time.Now(), Valid: true},
        })
    }
    return err
}
```

And add a field to the `Runner` struct:

```go
type Runner struct {
    db           DB
    hook         RunnerHooks
    DigestRunner *DigestRunner  // optional; only used for twitter_digest tasks
}
```

- [ ] **Step 6: Verify build**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go build ./...
```

Expected: exit 0.

- [ ] **Step 7: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/cron/digest_runner.go backend/internal/cron/digest_runner_test.go backend/internal/cron/runner.go backend/internal/twitter/store.go
git commit -m "feat(cron): wire DigestRunner into Runner for twitter_digest tasks"
```

---

## Task 11: AI integration — implement `GenerateDigestPage` on `AIHandler`

**Files:**
- Create: `backend/internal/handler/digest_ai.go`

- [ ] **Step 1: Implement the AI step**

Create `backend/internal/handler/digest_ai.go`:

```go
package handler

import (
	"context"
	"fmt"
	"time"

	"learn-helper/internal/ai"
	"learn-helper/internal/cron"
)

// digestSystemPrompt is appended to the wiki_maintainer system prompt
// when the AI is being asked to generate a daily digest. It instructs
// the AI to read the tweets table, structure the output, and write a
// single wiki page (create or update, not both).
const digestSystemPrompt = `
=== 特殊任务：AI 日报生成 ===
1. 先调 list_recent_tweets 读取本次 run 抓到的推文。
2. 按以下三段结构组织日报 markdown:
   ## 今日趋势   (3-5 条要点)
   ## 主题讨论   (按主题分组,每组 2-4 条)
   ## 关键引述   (3-5 条原推 + 背景解读)
3. 调 lookup_page(title="AI 日报 · YYYY-MM-DD")
   - 若存在:用 update_page,page_id=<该页 ID>,content=<上面的 markdown>
   - 若不存在:用 create_page,title="AI 日报 · YYYY-MM-DD",content=<上面的 markdown>
4. 不要调任何其他写工具。一次提议只产出一个 create_page 或 update_page action。
`

// GenerateDigestPage satisfies cron.DigestAI. It runs the AI in a
// single ReAct loop with a fixed digest-mode prompt, then auto-approves
// the resulting create_page / update_page call (writes are not gated in
// cron mode).
func (h *AIHandler) GenerateDigestPage(ctx context.Context, runID string, cfg cron.DigestConfig) error {
	if h.aiProviderFactory == nil {
		return fmt.Errorf("ai provider factory not configured")
	}
	providerName, modelName, apiKey, err := h.queries.GetActiveAIConfig(ctx)
	if err != nil {
		return err
	}
	provider, err := h.aiProviderFactory(ai.ProviderType(providerName), apiKey, modelName)
	if err != nil {
		return err
	}

	// Build a user message that includes the run boundary timestamp.
	// The AI's list_recent_tweets tool can filter on this.
	sinceHint := time.Now().Add(-time.Duration(cfg.SinceHours) * time.Hour).Format(time.RFC3339)
	userMsg := fmt.Sprintf("请为本次 digest run (id=%s, since=%s) 生成 AI 日报。\n完成后用 1 句话总结你做了什么。", runID, sinceHint)

	basePrompt := ai.BuildSystemPrompt(ai.RoleWikiMaintainer, "", h.SkillRegistry)
	systemPrompt := basePrompt + "\n" + digestSystemPrompt

	req := ai.ChatRequest{
		Messages:    []ai.Message{{Role: "user", Content: userMsg}},
		SystemPrompt: systemPrompt,
		Tools:       ai.WikiTools(),
		MaxTokens:   8192,
	}

	// Use a no-op sink — cron doesn't stream events anywhere.
	sink := &digestSink{}
	_, err = h.RunReAct(ctx, provider, req, ReActOptions{
		AutoApproveWrites: true,
		MaxSteps:          6, // 工具调用 + propose_plan
		Sink:              sink,
		RunID:             0,
	})
	return err
}

// digestSink is a no-op ReActEventSink used by the digest runner. It
// logs events at info level.
type digestSink struct{}

func (s *digestSink) WriteContent(text string)                          {}
func (s *digestSink) WriteToolCallStart(id, name, input string)         {}
func (s *digestSink) WriteToolResult(id, name, output, errStr string)   {}
func (s *digestSink) WritePermissionRequired(req PermissionRequest)      {}
func (s *digestSink) WriteAskUserRequest(req AskUserRequest)             {}
func (s *digestSink) WriteDone()                                         {}
func (s *digestSink) WriteError(msg string)                              {}
```

- [ ] **Step 2: Add `aiProviderFactory` field to AIHandler**

Open `backend/internal/handler/ai.go` and find the `AIHandler` struct definition. Add:

```go
aiProviderFactory func(ai.ProviderType, string, string) (ai.AIProvider, error)
```

(If the field is named differently, e.g. the codebase already exposes `ProviderFactory`, reuse that name and skip this step.)

- [ ] **Step 3: Verify build**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go build ./...
```

Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/internal/handler/digest_ai.go backend/internal/handler/ai.go
git commit -m "feat(handler): implement AIHandler.GenerateDigestPage for digest runs"
```

---

## Task 12: Wire everything in `cmd/server/main.go`

**Files:**
- Modify: `backend/cmd/server/main.go`

NOTE: This project inlines the actual `CREATE TABLE` / `ALTER TABLE` SQL in `cmd/server/main.go` (the `db/migrations/*.sql` files are documentation). The schema for the new tables must be inlined here, after the existing migration blocks (the 014 add-message-skill block ends at `db.Exec(\`ALTER TABLE messages ADD COLUMN skill TEXT NOT NULL DEFAULT ''\`)` around line 255). Use `CREATE TABLE IF NOT EXISTS` so the block is safe to re-run on existing DBs.

- [ ] **Step 1: Inline the new schema**

Open `backend/cmd/server/main.go`. Find the section that runs ALTER TABLE statements after the initial CREATE TABLE block (search for `ALTER TABLE messages ADD COLUMN skill` — that's the last one from migration 014). Add this block immediately after it:

```go
// ── Migration 015: Twitter digest ──
db.Exec(`ALTER TABLE cron_tasks ADD COLUMN task_type TEXT NOT NULL DEFAULT 'generic'`)
db.Exec(`ALTER TABLE cron_tasks ADD COLUMN since_hours INTEGER NOT NULL DEFAULT 24`)
db.Exec(`ALTER TABLE cron_tasks ADD COLUMN max_tweets_per_account INTEGER NOT NULL DEFAULT 50`)
db.Exec(`ALTER TABLE cron_tasks ADD COLUMN max_total_tweets INTEGER NOT NULL DEFAULT 200`)

db.Exec(`CREATE TABLE IF NOT EXISTS tracked_twitter_accounts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  handle TEXT NOT NULL UNIQUE,
  display_name TEXT,
  enabled INTEGER NOT NULL DEFAULT 1,
  added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  notes TEXT
)`)
db.Exec(`CREATE INDEX IF NOT EXISTS idx_tracked_twitter_enabled ON tracked_twitter_accounts(enabled)`)

db.Exec(`CREATE TABLE IF NOT EXISTS tweets (
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
)`)
db.Exec(`CREATE INDEX IF NOT EXISTS idx_tweets_handle_created ON tweets(handle, created_at DESC)`)
db.Exec(`CREATE INDEX IF NOT EXISTS idx_tweets_digest_run ON tweets(digest_run_id)`)

db.Exec(`CREATE TABLE IF NOT EXISTS twitter_digest_runs (
  id TEXT PRIMARY KEY,
  cron_run_id INTEGER REFERENCES cron_runs(id) ON DELETE SET NULL,
  started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at DATETIME,
  status TEXT NOT NULL,
  tweets_fetched INTEGER NOT NULL DEFAULT 0,
  wiki_page_id INTEGER,
  error TEXT
)`)
db.Exec(`CREATE INDEX IF NOT EXISTS idx_twitter_digest_runs_started ON twitter_digest_runs(started_at DESC)`)
```

Note: each `db.Exec` return value is intentionally ignored — these are idempotent (`IF NOT EXISTS` / default values), so failure on re-run is acceptable. The project's existing migration blocks follow the same pattern.

- [ ] **Step 2: Find the wiring point**

Open `backend/cmd/server/main.go`. Find where the existing `cron.Runner` is constructed and where the AIHandler is created.

- [ ] **Step 3: Construct the Twitter stack**

Insert this block immediately after the AIHandler is built (and after `*sql.DB` is available):

```go
// ── Twitter digest stack ──
twitterStore := twitter.NewStore(db)
twitterHandler := handler.NewTwitterAccountHandler(db)

// RSSHub client — base URL is read from ai_configs at call time, but
// we still need a default for the constructor.
twitterClient := twitter.NewRSSHubClient("https://rsshub.app", 15*time.Second)

// Mount the HTTP routes
r.Route("/api/twitter", func(r chi.Router) {
    r.Get("/accounts", twitterHandler.List)
    r.Post("/accounts", twitterHandler.Create)
    r.Put("/accounts/{id}", twitterHandler.Update)
    r.Delete("/accounts/{id}", twitterHandler.Delete)
    r.Get("/config", twitterHandler.GetConfig)
    r.Put("/config", twitterHandler.PutConfig)
})

// Build the digest runner
digestRunner := &cron.DigestRunner{
    Store:  twitterStore,
    Client: twitterClient,
    AI:     aiHandler, // implements cron.DigestAI via GenerateDigestPage
}
```

- [ ] **Step 4: Attach DigestRunner to the existing cron Runner**

Find where the existing `cron.NewRunner` is called and set the new field:

```go
cronRunner := cron.NewRunner(db, aiHandler)
cronRunner.DigestRunner = digestRunner
```

- [ ] **Step 5: Verify build**

Run:
```bash
cd /Users/irving/repo/learn-helper/backend
go build ./cmd/server
```

Expected: exit 0.

- [ ] **Step 6: Smoke test the HTTP endpoints**

Start the server in the background and curl the new endpoints. The env var is `DB_PATH` (not `DATABASE_PATH`):

```bash
cd /Users/irving/repo/learn-helper/backend
rm -f /tmp/tw-smoke.db /tmp/tw-smoke.db-shm /tmp/tw-smoke.db-wal
DB_PATH=/tmp/tw-smoke.db go run ./cmd/server &
SERVER_PID=$!
sleep 2
SERVER_PID=$!
sleep 2

curl -sS -X POST http://localhost:8080/api/twitter/accounts \
  -H 'Content-Type: application/json' \
  -d '{"handle":"karpathy","notes":"AI researcher"}'
echo

curl -sS http://localhost:8080/api/twitter/accounts
echo

curl -sS -X PUT http://localhost:8080/api/twitter/config \
  -H 'Content-Type: application/json' \
  -d '{"rsshub_base_url":"http://localhost:1200"}'
echo

curl -sS http://localhost:8080/api/twitter/config
echo

kill $SERVER_PID
rm -f /tmp/tw-smoke.db /tmp/tw-smoke.db-shm /tmp/tw-smoke.db-wal
```

Expected: 4 successful responses, no errors. List shows the inserted handle, config shows the new URL.

- [ ] **Step 6: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add backend/cmd/server/main.go
git commit -m "feat(server): mount twitter account routes + wire DigestRunner"
```

---

## Task 13: Frontend — API client functions

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Find an existing similar module**

In `frontend/src/lib/api.ts`, find the section that calls existing endpoints like `accounts` or `cron`. Use those as a template for fetch/JSON patterns.

- [ ] **Step 2: Add the new API functions**

Append the following to `frontend/src/lib/api.ts`:

```ts
// ── Twitter accounts ──
export type TrackedAccount = {
  id: number;
  handle: string;
  display_name?: string;
  enabled: boolean;
  notes?: string;
};

export async function listTwitterAccounts(): Promise<TrackedAccount[]> {
  const r = await fetch('/api/twitter/accounts');
  if (!r.ok) throw new Error('listTwitterAccounts failed');
  return r.json();
}

export async function createTwitterAccount(handle: string, notes?: string): Promise<TrackedAccount> {
  const r = await fetch('/api/twitter/accounts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ handle, notes }),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function updateTwitterAccount(id: number, patch: Partial<Pick<TrackedAccount, 'handle' | 'enabled' | 'notes'>>): Promise<void> {
  const r = await fetch(`/api/twitter/accounts/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  });
  if (!r.ok) throw new Error(await r.text());
}

export async function deleteTwitterAccount(id: number): Promise<void> {
  const r = await fetch(`/api/twitter/accounts/${id}`, { method: 'DELETE' });
  if (!r.ok) throw new Error(await r.text());
}

export async function getTwitterConfig(): Promise<{ rsshub_base_url: string }> {
  const r = await fetch('/api/twitter/config');
  if (!r.ok) throw new Error('getTwitterConfig failed');
  return r.json();
}

export async function setTwitterConfig(rsshub_base_url: string): Promise<void> {
  const r = await fetch('/api/twitter/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ rsshub_base_url }),
  });
  if (!r.ok) throw new Error(await r.text());
}

// ── Cron "run now" ──
export async function runCronTaskNow(taskId: number): Promise<{ run_id: number }> {
  const r = await fetch(`/api/cron/tasks/${taskId}/run-now`, { method: 'POST' });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/irving/repo/learn-helper/frontend
npm run build
```

Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add frontend/src/lib/api.ts
git commit -m "feat(frontend): add twitter account + cron run-now api functions"
```

---

## Task 14: Frontend — Settings page twitter panel

**Files:**
- Modify: `frontend/src/app/settings/page.tsx`

- [ ] **Step 1: Locate an existing settings section**

Open `frontend/src/app/settings/page.tsx`. Find a section that uses `useState` + `useEffect` to load/save config (e.g. the Tavily API Key field). Use that as the structural pattern.

- [ ] **Step 2: Add the Twitter accounts panel**

Add a new section below the AI config section:

```tsx
import { useEffect, useState } from 'react';
import {
  listTwitterAccounts, createTwitterAccount, updateTwitterAccount, deleteTwitterAccount,
  getTwitterConfig, setTwitterConfig,
  type TrackedAccount,
} from '../../lib/api';

// inside the component, alongside other useState:
const [accounts, setAccounts] = useState<TrackedAccount[]>([]);
const [newHandle, setNewHandle] = useState('');
const [rsshubURL, setRsshubURL] = useState('https://rsshub.app');

async function reloadAccounts() {
  setAccounts(await listTwitterAccounts());
}
async function reloadConfig() {
  const c = await getTwitterConfig();
  setRsshubURL(c.rsshub_base_url);
}
useEffect(() => { reloadAccounts(); reloadConfig(); }, []);

// JSX (place below the AI config section):
<section className="mt-8">
  <h2 className="text-lg font-semibold mb-3">推文账号</h2>
  <div className="flex gap-2 mb-3">
    <input
      className="border rounded px-2 py-1 flex-1"
      placeholder="@handle (留空自动去掉 @)"
      value={newHandle}
      onChange={e => setNewHandle(e.target.value)}
    />
    <button
      className="px-3 py-1 rounded bg-blue-500 text-white disabled:opacity-50"
      disabled={!newHandle.trim()}
      onClick={async () => {
        await createTwitterAccount(newHandle.trim());
        setNewHandle('');
        await reloadAccounts();
      }}
    >+ 添加</button>
  </div>
  <ul className="divide-y border rounded">
    {accounts.map(a => (
      <li key={a.id} className="flex items-center gap-3 px-3 py-2">
        <input
          type="checkbox"
          checked={a.enabled}
          onChange={async e => {
            await updateTwitterAccount(a.id, { enabled: e.target.checked });
            await reloadAccounts();
          }}
        />
        <span className="flex-1">
          <span className="font-mono">@{a.handle}</span>
          {a.display_name ? <span className="text-gray-500 ml-2">({a.display_name})</span> : null}
          {a.notes ? <span className="text-gray-500 ml-2">— {a.notes}</span> : null}
        </span>
        <button
          className="text-red-500 text-sm"
          onClick={async () => {
            if (confirm(`删除 @${a.handle}？`)) {
              await deleteTwitterAccount(a.id);
              await reloadAccounts();
            }
          }}
        >删除</button>
      </li>
    ))}
  </ul>

  <h3 className="text-sm font-medium mt-6 mb-2">RSSHub Base URL</h3>
  <div className="flex gap-2">
    <input
      className="border rounded px-2 py-1 flex-1"
      value={rsshubURL}
      onChange={e => setRsshubURL(e.target.value)}
    />
    <button
      className="px-3 py-1 rounded bg-blue-500 text-white"
      onClick={async () => {
        await setTwitterConfig(rsshubURL);
        alert('已保存');
      }}
    >保存</button>
  </div>
  <p className="text-xs text-gray-500 mt-2">
    默认 https://rsshub.app（公网实例常被 X 屏蔽）。建议自部署 RSSHub。
  </p>
</section>
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/irving/repo/learn-helper/frontend
npm run build
```

Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add frontend/src/app/settings/page.tsx
git commit -m "feat(frontend): add twitter accounts panel + RSSHub URL config to settings"
```

---

## Task 15: Frontend — cron form supports `task_type` + run-now button

**Files:**
- Modify: `frontend/src/components/CronTaskForm.tsx`
- Modify: `frontend/src/app/cron/page.tsx`

- [ ] **Step 1: Find the existing cron form**

Open `frontend/src/components/CronTaskForm.tsx`. Locate the place that defines the cron expression input and the submit handler.

- [ ] **Step 2: Add `task_type` selector + twitter_digest config fields**

Add the following fields to the form state and JSX (place the JSX right after the schedule input):

```tsx
const [taskType, setTaskType] = useState<'generic' | 'twitter_digest'>('generic');
const [sinceHours, setSinceHours] = useState(24);
const [maxTweetsPerAccount, setMaxTweetsPerAccount] = useState(50);
const [maxTotalTweets, setMaxTotalTweets] = useState(200);

// JSX:
<div>
  <label>任务类型</label>
  <select value={taskType} onChange={e => setTaskType(e.target.value as any)}>
    <option value="generic">通用 (prompt 由你提供)</option>
    <option value="twitter_digest">AI 日报 (twitter_digest)</option>
  </select>
</div>

{taskType === 'twitter_digest' && (
  <div className="grid grid-cols-3 gap-3">
    <div>
      <label>since_hours</label>
      <input type="number" min="1" value={sinceHours}
             onChange={e => setSinceHours(parseInt(e.target.value) || 24)} />
    </div>
    <div>
      <label>max_tweets_per_account</label>
      <input type="number" min="1" value={maxTweetsPerAccount}
             onChange={e => setMaxTweetsPerAccount(parseInt(e.target.value) || 50)} />
    </div>
    <div>
      <label>max_total_tweets</label>
      <input type="number" min="1" value={maxTotalTweets}
             onChange={e => setMaxTotalTweets(parseInt(e.target.value) || 200)} />
    </div>
  </div>
)}
```

In the form submit body, include the new fields in the payload:

```tsx
body: JSON.stringify({
  ...,
  task_type: taskType,
  since_hours: sinceHours,
  max_tweets_per_account: maxTweetsPerAccount,
  max_total_tweets: maxTotalTweets,
}),
```

- [ ] **Step 3: Add "立即运行" button on the task detail**

In `frontend/src/app/cron/page.tsx`, find the section that renders a single task's details (the `task.id` is in scope). Add a button next to the existing edit/delete controls:

```tsx
import { runCronTaskNow } from '../../lib/api';

<button
  className="px-3 py-1 rounded bg-green-500 text-white"
  onClick={async () => {
    if (!confirm(`立即运行任务「${task.name}」？`)) return;
    try {
      await runCronTaskNow(task.id);
      alert('已触发,稍后查看 run history');
      // optional: refresh run list
    } catch (e: any) {
      alert('失败: ' + e.message);
    }
  }}
>立即运行</button>
```

- [ ] **Step 4: Add backend `run-now` endpoint**

If `/api/cron/tasks/{id}/run-now` does not yet exist, add it. In `backend/internal/cron/http.go` (or wherever cron HTTP routes live), add a handler:

```go
r.Post("/api/cron/tasks/{id}/run-now", func(w http.ResponseWriter, req *http.Request) {
    id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
    if err != nil { writeJSONError(w, 400, "bad id"); return }
    task, err := cronDB.GetTaskByID(req.Context(), id)
    if err != nil { writeJSONError(w, 404, "task not found"); return }
    // Run in a goroutine; return 202 immediately.
    go func() {
        _ = runner.Run(context.Background(), task, RunOpts{})
    }()
    writeJSON(w, 202, map[string]any{"queued": true, "task_id": id})
})
```

(Adapt to the existing cron HTTP code: it likely already has a `Post` route pattern with the runner in scope.)

- [ ] **Step 5: Verify build**

```bash
cd /Users/irving/repo/learn-helper/frontend
npm run build
cd /Users/irving/repo/learn-helper/backend
go build ./...
```

Expected: both succeed.

- [ ] **Step 6: Commit**

```bash
cd /Users/irving/repo/learn-helper
git add frontend/src/components/CronTaskForm.tsx frontend/src/app/cron/page.tsx backend/internal/cron/http.go
git commit -m "feat(frontend,backend): add task_type to cron form + run-now button"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Covered by |
|---|---|
| 1. /settings 增删/启停账号 | Task 14 |
| 2. RSSHub URL 可配 | Tasks 7 (backend), 14 (UI) |
| 3. cron UI 可调度 + 立即运行 | Task 15 |
| 4. 抓推文走 RSSHub | Tasks 2, 3 |
| 5. tweets 表幂等 | Tasks 1, 4 |
| 6. AI 读 tweets 生成 3 段 | Tasks 5, 6, 11 |
| 7. propose_plan → wiki 页面 | Task 11 (uses create_page/update_page directly since propose_plan table dropped) |
| 8. auto_approve=true | Task 11 (ReActOptions.AutoApproveWrites=true) |
| 9. 当日已存在走 update | Task 11 (digest prompt instructs AI to lookup then choose create or update) |
| 10. 失败在 run history 可看 | Tasks 9, 10 (markFailed writes error) |

**Placeholders scan:** No TBD/TODO. All steps have full code or commands.

**Type consistency:**
- `Tweet` struct: defined in Task 2, used by `RSSHubClient` (Task 3), `Store.InsertTweet` (Task 4), `executeListRecentTweets` returns raw rows (Task 6), `DigestRunner.fetchAndPersist` (Task 9) — consistent.
- `DigestRunner`, `DigestConfig`, `DigestAI` interface: defined Task 9, used Task 10/11 — consistent.
- `Task.TaskType` field: added Task 8, used Task 10 — consistent.
- `AIHandler.GenerateDigestPage`: defined Task 11, satisfies `cron.DigestAI` — consistent.

**Out-of-scope risks noted in the plan:**
- The spec mentions `propose_plan` (which was dropped in migration 012). Plan adapts by using `create_page` / `update_page` directly through the ReAct loop's auto-approved writes. This is a behavior change from the spec text but matches current code reality.
- The "internal channel to capture plan_id" is not needed because the AI's `create_page` runs synchronously inside the ReAct loop with `AutoApproveWrites=true`. The page is created during the AI call, not after.

---

## Done criteria

All 15 tasks committed, `go build ./...` and `npm run build` clean, and the manual smoke test from Task 12 succeeds. The user can then: open `/settings`, add `@karpathy`, point RSSHub at a self-hosted instance, create a cron task of type "AI 日报" scheduled for the next minute, watch the run history show a successful run, and see `AI 日报/2026-06-03` appear in the wiki tree.
