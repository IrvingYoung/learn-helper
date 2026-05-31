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
	db      *sql.DB
	queries *model.Queries
}

// NewExecutionEngine creates a new engine backed by the given database.
func NewExecutionEngine(db *sql.DB, queries *model.Queries) *ExecutionEngine {
	return &ExecutionEngine{db: db, queries: queries}
}

// placeholderPattern matches {{action:a1.page_id}} style references.
var placeholderPattern = regexp.MustCompile(`\{\{action:([^.}]+)\.([^}]+)\}\}`)

// ExecutePlan loads actions for the given plan, sorts them topologically,
// and executes them in order with dependency propagation.
func (e *ExecutionEngine) ExecutePlan(ctx context.Context, planID string) (*ExecutionReport, error) {
	// 1. Load actions
	actions, err := e.loadActions(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("load actions: %w", err)
	}
	if len(actions) == 0 {
		return &ExecutionReport{PlanID: planID, Status: "completed"}, nil
	}

	// 2. Topological sort
	sorted, err := topoSort(actions)
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
		if action.Type == "create_page" || action.Type == "update_page" {
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
// with actual values from the actionResultMap.
func (e *ExecutionEngine) replacePlaceholders(paramsJSON string, actionResultMap map[string]any) (string, error) {
	if !strings.Contains(paramsJSON, "{{action:") {
		return paramsJSON, nil
	}

	result := placeholderPattern.ReplaceAllStringFunc(paramsJSON, func(match string) string {
		subs := placeholderPattern.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		actionID := subs[1]
		field := subs[2]

		res, ok := actionResultMap[actionID]
		if !ok {
			return match // leave unresolved; will likely fail at execution
		}

		val := resolveField(res, field)
		if val == nil {
			return match
		}
		return fmt.Sprintf("%v", val)
	})

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

	return map[string]any{
		"page_id": pageID,
	}, nil
}

// execDeletePage removes links/backlinks referencing this page,
// reparents children, and deletes the page.
func (e *ExecutionEngine) execDeletePage(ctx context.Context, params map[string]any) (any, error) {
	pageID, ok := toInt64(params["page_id"])
	if !ok {
		return nil, fmt.Errorf("delete_page: page_id is required")
	}

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

	return map[string]any{
		"page_id":  pageID,
		"new_path": newPath,
	}, nil
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

	// Update this page's links array
	linksJSON, _ := json.Marshal(linkIDs)
	if _, err := e.db.ExecContext(ctx,
		`UPDATE wiki_pages SET links = ? WHERE id = ?`,
		string(linksJSON), pageID); err != nil {
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
		`UPDATE wiki_pages SET backlinks = ? WHERE id = ?`,
		string(idsJSON), targetID); err != nil {
		log.Printf("WARN: failed to add backlink %d->%d: %v", pageID, targetID, err)
	}
}

// removeLinksForPage removes all references to pageID from other pages'
// links and backlinks arrays.
func (e *ExecutionEngine) removeLinksForPage(ctx context.Context, pageID int64) {
	// 1. Remove pageID from all pages that have it in their links array
	rows, err := e.db.QueryContext(ctx,
		`SELECT id, links FROM wiki_pages WHERE links LIKE ?`, fmt.Sprintf(`%%%d%%`, pageID))
	if err != nil {
		log.Printf("WARN: query links for cleanup: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var links string
		if err := rows.Scan(&id, &links); err != nil {
			continue
		}
		var ids []int64
		if err := json.Unmarshal([]byte(links), &ids); err != nil {
			continue
		}
		filtered := make([]int64, 0, len(ids))
		for _, lid := range ids {
			if lid != pageID {
				filtered = append(filtered, lid)
			}
		}
		linksJSON, _ := json.Marshal(filtered)
		if _, err := e.db.ExecContext(ctx,
			`UPDATE wiki_pages SET links = ? WHERE id = ?`, string(linksJSON), id); err != nil {
			log.Printf("WARN: failed to remove link ref %d from page %d: %v", pageID, id, err)
		}
	}

	// 2. Remove pageID from all pages that have it in their backlinks array
	rows2, err := e.db.QueryContext(ctx,
		`SELECT id, backlinks FROM wiki_pages WHERE backlinks LIKE ?`, fmt.Sprintf(`%%%d%%`, pageID))
	if err != nil {
		log.Printf("WARN: query backlinks for cleanup: %v", err)
		return
	}
	defer rows2.Close()

	for rows2.Next() {
		var id int64
		var backlinks string
		if err := rows2.Scan(&id, &backlinks); err != nil {
			continue
		}
		var ids []int64
		if err := json.Unmarshal([]byte(backlinks), &ids); err != nil {
			continue
		}
		filtered := make([]int64, 0, len(ids))
		for _, bid := range ids {
			if bid != pageID {
				filtered = append(filtered, bid)
			}
		}
		backlinksJSON, _ := json.Marshal(filtered)
		if _, err := e.db.ExecContext(ctx,
			`UPDATE wiki_pages SET backlinks = ? WHERE id = ?`, string(backlinksJSON), id); err != nil {
			log.Printf("WARN: failed to remove backlink ref %d from page %d: %v", pageID, id, err)
		}
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
