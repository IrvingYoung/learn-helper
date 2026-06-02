package cron

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handler is the HTTP entry point for cron task management. It is decoupled
// from the runner/scheduler so the API can be tested with a stub DB.
type Handler struct {
	db     DB
	runner *Runner
	now    func() time.Time
}

// NewHandler returns a Handler wired to the given DB and Runner.
func NewHandler(db DB, runner *Runner) *Handler {
	return &Handler{db: db, runner: runner, now: time.Now}
}

// --- Request/response DTOs ---

type createTaskRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CronExpr    string `json:"cron_expr"`
	Prompt      string `json:"prompt"`
	Enabled     *bool  `json:"enabled"`
	MaxSteps    *int   `json:"max_steps"`
	TimeoutSec  *int   `json:"timeout_sec"`
}

type patchTaskRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	CronExpr    *string `json:"cron_expr,omitempty"`
	Prompt      *string `json:"prompt,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
	MaxSteps    *int    `json:"max_steps,omitempty"`
	TimeoutSec  *int    `json:"timeout_sec,omitempty"`
}

type taskResponse struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	CronExpr    string     `json:"cron_expr"`
	Prompt      string     `json:"prompt"`
	Enabled     bool       `json:"enabled"`
	AutoApprove bool       `json:"auto_approve"`
	MaxSteps    int        `json:"max_steps"`
	TimeoutSec  int        `json:"timeout_sec"`
	NextRunAt   *time.Time `json:"next_run_at,omitempty"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	LastStatus  string     `json:"last_status,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type runResponse struct {
	ID             int64      `json:"id"`
	TaskID         int64      `json:"task_id"`
	TaskName       string     `json:"task_name,omitempty"`
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	DurationMS     *int64     `json:"duration_ms,omitempty"`
	OutputSummary  string     `json:"output_summary,omitempty"`
	Error          string     `json:"error,omitempty"`
	WriteCount     int64      `json:"write_count"`
	StepsUsed      int64      `json:"steps_used"`
	ConversationID *int64     `json:"conversation_id,omitempty"`
}

func toTaskResponse(t *Task) taskResponse {
	r := taskResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		CronExpr:    t.CronExpr,
		Prompt:      t.Prompt,
		Enabled:     t.Enabled,
		AutoApprove: t.AutoApprove,
		MaxSteps:    t.MaxSteps,
		TimeoutSec:  t.TimeoutSec,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
	if t.NextRunAt.Valid {
		t := t.NextRunAt.Time
		r.NextRunAt = &t
	}
	if t.LastRunAt.Valid {
		t := t.LastRunAt.Time
		r.LastRunAt = &t
	}
	if t.LastStatus.Valid {
		r.LastStatus = t.LastStatus.String
	}
	if t.LastError.Valid {
		r.LastError = t.LastError.String
	}
	return r
}

func toRunResponse(r *Run) runResponse {
	out := runResponse{
		ID:         r.ID,
		TaskID:     r.TaskID,
		TaskName:   r.TaskName,
		Status:     r.Status,
		StartedAt:  r.StartedAt,
		WriteCount: r.WriteCount,
		StepsUsed:  r.StepsUsed,
	}
	if r.FinishedAt.Valid {
		t := r.FinishedAt.Time
		out.FinishedAt = &t
	}
	if r.DurationMS.Valid {
		d := r.DurationMS.Int64
		out.DurationMS = &d
	}
	if r.OutputSummary.Valid {
		out.OutputSummary = r.OutputSummary.String
	}
	if r.Error.Valid {
		out.Error = r.Error.String
	}
	if r.ConversationID.Valid {
		cid := r.ConversationID.Int64
		out.ConversationID = &cid
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- Routes ---

// ListTasks handles GET /api/cron/tasks
func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.db.ListTasks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tasks: "+err.Error())
		return
	}
	out := make([]taskResponse, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toTaskResponse(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": out})
}

// CreateTask handles POST /api/cron/tasks
func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.CronExpr == "" {
		writeError(w, http.StatusBadRequest, "cron_expr is required")
		return
	}
	if err := ValidateCronExpr(req.CronExpr); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}
	if len(req.Prompt) > 4000 {
		writeError(w, http.StatusBadRequest, "prompt too long (max 4000 chars)")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	maxSteps := 10
	if req.MaxSteps != nil {
		maxSteps = *req.MaxSteps
	}
	timeout := 300
	if req.TimeoutSec != nil {
		timeout = *req.TimeoutSec
	}

	now := h.now()
	nextAt, err := NextRunAt(req.CronExpr, now)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	t := &Task{
		Name:        req.Name,
		Description: req.Description,
		CronExpr:    req.CronExpr,
		Prompt:      req.Prompt,
		Enabled:     enabled,
		AutoApprove: true, // v1 default; spec reserves the column for future
		MaxSteps:    maxSteps,
		TimeoutSec:  timeout,
	}

	id, err := h.db.CreateTask(r.Context(), t)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create task: "+err.Error())
		return
	}
	// Set next_run_at via the dedicated method.
	if err := h.db.SetNextRunAt(r.Context(), id, nextAt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set next_run_at: "+err.Error())
		return
	}

	created, err := h.db.GetTask(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load created task: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toTaskResponse(created))
}

// GetTask handles GET /api/cron/tasks/{id}
func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	t, err := h.db.GetTask(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, toTaskResponse(t))
}

// PatchTask handles PATCH /api/cron/tasks/{id}
func (h *Handler) PatchTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req patchTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	// If cron_expr is being changed, validate it and recompute next_run_at.
	if req.CronExpr != nil {
		if err := ValidateCronExpr(*req.CronExpr); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	patch := TaskPatch{
		Name:        req.Name,
		Description: req.Description,
		CronExpr:    req.CronExpr,
		Prompt:      req.Prompt,
		Enabled:     req.Enabled,
		MaxSteps:    req.MaxSteps,
		TimeoutSec:  req.TimeoutSec,
	}
	if err := h.db.UpdateTask(r.Context(), id, patch); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update: "+err.Error())
		return
	}

	// Recompute next_run_at if cron_expr changed
	if req.CronExpr != nil {
		nextAt, err := NextRunAt(*req.CronExpr, h.now())
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.db.SetNextRunAt(r.Context(), id, nextAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update next_run_at: "+err.Error())
			return
		}
	}

	updated, err := h.db.GetTask(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load updated task: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toTaskResponse(updated))
}

// DeleteTask handles DELETE /api/cron/tasks/{id}
func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.db.DeleteTask(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RunNow handles POST /api/cron/tasks/{id}/run-now
// Bypasses the enabled flag and the schedule. Returns 202 with the new run id.
func (h *Handler) RunNow(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	task, err := h.db.GetTask(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	// Dispatch asynchronously. Re-fetch after a brief delay so the
	// response carries the run id we just inserted.
	go func(t *Task) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_ = h.runner.Run(ctx, t, RunOpts{TriggerSource: "manual"})
	}(task)

	// For the response, we don't have a run id yet (it's created inside
	// Runner.Run). Return 202 with the task id and a placeholder; the
	// frontend will poll /runs to see the new run.
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":  "dispatched",
		"task_id": task.ID,
	})
}

// ListRuns handles GET /api/cron/tasks/{id}/runs
func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	runs, err := h.db.ListRunsByTask(r.Context(), id, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runs: "+err.Error())
		return
	}
	out := make([]runResponse, 0, len(runs))
	for _, r := range runs {
		out = append(out, toRunResponse(r))
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": out})
}

// GetRun handles GET /api/cron/runs/{id}
func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	run, err := h.db.GetRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, toRunResponse(run))
}

// ListAllRuns handles GET /api/cron/runs (runs across all tasks, paginated).
func (h *Handler) ListAllRuns(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	runs, err := h.db.ListAllRuns(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runs: "+err.Error())
		return
	}
	out := make([]runResponse, 0, len(runs))
	for _, r := range runs {
		out = append(out, toRunResponse(r))
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": out})
}

// --- helpers ---

func parseID(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("missing id")
	}
	return strconv.ParseInt(s, 10, 64)
}
