package cron

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

const cronTasksSchema = `
CREATE TABLE cron_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    cron_expr TEXT NOT NULL,
    prompt TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    auto_approve INTEGER NOT NULL DEFAULT 1,
    max_steps INTEGER NOT NULL DEFAULT 10,
    timeout_sec INTEGER NOT NULL DEFAULT 300,
    task_type TEXT NOT NULL DEFAULT 'generic',
    since_hours INTEGER NOT NULL DEFAULT 24,
    max_tweets_per_account INTEGER NOT NULL DEFAULT 50,
    max_total_tweets INTEGER NOT NULL DEFAULT 200,
    next_run_at DATETIME,
    last_run_at DATETIME,
    last_status TEXT,
    last_error TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE cron_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES cron_tasks(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    duration_ms INTEGER,
    output_summary TEXT,
    error TEXT,
    write_count INTEGER NOT NULL DEFAULT 0,
    steps_used INTEGER NOT NULL DEFAULT 0,
    conversation_id INTEGER REFERENCES conversations(id) ON DELETE SET NULL
);
CREATE TABLE conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic_id INTEGER REFERENCES topics(id),
    exercise_id INTEGER REFERENCES exercises(id),
    context_type TEXT DEFAULT 'wiki',
    role TEXT DEFAULT 'wiki_maintainer',
    title TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    model_provider TEXT,
    token_count INTEGER,
    tool_call_id TEXT,
    tool_name TEXT,
    tool_calls TEXT,
    tool_summary TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

func newTaskTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "cron-test.db")
	db, err := sql.Open("sqlite", dsn+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(cronTasksSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

// TestTask_TwitterDigestFieldsRoundTrip verifies that the four new fields
// (task_type, since_hours, max_tweets_per_account, max_total_tweets) survive
// the full CreateTask → GetTask cycle, and that a TaskPatch update for each
// new field also persists.
func TestTask_TwitterDigestFieldsRoundTrip(t *testing.T) {
	db := newTaskTestDB(t)
	a := NewSQLDBAdapter(db)
	ctx := context.Background()

	// Seed an active AI config so any later GetActiveAIConfig call doesn't
	// fail (not strictly required for this test, but mirrors main.go setup).
	_, err := db.ExecContext(ctx, `INSERT INTO conversations (title) VALUES ('seed')`)
	if err != nil {
		t.Fatal(err)
	}

	original := &Task{
		Name:               "tech digest",
		Description:        "morning tech digest",
		CronExpr:           "0 9 * * *",
		Prompt:             "summarize",
		Enabled:            true,
		AutoApprove:        true,
		MaxSteps:           5,
		TimeoutSec:         120,
		TaskType:           "twitter_digest",
		SinceHours:         12,
		MaxTweetsPerAccount: 25,
		MaxTotalTweets:     150,
	}
	id, err := a.CreateTask(ctx, original)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}

	got, err := a.GetTask(ctx, id)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.TaskType != "twitter_digest" {
		t.Errorf("TaskType round-trip: got %q want %q", got.TaskType, "twitter_digest")
	}
	if got.SinceHours != 12 {
		t.Errorf("SinceHours round-trip: got %d want 12", got.SinceHours)
	}
	if got.MaxTweetsPerAccount != 25 {
		t.Errorf("MaxTweetsPerAccount round-trip: got %d want 25", got.MaxTweetsPerAccount)
	}
	if got.MaxTotalTweets != 150 {
		t.Errorf("MaxTotalTweets round-trip: got %d want 150", got.MaxTotalTweets)
	}

	// Patch each of the 4 new fields and verify persistence.
	newType := "twitter_digest"
	newSince := 48
	newPerAcct := 10
	newTotal := 75
	patch := TaskPatch{
		TaskType:            &newType,
		SinceHours:          &newSince,
		MaxTweetsPerAccount: &newPerAcct,
		MaxTotalTweets:      &newTotal,
	}
	if err := a.UpdateTask(ctx, id, patch); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	got2, err := a.GetTask(ctx, id)
	if err != nil {
		t.Fatalf("GetTask after patch: %v", err)
	}
	if got2.TaskType != "twitter_digest" {
		t.Errorf("TaskType after patch: got %q want %q", got2.TaskType, "twitter_digest")
	}
	if got2.SinceHours != 48 {
		t.Errorf("SinceHours after patch: got %d want 48", got2.SinceHours)
	}
	if got2.MaxTweetsPerAccount != 10 {
		t.Errorf("MaxTweetsPerAccount after patch: got %d want 10", got2.MaxTweetsPerAccount)
	}
	if got2.MaxTotalTweets != 75 {
		t.Errorf("MaxTotalTweets after patch: got %d want 75", got2.MaxTotalTweets)
	}

	// Patch with nil pointers for the new fields should leave them
	// unchanged (covers the "no fields to update" early return path's
	// opposite — the "update some other field, leave new ones alone").
	otherName := "renamed"
	if err := a.UpdateTask(ctx, id, TaskPatch{Name: &otherName}); err != nil {
		t.Fatalf("UpdateTask name-only: %v", err)
	}
	got3, err := a.GetTask(ctx, id)
	if err != nil {
		t.Fatalf("GetTask after name patch: %v", err)
	}
	if got3.Name != "renamed" {
		t.Errorf("Name after patch: got %q want %q", got3.Name, "renamed")
	}
	if got3.TaskType != "twitter_digest" {
		t.Errorf("TaskType should be unchanged: got %q", got3.TaskType)
	}
	if got3.SinceHours != 48 {
		t.Errorf("SinceHours should be unchanged: got %d", got3.SinceHours)
	}
}
