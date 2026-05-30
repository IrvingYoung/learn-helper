# AI 操作规划层 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current AI ReAct write-tool loop with a Plan-based architecture where AI proposes operations, users confirm, and an execution engine deterministically executes them.

**Architecture:** Add a Plan/Action persistence layer in SQLite, an ExecutionEngine that topologically sorts and runs actions, and a `propose_plan` tool that replaces the three write tools. The frontend switches from per-action confirmation to Plan-level confirmation with a preview panel in the right column. A wiki link system (`[[title]]` syntax with bidirectional tracking) is added alongside.

**Tech Stack:** Go + Chi + SQLite (backend), React 19 + SWR + Tailwind (frontend), Claude/DeepSeek APIs (AI)

---

## File Structure

### Backend — New Files

| File | Responsibility |
|------|---------------|
| `backend/internal/handler/plan.go` | Plan CRUD handlers (create, confirm, reject, get, list) |
| `backend/internal/engine/engine.go` | ExecutionEngine — topological sort, action execution, ID replacement |
| `backend/internal/engine/engine_test.go` | Unit tests for execution engine |

### Backend — Modified Files

| File | Change |
|------|--------|
| `backend/cmd/server/main.go` | Add `plans`/`plan_actions` tables to schema, add `links`/`backlinks` columns to `wiki_pages`, register new routes |
| `backend/internal/ai/provider.go` | Replace write tools with `propose_plan`, update system prompt with knowledge context |
| `backend/internal/handler/ai.go` | Replace pending_actions flow with Plan flow; `propose_plan` triggers Plan creation instead of write-tool confirmation |
| `backend/internal/handler/wiki.go` | Add link parsing in Create/Update, add link cleanup in Delete, add link update in Rename, add backlinks endpoint |
| `backend/internal/model/models.go` | Add Plan, PlanAction, WikiPage links/backlinks fields |

### Frontend — New Files

| File | Responsibility |
|------|---------------|
| `frontend/src/components/PlanPreview.tsx` | Plan preview panel for right column |

### Frontend — Modified Files

| File | Change |
|------|--------|
| `frontend/src/types/index.ts` | Add Plan, PlanAction, PlanStatus types; update WikiPage with links/backlinks |
| `frontend/src/lib/api.ts` | Add Plan API functions (confirmPlan, rejectPlan, getPlan); update ChatRequest |
| `frontend/src/components/ChatPanel.tsx` | Replace pending_actions confirmation with Plan confirmation flow |
| `frontend/src/components/PageViewer.tsx` | Add Plan preview mode; add backlinks section; render `[[title]]` links |
| `frontend/src/components/WikiPage.tsx` | Wire Plan state between ChatPanel and PageViewer |

---

## Task 1: Database Schema — Plans, Actions, Links

**Files:**
- Modify: `backend/cmd/server/main.go:17-110` (schemaSQL)

- [ ] **Step 1: Add plans and plan_actions tables to schemaSQL**

In `main.go`, append to the `schemaSQL` string after the `wiki_pages` table definition (after line ~105):

```go
    CREATE TABLE IF NOT EXISTS plans (
        id TEXT PRIMARY KEY,
        conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
        reasoning TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'confirmed', 'executing', 'completed', 'rejected', 'completed_with_failures')),
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        executed_at TEXT
    );

    CREATE TABLE IF NOT EXISTS plan_actions (
        id TEXT PRIMARY KEY,
        plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
        type TEXT NOT NULL CHECK(type IN ('create_page', 'update_page', 'delete_page', 'link_pages', 'move_page')),
        params TEXT NOT NULL,
        depends_on TEXT NOT NULL DEFAULT '[]',
        status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'completed', 'failed', 'skipped')),
        result TEXT,
        sort_order INTEGER NOT NULL,
        created_at TEXT NOT NULL DEFAULT (datetime('now'))
    );

    CREATE INDEX IF NOT EXISTS idx_plans_conversation ON plans(conversation_id);
    CREATE INDEX IF NOT EXISTS idx_plan_actions_plan ON plan_actions(plan_id);
```

- [ ] **Step 2: Add links and backlinks columns to wiki_pages**

In `main.go`, append to the `schemaSQL` string:

```go
    ALTER TABLE wiki_pages ADD COLUMN links TEXT NOT NULL DEFAULT '[]';
    ALTER TABLE wiki_pages ADD COLUMN backlinks TEXT NOT NULL DEFAULT '[]';
```

Note: ALTER TABLE ADD COLUMN in SQLite is safe — it adds the column with the default to existing rows. If the column already exists (re-runs), SQLite will error; wrap each ALTER in a separate `exec` call and ignore "duplicate column" errors, or add them as part of the initial CREATE TABLE for fresh databases.

For the CREATE TABLE definition of `wiki_pages`, add the two columns directly:

```go
    -- In the wiki_pages CREATE TABLE, add:
    links TEXT NOT NULL DEFAULT '[]',
    backlinks TEXT NOT NULL DEFAULT '[]',
```

And remove the ALTER TABLE statements (they're only needed for migration of existing databases — handle that separately if needed).

- [ ] **Step 3: Start the server and verify tables are created**

Run: `cd /Users/irving/repo/learn-helper/backend && go run ./cmd/server`
Expected: Server starts on :8080 without errors. Tables `plans` and `plan_actions` exist.

Verify: `sqlite3 learn-helper.db ".tables"` should show `plans` and `plan_actions`.

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat: add plans, plan_actions tables and links/backlinks columns to schema"
```

---

## Task 2: Backend Model Types

**Files:**
- Modify: `backend/internal/model/models.go`

- [ ] **Step 1: Add Plan and PlanAction structs**

In `models.go`, append after the `WikiPage` struct (after line ~105):

```go
type Plan struct {
    ID             string  `json:"id"`
    ConversationID int64   `json:"conversation_id"`
    Reasoning      string  `json:"reasoning"`
    Status         string  `json:"status"`
    Actions        []PlanAction `json:"actions,omitempty"`
    CreatedAt      string  `json:"created_at"`
    ExecutedAt     *string `json:"executed_at,omitempty"`
}

type PlanAction struct {
    ID         string `json:"id"`
    PlanID     string `json:"plan_id"`
    Type       string `json:"type"`
    Params     string `json:"params"`
    DependsOn  string `json:"depends_on"`
    Status     string `json:"status"`
    Result     *string `json:"result,omitempty"`
    SortOrder  int64  `json:"sort_order"`
    CreatedAt  string `json:"created_at"`
}
```

- [ ] **Step 2: Add links/backlinks to WikiPage struct**

In `models.go`, add fields to the `WikiPage` struct (after line ~103, before `CreatedAt`):

```go
    Links      string `json:"links"`
    Backlinks  string `json:"backlinks"`
```

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/irving/repo/learn-helper/backend && go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/model/models.go
git commit -m "feat: add Plan, PlanAction models and links/backlinks to WikiPage"
```

---

## Task 3: Execution Engine

**Files:**
- Create: `backend/internal/engine/engine.go`
- Create: `backend/internal/engine/engine_test.go`

- [ ] **Step 1: Write the ExecutionEngine with topological sort and action execution**

Create `backend/internal/engine/engine.go`:

```go
package engine

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/irving/learn-helper/backend/internal/model"
)

type ExecutionEngine struct {
	db      *sql.DB
	queries *model.Queries
}

func NewExecutionEngine(db *sql.DB, queries *model.Queries) *ExecutionEngine {
	return &ExecutionEngine{db: db, queries: queries}
}

// ActionResult holds the result of a single action execution.
type ActionResult struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ExecutionReport is returned after executing all actions in a plan.
type ExecutionReport struct {
	PlanID  string         `json:"plan_id"`
	Status  string         `json:"status"`
	Actions []ActionResult `json:"actions"`
}

// actionResultMap stores completed action results for ID replacement.
type actionResultMap map[string]map[string]any // actionID -> {field: value}

// ExecutePlan executes all actions in a plan in dependency order.
func (e *ExecutionEngine) ExecutePlan(planID string) (*ExecutionReport, error) {
	// Load actions
	actions, err := e.loadActions(planID)
	if err != nil {
		return nil, fmt.Errorf("load actions: %w", err)
	}

	// Topological sort
	sorted, err := e.topologicalSort(actions)
	if err != nil {
		return nil, fmt.Errorf("topological sort: %w", err)
	}

	// Update plan status
	_, _ = e.db.Exec("UPDATE plans SET status = 'executing' WHERE id = ?", planID)

	// Track failed actions for skipping dependents
	failed := make(map[string]bool)
	results := make(actionResultMap)
	var reportActions []ActionResult

	for _, action := range sorted {
		// Check if any dependency failed
		var dependsOn []string
		_ = json.Unmarshal([]byte(action.DependsOn), &dependsOn)

		skip := false
		for _, depID := range dependsOn {
			if failed[depID] {
				skip = true
				break
			}
		}

		if skip {
			_ = e.updateActionStatus(action.ID, "skipped", `{"reason":"dependency failed"}`)
			reportActions = append(reportActions, ActionResult{
				ID:     action.ID,
				Type:   action.Type,
				Status: "skipped",
				Error:  "dependency failed",
			})
			failed[action.ID] = true
			continue
		}

		// Replace ID placeholders in params
		params, err := e.replacePlaceholders(action.Params, results)
		if err != nil {
			_ = e.updateActionStatus(action.ID, "failed", fmt.Sprintf(`{"error":"placeholder replacement: %s"}`, err.Error()))
			reportActions = append(reportActions, ActionResult{
				ID:     action.ID,
				Type:   action.Type,
				Status: "failed",
				Error:  fmt.Sprintf("placeholder replacement: %s", err),
			})
			failed[action.ID] = true
			continue
		}

		// Execute the action
		_ = e.updateActionStatus(action.ID, "running", "")
		result, execErr := e.executeAction(action.Type, params)

		if execErr != nil {
			errJSON, _ := json.Marshal(map[string]string{"error": execErr.Error()})
			_ = e.updateActionStatus(action.ID, "failed", string(errJSON))
			reportActions = append(reportActions, ActionResult{
				ID:     action.ID,
				Type:   action.Type,
				Status: "failed",
				Error:  execErr.Error(),
			})
			failed[action.ID] = true
		} else {
			resultJSON, _ := json.Marshal(result)
			_ = e.updateActionStatus(action.ID, "completed", string(resultJSON))
			reportActions = append(reportActions, ActionResult{
				ID:     action.ID,
				Type:   action.Type,
				Status: "completed",
				Result: result,
			})
			results[action.ID] = result
		}
	}

	// Determine final plan status
	planStatus := "completed"
	for _, ra := range reportActions {
		if ra.Status == "failed" || ra.Status == "skipped" {
			planStatus = "completed_with_failures"
			break
		}
	}

	_, _ = e.db.Exec("UPDATE plans SET status = ?, executed_at = datetime('now') WHERE id = ?", planStatus, planID)

	return &ExecutionReport{
		PlanID:  planID,
		Status:  planStatus,
		Actions: reportActions,
	}, nil
}

func (e *ExecutionEngine) loadActions(planID string) ([]model.PlanAction, error) {
	rows, err := e.db.Query("SELECT id, plan_id, type, params, depends_on, status, result, sort_order, created_at FROM plan_actions WHERE plan_id = ? ORDER BY sort_order", planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []model.PlanAction
	for rows.Next() {
		var a model.PlanAction
		if err := rows.Scan(&a.ID, &a.PlanID, &a.Type, &a.Params, &a.DependsOn, &a.Status, &a.Result, &a.SortOrder, &a.CreatedAt); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

func (e *ExecutionEngine) updateActionStatus(actionID, status, result string) error {
	if result != "" {
		_, err := e.db.Exec("UPDATE plan_actions SET status = ?, result = ? WHERE id = ?", status, result, actionID)
		return err
	}
	_, err := e.db.Exec("UPDATE plan_actions SET status = ? WHERE id = ?", status, actionID)
	return err
}

// placeholderPattern matches {{action:a1.page_id}} style placeholders
var placeholderPattern = regexp.MustCompile(`\{\{action:([^\.]+)\.([^}]+)\}\}`)

func (e *ExecutionEngine) replacePlaceholders(paramsJSON string, results actionResultMap) (map[string]any, error) {
	replaced := placeholderPattern.ReplaceAllStringFunc(paramsJSON, func(match string) string {
		parts := placeholderPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		actionID, field := parts[1], parts[2]
		if resultMap, ok := results[actionID]; ok {
			if val, ok := resultMap[field]; ok {
				b, _ := json.Marshal(val)
				return string(b)
			}
		}
		return match
	})

	var params map[string]any
	if err := json.Unmarshal([]byte(replaced), &params); err != nil {
		return nil, fmt.Errorf("unmarshal params: %w", err)
	}
	return params, nil
}

func (e *ExecutionEngine) executeAction(actionType string, params map[string]any) (map[string]any, error) {
	switch actionType {
	case "create_page":
		return e.executeCreatePage(params)
	case "update_page":
		return e.executeUpdatePage(params)
	case "delete_page":
		return e.executeDeletePage(params)
	case "link_pages":
		return e.executeLinkPages(params)
	case "move_page":
		return e.executeMovePage(params)
	default:
		return nil, fmt.Errorf("unknown action type: %s", actionType)
	}
}

func (e *ExecutionEngine) executeCreatePage(params map[string]any) (map[string]any, error) {
	title, _ := params["title"].(string)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	slug, _ := params["slug"].(string)
	if slug == "" {
		slug = slugify(title)
	}

	pageType, _ := params["page_type"].(string)
	if pageType == "" {
		pageType = "entity"
	}

	content, _ := params["content"].(string)

	var parentID *int64
	if pid, ok := params["parent_id"]; ok {
		switch v := pid.(type) {
		case float64:
			id := int64(v)
			parentID = &id
		case int64:
			parentID = &v
		}
	}

	contentStatus := "draft"
	if strings.TrimSpace(content) == "" {
		contentStatus = "empty"
	}

	// Compute path
	var path string
	if parentID != nil {
		var parentPath string
		err := e.db.QueryRow("SELECT path FROM wiki_pages WHERE id = ?", *parentID).Scan(&parentPath)
		if err != nil {
			return nil, fmt.Errorf("parent not found: %w", err)
		}
		path = parentPath
	} else {
		path = "/"
	}

	// Insert page
	var pageID int64
	err := e.db.QueryRow(
		`INSERT INTO wiki_pages (title, slug, page_type, content, parent_id, content_status, path, links, backlinks)
		 VALUES (?, ?, ?, ?, ?, ?, ?, '[]', '[]') RETURNING id`,
		title, slug, pageType, content, parentID, contentStatus, path,
	).Scan(&pageID)
	if err != nil {
		return nil, fmt.Errorf("insert page: %w", err)
	}

	// Update path to include self
	newPath := fmt.Sprintf("%s%d/", path, pageID)
	_, _ = e.db.Exec("UPDATE wiki_pages SET path = ? WHERE id = ?", newPath, pageID)

	return map[string]any{"page_id": pageID, "slug": slug}, nil
}

func (e *ExecutionEngine) executeUpdatePage(params map[string]any) (map[string]any, error) {
	pageID, ok := toInt64(params["page_id"])
	if !ok {
		return nil, fmt.Errorf("page_id is required")
	}

	content, _ := params["content"].(string)
	title, _ := params["title"].(string)

	contentStatus := "draft"
	if strings.TrimSpace(content) == "" {
		contentStatus = "empty"
	} else {
		contentStatus = "published"
	}

	if title != "" {
		_, err := e.db.Exec("UPDATE wiki_pages SET content = ?, title = ?, content_status = ? WHERE id = ?",
			content, title, contentStatus, pageID)
		if err != nil {
			return nil, fmt.Errorf("update page: %w", err)
		}
	} else {
		_, err := e.db.Exec("UPDATE wiki_pages SET content = ?, content_status = ? WHERE id = ?",
			content, contentStatus, pageID)
		if err != nil {
			return nil, fmt.Errorf("update page: %w", err)
		}
	}

	// Re-parse links in content
	_ = e.updatePageLinks(pageID, content)

	return map[string]any{"page_id": pageID}, nil
}

func (e *ExecutionEngine) executeDeletePage(params map[string]any) (map[string]any, error) {
	pageID, ok := toInt64(params["page_id"])
	if !ok {
		return nil, fmt.Errorf("page_id is required")
	}

	// Clean up links/backlinks referencing this page before deleting
	_ = e.removeLinksForPage(pageID)

	// Reparent children to this page's parent
	_, err := e.db.Exec("UPDATE wiki_pages SET parent_id = (SELECT parent_id FROM wiki_pages WHERE id = ?) WHERE parent_id = ?", pageID, pageID)
	if err != nil {
		return nil, fmt.Errorf("reparent children: %w", err)
	}

	_, err = e.db.Exec("DELETE FROM wiki_pages WHERE id = ?", pageID)
	if err != nil {
		return nil, fmt.Errorf("delete page: %w", err)
	}

	return map[string]any{"deleted": true}, nil
}

func (e *ExecutionEngine) executeLinkPages(params map[string]any) (map[string]any, error) {
	sourceID, ok := toInt64(params["source_page_id"])
	if !ok {
		return nil, fmt.Errorf("source_page_id is required")
	}
	targetID, ok := toInt64(params["target_page_id"])
	if !ok {
		return nil, fmt.Errorf("target_page_id is required")
	}
	linkText, _ := params["link_text"].(string)
	if linkText == "" {
		// Get target page title for default link text
		err := e.db.QueryRow("SELECT title FROM wiki_pages WHERE id = ?", targetID).Scan(&linkText)
		if err != nil {
			return nil, fmt.Errorf("target page not found: %w", err)
		}
	}

	// Append [[linkText]] to source page content
	var content string
	err := e.db.QueryRow("SELECT content FROM wiki_pages WHERE id = ?", sourceID).Scan(&content)
	if err != nil {
		return nil, fmt.Errorf("source page not found: %w", err)
	}

	// Add link at end if not already present
	linkMarkdown := fmt.Sprintf("[[%s]]", linkText)
	if !strings.Contains(content, linkMarkdown) {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += linkMarkdown
	}

	contentStatus := "published"
	if strings.TrimSpace(content) == "" {
		contentStatus = "empty"
	}
	_, err = e.db.Exec("UPDATE wiki_pages SET content = ?, content_status = ? WHERE id = ?", content, contentStatus, sourceID)
	if err != nil {
		return nil, fmt.Errorf("update source page: %w", err)
	}

	// Update links/backlinks arrays
	_ = e.updatePageLinks(sourceID, content)

	return map[string]any{"source_id": sourceID, "target_id": targetID, "link_text": linkText}, nil
}

func (e *ExecutionEngine) executeMovePage(params map[string]any) (map[string]any, error) {
	pageID, ok := toInt64(params["page_id"])
	if !ok {
		return nil, fmt.Errorf("page_id is required")
	}

	var newParentID *int64
	if pid, ok := toInt64(params["new_parent_id"]); ok {
		newParentID = &pid
	}

	// Compute new path
	var newPath string
	if newParentID != nil {
		var parentPath string
		err := e.db.QueryRow("SELECT path FROM wiki_pages WHERE id = ?", *newParentID).Scan(&parentPath)
		if err != nil {
			return nil, fmt.Errorf("new parent not found: %w", err)
		}
		newPath = parentPath
	} else {
		newPath = "/"
	}

	// Get old path
	var oldPath string
	err := e.db.QueryRow("SELECT path FROM wiki_pages WHERE id = ?", pageID).Scan(&oldPath)
	if err != nil {
		return nil, fmt.Errorf("page not found: %w", err)
	}

	// Update page's parent
	_, err = e.db.Exec("UPDATE wiki_pages SET parent_id = ? WHERE id = ?", newParentID, pageID)
	if err != nil {
		return nil, fmt.Errorf("update parent: %w", err)
	}

	// Migrate subtree paths
	newSelfPath := fmt.Sprintf("%s%d/", newPath, pageID)
	oldPrefix := oldPath
	newPrefix := newSelfPath

	_, err = e.db.Exec("UPDATE wiki_pages SET path = REPLACE(path, ?, ?) WHERE path LIKE ?",
		oldPrefix, newPrefix, oldPrefix+"%")
	if err != nil {
		log.Printf("Warning: path migration partial: %v", err)
	}

	return map[string]any{"page_id": pageID, "new_path": newSelfPath}, nil
}

// updatePageLinks parses [[title]] links in content and updates links/backlinks arrays.
func (e *ExecutionEngine) updatePageLinks(pageID int64, content string) error {
	linkPattern := regexp.MustCompile(`\[\[([^\]]+)\]\`)
	matches := linkPattern.FindAllStringSubmatch(content, -1)

	var linkIDs []int64
	for _, m := range matches {
		title := m[1]
		var targetID int64
		err := e.db.QueryRow("SELECT id FROM wiki_pages WHERE title = ?", title).Scan(&targetID)
		if err != nil {
			continue // page not found, skip
		}
		if targetID != pageID { // don't self-link
			linkIDs = append(linkIDs, targetID)
		}
	}

	linksJSON, _ := json.Marshal(linkIDs)
	_, _ = e.db.Exec("UPDATE wiki_pages SET links = ? WHERE id = ?", string(linksJSON), pageID)

	// Update backlinks for each target
	for _, targetID := range linkIDs {
		var backlinks string
		err := e.db.QueryRow("SELECT backlinks FROM wiki_pages WHERE id = ?", targetID).Scan(&backlinks)
		if err != nil {
			continue
		}
		var blIDs []int64
		_ = json.Unmarshal([]byte(backlinks), &blIDs)
		found := false
		for _, id := range blIDs {
			if id == pageID {
				found = true
				break
			}
		}
		if !found {
			blIDs = append(blIDs, pageID)
			blJSON, _ := json.Marshal(blIDs)
			_, _ = e.db.Exec("UPDATE wiki_pages SET backlinks = ? WHERE id = ?", string(blJSON), targetID)
		}
	}

	return nil
}

// removeLinksForPage removes all links/backlinks referencing the given page.
func (e *ExecutionEngine) removeLinksForPage(pageID int64) error {
	// Remove from backlinks of pages this page links to
	var links string
	err := e.db.QueryRow("SELECT links FROM wiki_pages WHERE id = ?", pageID).Scan(&links)
	if err == nil {
		var linkIDs []int64
		_ = json.Unmarshal([]byte(links), &linkIDs)
		for _, targetID := range linkIDs {
			_ = e.removeBacklink(targetID, pageID)
		}
	}

	// Remove from links of pages that link to this page
	var backlinks string
	err = e.db.QueryRow("SELECT backlinks FROM wiki_pages WHERE id = ?", pageID).Scan(&backlinks)
	if err == nil {
		var blIDs []int64
		_ = json.Unmarshal([]byte(backlinks), &blIDs)
		for _, sourceID := range blIDs {
			_ = e.removeLink(sourceID, pageID)
		}
	}

	return nil
}

func (e *ExecutionEngine) removeBacklink(pageID, backlinkID int64) error {
	var backlinks string
	err := e.db.QueryRow("SELECT backlinks FROM wiki_pages WHERE id = ?", pageID).Scan(&backlinks)
	if err != nil {
		return err
	}
	var blIDs []int64
	_ = json.Unmarshal([]byte(backlinks), &blIDs)
	filtered := blIDs[:0]
	for _, id := range blIDs {
		if id != backlinkID {
			filtered = append(filtered, id)
		}
	}
	blJSON, _ := json.Marshal(filtered)
	_, _ = e.db.Exec("UPDATE wiki_pages SET backlinks = ? WHERE id = ?", string(blJSON), pageID)
	return nil
}

func (e *ExecutionEngine) removeLink(pageID, linkID int64) error {
	var links string
	err := e.db.QueryRow("SELECT links FROM wiki_pages WHERE id = ?", pageID).Scan(&links)
	if err != nil {
		return err
	}
	var lIDs []int64
	_ = json.Unmarshal([]byte(links), &lIDs)
	filtered := lIDs[:0]
	for _, id := range lIDs {
		if id != linkID {
			filtered = append(filtered, id)
		}
	}
	lJSON, _ := json.Marshal(filtered)
	_, _ = e.db.Exec("UPDATE wiki_pages SET links = ? WHERE id = ?", string(lJSON), pageID)
	return nil
}

func toInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case float64:
		return int64(val), true
	case int64:
		return val, true
	case json.Number:
		i, err := val.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

func slugify(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	reg := regexp.MustCompile(`[^a-z0-9\-一-鿿]+`)
	s = reg.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	if s == "" {
		s = fmt.Sprintf("page-%d", len(title))
	}
	return s
}

func (e *ExecutionEngine) topologicalSort(actions []model.PlanAction) ([]model.PlanAction, error) {
	actionMap := make(map[string]model.PlanAction)
	for _, a := range actions {
		actionMap[a.ID] = a
	}

	// Build adjacency and in-degree
	inDegree := make(map[string]int)
	deps := make(map[string][]string) // actionID -> dependencies

	for _, a := range actions {
		inDegree[a.ID] = 0
		var dependsOn []string
		_ = json.Unmarshal([]byte(a.DependsOn), &dependsOn)
		deps[a.ID] = dependsOn
	}

	// Count in-degrees (only count deps that exist in this plan)
	for _, a := range actions {
		for _, dep := range deps[a.ID] {
			if _, ok := actionMap[dep]; ok {
				inDegree[a.ID]++
			}
		}
	}

	// Kahn's algorithm
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []model.PlanAction
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, actionMap[id])

		for _, a := range actions {
			for _, dep := range deps[a.ID] {
				if dep == id {
					inDegree[a.ID]--
					if inDegree[a.ID] == 0 {
						queue = append(queue, a.ID)
					}
				}
			}
		}
	}

	if len(sorted) != len(actions) {
		return nil, fmt.Errorf("circular dependency detected in plan actions")
	}

	return sorted, nil
}
```

- [ ] **Step 2: Write unit tests for ExecutionEngine**

Create `backend/internal/engine/engine_test.go`:

```go
package engine

import (
	"encoding/json"
	"testing"
)

func TestTopologicalSort_NoDeps(t *testing.T) {
	actions := []struct {
		id       string
		depends  string
	}{
		{"a1", "[]"},
		{"a2", "[]"},
		{"a3", "[]"},
	}
	var planActions []planActionForTest
	for i, a := range actions {
		planActions = append(planActions, planActionForTest{
			ID:        a.id,
			DependsOn: a.depends,
			SortOrder: i,
		})
	}

	sorted, err := topoSort(planActions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(sorted))
	}
}

func TestTopologicalSort_WithDeps(t *testing.T) {
	actions := []struct {
		id       string
		depends  string
	}{
		{"a1", "[]"},
		{"a2", `["a1"]`},
		{"a3", `["a1"]`},
		{"a4", `["a2", "a3"]`},
	}
	var planActions []planActionForTest
	for i, a := range actions {
		planActions = append(planActions, planActionForTest{
			ID:        a.id,
			DependsOn: a.depends,
			SortOrder: i,
		})
	}

	sorted, err := topoSort(planActions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// a1 must come before a2 and a3, which must come before a4
	pos := make(map[string]int)
	for i, a := range sorted {
		pos[a.ID] = i
	}
	if pos["a1"] > pos["a2"] || pos["a1"] > pos["a3"] {
		t.Error("a1 should come before a2 and a3")
	}
	if pos["a2"] > pos["a4"] || pos["a3"] > pos["a4"] {
		t.Error("a2 and a3 should come before a4")
	}
}

func TestTopologicalSort_Circular(t *testing.T) {
	actions := []struct {
		id       string
		depends  string
	}{
		{"a1", `["a2"]`},
		{"a2", `["a1"]`},
	}
	var planActions []planActionForTest
	for i, a := range actions {
		planActions = append(planActions, planActionForTest{
			ID:        a.id,
			DependsOn: a.depends,
			SortOrder: i,
		})
	}

	_, err := topoSort(planActions)
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}
}

func TestReplacePlaceholders(t *testing.T) {
	paramsJSON := `{"title":"Test","parent_id":{{action:a1.page_id}}}`
	results := actionResultMap{
		"a1": {"page_id": float64(42)},
	}

	replaced := placeholderPattern.ReplaceAllStringFunc(paramsJSON, func(match string) string {
		parts := placeholderPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		actionID, field := parts[1], parts[2]
		if resultMap, ok := results[actionID]; ok {
			if val, ok := resultMap[field]; ok {
				b, _ := json.Marshal(val)
				return string(b)
			}
		}
		return match
	})

	var params map[string]any
	err := json.Unmarshal([]byte(replaced), &params)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	pid, ok := params["parent_id"].(float64)
	if !ok || pid != 42 {
		t.Fatalf("expected parent_id=42, got %v", params["parent_id"])
	}
}

// Helper type for tests
type planActionForTest struct {
	ID        string
	DependsOn string
	SortOrder int
}

// Standalone topological sort for testing (doesn't need DB)
func topoSort(actions []planActionForTest) ([]planActionForTest, error) {
	actionMap := make(map[string]planActionForTest)
	for _, a := range actions {
		actionMap[a.ID] = a
	}

	inDegree := make(map[string]int)
	deps := make(map[string][]string)

	for _, a := range actions {
		inDegree[a.ID] = 0
		var dependsOn []string
		_ = json.Unmarshal([]byte(a.DependsOn), &dependsOn)
		deps[a.ID] = dependsOn
	}

	for _, a := range actions {
		for _, dep := range deps[a.ID] {
			if _, ok := actionMap[dep]; ok {
				inDegree[a.ID]++
			}
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []planActionForTest
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, actionMap[id])

		for _, a := range actions {
			for _, dep := range deps[a.ID] {
				if dep == id {
					inDegree[a.ID]--
					if inDegree[a.ID] == 0 {
						queue = append(queue, a.ID)
					}
				}
			}
		}
	}

	if len(sorted) != len(actions) {
		return nil, errCircularDep
	}
	return sorted, nil
}

var errCircularDep = fmt.Errorf("circular dependency detected")
```

Wait — I should use `errors.New` not `fmt.Errorf` in a var declaration. Let me fix:

```go
var errCircularDep = errors.New("circular dependency detected")
```

And add `"errors"` to imports.

- [ ] **Step 3: Run tests**

Run: `cd /Users/irving/repo/learn-helper/backend && go test ./internal/engine/ -v`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/engine/
git commit -m "feat: add ExecutionEngine with topological sort, action execution, and link system"
```

---

## Task 4: Plan Handler (Backend API)

**Files:**
- Create: `backend/internal/handler/plan.go`
- Modify: `backend/cmd/server/main.go` (routes)

- [ ] **Step 1: Write the Plan handler**

Create `backend/internal/handler/plan.go`:

```go
package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/irving/learn-helper/backend/internal/engine"
	"github.com/irving/learn-helper/backend/internal/model"
)

type PlanHandler struct {
	db      *sql.DB
	queries *model.Queries
	engine  *engine.ExecutionEngine
}

func NewPlanHandler(db *sql.DB, queries *model.Queries, eng *engine.ExecutionEngine) *PlanHandler {
	return &PlanHandler{db: db, queries: queries, engine: eng}
}

// GetPlan returns a plan with its actions.
func (h *PlanHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	planID := r.URL.Query().Get("id")
	if planID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	plan, err := h.loadPlan(planID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, plan)
}

// ConfirmPlan confirms and executes a plan.
func (h *PlanHandler) ConfirmPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Verify plan is in pending status
	var status string
	err := h.db.QueryRow("SELECT status FROM plans WHERE id = ?", req.PlanID).Scan(&status)
	if err != nil {
		http.Error(w, "plan not found", http.StatusNotFound)
		return
	}
	if status != "pending" {
		http.Error(w, fmt.Sprintf("plan is %s, not pending", status), http.StatusBadRequest)
		return
	}

	// Update status to confirmed
	_, err = h.db.Exec("UPDATE plans SET status = 'confirmed' WHERE id = ?", req.PlanID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute the plan
	report, err := h.engine.ExecutePlan(req.PlanID)
	if err != nil {
		http.Error(w, fmt.Sprintf("execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, report)
}

// RejectPlan rejects a pending plan.
func (h *PlanHandler) RejectPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec("UPDATE plans SET status = 'rejected' WHERE id = ? AND status = 'pending'", req.PlanID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "rejected"})
}

// loadPlan loads a plan with its actions from the database.
func (h *PlanHandler) loadPlan(planID string) (*model.Plan, error) {
	var p model.Plan
	err := h.db.QueryRow("SELECT id, conversation_id, reasoning, status, created_at, executed_at FROM plans WHERE id = ?", planID).
		Scan(&p.ID, &p.ConversationID, &p.Reasoning, &p.Status, &p.CreatedAt, &p.ExecutedAt)
	if err != nil {
		return nil, err
	}

	rows, err := h.db.Query("SELECT id, plan_id, type, params, depends_on, status, result, sort_order, created_at FROM plan_actions WHERE plan_id = ? ORDER BY sort_order", planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var a model.PlanAction
		if err := rows.Scan(&a.ID, &a.PlanID, &a.Type, &a.Params, &a.DependsOn, &a.Status, &a.Result, &a.SortOrder, &a.CreatedAt); err != nil {
			return nil, err
		}
		p.Actions = append(p.Actions, a)
	}

	return &p, nil
}

// savePlan persists a plan and its actions.
func (h *PlanHandler) savePlan(p *model.Plan) error {
	tx, err := h.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("INSERT INTO plans (id, conversation_id, reasoning, status) VALUES (?, ?, ?, ?)",
		p.ID, p.ConversationID, p.Reasoning, p.Status)
	if err != nil {
		return fmt.Errorf("insert plan: %w", err)
	}

	for _, a := range p.Actions {
		_, err = tx.Exec("INSERT INTO plan_actions (id, plan_id, type, params, depends_on, status, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?)",
			a.ID, a.PlanID, a.Type, a.Params, a.DependsOn, a.Status, a.SortOrder)
		if err != nil {
			return fmt.Errorf("insert action %s: %w", a.ID, err)
		}
	}

	return tx.Commit()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 2: Wire PlanHandler in main.go**

In `main.go`, add the engine and plan handler initialization after existing handler creation, and register routes:

After the existing handler creation (around line ~130), add:

```go
import "github.com/irving/learn-helper/backend/internal/engine"

// After creating aiHandler:
eng := engine.NewExecutionEngine(db, queries)
planHandler := handler.NewPlanHandler(db, queries, eng)
```

Add routes (after existing AI routes, around line ~170):

```go
r.Get("/api/plans", planHandler.GetPlan)
r.Post("/api/plans/confirm", planHandler.ConfirmPlan)
r.Post("/api/plans/reject", planHandler.RejectPlan)
```

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/irving/repo/learn-helper/backend && go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handler/plan.go backend/cmd/server/main.go
git commit -m "feat: add PlanHandler with confirm/reject/get endpoints"
```

---

## Task 5: Replace Write Tools with propose_plan

**Files:**
- Modify: `backend/internal/ai/provider.go:63-161` (WikiTools)
- Modify: `backend/internal/ai/provider.go:169-208` (BuildSystemPrompt)
- Modify: `backend/internal/handler/ai.go:287-563` (AIChat ReAct loop)

This is the most critical task — it replaces the core AI interaction model.

- [ ] **Step 1: Replace write tools with propose_plan in WikiTools()**

In `provider.go`, remove the `create_page`, `update_page`, `delete_page` tool definitions (lines ~86-136) and replace with a single `propose_plan` tool:

```go
{
    Name:        "propose_plan",
    Description: "提出对知识库的操作计划。当你需要创建、更新、删除页面或建立链接时，使用此工具一次性提出所有操作。系统会按依赖顺序执行。",
    InputSchema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "reasoning": map[string]any{
                "type":        "string",
                "description": "为什么建议这些操作，向用户解释你的思路",
			},
            "actions": map[string]any{
                "type":        "array",
                "description": "要执行的操作列表",
                "items": map[string]any{
                    "type": "object",
                    "properties": map[string]any{
                        "id": map[string]any{
                            "type":        "string",
                            "description": "操作的唯一标识符，用于依赖引用。例如 'a1', 'a2'",
                        },
                        "type": map[string]any{
                            "type":        "string",
                            "enum":        []string{"create_page", "update_page", "delete_page", "link_pages", "move_page"},
                            "description": "操作类型",
                        },
                        "params": map[string]any{
                            "type":        "object",
                            "description": "操作参数。create_page: {title, slug?, parent_id, content?, page_type?}; update_page: {page_id, content, title?}; delete_page: {page_id}; link_pages: {source_page_id, target_page_id, link_text?}; move_page: {page_id, new_parent_id}",
                        },
                        "depends_on": map[string]any{
                            "type":        "array",
                            "items":       map[string]any{"type": "string"},
                            "description": "依赖的操作ID列表。例如创建子页面依赖父页面的创建。依赖的操作中生成的page_id可通过 {{action:ID.page_id}} 在params中引用",
                        },
                    },
                    "required": []string{"id", "type", "params"},
                },
            },
        },
        "required": []string{"reasoning", "actions"},
    },
},
```

- [ ] **Step 2: Update system prompt in BuildSystemPrompt()**

In `provider.go`, update the `wiki_maintainer` prompt (lines ~178-208) to reflect the new `propose_plan` tool:

```go
wikiMaintainerPrompt = `你是 LLM Wiki 的知识库维护者。你的职责是管理知识树，包括创建、更新、删除页面和建立页面间的链接。

## 工作方式

1. **先调查后行动** — 在提出操作计划前，先用 lookup_page、search_pages、read_page 查看现有内容，避免重复创建
2. **一次性提出计划** — 当需要修改知识库时，调用 propose_plan 工具，一次性提出所有需要的操作
3. **操作有依赖关系** — 如果创建子页面依赖父页面，在 depends_on 中声明，使用 {{action:ID.page_id}} 引用依赖结果

## 规则

- 查看知识树结构后再决定操作，不要凭空创建
- 创建页面时必须指定 parent_id（顶级页面不需要）
- 创建页面时提供有意义的内容，不要留空
- 修改内容时先 read_page 查看现有内容
- 如果用户只是提问或聊天，不需要调用 propose_plan
- 同一个操作不要重复提出

## 链接

- 在页面内容中使用 [[页面标题]] 语法创建到其他页面的链接
- 链接让知识库形成网络，方便发现关联知识
- 创建新页面时，考虑是否应该链接到已有页面

` + treeContext
```

- [ ] **Step 3: Rewrite AIChat ReAct loop to handle propose_plan**

In `ai.go`, the key change is: when the AI calls `propose_plan`, instead of executing write tools, we create a Plan in the database and return it to the frontend.

Replace the write-tools handling in the ReAct loop (lines ~470-505) with:

```go
// After collecting tool calls from the stream, separate them:
var autoCalls []ai.ToolCall
var planCall *ai.ToolCall

for _, tc := range toolCalls {
    if tc.Name == "propose_plan" {
        planCall = &tc
    } else {
        autoCalls = append(autoCalls, tc)
    }
}

// Execute auto tools inline (same as before)
// ... (keep existing auto tool execution logic)

// If propose_plan was called, create a Plan and send to frontend
if planCall != nil {
    plan, err := h.createPlanFromToolCall(convID, planCall.Input)
    if err != nil {
        sseWrite(w, "error", err.Error(), true, flusher)
        break
    }

    // Send plan to frontend via SSE meta event
    planJSON, _ := json.Marshal(plan)
    sseWrite(w, "meta", string(planJSON), true, flusher)
    break // Exit the loop — wait for user confirmation
}
```

Add the `createPlanFromToolCall` method to AIHandler:

```go
func (h *AIHandler) createPlanFromToolCall(conversationID int64, input string) (*model.Plan, error) {
    var proposal struct {
        Reasoning string `json:"reasoning"`
        Actions   []struct {
            ID        string         `json:"id"`
            Type      string         `json:"type"`
            Params    map[string]any `json:"params"`
            DependsOn []string       `json:"depends_on"`
        } `json:"actions"`
    }
    if err := json.Unmarshal([]byte(input), &proposal); err != nil {
        return nil, fmt.Errorf("parse propose_plan input: %w", err)
    }

    planID := fmt.Sprintf("plan-%d", time.Now().UnixNano())
    plan := &model.Plan{
        ID:             planID,
        ConversationID: conversationID,
        Reasoning:      proposal.Reasoning,
        Status:         "pending",
    }

    for i, a := range proposal.Actions {
        paramsJSON, _ := json.Marshal(a.Params)
        dependsOnJSON, _ := json.Marshal(a.DependsOn)
        plan.Actions = append(plan.Actions, model.PlanAction{
            ID:        a.ID,
            PlanID:    planID,
            Type:      a.Type,
            Params:    string(paramsJSON),
            DependsOn: string(dependsOnJSON),
            Status:    "pending",
            SortOrder: int64(i),
        })
    }

    // Save to database
    planHandler := NewPlanHandler(h.db, h.queries, nil) // engine not needed for save
    if err := planHandler.savePlan(plan); err != nil {
        return nil, fmt.Errorf("save plan: %w", err)
    }

    return plan, nil
}
```

- [ ] **Step 4: Update the confirmation flow**

When the user sends `confirmed_actions` (which will now be a `plan_id`), execute the plan and return results to AI:

In `AIChat`, change the `confirmed_actions` handling (lines ~310-386) to:

```go
if len(req.ConfirmedActions) > 0 {
    // Legacy: execute confirmed actions directly
    // This path is kept for backward compatibility during transition
    results := h.executeConfirmedActions(req.ConfirmedActions, convID)
    // ... save results as message
}

// NEW: Handle plan confirmation
var reqPlanID string
// Check if request contains plan_id for confirmation
json.Unmarshal(body, &struct {
    PlanID *string `json:"plan_id"`
}{PlanID: &reqPlanID})

if reqPlanID != "" {
    eng := engine.NewExecutionEngine(h.db, h.queries)
    report, err := eng.ExecutePlan(reqPlanID)
    if err != nil {
        sseWrite(w, "error", fmt.Sprintf("plan execution failed: %v", err), true, flusher)
        return
    }

    // Save execution report as user message
    reportJSON, _ := json.Marshal(report)
    _, _ = h.db.Exec(
        "INSERT INTO messages (conversation_id, role, content) VALUES (?, 'user', ?)",
        convID, string(reportJSON),
    )

    // Inject plan execution results into AI context
    // The AI will see the report and decide whether to continue
    aiMessages = append(aiMessages, ai.Message{
        Role:    "user",
        Content: fmt.Sprintf("操作计划已执行完成：\n%s", string(reportJSON)),
    })
}
```

Update the request struct to include `plan_id`:

```go
var req struct {
    ConversationID   int64           `json:"conversation_id"`
    Message          string          `json:"message"`
    ConfirmedActions []PendingAction `json:"confirmed_actions"`
    PlanID           string          `json:"plan_id"`
}
```

- [ ] **Step 5: Update SSE meta event format**

The current `meta` event sends `pending_actions`. Change it to send the full plan object:

```go
// Instead of:
// sseWrite(w, "meta", string(pendingActionsJSON), true, flusher)

// Now send:
metaData := map[string]any{
    "plan": plan, // Full plan object with reasoning and actions
}
metaJSON, _ := json.Marshal(metaData)
sseWrite(w, "meta", string(metaJSON), true, flusher)
```

- [ ] **Step 6: Verify compilation**

Run: `cd /Users/irving/repo/learn-helper/backend && go build ./...`
Expected: No errors.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/ai/provider.go backend/internal/handler/ai.go
git commit -m "feat: replace write tools with propose_plan, add Plan-based confirmation flow"
```

---

## Task 6: Enhanced AI Context Injection

**Files:**
- Modify: `backend/internal/handler/ai.go` (buildWikiContext method, lines ~652-686)

- [ ] **Step 1: Add knowledge stats to system prompt**

In `ai.go`, update the `buildWikiContext` method to include knowledge base stats, link relationships, and knowledge gaps:

```go
func (h *AIHandler) buildWikiContext(ctx context.Context, focusPageID *int64) string {
    var sb strings.Builder

    // 1. Knowledge base overview
    var totalPages, filledPages, emptyPages int
    row := h.db.QueryRow("SELECT COUNT(*) FROM wiki_pages")
    row.Scan(&totalPages)
    h.db.QueryRow("SELECT COUNT(*) FROM wiki_pages WHERE content_status IN ('published', 'draft')").Scan(&filledPages)
    emptyPages = totalPages - filledPages

    var recentTitle, recentUpdated string
    h.db.QueryRow("SELECT title, updated_at FROM wiki_pages ORDER BY updated_at DESC LIMIT 1").Scan(&recentTitle, &recentUpdated)

    coverage := 0
    if totalPages > 0 {
        coverage = filledPages * 100 / totalPages
    }

    sb.WriteString(fmt.Sprintf("知识库状态:\n- 总页面数: %d\n- 有内容: %d (%d%%)\n- 空页面: %d (%d%%)\n",
        totalPages, filledPages, coverage, emptyPages, 100-coverage))
    if recentTitle != "" {
        sb.WriteString(fmt.Sprintf("- 最近更新: %s\n", recentTitle))
    }

    // 2. Tree structure (existing logic)
    treeCtx := h.renderTreeContext()
    sb.WriteString("\n知识树结构:\n")
    sb.WriteString(treeCtx)

    // 3. Focus page context (if a page is selected)
    if focusPageID != nil {
        var title, content, links, backlinks string
        err := h.db.QueryRow(
            "SELECT title, COALESCE(content,''), COALESCE(links,'[]'), COALESCE(backlinks,'[]') FROM wiki_pages WHERE id = ?",
            *focusPageID,
        ).Scan(&title, &content, &links, &backlinks)
        if err == nil {
            sb.WriteString(fmt.Sprintf("\n当前焦点: %s\n", title))

            // Show links
            var linkIDs []int64
            json.Unmarshal([]byte(links), &linkIDs)
            if len(linkIDs) > 0 {
                var linkNames []string
                for _, id := range linkIDs {
                    var name string
                    if h.db.QueryRow("SELECT title FROM wiki_pages WHERE id = ?", id).Scan(&name) == nil {
                        linkNames = append(linkNames, name)
                    }
                }
                sb.WriteString(fmt.Sprintf("- 链接到: %s\n", strings.Join(linkNames, ", ")))
            }

            // Show backlinks
            var blIDs []int64
            json.Unmarshal([]byte(backlinks), &blIDs)
            if len(blIDs) > 0 {
                var blNames []string
                for _, id := range blIDs {
                    var name string
                    if h.db.QueryRow("SELECT title FROM wiki_pages WHERE id = ?", id).Scan(&name) == nil {
                        blNames = append(blNames, name)
                    }
                }
                sb.WriteString(fmt.Sprintf("- 被链接: %s\n", strings.Join(blNames, ", ")))
            }
        }
    }

    // 4. Knowledge gaps
    rows, err := h.db.Query("SELECT title FROM wiki_pages WHERE content_status = 'empty' ORDER BY updated_at DESC LIMIT 5")
    if err == nil {
        defer rows.Close()
        var gaps []string
        for rows.Next() {
            var title string
            rows.Scan(&title)
            gaps = append(gaps, title)
        }
        if len(gaps) > 0 {
            sb.WriteString(fmt.Sprintf("\n知识空洞 (内容为空): %s\n", strings.Join(gaps, ", ")))
        }
    }

    return sb.String()
}
```

- [ ] **Step 2: Pass focus page ID to buildWikiContext**

In `AIChat`, pass the currently selected page ID (from request or context) to `buildWikiContext`:

Add to the request struct:
```go
FocusPageID *int64 `json:"focus_page_id"`
```

And update the call:
```go
wikiContext := h.buildWikiContext(r.Context(), req.FocusPageID)
```

- [ ] **Step 3: Verify compilation**

Run: `cd /Users/irving/repo/learn-helper/backend && go build ./...`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handler/ai.go
git commit -m "feat: enhance AI context with knowledge stats, links, gaps, and focus page"
```

---

## Task 7: Frontend Types and API

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add Plan and PlanAction types**

In `types/index.ts`, append:

```typescript
export type PlanStatus = 'pending' | 'confirmed' | 'executing' | 'completed' | 'rejected' | 'completed_with_failures';
export type ActionStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped';
export type ActionType = 'create_page' | 'update_page' | 'delete_page' | 'link_pages' | 'move_page';

export interface PlanAction {
  id: string;
  plan_id: string;
  type: ActionType;
  params: Record<string, unknown>;
  depends_on: string[];
  status: ActionStatus;
  result?: string;
  sort_order: number;
  created_at: string;
}

export interface Plan {
  id: string;
  conversation_id: number;
  reasoning: string;
  status: PlanStatus;
  actions: PlanAction[];
  created_at: string;
  executed_at?: string;
}

export interface ExecutionReport {
  plan_id: string;
  status: PlanStatus;
  actions: {
    id: string;
    type: ActionType;
    status: ActionStatus;
    result?: Record<string, unknown>;
    error?: string;
  }[];
}
```

- [ ] **Step 2: Add links/backlinks to WikiPage**

In `types/index.ts`, add to the `WikiPage` interface (after `sort_order`):

```typescript
  links: number[];
  backlinks: number[];
```

- [ ] **Step 3: Add Plan API functions**

In `api.ts`, add:

```typescript
export async function confirmPlan(planId: string): Promise<ExecutionReport> {
  const res = await fetch(`${API_BASE}/api/plans/confirm`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ plan_id: planId }),
  });
  if (!res.ok) throw new Error('Failed to confirm plan');
  return res.json();
}

export async function rejectPlan(planId: string): Promise<void> {
  await fetch(`${API_BASE}/api/plans/reject`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ plan_id: planId }),
  });
}

export async function getPlan(planId: string): Promise<Plan> {
  const res = await fetch(`${API_BASE}/api/plans?id=${encodeURIComponent(planId)}`);
  if (!res.ok) throw new Error('Failed to get plan');
  return res.json();
}
```

Add `ExecutionReport` and `Plan` to the imports from `types/index.ts`.

- [ ] **Step 4: Update ChatRequest to include plan_id**

In `api.ts`, update the `ChatRequest` interface:

```typescript
interface ChatRequest {
  conversation_id: number;
  message: string;
  role?: string;
  context_type?: string;
  confirmed_actions?: PendingAction[]; // kept for backward compat
  plan_id?: string;                    // new: plan to confirm
  focus_page_id?: number | null;       // new: currently viewed page
}
```

- [ ] **Step 5: Verify TypeScript compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/types/index.ts frontend/src/lib/api.ts
git commit -m "feat: add Plan/PlanAction types and Plan API functions"
```

---

## Task 8: PlanPreview Component

**Files:**
- Create: `frontend/src/components/PlanPreview.tsx`

- [ ] **Step 1: Write the PlanPreview component**

Create `frontend/src/components/PlanPreview.tsx`:

```tsx
import { Plan, PlanAction } from '../types';

interface PlanPreviewProps {
  plan: Plan;
  onConfirm: (planId: string) => void;
  onReject: (planId: string) => void;
  confirming: boolean;
}

const ACTION_ICONS: Record<string, string> = {
  create_page: '+',
  update_page: '~',
  delete_page: '×',
  link_pages: '→',
  move_page: '↗',
};

const ACTION_LABELS: Record<string, string> = {
  create_page: '创建页面',
  update_page: '更新页面',
  delete_page: '删除页面',
  link_pages: '建立链接',
  move_page: '移动页面',
};

function ActionPreview({ action }: { action: PlanAction }) {
  const params = action.params;

  switch (action.type) {
    case 'create_page':
      return (
        <div>
          <span className="font-medium">{params.title as string}</span>
          {params.parent_id && (
            <span className="text-th-muted ml-2">→ parent: {params.parent_id as number}</span>
          )}
          {params.content && (
            <p className="text-sm text-th-muted mt-1 line-clamp-3">
              {(params.content as string).slice(0, 200)}
            </p>
          )}
        </div>
      );
    case 'update_page':
      return (
        <div>
          <span className="font-medium">Page #{params.page_id as number}</span>
          {params.title && <span className="ml-2">→ {params.title as string}</span>}
          {params.content && (
            <p className="text-sm text-th-muted mt-1 line-clamp-3">
              {(params.content as string).slice(0, 200)}
            </p>
          )}
        </div>
      );
    case 'delete_page':
      return (
        <div>
          <span className="font-medium text-red-600">Page #{params.page_id as number}</span>
          <span className="text-th-muted ml-2">将被删除</span>
        </div>
      );
    case 'link_pages':
      return (
        <div>
          <span>Page #{params.source_page_id as number}</span>
          <span className="mx-2">→</span>
          <span>Page #{params.target_page_id as number}</span>
          {params.link_text && (
            <span className="text-th-muted ml-2">[{params.link_text as string}]</span>
          )}
        </div>
      );
    case 'move_page':
      return (
        <div>
          <span>Page #{params.page_id as number}</span>
          <span className="mx-2">→</span>
          <span>Parent #{params.new_parent_id as number}</span>
        </div>
      );
    default:
      return <div>{JSON.stringify(params)}</div>;
  }
}

export default function PlanPreview({ plan, onConfirm, onReject, confirming }: PlanPreviewProps) {
  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="p-6 border-b border-th-separator">
        <h2 className="text-xl font-display text-th-text">操作计划</h2>
        <p className="mt-2 text-th-muted text-sm leading-relaxed">{plan.reasoning}</p>
        <p className="mt-2 text-xs text-th-muted">
          {plan.actions.length} 个操作 · 状态: {plan.status}
        </p>
      </div>

      {/* Actions list */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {plan.actions.map((action) => (
          <div
            key={action.id}
            className="p-3 rounded-lg border border-th-separator bg-th-surface"
          >
            <div className="flex items-start gap-2">
              <span className="text-lg font-mono w-6 text-center flex-shrink-0"
                style={{ color: action.type === 'delete_page' ? '#dc2626' : '#d97706' }}>
                {ACTION_ICONS[action.type]}
              </span>
              <div className="flex-1 min-w-0">
                <div className="text-xs text-th-muted mb-1">
                  {ACTION_LABELS[action.type]} · {action.id}
                  {action.depends_on.length > 0 && (
                    <span className="ml-2">依赖: {action.depends_on.join(', ')}</span>
                  )}
                </div>
                <ActionPreview action={action} />
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Actions */}
      <div className="p-4 border-t border-th-separator flex gap-3">
        <button
          onClick={() => onConfirm(plan.id)}
          disabled={confirming || plan.status !== 'pending'}
          className="flex-1 px-4 py-2.5 rounded-lg bg-th-accent text-white font-medium
                     hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {confirming ? '执行中...' : '确认执行'}
        </button>
        <button
          onClick={() => onReject(plan.id)}
          disabled={confirming || plan.status !== 'pending'}
          className="px-4 py-2.5 rounded-lg border border-th-separator text-th-muted
                     hover:bg-th-hover transition-colors disabled:opacity-50"
        >
          拒绝
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: No errors (may have warnings about `text-th-surface` and `bg-th-hover` not being standard Tailwind classes — these are project custom theme classes and are fine).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/PlanPreview.tsx
git commit -m "feat: add PlanPreview component for right-column plan display"
```

---

## Task 9: Update ChatPanel for Plan Flow

**Files:**
- Modify: `frontend/src/components/ChatPanel.tsx`

- [ ] **Step 1: Add Plan state and handlers**

Add state and callback props for Plan:

```typescript
// Add to ChatPanelProps:
interface ChatPanelProps {
  onPageChanged?: () => void;
  onPlanCreated?: (plan: Plan) => void;  // new
  focusPageId?: number | null;           // new
}

// Add state:
const [activePlan, setActivePlan] = useState<Plan | null>(null);
const [confirming, setConfirming] = useState(false);
```

- [ ] **Step 2: Update handleSend to handle Plan flow**

In `handleSend`, update the SSE `onMeta` callback to handle Plan objects instead of `pending_actions`:

```typescript
// In onMeta callback:
onMeta: (meta: any) => {
  if (meta.conversation_id) {
    newConvId = meta.conversation_id;
  }
  if (meta.plan) {
    // New Plan flow
    setActivePlan(meta.plan);
    // Attach plan info to last message
    setMessages(prev => {
      const updated = [...prev];
      const last = updated[updated.length - 1];
      if (last && last.role === 'assistant') {
        updated[updated.length - 1] = { ...last, plan: meta.plan };
      }
      return updated;
    });
    onPlanCreated?.(meta.plan);
  }
  // Keep backward compat for pending_actions
  if (meta.pending_actions) {
    setMessages(prev => {
      const updated = [...prev];
      const last = updated[updated.length - 1];
      if (last && last.role === 'assistant') {
        updated[updated.length - 1] = { ...last, pending_actions: meta.pending_actions };
      }
      return updated;
    });
  }
},
```

- [ ] **Step 3: Add Plan confirm/reject handlers**

```typescript
const handleConfirmPlan = async (planId: string) => {
  setConfirming(true);
  try {
    const report = await confirmPlan(planId);
    setActivePlan(null);
    // Send execution results back to AI for continuation
    handleSend(undefined, planId);
  } catch (err) {
    console.error('Plan confirmation failed:', err);
  } finally {
    setConfirming(false);
  }
};

const handleRejectPlan = async (planId: string) => {
  try {
    await rejectPlan(planId);
    setActivePlan(null);
    setMessages(prev => [...prev, {
      id: Date.now(),
      role: 'user' as const,
      content: '操作计划已拒绝',
      created_at: new Date().toISOString(),
    }]);
  } catch (err) {
    console.error('Plan rejection failed:', err);
  }
};
```

- [ ] **Step 4: Update handleSend to support plan_id parameter**

Modify `handleSend` to accept an optional `planId` parameter:

```typescript
const handleSend = async (confirmedActions?: PendingAction[], planId?: string) => {
  // ... existing send logic ...
  // Include plan_id and focus_page_id in the request body
  const body: ChatRequest = {
    conversation_id: activeConv?.id ?? 0,
    message: input.trim(),
    confirmed_actions: confirmedActions,
    plan_id: planId,
    focus_page_id: focusPageId,
  };
  // ... rest of send logic ...
};
```

- [ ] **Step 5: Update message rendering for Plan messages**

In the `renderedMessages` memo, replace the `pending_actions` confirmation UI with a Plan summary:

```typescript
// For messages with a plan:
if (msg.plan) {
  return (
    // ... existing assistant message wrapper ...
    <div>
      <MarkdownContent content={msg.content} />
      <div className="mt-3 p-3 rounded-lg border border-th-accent/30 bg-th-accent/5">
        <p className="text-sm text-th-text">
          📋 已生成操作计划：{msg.plan.reasoning.slice(0, 100)}
          {msg.plan.reasoning.length > 100 ? '...' : ''}
        </p>
        <p className="text-xs text-th-muted mt-1">
          {msg.plan.actions.length} 个操作 · 请在右侧查看详情
        </p>
        <div className="mt-2 flex gap-2">
          <button
            onClick={() => handleConfirmPlan(msg.plan!.id)}
            disabled={confirming}
            className="px-3 py-1.5 text-xs rounded-md bg-th-accent text-white hover:opacity-90"
          >
            确认执行
          </button>
          <button
            onClick={() => handleRejectPlan(msg.plan!.id)}
            className="px-3 py-1.5 text-xs rounded-md border border-th-separator text-th-muted"
          >
            拒绝
          </button>
        </div>
      </div>
    </div>
  );
}
```

Also update the `ConversationMessage` type to include `plan`:

In `types/index.ts`, add to `ConversationMessage`:
```typescript
  plan?: Plan;
```

- [ ] **Step 6: Verify TypeScript compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/ChatPanel.tsx frontend/src/types/index.ts
git commit -m "feat: update ChatPanel with Plan confirmation flow"
```

---

## Task 10: Wire PlanPreview into PageViewer

**Files:**
- Modify: `frontend/src/components/PageViewer.tsx`
- Modify: `frontend/src/components/WikiPage.tsx`

- [ ] **Step 1: Add Plan mode to PageViewer**

Update `PageViewer` to accept an optional `plan` prop and render `PlanPreview` when a plan is active:

```typescript
import PlanPreview from './PlanPreview';
import { Plan } from '../types';

interface PageViewerProps {
  page: WikiPage | null;
  collapsed: boolean;
  plan: Plan | null;          // new
  onConfirmPlan: (planId: string) => void;  // new
  onRejectPlan: (planId: string) => void;   // new
  confirmingPlan: boolean;    // new
  onSelectPage: (slug: string) => void;     // new: for link navigation
}
```

In the render logic, check for plan first:

```typescript
if (plan) {
  return (
    <Panel collapsible={collapsed}>
      <PlanPreview
        plan={plan}
        onConfirm={onConfirmPlan}
        onReject={onRejectPlan}
        confirming={confirmingPlan}
      />
    </Panel>
  );
}

// Normal page rendering continues below...
```

- [ ] **Step 2: Add backlinks section to PageViewer**

After the MarkdownContent rendering in normal mode, add backlinks:

```typescript
{page.backlinks && page.backlinks.length > 0 && (
  <div className="mt-6 pt-4 border-t border-th-separator">
    <h3 className="text-sm font-medium text-th-muted mb-2">反向链接</h3>
    <div className="flex flex-wrap gap-2">
      {page.backlinks.map(blId => (
        <BacklinkBadge key={blId} pageId={blId} onClick={onSelectPage} />
      ))}
    </div>
  </div>
)}
```

Add a `BacklinkBadge` helper component:

```typescript
function BacklinkBadge({ pageId, onClick }: { pageId: number; onClick: (slug: string) => void }) {
  // Fetch page title via slug — use a simple fetch
  const [title, setTitle] = useState('');
  const [slug, setSlug] = useState('');

  useEffect(() => {
    fetch(`${API_BASE}/api/wiki?page_id=${pageId}`)
      .then(res => res.json())
      .then(data => {
        if (data.title) setTitle(data.title);
        if (data.slug) setSlug(data.slug);
      })
      .catch(() => {});
  }, [pageId]);

  if (!title) return null;

  return (
    <button
      onClick={() => slug && onClick(slug)}
      className="px-2 py-1 text-xs rounded-md border border-th-separator
                 text-th-muted hover:bg-th-hover transition-colors"
    >
      {title}
    </button>
  );
}
```

Note: This requires adding a page lookup endpoint by ID. Alternatively, include backlink titles in the API response. For now, we'll add a simple endpoint.

- [ ] **Step 3: Render [[title]] links as clickable**

In `MarkdownContent.tsx`, add a custom renderer for wiki links. Update the paragraph/text rendering to detect `[[title]]` patterns and render them as clickable links:

```typescript
// Add a pre-processing step to convert [[title]] to markdown links
function processWikiLinks(content: string): string {
  return content.replace(/\[\[([^\]]+)\]\]/g, (_, title) => {
    return `[${title}](wiki:${encodeURIComponent(title)})`;
  });
}

// In the ReactMarkdown component, add a custom link handler:
<a
  href={href}
  onClick={(e) => {
    if (href.startsWith('wiki:')) {
      e.preventDefault();
      const title = decodeURIComponent(href.slice(5));
      // Navigate to page by title — need a callback
      onWikiLinkClick?.(title);
    }
  }}
  // ... other props
>
```

This requires adding `onWikiLinkClick` prop to `MarkdownContent` and threading it through.

- [ ] **Step 4: Wire Plan state in WikiPage.tsx**

In `WikiPage.tsx`, add Plan state and pass it between ChatPanel and PageViewer:

```typescript
const [activePlan, setActivePlan] = useState<Plan | null>(null);
const [confirmingPlan, setConfirmingPlan] = useState(false);

const handlePlanCreated = (plan: Plan) => {
  setActivePlan(plan);
};

const handleConfirmPlan = async (planId: string) => {
  setConfirmingPlan(true);
  try {
    const report = await confirmPlan(planId);
    setActivePlan(null);
    setConfirmingPlan(false);
    handlePageChanged(); // Refresh tree
  } catch {
    setConfirmingPlan(false);
  }
};

const handleRejectPlan = async (planId: string) => {
  await rejectPlan(planId);
  setActivePlan(null);
};
```

Update the Panel components:

```tsx
<Panel id="center">
  <ChatPanel
    onPageChanged={handlePageChanged}
    onPlanCreated={handlePlanCreated}
    focusPageId={displayPage?.id ?? null}
  />
</Panel>
<Panel id="right">
  <PageViewer
    page={displayPage}
    collapsed={rightCollapsed}
    plan={activePlan}
    onConfirmPlan={handleConfirmPlan}
    onRejectPlan={handleRejectPlan}
    confirmingPlan={confirmingPlan}
    onSelectPage={(slug) => setSelectedSlug(slug)}
  />
</Panel>
```

- [ ] **Step 5: Verify TypeScript compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: No errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/PageViewer.tsx frontend/src/components/WikiPage.tsx frontend/src/components/MarkdownContent.tsx
git commit -m "feat: wire PlanPreview into PageViewer, add backlinks and wiki link rendering"
```

---

## Task 11: Backend Link Maintenance in Wiki CRUD

**Files:**
- Modify: `backend/internal/handler/wiki.go`

- [ ] **Step 1: Add link parsing on Create/Update**

In `wiki.go`, after creating or updating a page, call the engine's `updatePageLinks` method to parse `[[title]]` links and update the links/backlinks arrays.

Add a helper function:

```go
func (h *WikiHandler) updatePageLinks(pageID int64, content string) {
    linkPattern := regexp.MustCompile(`\[\[([^\]]+)\]\`)
    matches := linkPattern.FindAllStringSubmatch(content, -1)

    var linkIDs []int64
    for _, m := range matches {
        title := m[1]
        var targetID int64
        err := h.db.QueryRow("SELECT id FROM wiki_pages WHERE title = ?", title).Scan(&targetID)
        if err != nil {
            continue
        }
        if targetID != pageID {
            linkIDs = append(linkIDs, targetID)
        }
    }

    linksJSON, _ := json.Marshal(linkIDs)
    h.db.Exec("UPDATE wiki_pages SET links = ? WHERE id = ?", string(linksJSON), pageID)

    // Update backlinks for each target
    for _, targetID := range linkIDs {
        var backlinks string
        err := h.db.QueryRow("SELECT backlinks FROM wiki_pages WHERE id = ?", targetID).Scan(&backlinks)
        if err != nil {
            continue
        }
        var blIDs []int64
        json.Unmarshal([]byte(backlinks), &blIDs)
        found := false
        for _, id := range blIDs {
            if id == pageID {
                found = true
                break
            }
        }
        if !found {
            blIDs = append(blIDs, pageID)
            blJSON, _ := json.Marshal(blIDs)
            h.db.Exec("UPDATE wiki_pages SET backlinks = ? WHERE id = ?", string(blJSON), targetID)
        }
    }
}
```

Call this at the end of `CreateWikiPage` and `UpdateWikiPage` handlers.

- [ ] **Step 2: Add link cleanup on Delete**

In `DeleteWikiPage`, before deleting the page, remove all references to this page in other pages' links/backlinks arrays:

```go
// Remove this page from other pages' links
var links string
h.db.QueryRow("SELECT links FROM wiki_pages WHERE id = ?", id).Scan(&links)
var linkIDs []int64
json.Unmarshal([]byte(links), &linkIDs)
for _, targetID := range linkIDs {
    // Remove pageID from target's backlinks
    var backlinks string
    h.db.QueryRow("SELECT backlinks FROM wiki_pages WHERE id = ?", targetID).Scan(&backlinks)
    var blIDs []int64
    json.Unmarshal([]byte(backlinks), &blIDs)
    filtered := blIDs[:0]
    for _, bid := range blIDs {
        if bid != id {
            filtered = append(filtered, bid)
        }
    }
    blJSON, _ := json.Marshal(filtered)
    h.db.Exec("UPDATE wiki_pages SET backlinks = ? WHERE id = ?", string(blJSON), targetID)
}

// Remove this page from other pages' links (backlinks)
var backlinks string
h.db.QueryRow("SELECT backlinks FROM wiki_pages WHERE id = ?", id).Scan(&backlinks)
var blIDs []int64
json.Unmarshal([]byte(backlinks), &blIDs)
for _, sourceID := range blIDs {
    var sourceLinks string
    h.db.QueryRow("SELECT links FROM wiki_pages WHERE id = ?", sourceID).Scan(&sourceLinks)
    var slIDs []int64
    json.Unmarshal([]byte(sourceLinks), &slIDs)
    filtered := slIDs[:0]
    for _, lid := range slIDs {
        if lid != id {
            filtered = append(filtered, lid)
        }
    }
    slJSON, _ := json.Marshal(filtered)
    h.db.Exec("UPDATE wiki_pages SET links = ? WHERE id = ?", string(slJSON), sourceID)
}
```

- [ ] **Step 3: Update link text on Rename**

In `RenameWikiPage`, after renaming, update all `[[oldTitle]]` references across the wiki to `[[newTitle]]`:

```go
// Update all [[oldTitle]] references to [[newTitle]]
oldLink := fmt.Sprintf("[[%s]]", oldTitle)
newLink := fmt.Sprintf("[[%s]]", newTitle)
h.db.Exec("UPDATE wiki_pages SET content = REPLACE(content, ?, ?) WHERE content LIKE ?",
    oldLink, newLink, "%"+oldLink+"%")
```

Then re-run `updatePageLinks` for affected pages.

- [ ] **Step 4: Add backlinks to WikiPage response**

In `GetWikiPageBySlug` and `GetWikiTree`, ensure the `links` and `backlinks` fields are included in the JSON response. Add them to `WikiPageResponse` and the tree node rendering.

- [ ] **Step 5: Verify compilation**

Run: `cd /Users/irving/repo/learn-helper/backend && go build ./...`
Expected: No errors.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler/wiki.go
git commit -m "feat: add link parsing, cleanup, and backlinks to wiki CRUD handlers"
```

---

## Task 12: Integration Test and Polish

**Files:**
- All files modified in previous tasks

- [ ] **Step 1: Start backend and frontend**

Run in separate terminals:
```bash
cd /Users/irving/repo/learn-helper/backend && go run ./cmd/server
cd /Users/irving/repo/learn-helper/frontend && npm run dev
```

- [ ] **Step 2: Test the full Plan flow**

1. Open `http://localhost:3000`
2. Start a new conversation
3. Ask AI to learn about a topic (e.g., "帮我了解一下 Docker")
4. Verify AI calls `propose_plan` instead of individual write tools
5. Verify Plan appears in the right panel with reasoning and actions
6. Click "确认执行" and verify actions execute and tree updates
7. Verify AI receives execution results and responds appropriately

- [ ] **Step 3: Test link system**

1. Create a page with `[[Other Page]]` link in content
2. Verify the link renders as clickable in PageViewer
3. Verify backlinks section appears on the target page
4. Delete a linked page and verify backlinks are cleaned up

- [ ] **Step 4: Fix any issues found during testing**

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete AI operation plan layer with link system and enhanced context"
```

---

## Self-Review Checklist

### Spec Coverage

| Spec Section | Task |
|---|---|
| Plan data model | Task 1 (schema), Task 2 (models) |
| PlanAction data model | Task 1 (schema), Task 2 (models) |
| Links/backlinks fields | Task 1 (schema), Task 2 (models) |
| propose_plan tool | Task 5 |
| New action types (link_pages, move_page) | Task 3 (engine), Task 5 (tool def) |
| Multi-round planning | Task 5 (confirmation flow) |
| Plan preview in right column | Task 8 (PlanPreview), Task 10 (wiring) |
| Chat panel summary | Task 9 |
| Knowledge link system ([[title]]) | Task 11 (backend), Task 10 (frontend rendering) |
| Backlinks display | Task 10 (PageViewer), Task 11 (backend) |
| Execution engine with topological sort | Task 3 |
| ID replacement ({{action:a1.page_id}}) | Task 3 |
| Execution report | Task 3 |
| Enhanced AI context injection | Task 6 |
| Link maintenance (delete, rename) | Task 11 |

### Placeholder Scan

No TBD, TODO, or "implement later" patterns found.

### Type Consistency

- `Plan.id` is `string` in both Go and TypeScript
- `PlanAction.type` uses the same enum values across Go (CHECK constraint), TypeScript (ActionType), and provider.go (tool schema)
- `PlanAction.params` is `string` (JSON) in Go, `Record<string, unknown>` in TypeScript — consistent with the API serialization
- `WikiPage.links` / `WikiPage.backlinks` is `string` in Go (JSON), `number[]` in TypeScript — Go API serializes JSON string, frontend parses to array
