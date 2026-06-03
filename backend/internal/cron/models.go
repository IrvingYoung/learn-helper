package cron

import (
	"database/sql"
	"time"
)

// Task mirrors a row of the cron_tasks table.
type Task struct {
	ID               int64
	Name             string
	Description      string
	CronExpr         string
	Prompt           string
	Enabled          bool
	AutoApprove      bool
	MaxSteps         int
	TimeoutSec       int
	TaskType         string // 'generic' (default) or 'twitter_digest'
	SinceHours       int
	MaxTweetsPerAccount int
	MaxTotalTweets   int
	NextRunAt        sql.NullTime
	LastRunAt        sql.NullTime
	LastStatus       sql.NullString
	LastError        sql.NullString
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Run mirrors a row of the cron_runs table. TaskName is populated by a JOIN
// on cron_tasks in the read paths; for InsertRun it stays empty.
type Run struct {
	ID             int64
	TaskID         int64
	TaskName       string
	Status         string
	StartedAt      time.Time
	FinishedAt     sql.NullTime
	DurationMS     sql.NullInt64
	OutputSummary  sql.NullString
	Error          sql.NullString
	WriteCount     int64
	StepsUsed      int64
	ConversationID sql.NullInt64
}

// Run status values
const (
	RunStatusRunning = "running"
	RunStatusSuccess = "success"
	RunStatusFailed  = "failed"
	RunStatusTimeout = "timeout"
)

// RunOpts are passed to Runner.Run alongside a Task.
type RunOpts struct {
	// TriggerSource is "scheduler" or "manual" — stored in the log prefix
	// for clarity but not persisted as a column (use started_at + task_id
	// to disambiguate runs in the UI).
	TriggerSource string
}
