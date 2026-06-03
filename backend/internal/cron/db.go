package cron

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DB is the interface the cron package uses for persistence. It is satisfied
// by sqlDBAdapter (which wraps *model.Queries + a *sql.DB for raw queries
// not covered by the generated query methods).
type DB interface {
	// --- Task CRUD ---
	CreateTask(ctx context.Context, t *Task) (int64, error)
	GetTask(ctx context.Context, id int64) (*Task, error)
	ListTasks(ctx context.Context) ([]*Task, error)
	UpdateTask(ctx context.Context, id int64, patch TaskPatch) error
	DeleteTask(ctx context.Context, id int64) error

	// --- Scheduler ops ---
	ListEnabledTasksDue(ctx context.Context, now time.Time, staleRunningAfter time.Time) ([]*Task, error)
	ClaimTask(ctx context.Context, id int64, nextRunAt time.Time) error
	SetTaskRunningError(ctx context.Context, id int64, errMsg string) error
	SetTaskLastStatus(ctx context.Context, id int64, status string, errMsg string) error
	SetNextRunAt(ctx context.Context, id int64, nextRunAt time.Time) error

	// --- Run ops ---
	InsertRun(ctx context.Context, r *Run) (int64, error)
	UpdateRun(ctx context.Context, r *Run) error
	GetRun(ctx context.Context, id int64) (*Run, error)
	ListRunsByTask(ctx context.Context, taskID int64, limit, offset int) ([]*Run, error)
	ListAllRuns(ctx context.Context, limit, offset int) ([]*Run, error)
	LoadLastRunSummary(ctx context.Context, taskID int64) (string, error)

	// --- Conversation/messages (for cron) ---
	CreateConversation(ctx context.Context, title string) (int64, error)
	AppendMessage(ctx context.Context, convID int64, role, content, provider string, toolCallsJSON string) error

	// --- Active AI config (for runner) ---
	GetActiveAIConfig(ctx context.Context) (provider, model, apiKey string, err error)
}

// TaskPatch holds the fields that can be updated via PATCH /api/cron/tasks/{id}.
// Nil pointers mean "do not change". Empty strings are valid (e.g. clearing
// description); use sql.NullString fields where ambiguity matters.
type TaskPatch struct {
	Name        *string
	Description *string
	CronExpr    *string
	Prompt      *string
	Enabled     *bool
	AutoApprove *bool
	MaxSteps    *int
	TimeoutSec  *int
}

// sqlDBAdapter implements DB using *model.Queries + *sql.DB.
type sqlDBAdapter struct {
	db   *sql.DB
	q    Querier
	now  func() time.Time
}

// Querier is a minimal interface for the subset of model.Queries methods the
// cron package needs. Defined here to keep the cron package decoupled from
// the model's full surface.
type Querier interface {
	CreateConversation(ctx context.Context, arg CreateConversationParams) (ConversationRow, error)
}

// ConversationRow is the minimal projection needed from a created conversation.
type ConversationRow struct {
	ID int64
}

// CreateConversationParams matches the relevant fields of model.CreateConversationParams.
type CreateConversationParams struct {
	ContextType string
	Role        string
	Title       string
}

// NewSQLDBAdapter wraps a *sql.DB. The Querier arg can be nil; in that case
// the adapter uses raw SQL for conversation operations too.
func NewSQLDBAdapter(db *sql.DB) *sqlDBAdapter {
	return &sqlDBAdapter{db: db, now: time.Now}
}

// NewSQLDBAdapterWithQ wraps a *sql.DB + a Querier for conversation ops.
func NewSQLDBAdapterWithQ(db *sql.DB, q Querier) *sqlDBAdapter {
	return &sqlDBAdapter{db: db, q: q, now: time.Now}
}

// --- Task CRUD ---

func (a *sqlDBAdapter) CreateTask(ctx context.Context, t *Task) (int64, error) {
	res, err := a.db.ExecContext(ctx, `
		INSERT INTO cron_tasks
		  (name, description, cron_expr, prompt, enabled, auto_approve, max_steps, timeout_sec,
		   task_type, since_hours, max_tweets_per_account, max_total_tweets, next_run_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Name, t.Description, t.CronExpr, t.Prompt,
		boolToInt(t.Enabled), boolToInt(t.AutoApprove),
		t.MaxSteps, t.TimeoutSec,
		t.TaskType, t.SinceHours, t.MaxTweetsPerAccount, t.MaxTotalTweets,
		nullTime(t.NextRunAt),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (a *sqlDBAdapter) GetTask(ctx context.Context, id int64) (*Task, error) {
	row := a.db.QueryRowContext(ctx, taskSelectColumns+` FROM cron_tasks WHERE id = ?`, id)
	return scanTask(row)
}

func (a *sqlDBAdapter) ListTasks(ctx context.Context) ([]*Task, error) {
	rows, err := a.db.QueryContext(ctx, taskSelectColumns+` FROM cron_tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (a *sqlDBAdapter) UpdateTask(ctx context.Context, id int64, p TaskPatch) error {
	// Build dynamic SET clause
	sets := []string{}
	args := []any{}
	if p.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *p.Name)
	}
	if p.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *p.Description)
	}
	if p.CronExpr != nil {
		sets = append(sets, "cron_expr = ?")
		args = append(args, *p.CronExpr)
	}
	if p.Prompt != nil {
		sets = append(sets, "prompt = ?")
		args = append(args, *p.Prompt)
	}
	if p.Enabled != nil {
		sets = append(sets, "enabled = ?")
		args = append(args, boolToInt(*p.Enabled))
	}
	if p.AutoApprove != nil {
		sets = append(sets, "auto_approve = ?")
		args = append(args, boolToInt(*p.AutoApprove))
	}
	if p.MaxSteps != nil {
		sets = append(sets, "max_steps = ?")
		args = append(args, *p.MaxSteps)
	}
	if p.TimeoutSec != nil {
		sets = append(sets, "timeout_sec = ?")
		args = append(args, *p.TimeoutSec)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id)
	q := "UPDATE cron_tasks SET "
	for i, s := range sets {
		if i > 0 {
			q += ", "
		}
		q += s
	}
	q += " WHERE id = ?"
	_, err := a.db.ExecContext(ctx, q, args...)
	return err
}

func (a *sqlDBAdapter) DeleteTask(ctx context.Context, id int64) error {
	_, err := a.db.ExecContext(ctx, `DELETE FROM cron_tasks WHERE id = ?`, id)
	return err
}

// --- Scheduler ops ---

func (a *sqlDBAdapter) ListEnabledTasksDue(ctx context.Context, now time.Time, staleRunningAfter time.Time) ([]*Task, error) {
	rows, err := a.db.QueryContext(ctx, taskSelectColumns+`
		FROM cron_tasks
		WHERE enabled = 1
		  AND next_run_at IS NOT NULL
		  AND next_run_at <= ?
		  AND (last_status IS NULL OR last_status != 'running' OR last_run_at < ?)
		ORDER BY next_run_at ASC`,
		now, staleRunningAfter,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (a *sqlDBAdapter) ClaimTask(ctx context.Context, id int64, nextRunAt time.Time) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE cron_tasks
		SET last_status = 'running',
		    last_run_at = CURRENT_TIMESTAMP,
		    last_error = NULL,
		    next_run_at = ?
		WHERE id = ?`, nextRunAt, id)
	return err
}

func (a *sqlDBAdapter) SetTaskRunningError(ctx context.Context, id int64, errMsg string) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE cron_tasks
		SET last_status = 'failed',
		    last_error = ?,
		    next_run_at = NULL
		WHERE id = ?`, errMsg, id)
	return err
}

func (a *sqlDBAdapter) SetTaskLastStatus(ctx context.Context, id int64, status string, errMsg string) error {
	var errVal any
	if errMsg != "" {
		errVal = errMsg
	}
	_, err := a.db.ExecContext(ctx, `
		UPDATE cron_tasks
		SET last_status = ?,
		    last_error = ?
		WHERE id = ?`, status, errVal, id)
	return err
}

func (a *sqlDBAdapter) SetNextRunAt(ctx context.Context, id int64, nextRunAt time.Time) error {
	_, err := a.db.ExecContext(ctx, `UPDATE cron_tasks SET next_run_at = ? WHERE id = ?`, nextRunAt, id)
	return err
}

// --- Run ops ---

func (a *sqlDBAdapter) InsertRun(ctx context.Context, r *Run) (int64, error) {
	res, err := a.db.ExecContext(ctx, `
		INSERT INTO cron_runs
		  (task_id, status, started_at, output_summary, write_count, steps_used, conversation_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.TaskID, r.Status, r.StartedAt, nullStr(r.OutputSummary), r.WriteCount, r.StepsUsed, nullInt64(r.ConversationID),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (a *sqlDBAdapter) UpdateRun(ctx context.Context, r *Run) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE cron_runs
		SET status = ?,
		    finished_at = ?,
		    duration_ms = ?,
		    output_summary = ?,
		    error = ?,
		    write_count = ?,
		    steps_used = ?,
		    conversation_id = ?
		WHERE id = ?`,
		r.Status, nullTime(r.FinishedAt), nullInt64(r.DurationMS), nullStr(r.OutputSummary),
		nullStr(r.Error), r.WriteCount, r.StepsUsed, nullInt64(r.ConversationID), r.ID,
	)
	return err
}

func (a *sqlDBAdapter) GetRun(ctx context.Context, id int64) (*Run, error) {
	row := a.db.QueryRowContext(ctx, runSelectColumns+` WHERE r.id = ?`, id)
	return scanRun(row)
}

func (a *sqlDBAdapter) ListRunsByTask(ctx context.Context, taskID int64, limit, offset int) ([]*Run, error) {
	rows, err := a.db.QueryContext(ctx, runSelectColumns+`
		WHERE r.task_id = ?
		ORDER BY r.started_at DESC LIMIT ? OFFSET ?`,
		taskID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *sqlDBAdapter) ListAllRuns(ctx context.Context, limit, offset int) ([]*Run, error) {
	rows, err := a.db.QueryContext(ctx, runSelectColumns+`
		ORDER BY r.started_at DESC LIMIT ? OFFSET ?`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *sqlDBAdapter) LoadLastRunSummary(ctx context.Context, taskID int64) (string, error) {
	var s sql.NullString
	err := a.db.QueryRowContext(ctx, `
		SELECT output_summary FROM cron_runs
		WHERE task_id = ? AND status = 'success' AND output_summary IS NOT NULL AND output_summary != ''
		ORDER BY finished_at DESC LIMIT 1`, taskID).Scan(&s)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return s.String, nil
}

// --- Conversation / messages (raw SQL) ---

func (a *sqlDBAdapter) CreateConversation(ctx context.Context, title string) (int64, error) {
	res, err := a.db.ExecContext(ctx, `
		INSERT INTO conversations (context_type, role, title) VALUES ('cron_task', 'wiki_maintainer', ?)
		RETURNING id`, title)
	if err != nil {
		// SQLite RETURNING may not be supported on older versions; fall back to QueryRow.
		var id int64
		if qerr := a.db.QueryRowContext(ctx, `
			INSERT INTO conversations (context_type, role, title) VALUES ('cron_task', 'wiki_maintainer', ?);
			SELECT last_insert_rowid() AS id`, title).Scan(&id); qerr == nil {
			return id, nil
		}
		// final fallback: do it as two statements via a transaction
		tx, txErr := a.db.BeginTx(ctx, nil)
		if txErr != nil {
			return 0, txErr
		}
		defer tx.Rollback()
		_, txErr = tx.ExecContext(ctx, `INSERT INTO conversations (context_type, role, title) VALUES ('cron_task', 'wiki_maintainer', ?)`, title)
		if txErr != nil {
			return 0, txErr
		}
		if err := tx.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&id); err != nil {
			return 0, err
		}
		return id, tx.Commit()
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (a *sqlDBAdapter) AppendMessage(ctx context.Context, convID int64, role, content, provider string, toolCallsJSON string) error {
	var tc sql.NullString
	if toolCallsJSON != "" {
		tc = sql.NullString{String: toolCallsJSON, Valid: true}
	}
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO messages (conversation_id, role, content, model_provider, token_count, tool_calls)
		VALUES (?, ?, ?, ?, 0, ?)`,
		convID, role, content, provider, tc)
	return err
}

// --- AI config (raw SQL) ---

func (a *sqlDBAdapter) GetActiveAIConfig(ctx context.Context) (provider, model, apiKey string, err error) {
	row := a.db.QueryRowContext(ctx, `SELECT provider, model_name, api_key FROM ai_configs WHERE is_active = 1 LIMIT 1`)
	if err := row.Scan(&provider, &model, &apiKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", "", fmt.Errorf("no active AI config")
		}
		return "", "", "", err
	}
	return provider, model, apiKey, nil
}

// --- helpers ---

const taskSelectColumns = `SELECT id, name, description, cron_expr, prompt, enabled, auto_approve,
	max_steps, timeout_sec, task_type, since_hours, max_tweets_per_account, max_total_tweets,
	next_run_at, last_run_at, last_status, last_error, created_at, updated_at`

// runSelectColumns selects from cron_runs joined with cron_tasks to also
// return the task's name. The JOIN is LEFT because ON DELETE CASCADE means a
// task is gone by the time its runs are queried, so a task_name of "" is
// expected for cascade-deleted runs.
const runSelectColumns = `SELECT r.id, r.task_id, COALESCE(t.name, ''), r.status, r.started_at, r.finished_at, r.duration_ms,
	r.output_summary, r.error, r.write_count, r.steps_used, r.conversation_id
FROM cron_runs r LEFT JOIN cron_tasks t ON r.task_id = t.id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(s rowScanner) (*Task, error) {
	var t Task
	var enabled, autoApprove int
	if err := s.Scan(
		&t.ID, &t.Name, &t.Description, &t.CronExpr, &t.Prompt,
		&enabled, &autoApprove,
		&t.MaxSteps, &t.TimeoutSec,
		&t.TaskType, &t.SinceHours, &t.MaxTweetsPerAccount, &t.MaxTotalTweets,
		&t.NextRunAt, &t.LastRunAt, &t.LastStatus, &t.LastError,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	t.Enabled = enabled != 0
	t.AutoApprove = autoApprove != 0
	return &t, nil
}

func scanRun(s rowScanner) (*Run, error) {
	var r Run
	if err := s.Scan(
		&r.ID, &r.TaskID, &r.TaskName, &r.Status, &r.StartedAt, &r.FinishedAt, &r.DurationMS,
		&r.OutputSummary, &r.Error, &r.WriteCount, &r.StepsUsed, &r.ConversationID,
	); err != nil {
		return nil, err
	}
	return &r, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullTime(t sql.NullTime) any {
	if !t.Valid {
		return nil
	}
	return t.Time
}

func nullStr(s sql.NullString) any {
	if !s.Valid {
		return nil
	}
	return s.String
}

func nullInt64(i sql.NullInt64) any {
	if !i.Valid {
		return nil
	}
	return i.Int64
}
