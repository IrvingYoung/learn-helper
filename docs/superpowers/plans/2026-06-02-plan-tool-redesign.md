# Plan Tool Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `propose_plan` mega-tool with 6 atomic write tools + 1 `ask_user` tool, gated by a unified permission queue. Drop `plans` / `plan_actions` tables and all stage/limit machinery.

**Architecture:** ReAct loop classifies tool calls into read/write/ask_user batches. Write calls block on a per-conversation `chanPermission` waiting for user decisions; ask_user calls block on `chanAskUser`. Read calls run auto. Backend streams new SSE events (`permission_required`, `ask_user_request`, etc.) for the frontend to render.

**Tech Stack:** Go (Chi + SQLite) backend, React 19 + Vite + TypeScript frontend, existing SSE chat pipeline.

**Spec:** `docs/superpowers/specs/2026-06-02-plan-tool-redesign-design.md`

---

## File Structure

### New files
- `backend/internal/handler/ai_permission.go` — types + channel map for permission gate
- `backend/internal/handler/ai_askuser.go` — types + handler for ask_user
- `backend/internal/handler/ai_tools.go` — extracted write/read tool execution (one file per concern)
- `backend/db/migrations/012_drop_plans_tables.sql`
- `frontend/src/components/PermissionQueue.tsx`
- `frontend/src/components/AskUserCard.tsx`
- `frontend/src/components/AskUserContext.tsx`
- `frontend/src/lib/diff.ts` — small inline diff util

### Modified files
- `backend/internal/ai/provider.go` — `WikiTools()` rewrite, system prompt rewrite
- `backend/internal/handler/ai.go` — ReAct loop rewrite, new event helpers
- `backend/internal/model/models.go` — remove Plan, PlanAction
- `backend/internal/engine/engine.go` — remove stage/limit code
- `backend/cmd/server/main.go` — register new routes, load migration
- `frontend/src/types/index.ts` — add new event types, remove Plan types
- `frontend/src/components/ChatPanel.tsx` — wire new events
- `frontend/src/components/WikiPage.tsx` — replace PlanPreview import
- `frontend/src/components/ToolCallCard.tsx` — add `pending` state

### Deleted files
- `backend/internal/handler/plan.go`
- `frontend/src/components/PlanPreview.tsx`

### Archived specs
- `openspec/specs/ai-propose-plan-contract/spec.md` — move to `openspec/specs/archive/`
- `openspec/specs/plan-execution-semantics/spec.md` — move to `openspec/specs/archive/`

---

## Phase 1: Backend permission gate infrastructure

Goal: types + channel map + HTTP endpoints in place. No behavior change yet (ReAct loop still uses old path). After Phase 1: tests pass, new endpoints respond 400 with "not implemented in this phase" if hit.

### Task 1.1: Permission gate types

**Files:**
- Create: `backend/internal/handler/ai_permission.go`
- Test: `backend/internal/handler/ai_permission_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/handler/ai_permission_test.go`:

```go
package handler

import (
	"encoding/json"
	"testing"
)

func TestPermissionDecisionJSON(t *testing.T) {
	d := PermissionDecision{
		ID:     "toolu_abc",
		Action: "edit",
		EditedInput: map[string]any{
			"title": "Edited",
		},
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["id"] != "toolu_abc" {
		t.Errorf("id = %v, want toolu_abc", got["id"])
	}
	if got["action"] != "edit" {
		t.Errorf("action = %v, want edit", got["action"])
	}
	edited, ok := got["edited_input"].(map[string]any)
	if !ok {
		t.Fatalf("edited_input not object: %T", got["edited_input"])
	}
	if edited["title"] != "Edited" {
		t.Errorf("edited_input.title = %v, want Edited", edited["title"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/irving/repo/learn-helper/backend && go test ./internal/handler/ -run TestPermissionDecisionJSON -v
```

Expected: FAIL with "undefined: PermissionDecision"

- [ ] **Step 3: Write types**

Create `backend/internal/handler/ai_permission.go`:

```go
package handler

// PermissionDecision is a single user's decision for one tool call in a permission batch.
type PermissionDecision struct {
	ID          string         `json:"id"`
	Action      string         `json:"action"` // "approve" | "reject" | "edit"
	EditedInput map[string]any `json:"edited_input,omitempty"`
}

// PermissionRequest is the SSE event payload sent to the frontend.
type PermissionRequest struct {
	RequestID      string                  `json:"request_id"`
	ConversationID int64                   `json:"conversation_id"`
	Items          []PermissionRequestItem `json:"items"`
}

// PermissionRequestItem is one write op in a permission batch.
type PermissionRequestItem struct {
	ID      string         `json:"id"`
	Tool    string         `json:"tool"`
	Input   map[string]any `json:"input"`
	Preview string         `json:"preview"`
}

// PermissionResponse is the HTTP body the frontend posts back.
type PermissionResponse struct {
	RequestID string               `json:"request_id"`
	Decisions []PermissionDecision `json:"decisions"`
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestPermissionDecisionJSON -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/ai_permission.go backend/internal/handler/ai_permission_test.go
git commit -m "feat(ai): add permission gate types"
```

---

### Task 1.2: ask_user types

**Files:**
- Create: `backend/internal/handler/ai_askuser.go`
- Test: `backend/internal/handler/ai_askuser_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/handler/ai_askuser_test.go`:

```go
package handler

import (
	"encoding/json"
	"testing"
)

func TestAskUserAnswerJSON(t *testing.T) {
	cases := []struct {
		name string
		in   AskUserAnswer
		want string
	}{
		{
			name: "single option",
			in:   AskUserAnswer{Answer: "底层原理"},
			want: `{"answer":"底层原理"}`,
		},
		{
			name: "multi select",
			in:   AskUserAnswer{Answer: []any{"opt A", "opt C"}},
			want: `{"answer":["opt A","opt C"]}`,
		},
		{
			name: "no answer",
			in:   AskUserAnswer{Answer: "no_answer"},
			want: `{"answer":"no_answer"}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(c.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(b) != c.want {
				t.Errorf("got %s, want %s", b, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestAskUserAnswerJSON -v
```

Expected: FAIL with "undefined: AskUserAnswer"

- [ ] **Step 3: Write types**

Create `backend/internal/handler/ai_askuser.go`:

```go
package handler

// AskUserAnswer is the value returned to the LLM as tool_result content.
// Answer is one of: string (single option or free text), []any (multi-select), "no_answer".
type AskUserAnswer struct {
	Answer any `json:"answer"`
}

// AskUserRequest is the SSE event payload sent to the frontend.
type AskUserRequest struct {
	RequestID      string         `json:"request_id"`
	ConversationID int64          `json:"conversation_id"`
	Question       string         `json:"question"`
	Options        []string       `json:"options"`
	Context        *AskUserContext `json:"context,omitempty"`
	MultiSelect    bool           `json:"multi_select"`
	AllowFreeText  bool           `json:"allow_free_text"`
	Header         string         `json:"header,omitempty"`
}

// AskUserContext is the optional payload the LLM sends to show alongside the question.
type AskUserContext struct {
	Kind string `json:"kind"` // "outline" | "page" | "markdown" | "diff"
	Data any    `json:"data"`
}

// AskUserResponse is the HTTP body the frontend posts back.
type AskUserResponse struct {
	RequestID string `json:"request_id"`
	Answer    any    `json:"answer"` // string | []string | "no_answer"
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestAskUserAnswerJSON -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/ai_askuser.go backend/internal/handler/ai_askuser_test.go
git commit -m "feat(ai): add ask_user types"
```

---

### Task 1.3: Channel registry

**Files:**
- Modify: `backend/internal/handler/ai_permission.go`
- Test: `backend/internal/handler/ai_permission_test.go`

- [ ] **Step 1: Write failing test**

Append to `backend/internal/handler/ai_permission_test.go`:

```go
func TestPermissionRegistry(t *testing.T) {
	r := NewPermissionRegistry()

	ch := r.Register("perm-1", 5)
	if ch == nil {
		t.Fatal("Register returned nil chan")
	}
	if r.Pending() != 1 {
		t.Errorf("Pending = %d, want 1", r.Pending())
	}

	r.Resolve("perm-1", []PermissionDecision{{ID: "x", Action: "approve"}})
	select {
	case got := <-ch:
		if len(got) != 1 || got[0].ID != "x" {
			t.Errorf("got %+v, want one decision for x", got)
		}
	case <-time.After(time.Second):
		t.Fatal("resolve did not unblock Register")
	}

	if r.Pending() != 0 {
		t.Errorf("after resolve, Pending = %d, want 0", r.Pending())
	}

	// Resolve unknown id is a no-op (no panic)
	r.Resolve("perm-unknown", nil)
}
```

(Add `"time"` to imports.)

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestPermissionRegistry -v
```

Expected: FAIL with "undefined: NewPermissionRegistry"

- [ ] **Step 3: Implement registry**

Replace the bottom of `backend/internal/handler/ai_permission.go` with:

```go
import (
	"sync"
)

// PermissionRegistry tracks pending permission requests per conversation.
// requestID -> chan of decisions.
type PermissionRegistry struct {
	mu       sync.Mutex
	channels map[string]chan []PermissionDecision
}

func NewPermissionRegistry() *PermissionRegistry {
	return &PermissionRegistry{channels: map[string]chan []PermissionDecision{}}
}

// Register creates a pending channel for requestID. If a request with the same id
// is already registered (re-entrant), returns the existing channel.
func (r *PermissionRegistry) Register(requestID string, capacity int) chan []PermissionDecision {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.channels[requestID]; ok {
		return existing
	}
	ch := make(chan []PermissionDecision, capacity)
	r.channels[requestID] = ch
	return ch
}

// Resolve sends the decisions to the registered channel. No-op if unknown.
func (r *PermissionRegistry) Resolve(requestID string, decisions []PermissionDecision) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.channels[requestID]
	if !ok {
		return
	}
	ch <- decisions
	close(ch)
	delete(r.channels, requestID)
}

// CancelAll drops every pending channel. Used on SSE disconnect.
// Decisions are NOT sent — callers default to reject on cancel.
func (r *PermissionRegistry) CancelAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, ch := range r.channels {
		close(ch)
		delete(r.channels, id)
	}
}

func (r *PermissionRegistry) Pending() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.channels)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestPermissionRegistry -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/ai_permission.go backend/internal/handler/ai_permission_test.go
git commit -m "feat(ai): add permission registry with cancel-all"
```

---

### Task 1.4: ask_user channel registry

**Files:**
- Modify: `backend/internal/handler/ai_askuser.go`
- Test: `backend/internal/handler/ai_askuser_test.go`

- [ ] **Step 1: Write failing test**

Append to `backend/internal/handler/ai_askuser_test.go`:

```go
func TestAskUserRegistry(t *testing.T) {
	r := NewAskUserRegistry()

	ch := r.Register("ask-1")
	if ch == nil {
		t.Fatal("Register returned nil")
	}
	if r.Pending() != 1 {
		t.Errorf("Pending = %d, want 1", r.Pending())
	}

	r.Resolve("ask-1", AskUserResponse{Answer: "底层原理"})
	select {
	case got := <-ch:
		if got.Answer != "底层原理" {
			t.Errorf("Answer = %v, want 底层原理", got.Answer)
		}
	case <-time.After(time.Second):
		t.Fatal("resolve did not unblock")
	}
	if r.Pending() != 0 {
		t.Errorf("after resolve Pending = %d, want 0", r.Pending())
	}
}
```

(Add `"time"` to imports.)

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestAskUserRegistry -v
```

Expected: FAIL

- [ ] **Step 3: Implement registry**

Append to `backend/internal/handler/ai_askuser.go`:

```go
import (
	"sync"
)

// AskUserRegistry tracks pending ask_user requests.
type AskUserRegistry struct {
	mu       sync.Mutex
	channels map[string]chan AskUserResponse
}

func NewAskUserRegistry() *AskUserRegistry {
	return &AskUserRegistry{channels: map[string]chan AskUserResponse{}}
}

func (r *AskUserRegistry) Register(requestID string) chan AskUserResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.channels[requestID]; ok {
		return existing
	}
	ch := make(chan AskUserResponse, 1)
	r.channels[requestID] = ch
	return ch
}

func (r *AskUserRegistry) Resolve(requestID string, resp AskUserResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.channels[requestID]
	if !ok {
		return
	}
	ch <- resp
	close(ch)
	delete(r.channels, requestID)
}

// CancelAll drops every pending channel.
func (r *AskUserRegistry) CancelAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, ch := range r.channels {
		close(ch)
		delete(r.channels, id)
	}
}

func (r *AskUserRegistry) Pending() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.channels)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestAskUserRegistry -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/ai_askuser.go backend/internal/handler/ai_askuser_test.go
git commit -m "feat(ai): add ask_user registry with cancel-all"
```

---

### Task 1.5: HTTP endpoint for permission response

**Files:**
- Create: `backend/internal/handler/permission_http.go`
- Test: `backend/internal/handler/permission_http_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/handler/permission_http_test.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPermissionResponseHandler_NotFound(t *testing.T) {
	h := &AIHandler{permissions: NewPermissionRegistry()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ai/permission_response", h.HandlePermissionResponse)

	body := `{"request_id":"unknown","decisions":[]}`
	req := httptest.NewRequest("POST", "/api/ai/permission_response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (unknown id is a no-op, not an error)", rr.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status field = %v, want ok", resp["status"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestPermissionResponseHandler_NotFound -v
```

Expected: FAIL with "undefined: AIHandler.permissions" or "undefined: HandlePermissionResponse"

- [ ] **Step 3: Add field to AIHandler and handler**

Modify `backend/internal/handler/ai.go` — find the `AIHandler` struct and add a field. Look for the existing struct definition. Add:

```go
	permissions *PermissionRegistry
	askUsers    *AskUserRegistry
```

(Place it next to other handler fields; if there's no obvious spot, append at the end of the struct.)

Create `backend/internal/handler/permission_http.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
)

func (h *AIHandler) HandlePermissionResponse(w http.ResponseWriter, r *http.Request) {
	var req PermissionResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.RequestID == "" {
		http.Error(w, "missing request_id", http.StatusBadRequest)
		return
	}
	if h.permissions != nil {
		h.permissions.Resolve(req.RequestID, req.Decisions)
	}
	writeJSON(w, map[string]any{"status": "ok"})
}
```

(The `writeJSON` helper is in plan.go or ai.go; find the existing one and use it. If none exists, add:)

```go
func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(body)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestPermissionResponseHandler_NotFound -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/ai.go backend/internal/handler/permission_http.go backend/internal/handler/permission_http_test.go
git commit -m "feat(ai): add /api/ai/permission_response endpoint"
```

---

### Task 1.6: HTTP endpoint for ask_user response

**Files:**
- Create: `backend/internal/handler/askuser_http.go`
- Test: `backend/internal/handler/askuser_http_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/handler/askuser_http_test.go`:

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAskUserResponseHandler_MissingID(t *testing.T) {
	h := &AIHandler{askUsers: NewAskUserRegistry()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ai/ask_user_response", h.HandleAskUserResponse)

	body := `{"request_id":"","answer":"x"}`
	req := httptest.NewRequest("POST", "/api/ai/ask_user_response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestAskUserResponseHandler_MissingID -v
```

Expected: FAIL

- [ ] **Step 3: Implement handler**

Create `backend/internal/handler/askuser_http.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
)

func (h *AIHandler) HandleAskUserResponse(w http.ResponseWriter, r *http.Request) {
	var req AskUserResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.RequestID == "" {
		http.Error(w, "missing request_id", http.StatusBadRequest)
		return
	}
	if h.askUsers != nil {
		h.askUsers.Resolve(req.RequestID, req)
	}
	writeJSON(w, map[string]any{"status": "ok"})
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestAskUserResponseHandler_MissingID -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/askuser_http.go backend/internal/handler/askuser_http_test.go
git commit -m "feat(ai): add /api/ai/ask_user_response endpoint"
```

---

### Task 1.7: Wire registries in AIHandler constructor

**Files:**
- Modify: `backend/internal/handler/ai.go`

(Look for `NewAIHandler` or wherever the AIHandler is constructed. If there's no constructor, find the call site in main.go and add registry init there.)

- [ ] **Step 1: Find the AIHandler construction site**

```bash
grep -n "AIHandler{" /Users/irving/repo/learn-helper/backend
```

- [ ] **Step 2: Initialize registries**

Wherever the AIHandler is built, ensure:

```go
h.permissions = NewPermissionRegistry()
h.askUsers = NewAskUserRegistry()
```

(If using a constructor `NewAIHandler(db, queries, engine)`, add these lines at the end of the constructor.)

- [ ] **Step 3: Build & run existing tests**

```bash
cd /Users/irving/repo/learn-helper/backend && go build ./... && go test ./internal/handler/ -v
```

Expected: build OK, all existing tests pass.

- [ ] **Step 4: Commit**

```bash
git add -u
git commit -m "feat(ai): init permission and ask_user registries in handler"
```

---

### Task 1.8: Wire SSE disconnect → cancel all pending

**Files:**
- Modify: `backend/internal/handler/ai.go`

(Locate the SSE handler that streams the chat response. It's the one with `for chunk := range streamCh`. The disconnect signal comes from `r.Context().Done()`.)

- [ ] **Step 1: Find SSE chat handler**

```bash
grep -n "r.Context()\.Done\|func.*StreamChat\|func.*HandleAI\|func.*Chat" /Users/irving/repo/learn-helper/backend/internal/handler/ai.go | head -20
```

- [ ] **Step 2: Add disconnect cleanup**

In the SSE handler, at the end of the function (or in a `defer` near the top), add:

```go
defer func() {
    if h.permissions != nil {
        h.permissions.CancelAll()
    }
    if h.askUsers != nil {
        h.askUsers.CancelAll()
    }
}()
```

- [ ] **Step 3: Build & test**

```bash
go build ./... && go test ./internal/handler/ -v
```

Expected: all green.

- [ ] **Step 4: Commit**

```bash
git add -u
git commit -m "feat(ai): cancel pending gates on SSE disconnect"
```

---

### Phase 1 verification

```bash
go test ./internal/handler/ -v
go build ./...
```

Expected: all green, no regressions. Endpoints exist and respond correctly to bogus input. ReAct loop unchanged. New endpoints not yet wired in main.go — that's fine, Phase 5.

---

## Phase 2: WikiTools() rewrite

Goal: `WikiTools()` returns the new 12-tool inventory. Old `propose_plan` gone. The system prompt is updated to teach the new model. LLM cannot call `propose_plan` anymore, but no behavior change yet because the ReAct loop still routes it to old `createPlanFromToolCall` (which is removed in Phase 3). To avoid breakage: in Phase 2 we update tools, but the ReAct loop still tolerates legacy `propose_plan` tool calls by returning a graceful error.

### Task 2.1: New tool inventory

**Files:**
- Modify: `backend/internal/ai/provider.go`

- [ ] **Step 1: Replace WikiTools() body**

In `backend/internal/ai/provider.go`, find the `WikiTools()` function (around line 65-303). Replace the entire return value with the new inventory. The read tools keep their existing schemas (lines 247-302). Delete the entire `propose_plan` block (lines 67-246). After the read tools, append new write tools + `ask_user`.

The new function body is:

```go
func WikiTools() []Tool {
	return []Tool{
		// ── Write tools (gated by permission queue) ──
		{
			Name:        "create_page",
			Description: "在指定父页面下创建新页面。走权限闸门,需要用户批准。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":     map[string]any{"type": "string", "description": "页面标题(必填)"},
					"parent_id": map[string]any{"type": "integer", "description": "父页面 ID;顶级页面或留空走 focusPageID"},
					"content":   map[string]any{"type": "string", "description": "页面 markdown 内容,可选(空骨架页)"},
					"page_type": map[string]any{"type": "string", "enum": []string{"entity", "concept", "overview"}, "description": "页面类型,默认 entity"},
					"slug":      map[string]any{"type": "string", "description": "URL slug,可选,默认自动生成"},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "update_page",
			Description: "覆盖式更新页面内容。走权限闸门。改大段用这个。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "要更新的页面 ID(必填)"},
					"content": map[string]any{"type": "string", "description": "新 markdown 内容(必填)"},
					"title":   map[string]any{"type": "string", "description": "新标题,可选"},
				},
				"required": []string{"page_id", "content"},
			},
		},
		{
			Name:        "patch_page",
			Description: "增量编辑页面:按标题替换章节(replace)或在末尾追加(append)。走权限闸门。改小段用这个,避免重写整页。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "页面 ID(必填)"},
					"operations": map[string]any{
						"type":        "array",
						"description": "操作列表,按顺序执行",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"type":    map[string]any{"type": "string", "enum": []string{"replace", "append"}},
								"target":  map[string]any{"type": "string", "description": "replace 的目标标题(带 # 号,如 '## 核心概念')"},
								"content": map[string]any{"type": "string", "description": "markdown 内容"},
							},
							"required": []string{"type", "content"},
						},
					},
				},
				"required": []string{"page_id", "operations"},
			},
		},
		{
			Name:        "delete_page",
			Description: "删除页面。走权限闸门。慎用:能 move_page / update_page 解决的优先用那两个。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "页面 ID"},
				},
				"required": []string{"page_id"},
			},
		},
		{
			Name:        "link_pages",
			Description: "在 source 页面添加指向 target 的链接。走权限闸门。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_page_id": map[string]any{"type": "integer", "description": "源页 ID"},
					"target_page_id": map[string]any{"type": "integer", "description": "目标页 ID"},
					"link_text":      map[string]any{"type": "string", "description": "链接显示文本,可选(默认用目标页标题)"},
				},
				"required": []string{"source_page_id", "target_page_id"},
			},
		},
		{
			Name:        "move_page",
			Description: "把页面移到新父节点下。走权限闸门。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id":       map[string]any{"type": "integer", "description": "要移动的页面 ID"},
					"new_parent_id": map[string]any{"type": "integer", "description": "新父页 ID"},
				},
				"required": []string{"page_id", "new_parent_id"},
			},
		},

		// ── ask_user ──
		{
			Name:        "ask_user",
			Description: "向用户提一个澄清问题。可以附带 context(outline / page / markdown / diff)让用户看到具体物料。阻塞 ReAct loop 直到用户回答。不用于确认写操作(那是权限闸门的事)。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{"type": "string", "description": "问题正文"},
					"options": map[string]any{
						"type":        "array",
						"description": "2-4 个选项",
						"items":       map[string]any{"type": "string"},
						"minItems":    2,
						"maxItems":    4,
					},
					"context": map[string]any{
						"type": "object",
						"description": "可选,context.kind 决定渲染",
						"properties": map[string]any{
							"kind": map[string]any{"type": "string", "enum": []string{"outline", "page", "markdown", "diff"}},
							"data": map[string]any{"description": "按 kind 决定形状:outline→OutlineNode[];page→{page_id};markdown→string;diff→[{page_id,before,after,label?}]"},
						},
						"required": []string{"kind", "data"},
					},
					"multi_select":    map[string]any{"type": "boolean", "description": "默认 false"},
					"allow_free_text": map[string]any{"type": "boolean", "description": "默认 true"},
					"header":          map[string]any{"type": "string", "description": "短标签,最多 12 字符"},
				},
				"required": []string{"question", "options"},
			},
		},

		// ── Read tools (unchanged) ──
		{
			Name:        "lookup_page",
			Description: "根据页面标题查询页面信息,返回页面 ID、标题等元数据。可自动执行,无需用户确认。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{"type": "string", "description": "要查询的页面标题(精确匹配)"},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "read_page",
			Description: "根据页面 ID 读取 Wiki 页面的完整 Markdown 内容。可自动执行。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"page_id": map[string]any{"type": "integer", "description": "页面 ID"},
				},
				"required": []string{"page_id"},
			},
		},
		{
			Name:        "search_pages",
			Description: "在知识库中搜索页面标题和内容,返回匹配的页面列表。可自动执行。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "搜索关键词"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "websearch",
			Description: fmt.Sprintf("搜索网络获取相关信息。当前是 %d 年,注意搜索内容的时效性。", time.Now().Year()),
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":       map[string]any{"type": "string"},
					"max_results": map[string]any{"type": "integer"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "webfetch",
			Description: "获取指定 URL 的网页内容,提取正文文本返回。可自动执行。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{"type": "string"},
				},
				"required": []string{"url"},
			},
		},
	}
}
```

- [ ] **Step 2: Build to verify syntax**

```bash
cd /Users/irving/repo/learn-helper/backend && go build ./...
```

Expected: build OK.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/ai/provider.go
git commit -m "feat(ai): rewrite WikiTools — 6 atomic write + ask_user, drop propose_plan"
```

---

### Task 2.2: System prompt rewrite

**Files:**
- Modify: `backend/internal/ai/provider.go`

(Look for `BuildSystemPrompt` and the constant system prompt text — search for the long string starting with "你是 wiki_maintainer" or the existing stage rules.)

- [ ] **Step 1: Find the system prompt construction**

```bash
grep -n "三阶段\|propose_plan\|calibration_question\|BuildSystemPrompt" /Users/irving/repo/learn-helper/backend/internal/ai/provider.go
```

- [ ] **Step 2: Replace prompt body**

Find the existing prompt string (something like `const wikiMaintainerPrompt = ...` or inline in `BuildSystemPrompt`). Replace the **section** that covers 三阶段 / propose_plan / calibration_question with the new version. The knowledge map guide and date strings stay.

New section (replace the entire block from "## 工作节奏" through "## 行为规则" — or wherever the stage rules live):

```
## 工具集

- 读工具(自动执行):lookup_page / read_page / search_pages / websearch / webfetch
- 写工具(走权限闸门):create_page / update_page / patch_page / delete_page / link_pages / move_page
- ask_user:方向不确定时主动问用户,可附带 context 让用户看到具体物料
- 没有 propose_plan,不要试图调用

## 工作流

1. 写前先读:用 lookup_page / search_pages / read_page 了解上下文
2. 方向不确定 → 调 ask_user(context 传具体物料,kind: outline / page / markdown / diff),不调 ask_user 来确认写操作
3. 写操作一次可以调多个(同一批权限闸门),但**不要在同批里引用尚未执行的 op 结果**——需要等结果的话拆到下一轮
4. 页面内用 [页面标题] 语法做链接
5. 用户没让你做 → 不主动写
6. delete_page 慎用:能 move_page / update_page 解决的优先用那两个
7. 改大段用 update_page,改小段用 patch_page(避免重写整页)
8. 写操作后用户可能在右下面板批准/拒绝/编辑后批准。拒绝会回灌 error,你可以改方案重提

## ask_user 详解

context.kind 四种:
- outline: 树状大纲,递归结构 {id, title, page_type, children}
- page: { page_id },渲染页面预览
- markdown: 任意 markdown 字符串
- diff: [{ page_id, before, after, label? }],渲染修改前后对比,多 page 时顶部 tab 切换

2-4 个选项,默认单选 + 允许 free text。**不用于确认写操作**——那是权限闸门的事。

## 权限闸门

每个写工具调用都会进入权限闸门,ReAct loop 暂停等用户在右下面板批/拒/编辑后批。
拒绝的 op 会回灌 error("rejected by user"),你可以改方案重提或换思路。
```

Delete the entire sections covering: 三阶段分离、阶段一/二/三、`calibration_question` 字段说明、`{{action:X.field}}` 占位符、depends_on 规则、自检清单。

- [ ] **Step 3: Build & test**

```bash
go build ./... && go test ./internal/ai/ -v
```

Expected: build OK.

- [ ] **Step 4: Commit**

```bash
git add -u
git commit -m "feat(ai): rewrite system prompt — teach permission gate and ask_user, drop stages"
```

---

### Phase 2 verification

```bash
go build ./... && go test ./...
```

Expected: build OK, all tests green. The LLM is now seeing the new tools but the ReAct loop still has the old propose_plan handler. We'll route it in Phase 3.

---

## Phase 3: ReAct loop rewrite

Goal: replace the old "if propose_plan: handle & break; else: execute auto tools" logic with the new classifier + per-type dispatcher.

### Task 3.1: Tool classification

**Files:**
- Create: `backend/internal/handler/ai_classify.go`
- Test: `backend/internal/handler/ai_classify_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/handler/ai_classify_test.go`:

```go
package handler

import "testing"

func TestClassifyToolCalls(t *testing.T) {
	tcs := []struct {
		name    string
		in      []string // tool names
		wantR   []string
		wantW   []string
		wantAsk []string
	}{
		{
			name:    "all reads",
			in:      []string{"read_page", "search_pages"},
			wantR:   []string{"read_page", "search_pages"},
		},
		{
			name:    "writes batch",
			in:      []string{"create_page", "link_pages"},
			wantW:   []string{"create_page", "link_pages"},
		},
		{
			name:    "ask_user alone",
			in:      []string{"ask_user"},
			wantAsk: []string{"ask_user"},
		},
		{
			name:    "mixed",
			in:      []string{"read_page", "create_page", "ask_user", "update_page"},
			wantR:   []string{"read_page"},
			wantW:   []string{"create_page", "update_page"},
			wantAsk: []string{"ask_user"},
		},
		{
			name:    "unknown tool → writeBatch (treated as write, will fail at execute)",
			in:      []string{"propose_plan"},
			wantW:   []string{"propose_plan"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var calls []aiToolCall
			for _, n := range tc.in {
				calls = append(calls, aiToolCall{Name: n})
			}
			r, w, a := classifyToolCalls(calls)
			gotR := names(r)
			gotW := names(w)
			gotA := names(a)
			if !equal(gotR, tc.wantR) {
				t.Errorf("reads: got %v want %v", gotR, tc.wantR)
			}
			if !equal(gotW, tc.wantW) {
				t.Errorf("writes: got %v want %v", gotW, tc.wantW)
			}
			if !equal(gotA, tc.wantAsk) {
				t.Errorf("asks: got %v want %v", gotA, tc.wantAsk)
			}
		})
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func names(cs []aiToolCall) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestClassifyToolCalls -v
```

Expected: FAIL

- [ ] **Step 3: Implement classifier**

Create `backend/internal/handler/ai_classify.go`:

```go
package handler

// aiToolCall is the minimal shape needed by the classifier and dispatcher.
type aiToolCall struct {
	Name  string
	ID    string
	Input string // raw JSON
}

// classifyToolCalls splits a slice of tool calls into read / write / ask_user batches.
// Read tools: lookup_page, read_page, search_pages, websearch, webfetch.
// Write tools: create_page, update_page, patch_page, delete_page, link_pages, move_page.
// ask_user: ask_user.
// Unknown names are routed to writeBatch (will fail validation at execution).
func classifyToolCalls(calls []aiToolCall) (reads, writes, asks []aiToolCall) {
	readSet := map[string]bool{
		"lookup_page": true, "read_page": true, "search_pages": true,
		"websearch": true, "webfetch": true,
	}
	askSet := map[string]bool{"ask_user": true}

	for _, c := range calls {
		switch {
		case askSet[c.Name]:
			asks = append(asks, c)
		case readSet[c.Name]:
			reads = append(reads, c)
		default:
			writes = append(writes, c)
		}
	}
	return
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestClassifyToolCalls -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/ai_classify.go backend/internal/handler/ai_classify_test.go
git commit -m "feat(ai): add tool call classifier (read/write/ask_user)"
```

---

### Task 3.2: Write tool executor

**Files:**
- Create: `backend/internal/handler/ai_write.go`
- Test: `backend/internal/handler/ai_write_test.go`

(Each write tool routes to the existing engine logic. The actual mutation lives in `engine.ExecutePlan` which still works on `[]model.PlanAction`. To keep the change small, we adapt each write tool call into a single-action "ad-hoc plan" and call the engine.)

- [ ] **Step 1: Write failing test**

Create `backend/internal/handler/ai_write_test.go`:

```go
package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseWriteInput_CreatePage(t *testing.T) {
	in := json.RawMessage(`{"title":"Go 并发","parent_id":16}`)
	got, err := parseWriteInput("create_page", in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got["title"] != "Go 并发" {
		t.Errorf("title = %v", got["title"])
	}
	if got["page_id"] != nil {
		// create_page has no page_id, but a placeholder should be set
		t.Errorf("page_id should be nil, got %v", got["page_id"])
	}
}

func TestParseWriteInput_UpdatePage(t *testing.T) {
	in := json.RawMessage(`{"page_id":42,"content":"# x"}`)
	got, err := parseWriteInput("update_page", in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got["page_id"] != float64(42) {
		t.Errorf("page_id = %v (%T)", got["page_id"], got["page_id"])
	}
}

func TestParseWriteInput_MissingRequired(t *testing.T) {
	in := json.RawMessage(`{"content":"# x"}`)
	_, err := parseWriteInput("update_page", in)
	if err == nil {
		t.Fatal("expected error for missing page_id")
	}
}

func TestParseWriteInput_RejectsPlaceholder(t *testing.T) {
	in := json.RawMessage(`{"title":"x","parent_id":"{{action:a1.page_id}}"}`)
	_, err := parseWriteInput("create_page", in)
	if err == nil {
		t.Fatal("expected error for placeholder reference")
	}
	if !strings.Contains(err.Error(), "placeholder") {
		t.Errorf("error should mention placeholder, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestParseWriteInput -v
```

Expected: FAIL

- [ ] **Step 3: Implement parser**

Create `backend/internal/handler/ai_write.go`:

```go
package handler

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseWriteInput validates and decodes a write tool's input.
// Returns a map with the required fields populated.
// Returns an error if required fields are missing.
// Returns an error if any field value contains a {{action:X}} placeholder
// (same-batch chaining is not allowed — split across turns).
func parseWriteInput(tool string, raw json.RawMessage) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse %s input: %w", tool, err)
	}
	for k, v := range m {
		if s, ok := v.(string); ok && strings.Contains(s, "{{action:") {
			return nil, fmt.Errorf("%s: field %q references a placeholder from a pending tool call in the same batch; split the call into a later turn", tool, k)
		}
	}
	switch tool {
	case "create_page":
		if _, ok := m["title"].(string); !ok {
			return nil, fmt.Errorf("create_page: title (string) is required")
		}
	case "update_page":
		if _, ok := numberField(m, "page_id"); !ok {
			return nil, fmt.Errorf("update_page: page_id (integer) is required")
		}
		if _, ok := m["content"].(string); !ok {
			return nil, fmt.Errorf("update_page: content (string) is required")
		}
	case "patch_page":
		if _, ok := numberField(m, "page_id"); !ok {
			return nil, fmt.Errorf("patch_page: page_id (integer) is required")
		}
		if _, ok := m["operations"].([]any); !ok {
			return nil, fmt.Errorf("patch_page: operations (array) is required")
		}
	case "delete_page", "move_page":
		if _, ok := numberField(m, "page_id"); !ok {
			return nil, fmt.Errorf("%s: page_id (integer) is required", tool)
		}
		if tool == "move_page" {
			if _, ok := numberField(m, "new_parent_id"); !ok {
				return nil, fmt.Errorf("move_page: new_parent_id (integer) is required")
			}
		}
	case "link_pages":
		if _, ok := numberField(m, "source_page_id"); !ok {
			return nil, fmt.Errorf("link_pages: source_page_id (integer) is required")
		}
		if _, ok := numberField(m, "target_page_id"); !ok {
			return nil, fmt.Errorf("link_pages: target_page_id (integer) is required")
		}
	default:
		return nil, fmt.Errorf("unknown write tool: %s", tool)
	}
	return m, nil
}

func numberField(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestParseWriteInput -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/ai_write.go backend/internal/handler/ai_write_test.go
git commit -m "feat(ai): add write tool input parser/validator"
```

---

### Task 3.3: Preview generator

**Files:**
- Modify: `backend/internal/handler/ai_write.go`
- Test: `backend/internal/handler/ai_write_test.go`

- [ ] **Step 1: Write failing test**

Append to `ai_write_test.go`:

```go
func TestPreviewWrite(t *testing.T) {
	cases := []struct {
		tool string
		in   map[string]any
		want string
	}{
		{"create_page", map[string]any{"title": "Go 并发", "parent_id": float64(16)}, "在父页 16 下创建页面「Go 并发」"},
		{"update_page", map[string]any{"page_id": float64(42)}, "更新页面 42"},
		{"delete_page", map[string]any{"page_id": float64(42)}, "删除页面 42"},
		{"move_page", map[string]any{"page_id": float64(42), "new_parent_id": float64(16)}, "把页面 42 移到父页 16 下"},
		{"link_pages", map[string]any{"source_page_id": float64(1), "target_page_id": float64(2)}, "在页面 1 添加指向 2 的链接"},
		{"patch_page", map[string]any{"page_id": float64(42)}, "增量编辑页面 42"},
	}
	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			got := previewWrite(c.tool, c.in)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/handler/ -run TestPreviewWrite -v
```

Expected: FAIL

- [ ] **Step 3: Implement preview**

Append to `ai_write.go`:

```go
// previewWrite produces a human-readable summary of a write op for the permission queue UI.
func previewWrite(tool string, in map[string]any) string {
	id := intField(in, "page_id")
	switch tool {
	case "create_page":
		title, _ := in["title"].(string)
		pid := intField(in, "parent_id")
		if pid != 0 {
			return fmt.Sprintf("在父页 %d 下创建页面「%s」", pid, title)
		}
		return fmt.Sprintf("创建顶级页面「%s」", title)
	case "update_page":
		return fmt.Sprintf("更新页面 %d", id)
	case "patch_page":
		return fmt.Sprintf("增量编辑页面 %d", id)
	case "delete_page":
		return fmt.Sprintf("删除页面 %d", id)
	case "link_pages":
		s := intField(in, "source_page_id")
		t := intField(in, "target_page_id")
		return fmt.Sprintf("在页面 %d 添加指向 %d 的链接", s, t)
	case "move_page":
		np := intField(in, "new_parent_id")
		return fmt.Sprintf("把页面 %d 移到父页 %d 下", id, np)
	default:
		return fmt.Sprintf("%s: %v", tool, in)
	}
}

func intField(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return int64(f)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/handler/ -run TestPreviewWrite -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/ai_write.go backend/internal/handler/ai_write_test.go
git commit -m "feat(ai): add write op preview generator"
```

---

### Task 3.4: Replace ReAct loop body

**Files:**
- Modify: `backend/internal/handler/ai.go`

(Replace the entire `reactLoop` body — from the comment "Separate auto-executed tools from propose_plan" through the existing auto-tools + plan-call dispatch — with the new classifier-based dispatch. Keep the LLM streaming + assistant-message save + iter limit. The actual write/ask_user execution is stubbed in this task; we wire the engine call in Task 3.5.)

- [ ] **Step 1: Find the loop body**

```bash
grep -n "reactLoop\|Separate auto-executed\|tool_call_start\|propose_plan\|createPlanFromToolCall" /Users/irving/repo/learn-helper/backend/internal/handler/ai.go | head -30
```

- [ ] **Step 2: Replace the body**

Find the block from `// Separate auto-executed tools from propose_plan` (around line 555) through the end of the loop (`break reactLoop // Exit loop — wait for user confirmation`, around line 682). Replace it with:

```go
		// Classify tool calls
		var calls []aiToolCall
		for _, tc := range toolCalls {
			calls = append(calls, aiToolCall{Name: tc.Name, ID: tc.ID, Input: tc.Input})
		}
		readBatch, writeBatch, askBatch := classifyToolCalls(calls)

		log.Printf("[ReAct] iteration=%d reads=%d writes=%d asks=%d",
			iteration, len(readBatch), len(writeBatch), len(askBatch))

		// Build assistant turn blocks
		var blocks []ai.ContentBlock
		if respContent != "" {
			blocks = append(blocks, ai.ContentBlock{Type: ai.ContentTypeText, Text: respContent})
		}
		for _, tc := range toolCalls {
			var input json.RawMessage
			if tc.Input != "" {
				input = json.RawMessage(tc.Input)
			}
			blocks = append(blocks, ai.ContentBlock{Type: ai.ContentTypeToolUse, ID: tc.ID, Name: tc.Name, Input: input})
		}
		if assistantContent, err := ai.ContentBlocksToJSON(blocks); err == nil {
			aiMessages = append(aiMessages, ai.Message{Role: "assistant", Content: assistantContent})
		}

		// Execute read tools (auto)
		for _, c := range readBatch {
			log.Printf("[ReAct] read tool: %s", c.Name)
			sseWriteToolCallStart(w, c.ID, c.Name, c.Input, canFlush, flusher)
			result := h.executeReadTool(ctx, c)
			sseWriteToolResult(w, c.ID, c.Name, result, "", canFlush, flusher)
			aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: result, ToolCallID: c.ID})
		}

		// Execute write tools (permission gate, batched)
		if len(writeBatch) > 0 {
			requestID := fmt.Sprintf("perm-%d", time.Now().UnixNano())
			ch := h.permissions.Register(requestID, 1)

			items := make([]PermissionRequestItem, 0, len(writeBatch))
			for _, c := range writeBatch {
				sseWriteToolCallStart(w, c.ID, c.Name, c.Input, canFlush, flusher)
				parsed, _ := parseWriteInput(c.Name, json.RawMessage(c.Input))
				items = append(items, PermissionRequestItem{
					ID:      c.ID,
					Tool:    c.Name,
					Input:   parsed,
					Preview: previewWrite(c.Name, parsed),
				})
			}
			sseWritePermissionRequired(w, PermissionRequest{
				RequestID:      requestID,
				ConversationID: req.ConversationID,
				Items:          items,
			}, canFlush, flusher)

			log.Printf("[ReAct] waiting for permission decisions: %s (n=%d)", requestID, len(writeBatch))
			decisions := <-ch // BLOCKS

			// Index decisions by ID for matching
			decByID := map[string]PermissionDecision{}
			for _, d := range decisions {
				decByID[d.ID] = d
			}

			// Execute approved in order; default-reject any unmatched
			for _, c := range writeBatch {
				dec, ok := decByID[c.ID]
				if !ok {
					dec = PermissionDecision{ID: c.ID, Action: "reject"}
				}

				switch dec.Action {
				case "approve", "edit":
					input := c.Input
					if dec.Action == "edit" && dec.EditedInput != nil {
						b, _ := json.Marshal(dec.EditedInput)
						input = string(b)
					}
					result, execErr := h.executeWriteTool(ctx, c.Name, input, req.FocusPageID)
					if execErr != nil {
						sseWriteToolResult(w, c.ID, c.Name, "", execErr.Error(), canFlush, flusher)
						aiMessages = append(aiMessages, ai.Message{
							Role: "tool", Content: fmt.Sprintf("error: %s", execErr.Error()), ToolCallID: c.ID,
						})
					} else {
						sseWriteToolResult(w, c.ID, c.Name, result, "", canFlush, flusher)
						aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: result, ToolCallID: c.ID})
					}
				default: // reject or unknown
					sseWriteToolResult(w, c.ID, c.Name, "", "rejected by user", canFlush, flusher)
					aiMessages = append(aiMessages, ai.Message{
						Role: "tool", Content: `{"error":"rejected by user"}`, ToolCallID: c.ID,
					})
				}
			}
		}

		// Execute ask_user (one at a time, in order)
		for _, c := range askBatch {
			sseWriteToolCallStart(w, c.ID, c.Name, c.Input, canFlush, flusher)
			requestID := fmt.Sprintf("ask-%d", time.Now().UnixNano())
			ch := h.askUsers.Register(requestID)

			var parsed struct {
				Question      string         `json:"question"`
				Options       []string       `json:"options"`
				Context       *AskUserContext `json:"context,omitempty"`
				MultiSelect   bool           `json:"multi_select"`
				AllowFreeText bool           `json:"allow_free_text"`
				Header        string         `json:"header,omitempty"`
			}
			_ = json.Unmarshal([]byte(c.Input), &parsed)

			sseWriteAskUserRequest(w, AskUserRequest{
				RequestID:      requestID,
				ConversationID: req.ConversationID,
				Question:       parsed.Question,
				Options:        parsed.Options,
				Context:        parsed.Context,
				MultiSelect:    parsed.MultiSelect,
				AllowFreeText:  parsed.AllowFreeText,
				Header:         parsed.Header,
			}, canFlush, flusher)

			log.Printf("[ReAct] waiting for ask_user answer: %s", requestID)
			resp := <-ch // BLOCKS
			answerJSON, _ := json.Marshal(map[string]any{"answer": resp.Answer})
			sseWriteToolResult(w, c.ID, c.Name, string(answerJSON), "", canFlush, flusher)
			aiMessages = append(aiMessages, ai.Message{Role: "tool", Content: string(answerJSON), ToolCallID: c.ID})
		}

		// Accumulate text content (post-tool reasoning, if any)
		if respContent != "" && len(writeBatch) == 0 && len(askBatch) == 0 {
			fullContent.WriteString(respContent)
			fullContent.WriteString("\n\n")
		}
	}
```

- [ ] **Step 3: Add the SSE helper functions**

Append to `backend/internal/handler/ai.go`:

```go
func sseWriteToolCallStart(w http.ResponseWriter, id, name, input string, canFlush bool, flusher http.Flusher) {
	in := json.RawMessage(input)
	if in == nil {
		in = json.RawMessage("{}")
	}
	data, _ := json.Marshal(ToolCallStartEvent{ID: id, Name: name, Input: in})
	sseWrite(w, "tool_call_start", string(data), canFlush, flusher)
}

func sseWriteToolResult(w http.ResponseWriter, id, name, output, errStr string, canFlush bool, flusher http.Flusher) {
	data, _ := json.Marshal(ToolCallResult{
		ID: id, Name: name, Output: output, Error: errStr,
	})
	sseWrite(w, "tool_result", string(data), canFlush, flusher)
}

func sseWritePermissionRequired(w http.ResponseWriter, req PermissionRequest, canFlush bool, flusher http.Flusher) {
	data, _ := json.Marshal(req)
	sseWrite(w, "permission_required", string(data), canFlush, flusher)
}

func sseWriteAskUserRequest(w http.ResponseWriter, req AskUserRequest, canFlush bool, flusher http.Flusher) {
	data, _ := json.Marshal(req)
	sseWrite(w, "ask_user_request", string(data), canFlush, flusher)
}
```

- [ ] **Step 4: Build to find what's missing**

```bash
go build ./... 2>&1 | head -30
```

Expected: build fails with "undefined: h.executeReadTool" / "undefined: h.executeWriteTool" / "undefined: focusID" / etc. We'll wire these in Task 3.5.

(If build succeeds, skip to Step 5. Otherwise note the missing names and proceed.)

- [ ] **Step 5: Commit the loop body change (even if not yet building)**

```bash
git add backend/internal/handler/ai.go
git commit -m "feat(ai): rewrite ReAct loop body with classify + permission gate + ask_user"
```

---

### Task 3.5: Implement executeReadTool / executeWriteTool

**Files:**
- Create: `backend/internal/handler/ai_tools.go`

(Move the existing auto-tool execution logic from the old loop into `executeReadTool`. Add `executeWriteTool` that adapts a single write call to engine logic.)

- [ ] **Step 1: Find existing auto-tool code**

```bash
grep -n "executeAutoTool\|h.executeAuto\|handleLookup\|handleSearch" /Users/irving/repo/learn-helper/backend/internal/handler/ai.go | head -20
```

- [ ] **Step 2: Create ai_tools.go with the implementations**

Create `backend/internal/handler/ai_tools.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"learn-helper/internal/ai"
	"learn-helper/internal/engine"
	"learn-helper/internal/model"
)

// executeReadTool handles the auto-executed read tools.
// Returns a human-readable string that becomes the tool_result content.
func (h *AIHandler) executeReadTool(ctx context.Context, c aiToolCall) string {
	// Backward compat: AIHandler has an existing executeAutoTool that handles
	// lookup_page / read_page / search_pages / websearch / webfetch.
	// Wrap it as an ai.ToolCall.
	tc := ai.ToolCall{ID: c.ID, Name: c.Name, Input: c.Input}
	return h.executeAutoTool(ctx, tc)
}

// executeWriteTool handles one approved write tool call.
// focusPageID is the request's optional focus, used as parent_id fallback for create_page.
func (h *AIHandler) executeWriteTool(ctx context.Context, tool, input string, focusPageID *int64) (string, error) {
	parsed, err := parseWriteInput(tool, json.RawMessage(input))
	if err != nil {
		return "", err
	}

	// FocusPageID fallback for create_page with no parent_id
	if tool == "create_page" && focusPageID != nil {
		if _, hasParent := parsed["parent_id"]; !hasParent {
			parsed["parent_id"] = float64(*focusPageID)
		}
	}

	// Build a one-action ad-hoc plan and dispatch through the engine.
	action := model.PlanAction{
		ID:     c_actionID(tool),
		Type:   tool,
		Params: json.RawMessage(mustMarshal(parsed)),
		Status: "pending",
	}

	// Use the engine's internal exec functions by constructing a temporary plan row.
	// Simpler: call engine methods directly.
	switch tool {
	case "create_page":
		return h.engine.CreatePageFromAction(ctx, action, focusPageID)
	case "update_page":
		return h.engine.UpdatePageFromAction(ctx, action)
	case "patch_page":
		return h.engine.PatchPageFromAction(ctx, action)
	case "delete_page":
		return h.engine.DeletePageFromAction(ctx, action)
	case "link_pages":
		return h.engine.LinkPagesFromAction(ctx, action)
	case "move_page":
		return h.engine.MovePageFromAction(ctx, action)
	default:
		return "", fmt.Errorf("unsupported write tool: %s", tool)
	}
}

func c_actionID(tool string) string {
	return fmt.Sprintf("ad-hoc-%s-%d", tool, timeNow())
}

func mustMarshal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Reference to http.Flusher and json to avoid unused imports if not all are needed.
var (
	_ http.Flusher
	_ json.RawMessage
	_ engine.ExecutionEngine
	_ strings.Builder
)
```

- [ ] **Step 3: Add the engine methods**

In `backend/internal/engine/engine.go`, the per-action execution logic is currently inline inside `ExecutePlan`'s action loop (a switch on action type). First extract the six per-action cases into standalone engine methods so the new permission gate can call them directly. If the per-action cases are already extracted as helpers (e.g. `execCreatePage`), skip the extraction and just add the `*FromAction` wrappers below.

Names to use for extracted helpers (if not already present):

```go
func (e *ExecutionEngine) execCreatePage(ctx context.Context, p map[string]any) (int64, error) // returns new page id
func (e *ExecutionEngine) execUpdatePage(ctx context.Context, p map[string]any) error
func (e *ExecutionEngine) execPatchPage(ctx context.Context, p map[string]any) error
func (e *ExecutionEngine) execDeletePage(ctx context.Context, p map[string]any) error
func (e *ExecutionEngine) execLinkPages(ctx context.Context, p map[string]any) error
func (e *ExecutionEngine) execMovePage(ctx context.Context, p map[string]any) error
```

Then add six thin `*FromAction` wrappers:

```go
// CreatePageFromAction executes a single create_page action.
// The action's Params must have been validated.
func (e *ExecutionEngine) CreatePageFromAction(ctx context.Context, a model.PlanAction, focusPageID *int64) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", err
	}
	id, err := e.execCreatePage(ctx, p)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"page_id":%d}`, id), nil
}

func (e *ExecutionEngine) UpdatePageFromAction(ctx context.Context, a model.PlanAction) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", err
	}
	if err := e.execUpdatePage(ctx, p); err != nil {
		return "", err
	}
	return `{"status":"updated"}`, nil
}

func (e *ExecutionEngine) PatchPageFromAction(ctx context.Context, a model.PlanAction) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", err
	}
	if err := e.execPatchPage(ctx, p); err != nil {
		return "", err
	}
	return `{"status":"patched"}`, nil
}

func (e *ExecutionEngine) DeletePageFromAction(ctx context.Context, a model.PlanAction) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", err
	}
	if err := e.execDeletePage(ctx, p); err != nil {
		return "", err
	}
	return `{"status":"deleted"}`, nil
}

func (e *ExecutionEngine) LinkPagesFromAction(ctx context.Context, a model.PlanAction) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", err
	}
	if err := e.execLinkPages(ctx, p); err != nil {
		return "", err
	}
	return `{"status":"linked"}`, nil
}

func (e *ExecutionEngine) MovePageFromAction(ctx context.Context, a model.PlanAction) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", err
	}
	if err := e.execMovePage(ctx, p); err != nil {
		return "", err
	}
	return `{"status":"moved"}`, nil
}
```

Ensure `ExecutePlan` still calls the extracted helpers (or its existing per-action path). Behavior of the existing flow must not change, even though `/api/plans/*` endpoints are being deleted.

- [ ] **Step 4: Add timeNow helper**

In `backend/internal/handler/ai_tools.go`, add:

```go
import "time"

func timeNow() int64 { return time.Now().UnixNano() }
```

- [ ] **Step 5: Build & test**

```bash
go build ./... 2>&1 | head -30
go test ./internal/handler/ ./internal/engine/ -v
```

Expected: build OK, all green.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler/ai_tools.go backend/internal/engine/engine.go
git commit -m "feat(ai): add per-tool engine entrypoints for permission-gated writes"
```

---

### Task 3.6: Delete old propose_plan handler code

**Files:**
- Modify: `backend/internal/handler/ai.go`
- Modify: `backend/internal/engine/engine.go`

- [ ] **Step 1: Delete createPlanFromToolCall and helpers**

In `backend/internal/handler/ai.go`, find and remove:
- `createPlanFromToolCall` function (the whole body, around line 904-1105)
- `cleanProposalJSON` function (around line 1111+)
- `extractFirstJSON` (if no longer used elsewhere)
- `countBraces` (if no longer used)
- Any other propose_plan helpers (search for `func` declarations)

```bash
grep -n "^func" /Users/irving/repo/learn-helper/backend/internal/handler/ai.go
```

For each function that is now unused, delete it.

- [ ] **Step 2: Delete stage inference in engine**

In `backend/internal/engine/engine.go`, find and remove:
- `inferPlanStage` function
- `validatePersistedPlanStage` function
- `planProposalLite` / `planActionLite` types
- `PlanProposalLite` / `InferPlanStage` / `AppendAction` exports
- `MaxMainStageCreatePages` / `MaxOutlineStageNodes` / `MaxContentStageMutations` constants
- `StageMain` / `StageOutline` / `StageContent` constants
- `isEmptyJSONArray` / `countOutlineNodes` helpers
- The `stage` field in any `model.Plan` references (if model.Plan is deleted in Phase 5, this is moot)

```bash
grep -n "inferPlanStage\|MaxMainStage\|MaxOutlineStage\|MaxContentStage\|StageMain\|StageOutline\|StageContent" /Users/irving/repo/learn-helper/backend/internal/engine/engine.go
```

Remove every match and its enclosing function.

- [ ] **Step 3: Remove `createPlanFromToolCall` call site**

In `backend/internal/handler/ai.go`, the ReAct loop in Task 3.4 should not call `createPlanFromToolCall` anymore (it was replaced by classify + permission gate). Verify with:

```bash
grep -n "createPlanFromToolCall" /Users/irving/repo/learn-helper/backend
```

Expected: 0 matches.

- [ ] **Step 4: Build & test**

```bash
go build ./... 2>&1 | head -50
```

Iterate on errors. Likely culprits:
- Missing `engine.ExecutionReport` import
- Tests still referencing deleted functions

If a test fails, either delete it (it's testing deleted code) or update it.

- [ ] **Step 5: Commit**

```bash
git add -u
git commit -m "refactor(ai): remove propose_plan handler and stage inference"
```

---

### Phase 3 verification

```bash
go build ./... && go test ./...
```

Expected: all green. The ReAct loop is now using the new tool dispatch. LLM calls `propose_plan` → no match in classify → routed to writeBatch → write tool name not recognized → parse error "unknown write tool: propose_plan" → graceful failure. The migration to drop old data happens in Phase 4.

---

## Phase 4: Persistence migration

Goal: SQL migration drops `plans` and `plan_actions`. Go code references fully removed.

### Task 4.1: SQL migration

**Files:**
- Create: `backend/db/migrations/012_drop_plans_tables.sql`

- [ ] **Step 1: Write migration**

```sql
-- Drop plans / plan_actions tables. Tool migration happened in code; we don't
-- carry any history forward (user is single-tenant, old pending plans are discarded).
DROP TABLE IF EXISTS plan_actions;
DROP TABLE IF EXISTS plans;
```

- [ ] **Step 2: Register migration in main.go**

Find the migration loader in `backend/cmd/server/main.go` (search for `embed` or `migrations` or the file glob that loads them). Confirm the loader picks up files in `db/migrations/` automatically (it should — the existing pattern). If not, add `//go:embed` directive for the new file.

- [ ] **Step 3: Verify load**

```bash
cd /Users/irving/repo/learn-helper/backend && go build ./cmd/server && ./server --help 2>&1 | head -5
```

(Or just start it briefly and check it boots. The migration runs on startup.)

- [ ] **Step 4: Commit**

```bash
git add backend/db/migrations/012_drop_plans_tables.sql backend/cmd/server/main.go
git commit -m "feat(db): drop plans/plan_actions tables"
```

---

### Task 4.2: Remove model.Plan / model.PlanAction

**Files:**
- Modify: `backend/internal/model/models.go`

- [ ] **Step 1: Find references**

```bash
grep -rn "model\.Plan\b\|model\.PlanAction" /Users/irving/repo/learn-helper/backend
```

- [ ] **Step 2: Delete types**

In `models.go`, remove the `Plan` and `PlanAction` struct definitions (lines 78-105).

- [ ] **Step 3: Fix compile errors**

For each remaining reference:
- If it's an `engine.go` field/method, replace the field with the new design (e.g., `Plan` parameter → `model.PlanAction` parameter)
- If it's a `plan.go` reference, that file is being deleted in Task 4.3
- If it's in `ai_tools.go` from Task 3.5, the `model.PlanAction` reference there is OK (we keep the type for the action shape, just not the Plan parent)

- [ ] **Step 4: Build & fix errors**

```bash
go build ./... 2>&1 | head -50
```

Iterate.

- [ ] **Step 5: Commit**

```bash
git add -u
git commit -m "refactor(model): remove Plan and PlanAction types"
```

---

### Task 4.3: Delete plan.go handler

**Files:**
- Delete: `backend/internal/handler/plan.go`

- [ ] **Step 1: Check for remaining imports**

```bash
grep -rn "handler\.PlanHandler\|NewPlanHandler" /Users/irving/repo/learn-helper/backend
```

- [ ] **Step 2: Delete file**

```bash
rm /Users/irving/repo/learn-helper/backend/internal/handler/plan.go
```

- [ ] **Step 3: Fix any remaining references**

If any other file imported `NewPlanHandler`, find the call sites and remove them. Likely it's only in `ai.go` inside `createPlanFromToolCall` (already deleted in Phase 3).

- [ ] **Step 4: Build & verify**

```bash
go build ./...
```

Expected: build OK.

- [ ] **Step 5: Commit**

```bash
git add -u
git commit -m "refactor(ai): delete plan.go handler (no longer needed)"
```

---

### Task 4.4: Remove /api/plans/* routes

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Find route registration**

```bash
grep -n "/api/plans\|HandleFunc.*plan" /Users/irving/repo/learn-helper/backend/cmd/server/main.go
```

- [ ] **Step 2: Remove plan routes**

Delete the lines registering `/api/plans/confirm`, `/api/plans/reject`, etc. (Search-and-delete; the lines should be a contiguous block.)

- [ ] **Step 3: Add new routes**

Find where AI routes are registered (e.g., `/api/ai/chat`). Add right after:

```go
mux.HandleFunc("/api/ai/permission_response", aiHandler.HandlePermissionResponse)
mux.HandleFunc("/api/ai/ask_user_response", aiHandler.HandleAskUserResponse)
```

- [ ] **Step 4: Build & test**

```bash
go build ./... && go test ./...
```

Expected: build OK.

- [ ] **Step 5: Smoke test the new routes**

Start the server:

```bash
go run ./cmd/server &
SERVER_PID=$!
sleep 2

# Should return 200 with {"status":"ok"}
curl -s -X POST http://localhost:8080/api/ai/permission_response \
  -H "Content-Type: application/json" \
  -d '{"request_id":"unknown","decisions":[]}'

# Should return 400
curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/api/ai/ask_user_response \
  -H "Content-Type: application/json" \
  -d '{"request_id":""}'

# /api/plans/confirm should be 404 or 405
curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/api/plans/confirm

kill $SERVER_PID
```

Expected: 200 / 400 / 404.

- [ ] **Step 6: Commit**

```bash
git add -u
git commit -m "refactor(api): drop /api/plans/* routes, add /api/ai/permission+ask_user routes"
```

---

### Phase 4 verification

```bash
go build ./... && go test ./...
```

Expected: clean. The DB still has `plans` and `plan_actions` tables from before the migration — they get dropped on next server start.

---

## Phase 5: Frontend types and events

Goal: frontend type definitions updated. Old `Plan` types removed. New event types added.

### Task 5.1: Update types/index.ts

**Files:**
- Modify: `frontend/src/types/index.ts`

- [ ] **Step 1: Find existing plan/AI types**

```bash
grep -n "Plan\|PlanAction\|ToolCall\|OutlineNode" /Users/irving/repo/learn-helper/frontend/src/types/index.ts
```

- [ ] **Step 2: Remove plan types**

Delete the `Plan`, `PlanAction`, `OutlineNode` interface declarations (they are no longer used). Also remove any other plan-related types.

- [ ] **Step 3: Add new event types**

Add the following interfaces:

```typescript
export interface PermissionRequestItem {
  id: string;
  tool: string;
  input: Record<string, any>;
  preview: string;
}

export interface PermissionRequestEvent {
  request_id: string;
  conversation_id: number;
  items: PermissionRequestItem[];
}

export interface AskUserContext {
  kind: "outline" | "page" | "markdown" | "diff";
  data: any;
}

export interface AskUserRequestEvent {
  request_id: string;
  conversation_id: number;
  question: string;
  options: string[];
  context?: AskUserContext;
  multi_select: boolean;
  allow_free_text: boolean;
  header?: string;
}

export interface PermissionDecisionInput {
  id: string;
  action: "approve" | "reject" | "edit";
  edited_input?: Record<string, any>;
}
```

- [ ] **Step 4: Verify TypeScript compiles**

```bash
cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit
```

Expected: errors are OK at this stage (we haven't updated consumers yet). Note: don't fix errors here; do them in Phase 6.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types/index.ts
git commit -m "refactor(frontend): drop Plan types, add permission/ask_user event types"
```

---

### Task 5.2: Diff utility

**Files:**
- Create: `frontend/src/lib/diff.ts`

- [ ] **Step 1: Write the utility**

```typescript
// Tiny line-level inline diff. Returns hunks of {type, text} where type is
// "context" | "add" | "del". For display, render add in green, del in red.
//
// This is intentionally simple — not a Myers diff. Good enough for the
// inline diff view in the right panel.
export type DiffLine = { type: "context" | "add" | "del"; text: string };

export function inlineDiff(before: string, after: string): DiffLine[] {
  const a = before.split("\n");
  const b = after.split("\n");
  const out: DiffLine[] = [];

  // Simple LCS-based line diff. O(n*m) — fine for pages up to a few thousand lines.
  const m = a.length, n = b.length;
  const lcs: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
  for (let i = m - 1; i >= 0; i--) {
    for (let j = n - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
    }
  }

  let i = 0, j = 0;
  while (i < m && j < n) {
    if (a[i] === b[j]) {
      out.push({ type: "context", text: a[i] });
      i++; j++;
    } else if (lcs[i + 1][j] >= lcs[i][j + 1]) {
      out.push({ type: "del", text: a[i] });
      i++;
    } else {
      out.push({ type: "add", text: b[j] });
      j++;
    }
  }
  while (i < m) { out.push({ type: "del", text: a[i++] }); }
  while (j < n) { out.push({ type: "add", text: b[j++] }); }
  return out;
}

export function diffSummary(before: string, after: string): { added: number; removed: number } {
  const lines = inlineDiff(before, after);
  return {
    added: lines.filter(l => l.type === "add").length,
    removed: lines.filter(l => l.type === "del").length,
  };
}
```

- [ ] **Step 2: Verify it compiles**

```bash
npx tsc --noEmit
```

Expected: no new errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/diff.ts
git commit -m "feat(frontend): add inline diff utility"
```

---

### Phase 5 verification

```bash
cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit
```

Note: there will be TypeScript errors about `PlanPreview` and other consumers. That's expected — Phase 6 fixes them.

---

## Phase 6: Frontend components

Goal: new PermissionQueue, AskUserCard, AskUserContext components. ToolCallCard with pending state.

### Task 6.1: PermissionQueue component

**Files:**
- Create: `frontend/src/components/PermissionQueue.tsx`

- [ ] **Step 1: Write the component**

```tsx
import { useState } from "react";
import type { PermissionRequestEvent, PermissionDecisionInput } from "../types";

interface Props {
  request: PermissionRequestEvent | null;
  onResolve: (decisions: PermissionDecisionInput[]) => void;
}

export function PermissionQueue({ request, onResolve }: Props) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<string | null>(null);
  const [editText, setEditText] = useState("");

  if (!request) {
    return (
      <div className="text-sm text-th-text-muted p-3">
        当前没有待批准的操作
      </div>
    );
  }

  const items = request.items;
  const allSelected = selected.size === items.length;
  const noneSelected = selected.size === 0;

  function toggle(id: string) {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setSelected(next);
  }

  function submit(action: "approve" | "reject") {
    const targets = noneSelected ? items.map(i => i.id) : Array.from(selected);
    onResolve(targets.map(id => ({ id, action })));
    setSelected(new Set());
    setEditing(null);
  }

  function startEdit(id: string) {
    const item = items.find(i => i.id === id);
    if (!item) return;
    setEditing(id);
    setEditText(JSON.stringify(item.input, null, 2));
  }

  function saveEdit(id: string) {
    let parsed: any;
    try {
      parsed = JSON.parse(editText);
    } catch {
      alert("JSON 解析失败");
      return;
    }
    onResolve([{ id, action: "edit", edited_input: parsed }]);
    setEditing(null);
    setEditText("");
  }

  return (
    <div className="rounded-lg border border-th-border bg-th-bg-secondary p-3">
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-medium text-th-text-primary">
          待批准 ({items.length})
        </h3>
        <div className="flex gap-2">
          <button
            onClick={() => setSelected(allSelected ? new Set() : new Set(items.map(i => i.id)))}
            className="text-xs px-2 py-1 rounded border border-th-border text-th-text-secondary hover:bg-th-bg-tertiary"
          >
            {allSelected ? "全不选" : "全选"}
          </button>
          <button
            onClick={() => submit("approve")}
            className="text-xs px-2 py-1 rounded bg-th-accent text-white hover:opacity-90"
          >
            全部批准
          </button>
          <button
            onClick={() => submit("reject")}
            className="text-xs px-2 py-1 rounded border border-th-border text-th-text-secondary hover:bg-th-bg-tertiary"
          >
            全部拒绝
          </button>
        </div>
      </div>

      <ul className="space-y-2">
        {items.map(item => (
          <li key={item.id} className="text-sm border border-th-border rounded p-2">
            {editing === item.id ? (
              <div>
                <textarea
                  className="w-full h-32 font-mono text-xs p-2 rounded border border-th-border bg-th-bg-primary"
                  value={editText}
                  onChange={e => setEditText(e.target.value)}
                />
                <div className="flex gap-2 mt-2">
                  <button onClick={() => saveEdit(item.id)} className="text-xs px-2 py-1 rounded bg-th-accent text-white">
                    保存并批准
                  </button>
                  <button onClick={() => setEditing(null)} className="text-xs px-2 py-1 rounded border border-th-border">
                    取消
                  </button>
                </div>
              </div>
            ) : (
              <div className="flex items-start gap-2">
                <input
                  type="checkbox"
                  checked={selected.has(item.id)}
                  onChange={() => toggle(item.id)}
                  className="mt-1"
                />
                <div className="flex-1">
                  <div className="font-mono text-xs text-th-text-muted">{item.tool}</div>
                  <div className="text-th-text-primary">{item.preview}</div>
                </div>
                <button
                  onClick={() => startEdit(item.id)}
                  className="text-xs text-th-text-muted hover:text-th-text-primary"
                >
                  编辑
                </button>
              </div>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
```

- [ ] **Step 2: Verify it compiles**

```bash
npx tsc --noEmit
```

Expected: no new errors related to this file.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/PermissionQueue.tsx
git commit -m "feat(frontend): add PermissionQueue component"
```

---

### Task 6.2: AskUserCard component

**Files:**
- Create: `frontend/src/components/AskUserCard.tsx`

- [ ] **Step 1: Write the component**

```tsx
import { useState } from "react";
import type { AskUserRequestEvent } from "../types";

interface Props {
  request: AskUserRequestEvent;
  onAnswer: (answer: string | string[] | "no_answer") => void;
}

export function AskUserCard({ request, onAnswer }: Props) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [freeText, setFreeText] = useState("");

  function toggle(opt: string) {
    if (request.multi_select) {
      const next = new Set(selected);
      if (next.has(opt)) next.delete(opt);
      else next.add(opt);
      setSelected(next);
    } else {
      onAnswer(opt);
    }
  }

  function submitMulti() {
    if (selected.size > 0) {
      onAnswer(Array.from(selected));
    }
  }

  function submitFreeText() {
    const t = freeText.trim();
    if (t) onAnswer(t);
  }

  return (
    <div className="rounded-lg border-l-4 border-blue-500 bg-blue-50 dark:bg-blue-950 p-3 my-2">
      {request.header && (
        <div className="text-[10px] uppercase tracking-wide text-blue-700 dark:text-blue-300 mb-1">
          {request.header}
        </div>
      )}
      <div className="text-sm text-th-text-primary font-medium mb-2">
        {request.question}
      </div>
      <div className="flex flex-wrap gap-2">
        {request.options.map(opt => (
          <button
            key={opt}
            onClick={() => toggle(opt)}
            className={`text-sm px-3 py-1 rounded border ${
              selected.has(opt)
                ? "border-blue-500 bg-blue-100 dark:bg-blue-900"
                : "border-th-border hover:bg-th-bg-tertiary"
            }`}
          >
            {opt}
          </button>
        ))}
      </div>
      {request.multi_select && selected.size > 0 && (
        <button
          onClick={submitMulti}
          className="mt-2 text-xs px-2 py-1 rounded bg-blue-500 text-white"
        >
          确认 ({selected.size})
        </button>
      )}
      {request.allow_free_text && (
        <div className="mt-2 flex gap-2">
          <input
            type="text"
            value={freeText}
            onChange={e => setFreeText(e.target.value)}
            placeholder="其它想法..."
            className="flex-1 text-sm px-2 py-1 rounded border border-th-border bg-th-bg-primary"
            onKeyDown={e => e.key === "Enter" && submitFreeText()}
          />
          <button
            onClick={submitFreeText}
            disabled={!freeText.trim()}
            className="text-xs px-2 py-1 rounded border border-th-border disabled:opacity-50"
          >
            发送
          </button>
        </div>
      )}
      <button
        onClick={() => onAnswer("no_answer")}
        className="mt-2 text-xs text-th-text-muted hover:text-th-text-primary"
      >
        跳过
      </button>
    </div>
  );
}
```

- [ ] **Step 2: Verify it compiles**

```bash
npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/AskUserCard.tsx
git commit -m "feat(frontend): add AskUserCard component"
```

---

### Task 6.3: AskUserContext component (4 kinds)

**Files:**
- Create: `frontend/src/components/AskUserContext.tsx`

- [ ] **Step 1: Write the component**

```tsx
import { useState } from "react";
import { inlineDiff, type DiffLine } from "../lib/diff";
import { MarkdownContent } from "./MarkdownContent";
import type { AskUserContext as AskUserContextT } from "../types";

interface OutlineNode {
  id?: string;
  title: string;
  page_type?: string;
  children?: OutlineNode[];
}

interface DiffEntry {
  page_id: number;
  before: string;
  after: string;
  label?: string;
}

interface Props {
  context: AskUserContextT;
}

function OutlineTree({ nodes, depth = 0 }: { nodes: OutlineNode[]; depth?: number }) {
  return (
    <ul className="text-sm">
      {nodes.map((n, i) => (
        <li key={n.id ?? `${depth}-${i}`} className="py-0.5" style={{ paddingLeft: depth * 16 }}>
          <span className="font-medium">{n.title}</span>
          {n.page_type && <span className="ml-2 text-xs text-th-text-muted">({n.page_type})</span>}
          {n.children && n.children.length > 0 && <OutlineTree nodes={n.children} depth={depth + 1} />}
        </li>
      ))}
    </ul>
  );
}

function DiffView({ diffs }: { diffs: DiffEntry[] }) {
  const [active, setActive] = useState(0);
  const d = diffs[active];
  const lines: DiffLine[] = inlineDiff(d.before, d.after);
  return (
    <div>
      {diffs.length > 1 && (
        <div className="flex gap-1 mb-2 border-b border-th-border">
          {diffs.map((x, i) => (
            <button
              key={x.page_id}
              onClick={() => setActive(i)}
              className={`text-xs px-2 py-1 ${
                i === active ? "border-b-2 border-th-accent" : "text-th-text-muted"
              }`}
            >
              {x.label ?? `Page ${x.page_id}`}
            </button>
          ))}
        </div>
      )}
      <pre className="text-xs font-mono whitespace-pre-wrap max-h-96 overflow-y-auto">
        {lines.map((l, i) => (
          <div
            key={i}
            className={
              l.type === "add"
                ? "bg-green-100 dark:bg-green-950 text-green-900 dark:text-green-100"
                : l.type === "del"
                ? "bg-red-100 dark:bg-red-950 text-red-900 dark:text-red-100 line-through"
                : "text-th-text-muted"
            }
          >
            {l.type === "add" ? "+ " : l.type === "del" ? "- " : "  "}{l.text || " "}
          </div>
        ))}
      </pre>
    </div>
  );
}

function PagePreview({ pageId }: { pageId: number }) {
  // Lazy fetch: simple SWR-style. For single-user, a useEffect is fine.
  const [content, setContent] = useState<string | null>(null);
  if (content === null) {
    fetch(`/api/pages/${pageId}`)
      .then(r => r.ok ? r.json() : Promise.reject(r.status))
      .then((p: any) => setContent(p.content ?? ""))
      .catch(() => setContent("(加载失败)"));
  }
  return (
    <div className="text-sm">
      {content === null ? <span className="text-th-text-muted">加载中...</span> : (
        <MarkdownContent content={content.slice(0, 500) + (content.length > 500 ? "..." : "")} />
      )}
    </div>
  );
}

export function AskUserContextView({ context }: Props) {
  switch (context.kind) {
    case "outline":
      return <OutlineTree nodes={context.data as OutlineNode[]} />;
    case "markdown":
      return <MarkdownContent content={context.data as string} />;
    case "diff":
      return <DiffView diffs={context.data as DiffEntry[]} />;
    case "page":
      return <PagePreview pageId={(context.data as { page_id: number }).page_id} />;
    default:
      return <pre className="text-xs">{JSON.stringify(context, null, 2)}</pre>;
  }
}
```

- [ ] **Step 2: Verify it compiles**

```bash
npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/AskUserContext.tsx
git commit -m "feat(frontend): add AskUserContext (outline/page/markdown/diff)"
```

---

### Task 6.4: ToolCallCard pending state

**Files:**
- Modify: `frontend/src/components/ToolCallCard.tsx`

- [ ] **Step 1: Find existing state types**

```bash
grep -n "status\|state" /Users/irving/repo/learn-helper/frontend/src/components/ToolCallCard.tsx | head -20
```

- [ ] **Step 2: Add pending state**

Find the component's status enum / state variable. Add `pending` as a value, with a visual treatment (semi-transparent + spinner).

- [ ] **Step 3: Wire up in parent**

Find where ToolCallCard receives its status. In Phase 7, ChatPanel will pass `pending` for write tools that are in the queue. For now, just ensure the prop accepts the value.

- [ ] **Step 4: Verify it compiles**

```bash
npx tsc --noEmit
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/ToolCallCard.tsx
git commit -m "feat(frontend): add pending state to ToolCallCard"
```

---

### Phase 6 verification

```bash
npx tsc --noEmit 2>&1 | head -40
```

Expected: still some errors about PlanPreview / WikiPage / ChatPanel. Those are fixed in Phase 7.

---

## Phase 7: Frontend integration

Goal: ChatPanel handles new events. WikiPage replaces PlanPreview. Old `PlanPreview` deleted.

### Task 7.1: ChatPanel handles permission + ask_user events

**Files:**
- Modify: `frontend/src/components/ChatPanel.tsx`

- [ ] **Step 1: Find event handler**

```bash
grep -n "sse\|EventSource\|onMessage\|tool_call_start\|tool_result" /Users/irving/repo/learn-helper/frontend/src/components/ChatPanel.tsx | head -20
```

- [ ] **Step 2: Add state for pending events**

Add to component state:

```tsx
const [permissionRequest, setPermissionRequest] = useState<PermissionRequestEvent | null>(null);
const [askUserRequest, setAskUserRequest] = useState<AskUserRequestEvent | null>(null);
```

- [ ] **Step 3: Add SSE event handlers**

In the SSE event switch (whatever structure the code uses), add cases for:

```tsx
case "permission_required":
  setPermissionRequest(event.payload);
  break;
case "ask_user_request":
  setAskUserRequest(event.payload);
  break;
case "permission_resolved":
case "ask_user_resolved":
  // Clear local state if it matches
  if (permissionRequest?.request_id === event.payload.request_id) {
    setPermissionRequest(null);
  }
  if (askUserRequest?.request_id === event.payload.request_id) {
    setAskUserRequest(null);
  }
  break;
```

- [ ] **Step 4: Wire up response senders**

Add handlers:

```tsx
async function respondToPermission(decisions: PermissionDecisionInput[]) {
  if (!permissionRequest) return;
  await fetch("/api/ai/permission_response", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      request_id: permissionRequest.request_id,
      decisions,
    }),
  });
  setPermissionRequest(null);
}

async function respondToAskUser(answer: string | string[] | "no_answer") {
  if (!askUserRequest) return;
  await fetch("/api/ai/ask_user_response", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      request_id: askUserRequest.request_id,
      answer,
    }),
  });
  setAskUserRequest(null);
}
```

- [ ] **Step 5: Render new components**

Where the ChatPanel renders messages, add:

```tsx
{askUserRequest && (
  <AskUserCard request={askUserRequest} onAnswer={respondToAskUser} />
)}
{askUserRequest?.context && (
  <AskUserContextView context={askUserRequest.context} />
)}
```

(Place this near the message list.)

In the right panel area (if ChatPanel owns it) or wherever PermissionQueue goes:

```tsx
<PermissionQueue request={permissionRequest} onResolve={respondToPermission} />
```

(If the right panel is owned by a different component, see Task 7.2.)

- [ ] **Step 6: Verify it compiles**

```bash
npx tsc --noEmit
```

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/ChatPanel.tsx
git commit -m "feat(frontend): handle permission/ask_user events in ChatPanel"
```

---

### Task 7.2: Replace PlanPreview usage

**Files:**
- Modify: `frontend/src/components/WikiPage.tsx` (and any other file using PlanPreview)
- Delete: `frontend/src/components/PlanPreview.tsx`

- [ ] **Step 1: Find PlanPreview usage**

```bash
grep -rn "PlanPreview" /Users/irving/repo/learn-helper/frontend
```

- [ ] **Step 2: Update each usage site**

For each file using `<PlanPreview ... />`:
- Remove the import
- Remove the JSX usage
- (The right panel it lived in now shows `<PermissionQueue />` — likely ChatPanel already covers this; if not, add it there)

- [ ] **Step 3: Delete PlanPreview.tsx**

```bash
rm /Users/irving/repo/learn-helper/frontend/src/components/PlanPreview.tsx
```

- [ ] **Step 4: Verify build**

```bash
cd /Users/irving/repo/learn-helper/frontend && npm run build
```

Expected: build OK, no missing references.

- [ ] **Step 5: Commit**

```bash
git add -u
git commit -m "refactor(frontend): remove PlanPreview, route through ChatPanel"
```

---

### Phase 7 verification

```bash
cd /Users/irving/repo/learn-helper/frontend && npm run build
```

Expected: clean build. No `Plan` / `PlanAction` / `PlanPreview` references.

---

## Phase 8: Archive old specs

Goal: move obsolete specs to archive directory.

### Task 8.1: Archive old plan-related specs

**Files:**
- Move: `openspec/specs/ai-propose-plan-contract/spec.md` → `openspec/specs/archive/ai-propose-plan-contract/spec.md`
- Move: `openspec/specs/plan-execution-semantics/spec.md` → `openspec/specs/archive/plan-execution-semantics/spec.md`

- [ ] **Step 1: Create archive dirs**

```bash
mkdir -p /Users/irving/repo/learn-helper/openspec/specs/archive/ai-propose-plan-contract
mkdir -p /Users/irving/repo/learn-helper/openspec/specs/archive/plan-execution-semantics
```

- [ ] **Step 2: Move specs**

```bash
git mv openspec/specs/ai-propose-plan-contract/spec.md openspec/specs/archive/ai-propose-plan-contract/spec.md
git mv openspec/specs/plan-execution-semantics/spec.md openspec/specs/archive/plan-execution-semantics/spec.md
```

- [ ] **Step 3: Add a note at the top of each**

Edit each file to prepend:

```markdown
> **ARCHIVED 2026-06-02** — superseded by the plan tool redesign. See `docs/superpowers/specs/2026-06-02-plan-tool-redesign-design.md`.

```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "docs(spec): archive propose-plan-contract and plan-execution-semantics"
```

---

## Phase 9: End-to-end verification

Goal: manually walk through the spec's acceptance criteria.

### Task 9.1: Manual smoke tests

- [ ] **Step 1: Start the server**

```bash
cd /Users/irving/repo/learn-helper/backend && go run ./cmd/server &
sleep 3
```

- [ ] **Step 2: Start the frontend**

```bash
cd /Users/irving/repo/learn-helper/frontend && npm run dev &
sleep 5
```

- [ ] **Step 3: Test 1 — read tools auto-execute**

Open `http://localhost:3000`, ask the AI "what pages do I have?". Verify:
- AI uses `search_pages` or `lookup_page` automatically
- No permission prompt appears
- Tool cards show in chat

- [ ] **Step 4: Test 2 — single write tool**

Ask the AI to "create a page called Test under the Go node". Verify:
- Right panel shows `待批准 (1)` with the create_page op
- Approve → page is created
- AI gets `{"page_id": N}` as tool_result

- [ ] **Step 5: Test 3 — batched writes**

Ask the AI to "create pages A, B, C under Go". Verify:
- Right panel shows 3 items in one batch
- Approve all → all 3 created
- AI receives 3 separate tool_result messages

- [ ] **Step 6: Test 4 — reject**

Same as Test 3, but click Reject. Verify:
- AI receives `{"error":"rejected by user"}` 3 times
- AI responds conversationally (no panic, no infinite loop)

- [ ] **Step 7: Test 5 — ask_user with context**

Ask the AI "what should I learn about Go?". Verify:
- ask_user card appears in chat with question + options
- Right panel shows the ask_user context (probably empty in this case)
- User picks option → AI gets the answer

- [ ] **Step 8: Test 6 — ask_user with diff context**

Ask the AI to "rewrite the introduction of Go page to be more concise". Verify:
- AI calls read_page, then ask_user with diff context
- Right panel shows inline diff (green/red)
- User approves the approach
- AI submits update_page

- [ ] **Step 9: Test 7 — SSE disconnect**

Open a chat, trigger a permission prompt, then close the browser tab. Verify:
- Server logs show `permissions.CancelAll()` was called
- No zombie state

- [ ] **Step 10: Test 8 — old propose_plan no longer works**

In dev tools, manually trigger a chat with a fake `propose_plan` tool call (or ask the AI to call it explicitly). Verify:
- AI doesn't have propose_plan in its tool list
- If somehow it tries, backend returns a graceful error

- [ ] **Step 11: Test 9 — knowledge map unaffected**

Open the knowledge map view. Verify pages still load, links still work, search still works.

- [ ] **Step 12: Test 10 — historical messages render**

Open an old conversation that was created before the migration. Verify:
- Messages load
- Any `propose_plan` tool call cards render (or are filtered out cleanly)

- [ ] **Step 13: Stop servers**

```bash
pkill -f "go run ./cmd/server" || true
pkill -f "vite" || true
```

- [ ] **Step 14: Commit any test artifacts**

If you added any test fixtures or scripts, commit them.

```bash
git add -A
git commit -m "test: manual smoke test artifacts" || echo "nothing to commit"
```

---

## Final verification

Run the full suite:

```bash
cd /Users/irving/repo/learn-helper/backend && go test ./... && go build ./...
cd /Users/irving/repo/learn-helper/frontend && npm run build
```

Expected: all green.

Sanity check the spec's acceptance criteria one by one against the manual tests above.

---

## Rollback

If something goes wrong after migration:

1. Stop the new server
2. Restore the pre-migration SQLite: `cp learn-helper-pre-permission-gate.db learn-helper.db`
3. Revert the migration: `git revert` the migration commit
4. Restart the old server (built from a previous commit)

No automated rollback is provided. The user is single-tenant, so the cost of manual recovery is acceptable.
