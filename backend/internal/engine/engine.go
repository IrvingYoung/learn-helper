package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"unicode"

	"learn-helper/internal/model"
)

// ExecutionReport summarizes the result of executing a plan.
type ExecutionReport struct {
	PlanID  string          `json:"plan_id"`
	Status  string          `json:"status"`
	Actions []ActionResult  `json:"actions"`
}

// ActionResult captures the outcome of a single action execution.
type ActionResult struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ExecutionEngine orchestrates plan execution with topological ordering,
// placeholder replacement, and wiki page mutations.
type ExecutionEngine struct {
	db            *sql.DB
	queries       *model.Queries
	onPageWritten func(pageID int64) // optional; called after a successful create/update; nil-safe
}

// NewExecutionEngine creates a new engine backed by the given database.
func NewExecutionEngine(db *sql.DB, queries *model.Queries) *ExecutionEngine {
	return &ExecutionEngine{db: db, queries: queries}
}

// SetOnPageWritten registers a callback to be invoked after a successful
// create or update action. Used by main.go to wire the SummaryWorker.
// Passing nil clears the callback.
func (e *ExecutionEngine) SetOnPageWritten(fn func(pageID int64)) {
	e.onPageWritten = fn
}

// placeholderPattern matches {{action:a1.page_id}} style references.
var placeholderPattern = regexp.MustCompile(`\{\{action:([^.}]+)\.([^}]+)\}\}`)

// ExecutePlan loads actions for the given plan, sorts them topologically,
// and executes them in order with dependency propagation.
func (e *ExecutionEngine) ExecutePlan(ctx context.Context, planID string, focusPageID *int64) (*ExecutionReport, error) {
	// 1. Load actions
	actions, err := e.loadActions(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("load actions: %w", err)
	}
	if len(actions) == 0 {
		return &ExecutionReport{PlanID: planID, Status: "completed"}, nil
	}

	// 1.4 Inject focusPageID into create_page actions that lack parent_id
	if focusPageID != nil {
		for i := range actions {
			if actions[i].Type == "create_page" && !hasParentID(actions[i].Params) {
				// Distinguish "no parent_id key" from "placeholder parent_id"
				// so we can log differently — the latter is a real risk
				// (parent may resolve to wrong page if engine continues).
				if hasPlaceholderParentID(actions[i].Params) {
					log.Printf("WARN: focus fallback applied for action %s — parent_id is unresolved placeholder, falling back to focusPageID=%d", actions[i].ID, *focusPageID)
				}
				actions[i].Params = injectParentID(actions[i].Params, *focusPageID)
			}
		}
	}

	// 1.6 Implicit dependency inference: scan each action's params for
	// {{action:X.field}} placeholders and add X to the in-memory depends_on
	// set if not already present. Only used for topo sort; the loaded
	// model.PlanAction.DependsOn is left untouched for audit.
	augmentedActions := make([]model.PlanAction, len(actions))
	for i, a := range actions {
		augmentedActions[i] = a
		declared := parseDependsOn(string(a.DependsOn))
		declaredSet := map[string]bool{}
		for _, d := range declared {
			declaredSet[d] = true
		}
		inferred := inferDependencies(string(a.Params))
		added := false
		for _, ref := range inferred {
			if !declaredSet[ref] {
				declared = append(declared, ref)
				declaredSet[ref] = true
				added = true
			}
		}
		if added {
			merged, _ := json.Marshal(declared)
			augmentedActions[i].DependsOn = merged
		}
	}

	// 2. Topological sort (using augmented depends_on; original loaded actions unchanged)
	sorted, err := topoSort(augmentedActions)
	if err != nil {
		return nil, fmt.Errorf("topological sort: %w", err)
	}

	// 3. Update plan status to 'executing'
	if _, err := e.db.ExecContext(ctx,
		`UPDATE plans SET status = 'executing' WHERE id = ?`, planID); err != nil {
		return nil, fmt.Errorf("update plan status: %w", err)
	}

	// 4. Execute actions in order
	actionResultMap := make(map[string]any) // actionID -> result
	failedSet := make(map[string]bool)
	results := make([]ActionResult, 0, len(sorted))

	for _, action := range sorted {
		ar := ActionResult{ID: action.ID, Type: action.Type}

		// Check if any dependency failed
		if e.dependsOnFailed(action, failedSet) {
			ar.Status = "skipped"
			ar.Error = "dependency failed"
			failedSet[action.ID] = true
			_ = e.updateActionStatus(ctx, action.ID, "skipped", "")
			results = append(results, ar)
			continue
		}

		// Replace placeholders in params
		resolvedParams, err := e.replacePlaceholders(string(action.Params), actionResultMap)
		if err != nil {
			ar.Status = "failed"
			ar.Error = fmt.Sprintf("replace placeholders: %v", err)
			failedSet[action.ID] = true
			_ = e.updateActionStatus(ctx, action.ID, "failed", ar.Error)
			results = append(results, ar)
			continue
		}

		_ = e.updateActionStatus(ctx, action.ID, "running", "")
		// Since this is a user-confirmed plan execution, inject published content_status
		// so pages skip the draft confirmation step.
		if action.Type == "create_page" || action.Type == "update_page" || action.Type == "patch_page" {
			resolvedParams = injectContentStatus(resolvedParams, "published")
		}

		// Execute the action
		result, err := e.executeAction(ctx, action.Type, resolvedParams)
		if err != nil {
			ar.Status = "failed"
			ar.Error = err.Error()
			failedSet[action.ID] = true
			_ = e.updateActionStatus(ctx, action.ID, "failed", ar.Error)
			results = append(results, ar)
			continue
		}

		ar.Status = "completed"
		ar.Result = result
		actionResultMap[action.ID] = result

		resultJSON, _ := json.Marshal(result)
		_ = e.updateActionStatus(ctx, action.ID, "completed", string(resultJSON))
		results = append(results, ar)
	}

	// 5. Determine final plan status
	finalStatus := "completed"
	for _, ar := range results {
		if ar.Status == "failed" || ar.Status == "skipped" {
			finalStatus = "completed_with_failures"
			break
		}
	}

	// 6. Update plan status and executed_at
	if _, err := e.db.ExecContext(ctx,
		`UPDATE plans SET status = ?, executed_at = datetime('now') WHERE id = ?`,
		finalStatus, planID); err != nil {
		log.Printf("WARN: failed to update plan final status: %v", err)
	}

	return &ExecutionReport{PlanID: planID, Status: finalStatus, Actions: results}, nil
}

// ---------------------------------------------------------------------------
// Topological sort (Kahn's algorithm)
// ---------------------------------------------------------------------------

// topoSortNode is a minimal representation used by the generic topoSort.
type topoSortNode struct {
	ID        string
	DependsOn []string
}

// topoSort performs a topological sort using Kahn's algorithm.
// Returns an error if a cycle is detected.
func topoSort(actions []model.PlanAction) ([]model.PlanAction, error) {
	// Build adjacency and in-degree maps
	nodes := make(map[string]*topoSortNode)
	actionMap := make(map[string]*model.PlanAction)
	inDegree := make(map[string]int)
	adj := make(map[string][]string) // dependency -> dependents

	for i := range actions {
		a := &actions[i]
		nodes[a.ID] = &topoSortNode{ID: a.ID, DependsOn: parseDependsOn(string(a.DependsOn))}
		actionMap[a.ID] = a
		inDegree[a.ID] = 0
		adj[a.ID] = nil
	}

	for _, node := range nodes {
		for _, dep := range node.DependsOn {
			if _, ok := nodes[dep]; !ok {
				// Reference to non-existent action; skip (will fail at execution)
				continue
			}
			adj[dep] = append(adj[dep], node.ID)
			inDegree[node.ID]++
		}
	}

	// Seed queue with zero in-degree nodes
	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []model.PlanAction
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, *actionMap[id])

		for _, dependent := range adj[id] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(sorted) != len(actions) {
		return nil, fmt.Errorf("circular dependency detected among plan actions")
	}

	return sorted, nil
}

// parseDependsOn parses a JSON array string like '["a1","a2"]' into a string slice.
func parseDependsOn(dependsOn string) []string {
	dependsOn = strings.TrimSpace(dependsOn)
	if dependsOn == "" || dependsOn == "[]" {
		return nil
	}
	var deps []string
	if err := json.Unmarshal([]byte(dependsOn), &deps); err != nil {
		// Try comma-separated fallback
		parts := strings.Split(dependsOn, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				deps = append(deps, p)
			}
		}
	}
	return deps
}

// ---------------------------------------------------------------------------
// Placeholder replacement
// ---------------------------------------------------------------------------

// replacePlaceholders replaces {{action:a1.page_id}} patterns in paramsJSON
// with actual values from the actionResultMap. If any placeholder cannot
// be resolved (referenced action not yet executed, or field missing from
// the result), it returns an error naming every unresolved placeholder.
func (e *ExecutionEngine) replacePlaceholders(paramsJSON string, actionResultMap map[string]any) (string, error) {
	if !strings.Contains(paramsJSON, "{{action:") {
		return paramsJSON, nil
	}

	var unresolved []string
	result := placeholderPattern.ReplaceAllStringFunc(paramsJSON, func(match string) string {
		subs := placeholderPattern.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		actionID := subs[1]
		field := subs[2]

		res, ok := actionResultMap[actionID]
		if !ok {
			unresolved = append(unresolved, actionID+"."+field)
			return match
		}

		val := resolveField(res, field)
		if val == nil {
			unresolved = append(unresolved, actionID+"."+field)
			return match
		}
		return fmt.Sprintf("%v", val)
	})

	if len(unresolved) > 0 {
		return "", fmt.Errorf("unresolved placeholders: %s", strings.Join(unresolved, ", "))
	}
	return result, nil
}

// resolveField extracts a field from a map result (e.g., "page_id" from a create_page result).
func resolveField(result any, field string) any {
	m, ok := result.(map[string]any)
	if !ok {
		return nil
	}
	v, ok := m[field]
	if !ok {
		return nil
	}
	return v
}

// ---------------------------------------------------------------------------
// Outline execution
// ---------------------------------------------------------------------------

// OutlineNode represents a node in the knowledge outline tree.
type OutlineNode struct {
	ID        string         `json:"id,omitempty"`
	Title     string         `json:"title"`
	PageType  string         `json:"page_type"`
	Children  []OutlineNode  `json:"children,omitempty"`
}

// ExecOutline recursively creates skeleton pages from an outline tree.
// Each page is created with empty content and content_status=empty.
// Returns a map of node ID -> create result (page_id, slug).
func (e *ExecutionEngine) ExecOutline(ctx context.Context, outlineJSON string, parentID *int64) (map[string]any, error) {
	if outlineJSON == "" {
		return nil, fmt.Errorf("exec_outline: empty outline")
	}

	var nodes []OutlineNode
	if err := json.Unmarshal([]byte(outlineJSON), &nodes); err != nil {
		return nil, fmt.Errorf("exec_outline: parse outline: %w", err)
	}

	results := make(map[string]any)
	for _, node := range nodes {
		nodeResults, err := e.execOutlineNode(ctx, node, parentID)
		if err != nil {
			return nil, fmt.Errorf("exec_outline: create %q: %w", node.Title, err)
		}
		for k, v := range nodeResults {
			results[k] = v
		}
	}
	return results, nil
}

// execOutlineNode creates a single outline node and its children recursively.
func (e *ExecutionEngine) execOutlineNode(ctx context.Context, node OutlineNode, parentID *int64) (map[string]any, error) {
	results := make(map[string]any)

	// Create the page for this node
	params := map[string]any{
		"title":    node.Title,
		"page_type": node.PageType,
	}
	if parentID != nil {
		params["parent_id"] = *parentID
	}

	createResult, err := e.execCreatePage(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create %q: %w", node.Title, err)
	}

	pageID := int64(0)
	if resultMap, ok := createResult.(map[string]any); ok {
		if id, ok := resultMap["page_id"].(int64); ok {
			pageID = id
		}
	}

	// Record result by node ID if specified
	if node.ID != "" {
		results[node.ID] = createResult
	}

	// Recursively process children
	newParentID := &pageID
	for _, child := range node.Children {
		childResults, err := e.execOutlineNode(ctx, child, newParentID)
		if err != nil {
			return nil, fmt.Errorf("child %q of %q: %w", child.Title, node.Title, err)
		}
		for k, v := range childResults {
			results[k] = v
		}
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// Action execution
// ---------------------------------------------------------------------------

func (e *ExecutionEngine) executeAction(ctx context.Context, actionType string, paramsJSON string) (any, error) {
	var params map[string]any
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return nil, fmt.Errorf("invalid params JSON: %w", err)
	}

	switch actionType {
	case "create_page":
		return e.execCreatePage(ctx, params)
	case "update_page":
		return e.execUpdatePage(ctx, params)
	case "patch_page":
		return e.execPatchPage(ctx, params)
	case "delete_page":
		return e.execDeletePage(ctx, params)
	case "link_pages":
		return e.execLinkPages(ctx, params)
	case "move_page":
		return e.execMovePage(ctx, params)
	default:
		return nil, fmt.Errorf("unknown action type: %s", actionType)
	}
}

// execCreatePage inserts a new wiki page, computes its path, and returns page_id + slug.
func (e *ExecutionEngine) execCreatePage(ctx context.Context, params map[string]any) (any, error) {
	title, _ := params["title"].(string)
	if title == "" {
		return nil, fmt.Errorf("create_page: title is required")
	}

	slug := slugify(title)
	pageType := strVal(params, "page_type", "entity")
	content := strVal(params, "content", "")
	contentStatus := strVal(params, "content_status", "empty")
	if content != "" && contentStatus == "empty" {
		contentStatus = "draft"
	}
	sortOrder := int64Val(params, "sort_order", 0)
	tags := strVal(params, "tags", "[]")

	var parentID sql.NullInt64
	if pid, ok := toInt64(params["parent_id"]); ok {
		parentID = sql.NullInt64{Int64: pid, Valid: true}
	}

	// Insert page (without path initially)
	result, err := e.queries.CreateWikiPage(ctx, model.CreateWikiPageParams{
		Title:         title,
		Slug:          slug,
		PageType:      pageType,
		Content:       content,
		Tags:          sql.NullString{String: tags, Valid: true},
		ParentID:      parentID,
		ContentStatus: contentStatus,
		SortOrder:     sortOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}

	pageID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	// Compute path: parent_path + new_id + "/"
	var path string
	if parentID.Valid {
		parentPath, err := e.queries.GetWikiPagePathByID(ctx, parentID.Int64)
		if err != nil {
			// Fallback: just use own ID
			path = fmt.Sprintf("%d/", pageID)
		} else {
			path = parentPath + fmt.Sprintf("%d/", pageID)
		}
	} else {
		path = fmt.Sprintf("%d/", pageID)
	}

	if err := e.queries.UpdateWikiPagePath(ctx, model.UpdateWikiPagePathParams{
		Path: path,
		ID:   pageID,
	}); err != nil {
		return nil, fmt.Errorf("update page path: %w", err)
	}

	// Parse links from content and update
	if content != "" {
		e.updatePageLinks(ctx, pageID, content)
	}

	// Log the create action (best-effort: don't fail the action on log error).
	if err := e.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
		Action:    "create",
		PageID:    sql.NullInt64{Int64: pageID, Valid: true},
		PageTitle: title,
		PagePath:  sql.NullString{String: path, Valid: true},
		Source:    "plan",
		Summary:   sql.NullString{String: "通过 plan 创建页面", Valid: true},
	}); err != nil { log.Printf("WARN: wiki_log write failed: %v", err) }

	// Trigger summary regeneration (best-effort, non-blocking).
	_ = e.queries.MarkSummaryPending(ctx, pageID)
	if e.onPageWritten != nil {
		e.onPageWritten(pageID)
	}

	return map[string]any{
		"page_id": pageID,
		"slug":    slug,
		"path":    path,
	}, nil
}

// execUpdatePage updates content/title/content_status and re-parses links.
func (e *ExecutionEngine) execUpdatePage(ctx context.Context, params map[string]any) (any, error) {
	pageID, ok := toInt64(params["page_id"])
	if !ok {
		return nil, fmt.Errorf("update_page: page_id is required")
	}

	page, err := e.queries.GetWikiPageByID(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("get page %d: %w", pageID, err)
	}

	title := page.Title
	if t, ok := params["title"].(string); ok && t != "" {
		title = t
	}

	content := page.Content
	if c, ok := params["content"].(string); ok {
		content = c
	}

	contentStatus := page.ContentStatus
	if cs, ok := params["content_status"].(string); ok && cs != "" {
		contentStatus = cs
	}
	if content != page.Content && contentStatus == "empty" {
		contentStatus = "draft"
	}

	slug := page.Slug
	if title != page.Title {
		slug = slugify(title)
	}

	if err := e.queries.UpdateWikiPageContent(ctx, model.UpdateWikiPageContentParams{
		Content:       content,
		ContentStatus: contentStatus,
		ID:            pageID,
	}); err != nil {
		return nil, fmt.Errorf("update page content: %w", err)
	}

	// Update slug/title if changed
	if title != page.Title {
		if err := e.queries.UpdateWikiPage(ctx, model.UpdateWikiPageParams{
			Title:         title,
			Slug:          slug,
			PageType:      page.PageType,
			Content:       content,
			Tags:          page.Tags,
			ParentID:      page.ParentID,
			ContentStatus: contentStatus,
			SortOrder:     page.SortOrder,
			ID:            pageID,
		}); err != nil {
			return nil, fmt.Errorf("update page title: %w", err)
		}
	}

	// Re-parse links
	e.updatePageLinks(ctx, pageID, content)

	// Log the update action.
	if err := e.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
		Action:    "update",
		PageID:    sql.NullInt64{Int64: pageID, Valid: true},
		PageTitle: title,
		Source:    "plan",
		Summary:   sql.NullString{String: "通过 plan 更新页面", Valid: true},
	}); err != nil { log.Printf("WARN: wiki_log write failed: %v", err) }

	// Trigger summary regeneration (best-effort, non-blocking).
	_ = e.queries.MarkSummaryPending(ctx, pageID)
	if e.onPageWritten != nil {
		e.onPageWritten(pageID)
	}

	return map[string]any{
		"page_id": pageID,
	}, nil
}

// execPatchPage applies incremental patch operations to a wiki page.
// Supports: replace (heading-based section replacement) and append.
func (e *ExecutionEngine) execPatchPage(ctx context.Context, params map[string]any) (any, error) {
	pageID, ok := toInt64(params["page_id"])
	if !ok {
		return nil, fmt.Errorf("patch_page: page_id is required")
	}

	page, err := e.queries.GetWikiPageByID(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("get page %d: %w", pageID, err)
	}

	opsRaw, ok := params["operations"]
	if !ok {
		return nil, fmt.Errorf("patch_page: operations is required")
	}
	opsList, ok := opsRaw.([]any)
	if !ok || len(opsList) == 0 {
		return nil, fmt.Errorf("patch_page: operations must be a non-empty array")
	}

	content := page.Content

	for i, opRaw := range opsList {
		op, ok := opRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("patch_page: operation %d is invalid", i)
		}

		opType, _ := op["type"].(string)
		switch opType {
		case "replace":
			target, _ := op["target"].(string)
			if target == "" {
				return nil, fmt.Errorf("patch_page: operation %d (replace): target is required", i)
			}
			newContent, _ := op["content"].(string)
			if newContent == "" {
				return nil, fmt.Errorf("patch_page: operation %d (replace): content is required", i)
			}
			content, err = replaceSection(content, target, newContent)
			if err != nil {
				return nil, fmt.Errorf("patch_page: operation %d: %w", i, err)
			}
		case "append":
			appendContent, _ := op["content"].(string)
			if appendContent == "" {
				return nil, fmt.Errorf("patch_page: operation %d (append): content is required", i)
			}
			if content != "" && !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += appendContent
		default:
			return nil, fmt.Errorf("patch_page: operation %d: unknown type %q (supported: replace, append)", i, opType)
		}
	}

	// Update content
	contentStatus := page.ContentStatus
	if content != page.Content && contentStatus == "empty" {
		contentStatus = "draft"
	}
	if err := e.queries.UpdateWikiPageContent(ctx, model.UpdateWikiPageContentParams{
		Content:       content,
		ContentStatus: contentStatus,
		ID:            pageID,
	}); err != nil {
		return nil, fmt.Errorf("patch page content: %w", err)
	}

	// Re-parse links
	e.updatePageLinks(ctx, pageID, content)

	return map[string]any{
		"page_id": pageID,
	}, nil
}

// headingLevel returns the markdown heading level of a line (0 if not a heading).
func headingLevel(line string) int {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.HasPrefix(trimmed, "#") {
		return 0
	}
	level := 0
	for _, ch := range trimmed {
		if ch == '#' {
			level++
		} else if ch == ' ' {
			break
		} else {
			return 0
		}
	}
	return level
}

// replaceSection replaces content from a markdown heading to the next same-or-higher-level heading.
func replaceSection(content, target, newContent string) (string, error) {
	lines := strings.Split(content, "\n")

	// Find target heading
	targetLine := strings.TrimSpace(target)
	targetIndex := -1
	targetLevel := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == targetLine {
			targetIndex = i
			targetLevel = headingLevel(trimmed)
			break
		}
	}

	if targetIndex == -1 {
		// Collect available headings for error message
		var headings []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if headingLevel(trimmed) > 0 {
				headings = append(headings, trimmed)
			}
		}
		return "", fmt.Errorf("heading %q not found.\nAvailable headings:\n%s", target, strings.Join(headings, "\n"))
	}

	// Find end of section: next heading with level <= targetLevel
	endIndex := len(lines)
	for i := targetIndex + 1; i < len(lines); i++ {
		if level := headingLevel(lines[i]); level > 0 && level <= targetLevel {
			endIndex = i
			break
		}
	}

	// Replace section
	var result []string
	result = append(result, lines[:targetIndex]...)
	result = append(result, newContent)
	result = append(result, lines[endIndex:]...)

	return strings.Join(result, "\n"), nil
}

// execDeletePage removes links/backlinks referencing this page,
// reparents children, and deletes the page.
func (e *ExecutionEngine) execDeletePage(ctx context.Context, params map[string]any) (any, error) {
	pageID, ok := toInt64(params["page_id"])
	if !ok {
		return nil, fmt.Errorf("delete_page: page_id is required")
	}

	// Capture the page title before we delete it (needed for the wiki_log entry
	// since page_id will be NULL after deletion).
	page, err := e.queries.GetWikiPageByID(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("get page %d for delete: %w", pageID, err)
	}
	pageTitle := page.Title
	pagePath := page.Path

	// 1. Remove all link/backlink references to this page
	e.removeLinksForPage(ctx, pageID)

	// 2. Reparent children to the deleted page's parent using direct SQL
	//    (only update parent_id, preserve all other fields including content)
	var deletedParentID sql.NullInt64
	e.db.QueryRowContext(ctx, "SELECT parent_id FROM wiki_pages WHERE id = ?", pageID).Scan(&deletedParentID)
	if _, err := e.db.ExecContext(ctx,
		"UPDATE wiki_pages SET parent_id = ? WHERE parent_id = ?",
		deletedParentID, pageID); err != nil {
		log.Printf("WARN: failed to reparent children of page %d: %v", pageID, err)
	}

	// 3. Delete the page
	if err := e.queries.DeleteWikiPage(ctx, pageID); err != nil {
		return nil, fmt.Errorf("delete page %d: %w", pageID, err)
	}

	// 4. Log the delete action.
	//    On delete, page_id is NULL (the row is gone) but page_title is preserved
	//    so the AI can still see what was just removed.
	if err := e.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
		Action:    "delete",
		PageID:    sql.NullInt64{}, // explicitly invalid
		PageTitle: pageTitle,
		PagePath:  sql.NullString{String: pagePath, Valid: pagePath != ""},
		Source:    "plan",
		Summary:   sql.NullString{String: "通过 plan 删除页面", Valid: true},
	}); err != nil { log.Printf("WARN: wiki_log write failed: %v", err) }

	return map[string]any{
		"deleted": true,
	}, nil
}

// execLinkPages appends [[linkText]] to source page content and updates links/backlinks.
func (e *ExecutionEngine) execLinkPages(ctx context.Context, params map[string]any) (any, error) {
	sourceID, ok := toInt64(params["source_page_id"])
	if !ok {
		return nil, fmt.Errorf("link_pages: source_page_id is required")
	}
	targetID, ok := toInt64(params["target_page_id"])
	if !ok {
		return nil, fmt.Errorf("link_pages: target_page_id is required")
	}
	linkText, _ := params["link_text"].(string)
	if linkText == "" {
		return nil, fmt.Errorf("link_pages: link_text is required")
	}

	// Get source page
	source, err := e.queries.GetWikiPageByID(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("get source page %d: %w", sourceID, err)
	}

	// Check if link already exists in content
	linkMarkup := fmt.Sprintf("[[%s]]", linkText)
	if strings.Contains(source.Content, linkMarkup) {
		// Link already present; just update the link arrays
		e.updatePageLinks(ctx, sourceID, source.Content)
		// Log the link action so the AI sees the user added/refreshed a link.
		if err := e.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
			Action:    "link",
			PageID:    sql.NullInt64{Int64: sourceID, Valid: true},
			PageTitle: source.Title,
			Source:    "plan",
			Summary:   sql.NullString{String: fmt.Sprintf("通过 plan 添加链接 [[%s]]", linkText), Valid: true},
		}); err != nil {
			log.Printf("WARN: wiki_log write failed: %v", err)
		}
		return map[string]any{
			"source_page_id": sourceID,
			"target_page_id": targetID,
			"link_text":      linkText,
		}, nil
	}

	// Append link markup to content
	newContent := source.Content
	if newContent != "" && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += linkMarkup

	contentStatus := source.ContentStatus
	if contentStatus == "empty" {
		contentStatus = "draft"
	}

	if err := e.queries.UpdateWikiPageContent(ctx, model.UpdateWikiPageContentParams{
		Content:       newContent,
		ContentStatus: contentStatus,
		ID:            sourceID,
	}); err != nil {
		return nil, fmt.Errorf("update source page content: %w", err)
	}

	// Update links/backlinks arrays
	e.updatePageLinks(ctx, sourceID, newContent)

	// Log the link action.
	if err := e.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
		Action:    "link",
		PageID:    sql.NullInt64{Int64: sourceID, Valid: true},
		PageTitle: source.Title,
		Source:    "plan",
		Summary:   sql.NullString{String: fmt.Sprintf("通过 plan 添加链接 [[%s]]", linkText), Valid: true},
	}); err != nil { log.Printf("WARN: wiki_log write failed: %v", err) }

	return map[string]any{
		"source_page_id": sourceID,
		"target_page_id": targetID,
		"link_text":      linkText,
	}, nil
}

// execMovePage updates parent_id and migrates subtree paths.
func (e *ExecutionEngine) execMovePage(ctx context.Context, params map[string]any) (any, error) {
	pageID, ok := toInt64(params["page_id"])
	if !ok {
		return nil, fmt.Errorf("move_page: page_id is required")
	}

	var newParentID sql.NullInt64
	if pid, ok := toInt64(params["new_parent_id"]); ok {
		newParentID = sql.NullInt64{Int64: pid, Valid: true}
	}

	page, err := e.queries.GetWikiPageByID(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("get page %d: %w", pageID, err)
	}

	oldPath := page.Path

	// Compute new path
	var newPath string
	if newParentID.Valid {
		parentPath, err := e.queries.GetWikiPagePathByID(ctx, newParentID.Int64)
		if err != nil {
			return nil, fmt.Errorf("get parent path: %w", err)
		}
		newPath = parentPath + fmt.Sprintf("%d/", pageID)
	} else {
		newPath = fmt.Sprintf("%d/", pageID)
	}

	// Update the page's parent and path
	if err := e.queries.UpdateWikiPage(ctx, model.UpdateWikiPageParams{
		Title:         page.Title,
		Slug:          page.Slug,
		PageType:      page.PageType,
		Content:       page.Content,
		Tags:          page.Tags,
		ParentID:      newParentID,
		ContentStatus: page.ContentStatus,
		SortOrder:     page.SortOrder,
		ID:            pageID,
	}); err != nil {
		return nil, fmt.Errorf("update page parent: %w", err)
	}

	if err := e.queries.UpdateWikiPagePath(ctx, model.UpdateWikiPagePathParams{
		Path: newPath,
		ID:   pageID,
	}); err != nil {
		return nil, fmt.Errorf("update page path: %w", err)
	}

	// Migrate subtree paths using REPLACE prefix
	if oldPath != newPath {
		if err := e.queries.BatchUpdateWikiPagePath(ctx, model.BatchUpdateWikiPagePathParams{
			OldPrefix:  oldPath,
			NewPrefix:  newPath,
			LikePrefix: sql.NullString{String: oldPath, Valid: true},
		}); err != nil {
			log.Printf("WARN: failed to migrate subtree paths: %v", err)
		}
	}

	// Log the move action.
	if err := e.queries.InsertWikiLog(ctx, model.InsertWikiLogParams{
		Action:    "move",
		PageID:    sql.NullInt64{Int64: pageID, Valid: true},
		PageTitle: page.Title,
		PagePath:  sql.NullString{String: newPath, Valid: true},
		Source:    "plan",
		Summary:   sql.NullString{String: "通过 plan 移动页面", Valid: true},
	}); err != nil { log.Printf("WARN: wiki_log write failed: %v", err) }

	return map[string]any{
		"page_id":  pageID,
		"new_path": newPath,
	}, nil
}

// ---------------------------------------------------------------------------
// *FromAction wrappers
//
// These thin wrappers accept a model.PlanAction (typically constructed ad-hoc
// by the AI handler from an approved tool call) and dispatch to the per-action
// helpers above. They are the entry points used by the per-tool ReAct loop,
// which has no plan row, no action row, and no placeholder resolution.
// ---------------------------------------------------------------------------

// marshalActionResult JSON-encodes a helper result so the AI can parse it.
// Returns "" on marshal error (caller is expected to handle empty).
func marshalActionResult(r any) string {
	b, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(b)
}

// CreatePageFromAction executes a create_page action built ad-hoc by the handler.
// If focusPageID is set and the action's params lack a parent_id, focusPageID
// is used as parent_id (matches ExecutePlan's focus fallback semantics).
func (e *ExecutionEngine) CreatePageFromAction(ctx context.Context, a model.PlanAction, focusPageID *int64) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", fmt.Errorf("create_page: parse params: %w", err)
	}
	// Focus fallback: if no parent_id, use focusPageID
	if focusPageID != nil {
		if _, hasParent := p["parent_id"]; !hasParent {
			p["parent_id"] = *focusPageID
		}
	}
	result, err := e.execCreatePage(ctx, p)
	if err != nil {
		return "", err
	}
	return marshalActionResult(result), nil
}

// UpdatePageFromAction executes an update_page action built ad-hoc by the handler.
func (e *ExecutionEngine) UpdatePageFromAction(ctx context.Context, a model.PlanAction) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", fmt.Errorf("update_page: parse params: %w", err)
	}
	result, err := e.execUpdatePage(ctx, p)
	if err != nil {
		return "", err
	}
	return marshalActionResult(result), nil
}

// PatchPageFromAction executes a patch_page action built ad-hoc by the handler.
func (e *ExecutionEngine) PatchPageFromAction(ctx context.Context, a model.PlanAction) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", fmt.Errorf("patch_page: parse params: %w", err)
	}
	result, err := e.execPatchPage(ctx, p)
	if err != nil {
		return "", err
	}
	return marshalActionResult(result), nil
}

// DeletePageFromAction executes a delete_page action built ad-hoc by the handler.
func (e *ExecutionEngine) DeletePageFromAction(ctx context.Context, a model.PlanAction) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", fmt.Errorf("delete_page: parse params: %w", err)
	}
	result, err := e.execDeletePage(ctx, p)
	if err != nil {
		return "", err
	}
	return marshalActionResult(result), nil
}

// LinkPagesFromAction executes a link_pages action built ad-hoc by the handler.
func (e *ExecutionEngine) LinkPagesFromAction(ctx context.Context, a model.PlanAction) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", fmt.Errorf("link_pages: parse params: %w", err)
	}
	result, err := e.execLinkPages(ctx, p)
	if err != nil {
		return "", err
	}
	return marshalActionResult(result), nil
}

// MovePageFromAction executes a move_page action built ad-hoc by the handler.
func (e *ExecutionEngine) MovePageFromAction(ctx context.Context, a model.PlanAction) (string, error) {
	var p map[string]any
	if err := json.Unmarshal(a.Params, &p); err != nil {
		return "", fmt.Errorf("move_page: parse params: %w", err)
	}
	result, err := e.execMovePage(ctx, p)
	if err != nil {
		return "", err
	}
	return marshalActionResult(result), nil
}

// ---------------------------------------------------------------------------
// Link system helpers
// ---------------------------------------------------------------------------

// linkPattern matches [[title]] style wiki links.
var linkPattern = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

// updatePageLinks parses [[title]] patterns from content, resolves them to IDs,
// and updates the links/backlinks arrays for the page.
func (e *ExecutionEngine) updatePageLinks(ctx context.Context, pageID int64, content string) {
	matches := linkPattern.FindAllStringSubmatch(content, -1)
	var linkIDs []int64

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		title := m[1]
		// Resolve title to page ID
		resolved, err := e.queries.GetWikiPageByTitle(ctx, title)
		if err != nil {
			continue // skip unresolved links
		}
		linkIDs = append(linkIDs, resolved.ID)

		// Add backlink on target page
		e.addBacklink(ctx, resolved.ID, pageID)
	}

	// Update this page's links array and denormalized link_count.
	linksJSON, _ := json.Marshal(linkIDs)
	if _, err := e.db.ExecContext(ctx,
		`UPDATE wiki_pages SET links = ?, link_count = ? WHERE id = ?`,
		string(linksJSON), len(linkIDs), pageID); err != nil {
		log.Printf("WARN: failed to update links for page %d: %v", pageID, err)
	}
}

// addBacklink adds pageID to the backlinks array of targetID.
func (e *ExecutionEngine) addBacklink(ctx context.Context, targetID, pageID int64) {
	var backlinks string
	err := e.db.QueryRowContext(ctx,
		`SELECT backlinks FROM wiki_pages WHERE id = ?`, targetID).Scan(&backlinks)
	if err != nil {
		return
	}

	var ids []int64
	_ = json.Unmarshal([]byte(backlinks), &ids)

	// Check if already present
	for _, id := range ids {
		if id == pageID {
			return
		}
	}

	ids = append(ids, pageID)
	idsJSON, _ := json.Marshal(ids)
	if _, err := e.db.ExecContext(ctx,
		`UPDATE wiki_pages SET backlinks = ?, backlink_count = ? WHERE id = ?`,
		string(idsJSON), len(ids), targetID); err != nil {
		log.Printf("WARN: failed to add backlink %d->%d: %v", pageID, targetID, err)
	}
}

// removeLinksForPage removes all references to pageID from other pages'
// links and backlinks arrays, and decrements backlink_count on pages that
// no longer reference the deleted page.
//
// Implemented as two atomic UPDATE statements (one for `links`, one for
// `backlinks`) that use `json_each` to filter the array. This avoids the
// per-row read/modify/write loop, which deadlocks on a single-connection
// sqlite handle while iterating open rows.
func (e *ExecutionEngine) removeLinksForPage(ctx context.Context, pageID int64) {
	// 1. Strip pageID from every other page's outgoing `links` array.
	if _, err := e.db.ExecContext(ctx,
		`UPDATE wiki_pages
		 SET links = (
		   SELECT COALESCE(json_group_array(value), '[]')
		   FROM json_each(links)
		   WHERE value != ?
		 )
		 WHERE id != ?
		   AND EXISTS (SELECT 1 FROM json_each(links) WHERE value = ?)`,
		pageID, pageID, pageID); err != nil {
		log.Printf("WARN: failed to remove link refs to page %d: %v", pageID, err)
	}

	// 2. Strip pageID from every other page's `backlinks` array AND
	//    decrement backlink_count by 1 (clamped at 0 to absorb any
	//    pre-existing drift between the array and the count).
	if _, err := e.db.ExecContext(ctx,
		`UPDATE wiki_pages
		 SET backlinks = (
		   SELECT COALESCE(json_group_array(value), '[]')
		   FROM json_each(backlinks)
		   WHERE value != ?
		 ),
		 backlink_count = MAX(0, backlink_count - 1)
		 WHERE id != ?
		   AND EXISTS (SELECT 1 FROM json_each(backlinks) WHERE value = ?)`,
		pageID, pageID, pageID); err != nil {
		log.Printf("WARN: failed to remove backlink refs to page %d: %v", pageID, err)
	}
}

// removeBacklink removes a specific backlink entry from a page.
func (e *ExecutionEngine) removeBacklink(ctx context.Context, pageID, backlinkID int64) {
	var backlinks string
	err := e.db.QueryRowContext(ctx,
		`SELECT backlinks FROM wiki_pages WHERE id = ?`, pageID).Scan(&backlinks)
	if err != nil {
		return
	}

	var ids []int64
	if err := json.Unmarshal([]byte(backlinks), &ids); err != nil {
		return
	}

	filtered := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != backlinkID {
			filtered = append(filtered, id)
		}
	}

	backlinksJSON, _ := json.Marshal(filtered)
	_, _ = e.db.ExecContext(ctx,
		`UPDATE wiki_pages SET backlinks = ? WHERE id = ?`, string(backlinksJSON), pageID)
}

// removeLink removes a specific link entry from a page.
func (e *ExecutionEngine) removeLink(ctx context.Context, pageID, linkID int64) {
	var links string
	err := e.db.QueryRowContext(ctx,
		`SELECT links FROM wiki_pages WHERE id = ?`, pageID).Scan(&links)
	if err != nil {
		return
	}

	var ids []int64
	if err := json.Unmarshal([]byte(links), &ids); err != nil {
		return
	}

	filtered := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != linkID {
			filtered = append(filtered, id)
		}
	}

	linksJSON, _ := json.Marshal(filtered)
	_, _ = e.db.ExecContext(ctx,
		`UPDATE wiki_pages SET links = ? WHERE id = ?`, string(linksJSON), pageID)
}

// ---------------------------------------------------------------------------
// Database helpers
// ---------------------------------------------------------------------------

func (e *ExecutionEngine) loadActions(ctx context.Context, planID string) ([]model.PlanAction, error) {
	rows, err := e.db.QueryContext(ctx,
		`SELECT id, plan_id, type, params, depends_on, status, result, sort_order, created_at
		 FROM plan_actions WHERE plan_id = ? ORDER BY sort_order`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []model.PlanAction
	for rows.Next() {
		var a model.PlanAction
		if err := rows.Scan(
			&a.ID, &a.PlanID, &a.Type, &a.Params, &a.DependsOn,
			&a.Status, &a.Result, &a.SortOrder, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

func (e *ExecutionEngine) updateActionStatus(ctx context.Context, actionID, status, result string) error {
	if result != "" {
		_, err := e.db.ExecContext(ctx,
			`UPDATE plan_actions SET status = ?, result = ? WHERE id = ?`,
			status, result, actionID)
		return err
	}
	_, err := e.db.ExecContext(ctx,
		`UPDATE plan_actions SET status = ? WHERE id = ?`,
		status, actionID)
	return err
}

func (e *ExecutionEngine) dependsOnFailed(action model.PlanAction, failedSet map[string]bool) bool {
	deps := parseDependsOn(string(action.DependsOn))
	for _, dep := range deps {
		if failedSet[dep] {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

// toInt64 converts a value from JSON unmarshaling to int64.
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i, true
		}
		return 0, false
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

// slugify converts a title to a URL-friendly slug, preserving CJK characters.
func slugify(title string) string {
	title = strings.ToLower(title)

	var b strings.Builder
	lastWasHyphen := false
	for _, r := range title {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hiragana, r) {
			b.WriteRune(r)
			lastWasHyphen = false
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastWasHyphen = false
		} else if unicode.IsSpace(r) || r == '-' || r == '_' {
			if !lastWasHyphen {
				b.WriteRune('-')
				lastWasHyphen = true
			}
		}
	}

	result := b.String()
	result = strings.Trim(result, "-")
	return result
}

// strVal extracts a string from a params map with a default.
func strVal(params map[string]any, key, defaultVal string) string {
	if v, ok := params[key].(string); ok {
		return v
	}
	return defaultVal
}

// int64Val extracts an int64 from a params map with a default.
func int64Val(params map[string]any, key string, defaultVal int64) int64 {
	if v, ok := toInt64(params[key]); ok {
		return v
	}
	return defaultVal
}

// injectContentStatus sets content_status in the params JSON for create_page/update_page
// actions during user-confirmed plan execution, so pages skip the draft confirmation step.
func injectContentStatus(paramsJSON string, status string) string {
	if strings.Contains(paramsJSON, `"content_status"`) {
		return paramsJSON // already set, don't override
	}
	// Insert content_status before the closing brace
	idx := strings.LastIndex(paramsJSON, "}")
	if idx < 0 {
		return paramsJSON
	}
	return paramsJSON[:idx] + `,"content_status":"` + status + `"` + paramsJSON[idx:]
}

// hasParentID checks if the action params JSON contains a parent_id field.
// A parent_id whose value is an unresolved {{action:...}} placeholder string
// is treated as "no parent_id" so the focusPageID fallback can fire.
func hasParentID(paramsJSON json.RawMessage) bool {
	var p map[string]any
	if err := json.Unmarshal(paramsJSON, &p); err != nil {
		return false
	}
	v, ok := p["parent_id"]
	if !ok {
		return false
	}
	if s, isStr := v.(string); isStr && isPlaceholderString(s) {
		return false
	}
	return true
}

// isPlaceholderString reports whether s looks like a {{action:...}} placeholder.
func isPlaceholderString(s string) bool {
	return strings.Contains(s, "{{action:")
}

// inferDependencies scans a params JSON for {{action:X.field}} placeholders
// and returns the set of referenced action IDs. Order is not significant.
// Returns nil if no placeholders are present.
func inferDependencies(paramsJSON string) []string {
	if !strings.Contains(paramsJSON, "{{action:") {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	matches := placeholderPattern.FindAllStringSubmatch(paramsJSON, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		id := m[1]
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// hasPlaceholderParentID checks if the action params JSON contains a
// parent_id field whose value is an unresolved {{action:...}} placeholder.
func hasPlaceholderParentID(paramsJSON json.RawMessage) bool {
	var p map[string]any
	if err := json.Unmarshal(paramsJSON, &p); err != nil {
		return false
	}
	v, ok := p["parent_id"]
	if !ok {
		return false
	}
	s, isStr := v.(string)
	return isStr && isPlaceholderString(s)
}

// injectParentID adds parent_id to the action params JSON.
func injectParentID(paramsJSON json.RawMessage, parentID int64) json.RawMessage {
	var p map[string]any
	if err := json.Unmarshal(paramsJSON, &p); err != nil {
		return paramsJSON
	}
	p["parent_id"] = parentID
	result, err := json.Marshal(p)
	if err != nil {
		return paramsJSON
	}
	return json.RawMessage(result)
}

