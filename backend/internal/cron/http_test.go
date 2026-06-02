package cron

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// stubDB is an in-memory implementation of DB for handler tests. It does not
// persist between test cases — each test gets a fresh stubDB.
type stubDB struct {
	tasks     map[int64]*Task
	runs      map[int64]*Run
	nextTaskID int64
	nextRunID  int64
}

func newStubDB() *stubDB {
	return &stubDB{
		tasks: map[int64]*Task{},
		runs:  map[int64]*Run{},
	}
}

func (s *stubDB) CreateTask(ctx context.Context, t *Task) (int64, error) {
	s.nextTaskID++
	t.ID = s.nextTaskID
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	s.tasks[t.ID] = t
	return t.ID, nil
}

func (s *stubDB) GetTask(ctx context.Context, id int64) (*Task, error) {
	t, ok := s.tasks[id]
	if !ok {
		return nil, errNotFound
	}
	return t, nil
}

func (s *stubDB) ListTasks(ctx context.Context) ([]*Task, error) {
	out := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, t)
	}
	return out, nil
}

func (s *stubDB) UpdateTask(ctx context.Context, id int64, p TaskPatch) error {
	t, ok := s.tasks[id]
	if !ok {
		return errNotFound
	}
	if p.Name != nil {
		t.Name = *p.Name
	}
	if p.Description != nil {
		t.Description = *p.Description
	}
	if p.CronExpr != nil {
		t.CronExpr = *p.CronExpr
	}
	if p.Prompt != nil {
		t.Prompt = *p.Prompt
	}
	if p.Enabled != nil {
		t.Enabled = *p.Enabled
	}
	if p.AutoApprove != nil {
		t.AutoApprove = *p.AutoApprove
	}
	if p.MaxSteps != nil {
		t.MaxSteps = *p.MaxSteps
	}
	if p.TimeoutSec != nil {
		t.TimeoutSec = *p.TimeoutSec
	}
	t.UpdatedAt = time.Now()
	return nil
}

func (s *stubDB) DeleteTask(ctx context.Context, id int64) error {
	delete(s.tasks, id)
	return nil
}

func (s *stubDB) ListEnabledTasksDue(ctx context.Context, now time.Time, stale time.Time) ([]*Task, error) {
	return nil, nil
}

func (s *stubDB) ClaimTask(ctx context.Context, id int64, nextRunAt time.Time) error {
	return nil
}

func (s *stubDB) SetTaskRunningError(ctx context.Context, id int64, errMsg string) error {
	return nil
}

func (s *stubDB) SetTaskLastStatus(ctx context.Context, id int64, status string, errMsg string) error {
	return nil
}

func (s *stubDB) SetNextRunAt(ctx context.Context, id int64, nextRunAt time.Time) error {
	if t, ok := s.tasks[id]; ok {
		t.NextRunAt = sql.NullTime{Time: nextRunAt, Valid: true}
	}
	return nil
}

func (s *stubDB) InsertRun(ctx context.Context, r *Run) (int64, error) {
	s.nextRunID++
	r.ID = s.nextRunID
	s.runs[r.ID] = r
	return r.ID, nil
}

func (s *stubDB) UpdateRun(ctx context.Context, r *Run) error {
	s.runs[r.ID] = r
	return nil
}

func (s *stubDB) GetRun(ctx context.Context, id int64) (*Run, error) {
	r, ok := s.runs[id]
	if !ok {
		return nil, errNotFound
	}
	return r, nil
}

func (s *stubDB) ListRunsByTask(ctx context.Context, taskID int64, limit, offset int) ([]*Run, error) {
	out := []*Run{}
	for _, r := range s.runs {
		if r.TaskID == taskID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *stubDB) ListAllRuns(ctx context.Context, limit, offset int) ([]*Run, error) {
	out := []*Run{}
	for _, r := range s.runs {
		out = append(out, r)
	}
	return out, nil
}

func (s *stubDB) LoadLastRunSummary(ctx context.Context, taskID int64) (string, error) {
	return "", nil
}

func (s *stubDB) CreateConversation(ctx context.Context, title string) (int64, error) {
	return 0, nil
}

func (s *stubDB) AppendMessage(ctx context.Context, convID int64, role, content, provider, toolCallsJSON string) error {
	return nil
}

func (s *stubDB) GetActiveAIConfig(ctx context.Context) (string, string, string, error) {
	return "", "", "", errNotFound
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }

// stubHooks is a no-op RunnerHooks (returns a synthetic successful result).
type stubHooks struct{}

func (s *stubHooks) RunReAct(ctx context.Context, provider interface{}, chatReq interface{}, opts interface{}) (interface{}, error) {
	return nil, nil
}

// We can't use the real RunnerHooks interface here without coupling the test
// to handler.ReActOptions, so the stub returns zero values. The handler's
// RunNow dispatches async so the test doesn't need the result.

func newTestHandler() *Handler {
	db := newStubDB()
	// Pass a nil runner — RunNow will panic on call, which is OK for tests
	// that don't exercise run-now.
	return NewHandler(db, nil)
}

// newTestRouter returns a chi router wired to the test handler with the same
// URL params as production. Tests should call doRouter instead of do.
func newTestRouter() (http.Handler, *Handler) {
	h := newTestHandler()
	r := chi.NewRouter()
	r.Get("/api/cron/tasks", h.ListTasks)
	r.Post("/api/cron/tasks", h.CreateTask)
	r.Get("/api/cron/tasks/{id}", h.GetTask)
	r.Patch("/api/cron/tasks/{id}", h.PatchTask)
	r.Delete("/api/cron/tasks/{id}", h.DeleteTask)
	r.Post("/api/cron/tasks/{id}/run-now", h.RunNow)
	r.Get("/api/cron/tasks/{id}/runs", h.ListRuns)
	r.Get("/api/cron/runs/{id}", h.GetRun)
	return r, h
}

func doRouter(t *testing.T, h http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func do(t *testing.T, h http.HandlerFunc, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestCreateTask_Valid(t *testing.T) {
	r, _ := newTestRouter()
	body := map[string]any{
		"name":      "GitHub 趋势",
		"cron_expr": "0 9 * * *",
		"prompt":    "fetch github.com/trending and summarize",
	}
	rr := doRouter(t, r, "POST", "/api/cron/tasks", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rr.Code, rr.Body.String())
	}
	var resp taskResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Name != "GitHub 趋势" {
		t.Errorf("name = %q, want %q", resp.Name, "GitHub 趋势")
	}
	if !resp.AutoApprove {
		t.Error("auto_approve should default to true")
	}
	if resp.NextRunAt == nil {
		t.Error("next_run_at should be set")
	}
}

func TestCreateTask_InvalidCron(t *testing.T) {
	r, _ := newTestRouter()
	body := map[string]any{
		"name":      "test",
		"cron_expr": "not a cron",
		"prompt":    "x",
	}
	rr := doRouter(t, r, "POST", "/api/cron/tasks", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "invalid cron") {
		t.Errorf("body = %q, want to mention 'invalid cron'", rr.Body.String())
	}
}

func TestCreateTask_MissingName(t *testing.T) {
	r, _ := newTestRouter()
	body := map[string]any{
		"cron_expr": "0 9 * * *",
		"prompt":    "x",
	}
	rr := doRouter(t, r, "POST", "/api/cron/tasks", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
}

func TestCreateTask_PromptTooLong(t *testing.T) {
	r, _ := newTestRouter()
	body := map[string]any{
		"name":      "test",
		"cron_expr": "0 9 * * *",
		"prompt":    strings.Repeat("a", 5000),
	}
	rr := doRouter(t, r, "POST", "/api/cron/tasks", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
}

func TestListTasks_Empty(t *testing.T) {
	r, _ := newTestRouter()
	rr := doRouter(t, r, "GET", "/api/cron/tasks", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string][]taskResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp["tasks"]) != 0 {
		t.Errorf("len(tasks) = %d, want 0", len(resp["tasks"]))
	}
}

func TestPatchTask_ToggleEnabled(t *testing.T) {
	r, _ := newTestRouter()
	body := map[string]any{
		"name":      "t",
		"cron_expr": "0 9 * * *",
		"prompt":    "p",
	}
	rr := doRouter(t, r, "POST", "/api/cron/tasks", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d", rr.Code)
	}
	var created taskResponse
	json.NewDecoder(rr.Body).Decode(&created)

	patch := map[string]any{"enabled": false}
	rr = doRouter(t, r, "PATCH", "/api/cron/tasks/"+itoa(created.ID), patch)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch: %d body=%s", rr.Code, rr.Body.String())
	}
	var updated taskResponse
	json.NewDecoder(rr.Body).Decode(&updated)
	if updated.Enabled {
		t.Error("enabled should be false after patch")
	}
}

func TestDeleteTask(t *testing.T) {
	r, _ := newTestRouter()
	body := map[string]any{
		"name":      "t",
		"cron_expr": "0 9 * * *",
		"prompt":    "p",
	}
	rr := doRouter(t, r, "POST", "/api/cron/tasks", body)
	var created taskResponse
	json.NewDecoder(rr.Body).Decode(&created)

	rr = doRouter(t, r, "DELETE", "/api/cron/tasks/"+itoa(created.ID), nil)
	if rr.Code != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", rr.Code)
	}

	rr = doRouter(t, r, "GET", "/api/cron/tasks/"+itoa(created.ID), nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("get after delete status = %d, want 404", rr.Code)
	}
}

func itoa(i int64) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
