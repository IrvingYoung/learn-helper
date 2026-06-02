# AI Agent Skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a SKILL.md-based skill system to the wiki AI agent, where users trigger skills via `/<name> [args]` and the skill body is appended to the system prompt for that one request. Format is compatible with Claude Code skills.

**Architecture:** New `internal/ai/skills` package loads `SKILL.md` files from `embed.FS` (or `os.DirFS` when `LH_SKILLS_DIR` is set) at startup into a `Registry`. `BuildChatSystemPrompt` appends the skill body when present. `POST /api/ai/chat` accepts an optional `skill` field; `GET /api/skills` exposes the catalog. Frontend parses `/<name>` in the input box, sends `skill` in the request, and shows a `/` autocomplete panel.

**Tech Stack:** Go 1.25 + chi + yaml.v3, React 19 + SWR, `embed.FS`

---

## File Structure

**New backend files:**
- `backend/internal/ai/skills/skill.go` — Skill struct, Registry, error types
- `backend/internal/ai/skills/loader.go` — `LoadFromFS(fs.FS) (*Registry, error)`, frontmatter parsing
- `backend/internal/ai/skills/loader_test.go` — TDD tests for loader
- `backend/internal/ai/skills/embed.go` — `//go:embed *.md`, exported `embedFS`
- `backend/internal/ai/skills/explain-page.md` — example skill
- `backend/internal/handler/skills.go` — `GET /api/skills` handler
- `backend/internal/handler/skills_test.go` — handler tests using `httptest`

**Modified backend files:**
- `backend/go.mod` / `backend/go.sum` — add `gopkg.in/yaml.v3`
- `backend/internal/ai/provider.go` — add `BuildChatSystemPrompt`
- `backend/internal/handler/ai.go` — read `req.Skill`, look up in registry, pass to prompt builder; return 400 for unknown skill
- `backend/cmd/server/main.go` — load registry, register route, pass registry to AIHandler

**Modified frontend files:**
- `frontend/src/lib/api.ts` — add `skill?: string` to `ChatRequest`, add `fetchSkills()` helper
- `frontend/src/components/ChatPanel.tsx` — parse `/<name>` in `handleSend`, add `/` panel UI

**Unchanged** (per spec):
- `internal/handler/ai_react.go` — ReAct loop unaware of skills
- `internal/handler/cron/*` and `internal/ai/cron_prompt.go` — cron mode doesn't support skills
- `WikiTools()` — skills don't redefine tools

---

### Task 1: Add yaml.v3 dependency

**Files:**
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`

- [ ] **Step 1: Add the dependency**

```bash
cd backend && go get gopkg.in/yaml.v3
```

- [ ] **Step 2: Verify it compiled**

```bash
cd backend && go build ./...
```

Expected: no errors. `go.mod` and `go.sum` updated.

- [ ] **Step 3: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "deps: add gopkg.in/yaml.v3 for skill frontmatter parsing"
```

---

### Task 2: Skill struct + empty Registry

**Files:**
- Create: `backend/internal/ai/skills/skill.go`
- Create: `backend/internal/ai/skills/skill_test.go`

- [ ] **Step 1: Write the failing test**

`backend/internal/ai/skills/skill_test.go`:

```go
package skills

import "testing"

func TestEmptyRegistryHasNoSkills(t *testing.T) {
	r := NewRegistry()
	if got := r.List(); len(got) != 0 {
		t.Fatalf("expected empty list, got %d skills", len(got))
	}
}

func TestRegistryGetReturnsFalseForUnknown(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Get("nope"); ok {
		t.Fatal("expected Get to return false for unknown name")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/ai/skills/...
```

Expected: build error (package doesn't exist yet).

- [ ] **Step 3: Write minimal implementation**

`backend/internal/ai/skills/skill.go`:

```go
// Package skills loads SKILL.md files into a registry. The markdown format
// is compatible with Claude Code skills: YAML frontmatter (name, description,
// optional extras) + a free-form markdown body.
package skills

import (
	"sort"
	"sync"
)

// Skill is one loaded SKILL.md file.
type Skill struct {
	Name        string
	Description string
	Body        string
	Extra       map[string]any
	SourcePath  string
}

// Registry is a thread-safe collection of skills indexed by name.
type Registry struct {
	mu    sync.RWMutex
	byKey map[string]*Skill
	order []string
}

func NewRegistry() *Registry {
	return &Registry{byKey: map[string]*Skill{}}
}

// Add inserts a skill. Returns false if the name is already registered.
func (r *Registry) Add(s *Skill) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byKey[s.Name]; exists {
		return false
	}
	r.byKey[s.Name] = s
	r.order = append(r.order, s.Name)
	sort.Strings(r.order)
	return true
}

// Get returns the skill with the given name and a bool indicating presence.
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byKey[name]
	return s, ok
}

// List returns all skills sorted by name.
func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Skill, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.byKey[n])
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd backend && go test ./internal/ai/skills/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ai/skills/skill.go backend/internal/ai/skills/skill_test.go
git commit -m "feat(skills): add Skill struct and empty Registry"
```

---

### Task 3: Loader — parse a single SKILL.md file

**Files:**
- Create: `backend/internal/ai/skills/loader.go`
- Create: `backend/internal/ai/skills/loader_test.go`

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/ai/skills/loader_test.go`:

```go
package skills

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadFromFS_HappyPath(t *testing.T) {
	fsys := fstest.MapFS{
		"explain-page.md": &fstest.MapFile{
			Data: []byte("---\n" +
				"name: explain-page\n" +
				"description: 解释当前页面的核心概念\n" +
				"license: MIT\n" +
				"---\n\n" +
				"你正在用通俗解释模式。\n"),
		},
	}
	r, err := LoadFromFS(fsys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := r.Get("explain-page")
	if !ok {
		t.Fatal("expected explain-page to be registered")
	}
	if s.Description != "解释当前页面的核心概念" {
		t.Errorf("Description = %q", s.Description)
	}
	if !strings.Contains(s.Body, "通俗解释模式") {
		t.Errorf("Body missing expected text: %q", s.Body)
	}
	if got, ok := s.Extra["license"]; !ok || got != "MIT" {
		t.Errorf("Extra[license] = %v", got)
	}
	if !strings.HasSuffix(s.SourcePath, "explain-page.md") {
		t.Errorf("SourcePath = %q", s.SourcePath)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/ai/skills/ -run TestLoadFromFS_HappyPath -v
```

Expected: build error (`LoadFromFS` undefined).

- [ ] **Step 3: Write minimal implementation**

`backend/internal/ai/skills/loader.go`:

```go
package skills

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFromFS walks the given fs.FS for *.md files, parses each as a
// SKILL.md (YAML frontmatter + markdown body), and returns a populated
// Registry. Errors are aggregated: if any file is invalid, LoadFromFS
// returns the aggregated error and a nil registry.
func LoadFromFS(fsys fs.FS) (*Registry, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	var mdFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".md") {
			mdFiles = append(mdFiles, e.Name())
		}
	}
	sort.Strings(mdFiles)

	r := NewRegistry()
	var errs []string
	for _, name := range mdFiles {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: read: %v", name, err))
			continue
		}
		s, err := parseOne(name, raw)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if !r.Add(s) {
			errs = append(errs, fmt.Sprintf("%s: duplicate name %q", name, s.Name))
			continue
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("skill loader: %d error(s):\n  %s",
			len(errs), strings.Join(errs, "\n  "))
	}
	return r, nil
}

// parseOne splits frontmatter from body and decodes frontmatter as YAML.
// Unknown fields land in Extra so we don't reject Claude Code-compatible
// fields like license/compatibility/metadata.
func parseOne(sourcePath string, raw []byte) (*Skill, error) {
	front, body, err := splitFrontmatter(raw)
	if err != nil {
		return nil, err
	}
	if len(front) == 0 {
		return nil, fmt.Errorf("missing frontmatter (expected `---` delimited YAML at top)")
	}

	// Decode into a generic map so unknown fields are preserved.
	var generic map[string]any
	if err := yaml.Unmarshal(front, &generic); err != nil {
		return nil, fmt.Errorf("parse frontmatter yaml: %w", err)
	}

	name, _ := generic["name"].(string)
	desc, _ := generic["description"].(string)
	if name == "" {
		return nil, fmt.Errorf("frontmatter missing required field `name`")
	}
	if desc == "" {
		return nil, fmt.Errorf("frontmatter missing required field `description`")
	}

	// Normalize metadata sub-map if present.
	if md, ok := generic["metadata"].(map[string]any); ok {
		generic["metadata"] = md
	}

	return &Skill{
		Name:        name,
		Description: desc,
		Body:        string(body),
		Extra:       generic,
		SourcePath:  sourcePath,
	}, nil
}

// splitFrontmatter returns the bytes between the first pair of `---` lines
// and everything after. If no frontmatter is present, front is empty.
func splitFrontmatter(raw []byte) (front, body []byte, err error) {
	const sep = []byte("---")
	// Allow optional leading BOM / whitespace.
	trimmed := bytes.TrimLeft(raw, "﻿ \t\r\n")
	if !bytes.HasPrefix(trimmed, sep) {
		return nil, raw, nil
	}
	rest := trimmed[len(sep):]
	// Must be followed by a newline.
	if len(rest) == 0 || rest[0] != '\n' {
		return nil, raw, fmt.Errorf("frontmatter opening `---` must be followed by newline")
	}
	rest = rest[1:]
	nl := bytes.IndexByte(rest, '\n')
	if nl < 0 {
		return nil, raw, fmt.Errorf("frontmatter not closed (no closing `---`)")
	}
	front = bytes.TrimRight(rest[:nl], " \t\r")
	tail := rest[nl+1:]
	// Closing fence must be on its own line.
	for len(tail) > 0 && (tail[0] == ' ' || tail[0] == '\t') {
		tail = tail[1:]
	}
	if !bytes.HasPrefix(tail, sep) {
		return nil, raw, fmt.Errorf("frontmatter closing `---` must start a line")
	}
	tail = tail[len(sep):]
	if len(tail) > 0 && tail[0] != '\n' {
		return nil, raw, fmt.Errorf("frontmatter closing `---` must be followed by newline")
	}
	body = bytes.TrimLeft(tail, "\r\n")
	return front, body, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd backend && go test ./internal/ai/skills/ -run TestLoadFromFS_HappyPath -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ai/skills/loader.go backend/internal/ai/skills/loader_test.go
git commit -m "feat(skills): load SKILL.md files from fs.FS"
```

---

### Task 4: Loader — validation cases

**Files:**
- Modify: `backend/internal/ai/skills/loader_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `backend/internal/ai/skills/loader_test.go`:

```go
func TestLoadFromFS_MissingName(t *testing.T) {
	fsys := fstest.MapFS{
		"bad.md": &fstest.MapFile{Data: []byte("---\ndescription: hi\n---\nbody")},
	}
	_, err := LoadFromFS(fsys)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "bad.md") {
		t.Errorf("error should mention file path: %v", err)
	}
}

func TestLoadFromFS_MissingDescription(t *testing.T) {
	fsys := fstest.MapFS{
		"bad.md": &fstest.MapFile{Data: []byte("---\nname: x\n---\nbody")},
	}
	_, err := LoadFromFS(fsys)
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("expected error mentioning description, got %v", err)
	}
}

func TestLoadFromFS_DuplicateName(t *testing.T) {
	fsys := fstest.MapFS{
		"a.md": &fstest.MapFile{Data: []byte("---\nname: foo\ndescription: A\n---\nx")},
		"b.md": &fstest.MapFile{Data: []byte("---\nname: foo\ndescription: B\n---\ny")},
	}
	_, err := LoadFromFS(fsys)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestLoadFromFS_EmptyBodyIsOK(t *testing.T) {
	fsys := fstest.MapFS{
		"empty.md": &fstest.MapFile{Data: []byte("---\nname: e\ndescription: E\n---\n")},
	}
	r, err := LoadFromFS(fsys)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	s, _ := r.Get("e")
	if s.Body != "" {
		t.Errorf("expected empty body, got %q", s.Body)
	}
}

func TestLoadFromFS_OrderingIsAlphabetical(t *testing.T) {
	fsys := fstest.MapFS{
		"z.md": &fstest.MapFile{Data: []byte("---\nname: zed\ndescription: Z\n---\n")},
		"a.md": &fstest.MapFile{Data: []byte("---\nname: aardvark\ndescription: A\n---\n")},
	}
	r, _ := LoadFromFS(fsys)
	got := r.List()
	if len(got) != 2 || got[0].Name != "aardvark" || got[1].Name != "zed" {
		t.Errorf("unexpected order: %+v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass (loader already supports them)**

```bash
cd backend && go test ./internal/ai/skills/ -run TestLoadFromFS -v
```

Expected: all 5 tests PASS (loader from Task 3 already handles these cases — these tests pin the behavior).

If any fail, fix `loader.go` until they all pass.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/ai/skills/loader_test.go
git commit -m "test(skills): cover loader validation cases"
```

---

### Task 5: embed.FS source for skills

**Files:**
- Create: `backend/internal/ai/skills/embed.go`
- Create: `backend/internal/ai/skills/explain-page.md`

- [ ] **Step 1: Create the example skill file**

`backend/internal/ai/skills/explain-page.md`:

```markdown
---
name: explain-page
description: 把当前页面的核心概念用通俗语言讲给非专家，控制在 200 字以内
license: MIT
---

你正在用「通俗解释」模式。用户会给你一个页面 ID。

要求：
- 假设读者没有相关背景
- 用类比和例子，不堆术语
- 控制在 200 字以内
- 不修改页面内容
```

- [ ] **Step 2: Create the embed file**

`backend/internal/ai/skills/embed.go`:

```go
package skills

import "embed"

//go:embed *.md
var embedFS embed.FS
```

- [ ] **Step 3: Verify build still passes (embed compiles) and test the integration**

```bash
cd backend && go build ./... && go test ./internal/ai/skills/ -v
```

Expected: all tests pass; `go build` succeeds.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/ai/skills/embed.go backend/internal/ai/skills/explain-page.md
git commit -m "feat(skills): embed SKILL.md files into binary"
```

---

### Task 6: BuildChatSystemPrompt

**Files:**
- Modify: `backend/internal/ai/provider.go`
- Create: `backend/internal/ai/provider_test.go` (or add to existing if present)

- [ ] **Step 1: Check if provider_test.go exists**

```bash
ls backend/internal/ai/provider_test.go 2>&1
```

If yes, append to it. If no, create it.

- [ ] **Step 2: Write the failing test**

`backend/internal/ai/provider_test.go`:

```go
package ai

import (
	"strings"
	"testing"

	"learn-helper/internal/ai/skills"
)

func TestBuildChatSystemPrompt_NilSkillUnchanged(t *testing.T) {
	base := BuildSystemPrompt(RoleWikiMaintainer, "(wiki context)")
	got := BuildChatSystemPrompt(RoleWikiMaintainer, "(wiki context)", nil)
	if got != base {
		t.Errorf("nil skill should produce base prompt unchanged\nbase=%q\ngot =%q", base, got)
	}
}

func TestBuildChatSystemPrompt_AppendsSkillBody(t *testing.T) {
	skill := &skills.Skill{
		Name: "explain-page",
		Body: "你正在用通俗解释模式。",
	}
	got := BuildChatSystemPrompt(RoleWikiMaintainer, "(ctx)", skill)
	if !strings.Contains(got, "你正在用通俗解释模式。") {
		t.Errorf("prompt missing skill body: %q", got)
	}
	if !strings.Contains(got, "## 当前 Skill: explain-page") {
		t.Errorf("prompt missing skill header: %q", got)
	}
	// Wiki context must still be there.
	if !strings.Contains(got, "(ctx)") {
		t.Errorf("prompt missing wiki context: %q", got)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd backend && go test ./internal/ai/ -run TestBuildChatSystemPrompt -v
```

Expected: build error (`BuildChatSystemPrompt` undefined).

- [ ] **Step 4: Add the function**

In `backend/internal/ai/provider.go`, add at the bottom (and import `learn-helper/internal/ai/skills`):

```go
import (
	// ... existing imports ...
	"learn-helper/internal/ai/skills"
)

// BuildChatSystemPrompt constructs the system prompt with wiki context and
// (optionally) appends a user-invoked skill's body. A nil skill is a no-op
// (returns the base prompt unchanged). When skill is non-nil, the body is
// appended verbatim under a "## 当前 Skill: <name>" header so the LLM sees
// the skill context as an addition to, not a replacement for, the base.
func BuildChatSystemPrompt(role, wikiContext string, skill *skills.Skill) string {
	base := BuildSystemPrompt(role, wikiContext)
	if skill == nil {
		return base
	}
	return base + "\n\n## 当前 Skill: " + skill.Name + "\n\n" + skill.Body
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd backend && go test ./internal/ai/ -run TestBuildChatSystemPrompt -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ai/provider.go backend/internal/ai/provider_test.go
git commit -m "feat(ai): BuildChatSystemPrompt appends skill body"
```

---

### Task 7: GET /api/skills handler

**Files:**
- Create: `backend/internal/handler/skills.go`
- Create: `backend/internal/handler/skills_test.go`

- [ ] **Step 1: Write the failing test**

`backend/internal/handler/skills_test.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"learn-helper/internal/ai/skills"
)

func TestSkillsHandler_ListSorted(t *testing.T) {
	r := skills.NewRegistry()
	_ = r.Add(&skills.Skill{Name: "zebra", Description: "Z"})
	_ = r.Add(&skills.Skill{Name: "alpha", Description: "A"})

	h := &SkillsHandler{Registry: r}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/skills", h.HandleList)

	req := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var got []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "zebra" {
		t.Errorf("unexpected order/content: %+v", got)
	}
	// Body / Extra / SourcePath must NOT be exposed.
	raw := w.Body.String()
	for _, leaked := range []string{"body", "Body", "Extra", "SourcePath"} {
		if containsAny(raw, leaked) {
			t.Errorf("response leaked field %q: %s", leaked, raw)
		}
	}
}

func containsAny(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/handler/ -run TestSkillsHandler -v
```

Expected: build error.

- [ ] **Step 3: Write the handler**

`backend/internal/handler/skills.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"

	"learn-helper/internal/ai/skills"
)

// SkillsHandler exposes the catalog of available skills to the frontend.
type SkillsHandler struct {
	Registry *skills.Registry
}

type skillListItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// HandleList responds with the sorted list of skills (name + description only).
// Body, Extra, and SourcePath are intentionally NOT exposed.
func (h *SkillsHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	all := h.Registry.List()
	out := make([]skillListItem, 0, len(all))
	for _, s := range all {
		out = append(out, skillListItem{
			Name:        s.Name,
			Description: s.Description,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd backend && go test ./internal/handler/ -run TestSkillsHandler -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ai/skills/skills.go backend/internal/handler/skills_test.go
git commit -m "feat(handler): GET /api/skills lists skill catalog"
```

---

### Task 8: AI handler reads `skill` field and returns 400 for unknown

**Files:**
- Modify: `backend/internal/handler/ai.go`

- [ ] **Step 1: Read the current request struct**

Look at `backend/internal/handler/ai.go` and find the request type for `POST /api/ai/chat`. (It's likely named `AIChatRequest` or similar.) Note the field that holds the user message — we'll add `Skill` next to it.

```bash
grep -n "type AIChatRequest\|type.*Request.*struct\|message " backend/internal/handler/ai.go | head -20
```

- [ ] **Step 2: Add the `Skill` field to the request struct**

In the request struct, add a new field:

```go
type AIChatRequest struct {
	// ... existing fields ...
	Message string `json:"message"`
	Skill   string `json:"skill,omitempty"` // optional: SKILL.md name
}
```

- [ ] **Step 3: Update the handler to look up the skill**

In the handler function (around line 477 where `BuildSystemPrompt` is called), modify to use the registry-aware builder:

```go
// Replace this:
//   systemPrompt := ai.BuildSystemPrompt(convRole, wikiContext)
// With:
var skillObj *skills.Skill
if req.Skill != "" {
	s, ok := h.SkillRegistry.Get(req.Skill)
	if !ok {
		available := h.SkillRegistry.List()
		names := make([]string, 0, len(available))
		for _, x := range available {
			names = append(names, x.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":     "unknown skill: " + req.Skill,
			"available": names,
		})
		return
	}
	skillObj = s
}
systemPrompt := ai.BuildChatSystemPrompt(convRole, wikiContext, skillObj)
```

Add the import:

```go
import "learn-helper/internal/ai/skills"
```

- [ ] **Step 4: Add `SkillRegistry` to the AIHandler struct**

In `backend/internal/handler/ai.go`, find the `AIHandler` (or `Handler`) struct definition and add:

```go
type AIHandler struct {
	// ... existing fields ...
	SkillRegistry *skills.Registry
}
```

(If the existing struct uses a different name, match the existing pattern. The actual struct name is whatever the existing file uses — check `grep "type.*Handler.*struct" backend/internal/handler/ai.go`.)

- [ ] **Step 5: Build and test**

```bash
cd backend && go build ./...
```

Expected: build error in main.go (`AIHandler` now requires `SkillRegistry`) — fix that in Task 9. For now, **temporarily** initialize an empty registry in main.go if the build fails, just to keep the build green:

In `cmd/server/main.go`, find where `AIHandler` is constructed and add:

```go
import "learn-helper/internal/ai/skills"
// ...
reg, _ := skills.LoadFromFS(skills.EmbedFS()) // we'll add EmbedFS in Task 9
// pass reg to AIHandler.SkillRegistry
```

If `EmbedFS` doesn't exist yet, comment out the registration and pass `nil` to satisfy the type, then proceed:

```go
// temporary: SkillRegistry wired in Task 9
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler/ai.go backend/cmd/server/main.go
git commit -m "feat(handler): AIChat reads skill field, returns 400 for unknown"
```

---

### Task 9: Wire up Registry in main.go

**Files:**
- Modify: `backend/internal/ai/skills/embed.go` (add `EmbedFS` accessor)
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add an accessor for the embed.FS**

In `backend/internal/ai/skills/embed.go`, append:

```go
// EmbedFS returns the embedded filesystem containing all SKILL.md files.
// Used as the default load source; override with os.DirFS(LH_SKILLS_DIR) in dev.
func EmbedFS() embed.FS {
	return embedFS
}
```

- [ ] **Step 2: Find the AIHandler construction in main.go**

```bash
grep -n "AIHandler{\|SkillRegistry\|ai.ReAct" backend/cmd/server/main.go
```

- [ ] **Step 3: Load the registry and pass it in**

At the top of `main.go`, add the import:

```go
import "learn-helper/internal/ai/skills"
```

Right before constructing the AIHandler (or its dependencies), load the registry:

```go
var reg *skills.Registry
if dir := os.Getenv("LH_SKILLS_DIR"); dir != "" {
	fsys := os.DirFS(dir)
	reg, err = skills.LoadFromFS(fsys)
	if err != nil {
		log.Fatalf("load skills from %s: %v", dir, err)
	}
} else {
	reg, err = skills.LoadFromFS(skills.EmbedFS())
	if err != nil {
		log.Fatalf("load embedded skills: %v", err)
	}
}
log.Printf("[boot] loaded %d skills", len(reg.List()))
```

(`os` and `log` should already be imported. If not, add them.)

When constructing the AIHandler, pass:

```go
aiHandler := &handler.AIHandler{
	// ... existing fields ...
	SkillRegistry: reg,
}
```

(Adjust the field name / constructor pattern to match the existing main.go style.)

Also register the new route. Find the existing `/api/ai` route block and add:

```go
r.Get("/api/skills", (&handler.SkillsHandler{Registry: reg}).HandleList)
```

(Or use whatever route registration style the file uses — `r.Method("GET", "/api/skills", ...)` if it goes through chi.)

- [ ] **Step 4: Build and run**

```bash
cd backend && go build ./... && go run ./cmd/server &
sleep 2
curl -s http://localhost:8080/api/skills | head -c 500
kill %1 2>/dev/null
```

Expected: JSON array containing at least `{"name":"explain-page","description":"..."}`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ai/skills/embed.go backend/cmd/server/main.go
git commit -m "feat(boot): load skill registry and expose /api/skills"
```

---

### Task 10: Frontend — add `skill` to `ChatRequest`

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Find the `ChatRequest` type**

```bash
grep -n "interface ChatRequest\|type ChatRequest\|message: string" frontend/src/lib/api.ts | head -5
```

- [ ] **Step 2: Add the optional `skill` field**

Add `skill?: string;` to the `ChatRequest` interface (next to the existing fields). No other changes to `streamChat` are needed — the request body is `JSON.stringify(req)`, so the new field is sent automatically.

- [ ] **Step 3: Add a `fetchSkills` helper**

At the bottom of the file (or next to `streamChat`), add:

```typescript
export interface SkillInfo {
  name: string;
  description: string;
}

export async function fetchSkills(): Promise<SkillInfo[]> {
  const res = await fetch(`${BASE}/skills`);
  if (!res.ok) throw new Error(`fetchSkills failed: ${res.status}`);
  return res.json();
}
```

- [ ] **Step 4: Verify type-check passes**

```bash
cd frontend && npm run build
```

Expected: no TypeScript errors. (Build may take a moment.)

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(frontend): add skill field to ChatRequest + fetchSkills helper"
```

---

### Task 11: Frontend — parse `/<name>` in handleSend

**Files:**
- Modify: `frontend/src/components/ChatPanel.tsx`

- [ ] **Step 1: Add the parser and wire it into the request**

Near the top of the file (after imports), add the constant:

```typescript
const SKILL_CMD_RE = /^\/([a-z][a-z0-9-]*)(?:\s+([\s\S]*))?$/;
```

In `handleSend`, just before calling `streamChat`, find the call site (the call inside the `if (messageOverride) { ... }` block AND the one for new conversations). Both call sites construct an object like `{ conversation_id, message, focus_page_id, current_slug }`. Modify both to inject `skill`:

```typescript
const match = (messageOverride ?? input).match(SKILL_CMD_RE);
const skillName = match ? match[1] : undefined;
const userMsg = match ? (match[2] ?? "") : (messageOverride ?? input);

// ... in the request object:
{
  conversation_id: ...,
  message: userMsg,
  focus_page_id: focusPageId,
  current_slug: currentSlug,
  ...(skillName ? { skill: skillName } : {}),
}
```

(There are two call sites that build the request body — apply this change to both. If the existing code only has one, just apply once.)

- [ ] **Step 2: Type-check**

```bash
cd frontend && npm run build
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ChatPanel.tsx
git commit -m "feat(frontend): parse /<name> in chat input and send skill field"
```

---

### Task 12: Frontend — `/` autocomplete panel

**Files:**
- Modify: `frontend/src/components/ChatPanel.tsx`

- [ ] **Step 1: Add state for the panel**

In the existing `useState` block, add:

```typescript
const [skills, setSkills] = useState<SkillInfo[]>([]);
const [skillPanelOpen, setSkillPanelOpen] = useState(false);
const [skillPanelIndex, setSkillPanelIndex] = useState(0);
```

- [ ] **Step 2: Load skills on mount**

In the existing `useEffect` that fires on mount (find `useEffect(() => {` near the top), add a call:

```typescript
fetchSkills()
  .then(setSkills)
  .catch(() => setSkills([]));
```

(If `fetchSkills` isn't imported yet, add it to the import list at the top: `import { streamChat, fetchSkills, type SkillInfo } from "../lib/api";`)

- [ ] **Step 3: Add the open/close logic to the input**

Replace the existing `onChange` handler on the `<input>` (line ~724) with:

```typescript
onChange={(e) => {
  const v = e.target.value;
  setInput(v);
  if (v === "/" || v.startsWith("/")) {
    setSkillPanelOpen(true);
    setSkillPanelIndex(0);
  } else {
    setSkillPanelOpen(false);
  }
}}
onKeyDown={(e) => {
  if (skillPanelOpen && skills.length > 0) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSkillPanelIndex((i) => (i + 1) % skills.length);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      setSkillPanelIndex((i) => (i - 1 + skills.length) % skills.length);
      return;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      setInput("");
      setSkillPanelOpen(false);
      return;
    }
    if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
      // Only intercept Enter if a skill is fully named in the input.
      const match = input.match(SKILL_CMD_RE);
      if (match) {
        e.preventDefault();
        setSkillPanelOpen(false);
        handleSend();
        return;
      }
    }
  }
  if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
    e.preventDefault();
    handleSend();
  }
}}
```

- [ ] **Step 4: Render the panel**

Just above the `<input>` (around line 717, where the `<div className="flex gap-2">` starts), insert the panel — but **inside** the relative wrapper, before the input row. Restructure as:

```tsx
<div className="relative">
  {skillPanelOpen && filteredSkills.length > 0 && (
    <div className="absolute bottom-full mb-2 left-0 right-12 max-h-64 overflow-y-auto rounded-xl border border-th-input-border bg-th-bg-secondary shadow-lg">
      {filteredSkills.map((s, i) => (
        <button
          key={s.name}
          type="button"
          onClick={() => {
            setInput(`/${s.name} `);
            setSkillPanelOpen(false);
            inputRef.current?.focus();
          }}
          onMouseEnter={() => setSkillPanelIndex(i)}
          className={`w-full text-left px-3 py-2 text-sm ${
            i === skillPanelIndex ? "bg-th-accent/20" : ""
          }`}
        >
          <div className="font-mono text-th-text-primary">/{s.name}</div>
          <div className="text-xs text-th-text-secondary mt-0.5">{s.description}</div>
        </button>
      ))}
    </div>
  )}
  <div className="flex gap-2">
    {/* existing input + buttons */}
  </div>
</div>
```

- [ ] **Step 5: Add the filter derivation**

Next to other `useMemo` calls, add:

```typescript
const filteredSkills = useMemo(() => {
  if (!skillPanelOpen) return [];
  const q = input.slice(1).toLowerCase(); // strip leading `/`
  if (q === "") return skills;
  return skills.filter((s) => s.name.toLowerCase().startsWith(q));
}, [skillPanelOpen, input, skills]);
```

- [ ] **Step 6: Type-check**

```bash
cd frontend && npm run build
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/ChatPanel.tsx
git commit -m "feat(frontend): / autocomplete panel in chat input"
```

---

### Task 13: Manual end-to-end smoke test

**Files:** (none)

- [ ] **Step 1: Start the backend**

```bash
cd backend && go run ./cmd/server
```

Expected: log line `[boot] loaded 1 skills` (or similar).

- [ ] **Step 2: Verify GET /api/skills**

```bash
curl -s http://localhost:8080/api/skills
```

Expected:

```json
[{"name":"explain-page","description":"把当前页面的核心概念用通俗语言讲给非专家，控制在 200 字以内"}]
```

- [ ] **Step 3: Start the frontend dev server**

```bash
cd frontend && npm run dev
```

- [ ] **Step 4: Open the chat panel in browser**

- [ ] **Step 5: Type `/` in the input**

Expected: panel appears showing `explain-page`.

- [ ] **Step 6: Type `/explain-page 给我讲讲` and send**

Expected: AI responds in the "explain-page" style (short, no jargon). The system prompt should contain the skill body.

- [ ] **Step 7: Send a regular message (no `/`)**

Expected: AI responds normally (no skill body in prompt).

- [ ] **Step 8: Try `/nonsense` and send**

Expected: 400 error from backend, frontend shows error.

- [ ] **Step 9: Commit any final adjustments**

If anything in steps 5-8 didn't behave as expected, fix and commit before declaring done. If all good, this task is a no-op commit.

---

## Self-Review Checklist

After completing all tasks, verify against the spec:

| Spec section | Implemented in |
|---|---|
| File format (frontmatter + body) | Task 3 (`parseOne`) |
| Format compatible with Claude Code | Task 3 (uses yaml.v3 generic decode, preserves `Extra`) |
| `name` / `description` required | Task 4 |
| Duplicate name fails fast | Task 4 |
| Empty body allowed | Task 4 |
| Unknown fields preserved | Task 3 (`Extra`) |
| `embed.FS` for default | Task 5 |
| `os.DirFS(LH_SKILLS_DIR)` dev override | Task 9 |
| `GET /api/skills` returns name + description only | Task 7 |
| `POST /api/ai/chat` accepts `skill` | Task 8 |
| 400 + `available` for unknown skill | Task 8 |
| `BuildChatSystemPrompt` appends body | Task 6 |
| Cron mode unchanged | No tasks touch cron (verified) |
| Frontend parses `/<name>` | Task 11 |
| Frontend `/` panel | Task 12 |
| Unit tests for loader | Tasks 3, 4 |
| Unit tests for prompt builder | Task 6 |
| Unit tests for handler | Task 7 |
| E2E smoke test | Task 13 |
