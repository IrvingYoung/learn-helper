package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"learn-helper/internal/ai"
)

// wikiLinkPattern is shared with wiki.go (see that file for the regex).

// executeListBacklinks lists pages that reference the target page, by
// reading wiki_pages.backlinks (a JSON array of int IDs maintained by the
// engine on every write).
func (h *AIHandler) executeListBacklinks(ctx context.Context, tc ai.ToolCall) string {
	var input struct {
		PageID float64 `json:"page_id"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &input); err != nil || input.PageID == 0 {
		return "[系统] list_backlinks 执行失败：page_id 必填"
	}
	pageID := int64(input.PageID)

	target, err := h.queries.GetWikiPageByID(ctx, pageID)
	if err != nil {
		return fmt.Sprintf("[系统] list_backlinks 未找到页面 #%d", pageID)
	}

	var ids []int64
	_ = json.Unmarshal([]byte(target.Backlinks), &ids)
	if len(ids) == 0 {
		return fmt.Sprintf("[系统] 页面「%s」(ID=%d) 没有反链", target.Title, pageID)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[系统] 页面「%s」(ID=%d) 的 %d 条反链：\n\n", target.Title, pageID, len(ids)))
	for _, id := range ids {
		p, err := h.queries.GetWikiPageByID(ctx, id)
		if err != nil {
			b.WriteString(fmt.Sprintf("- [ID=%d] (已删除)\n", id))
			continue
		}
		b.WriteString(fmt.Sprintf("- [ID=%d] %s\n  %s\n", p.ID, p.Title, snippetFromContent(p.Content, 80)))
	}
	return b.String()
}

// executeListLinks lists pages referenced by the source page (its outgoing
// links), by reading wiki_pages.links.
func (h *AIHandler) executeListLinks(ctx context.Context, tc ai.ToolCall) string {
	var input struct {
		PageID float64 `json:"page_id"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &input); err != nil || input.PageID == 0 {
		return "[系统] list_links 执行失败：page_id 必填"
	}
	pageID := int64(input.PageID)

	src, err := h.queries.GetWikiPageByID(ctx, pageID)
	if err != nil {
		return fmt.Sprintf("[系统] list_links 未找到页面 #%d", pageID)
	}

	var ids []int64
	_ = json.Unmarshal([]byte(src.Links), &ids)
	if len(ids) == 0 {
		return fmt.Sprintf("[系统] 页面「%s」(ID=%d) 没有出链", src.Title, pageID)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[系统] 页面「%s」(ID=%d) 的 %d 条出链：\n\n", src.Title, pageID, len(ids)))
	for _, id := range ids {
		p, err := h.queries.GetWikiPageByID(ctx, id)
		if err != nil {
			b.WriteString(fmt.Sprintf("- [ID=%d] (已删除)\n", id))
			continue
		}
		b.WriteString(fmt.Sprintf("- [ID=%d] %s\n  %s\n", p.ID, p.Title, snippetFromContent(p.Content, 80)))
	}
	return b.String()
}

// executeListChildren lists direct children of a parent page, or recursively
// expands the subtree when depth > 1 (capped at 5 to bound token cost).
// parent_id == 0 means top-level pages.
func (h *AIHandler) executeListChildren(ctx context.Context, tc ai.ToolCall) string {
	var input struct {
		ParentID float64 `json:"parent_id"`
		Depth    float64 `json:"depth"`
	}
	_ = json.Unmarshal([]byte(tc.Input), &input)
	depth := int(input.Depth)
	if depth <= 0 {
		depth = 1
	}
	if depth > 5 {
		depth = 5
	}

	parentID := int64(input.ParentID)
	label := "顶层"
	if parentID > 0 {
		parent, err := h.queries.GetWikiPageByID(ctx, parentID)
		if err != nil {
			return fmt.Sprintf("[系统] list_children 未找到父页 #%d", parentID)
		}
		label = fmt.Sprintf("「%s」(ID=%d)", parent.Title, parentID)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[系统] %s 下的子页（depth=%d）：\n\n", label, depth))
	n := h.renderChildrenTree(ctx, &b, parentID, 0, depth)
	if n == 0 {
		return fmt.Sprintf("[系统] %s 下没有子页", label)
	}
	return b.String()
}

func (h *AIHandler) renderChildrenTree(ctx context.Context, b *strings.Builder, parentID int64, depth, maxDepth int) int {
	type child struct {
		ID            int64
		Title         string
		PageType      string
		ContentStatus string
	}
	var children []child

	if parentID > 0 {
		rows, err := h.queries.GetWikiPageChildren(ctx, sql.NullInt64{Int64: parentID, Valid: true})
		if err != nil {
			return 0
		}
		for _, c := range rows {
			children = append(children, child{c.ID, c.Title, c.PageType, c.ContentStatus})
		}
	} else {
		// Top-level pages — parent_id IS NULL. Cannot use the sqlc query
		// because WHERE parent_id = NULL never matches in SQL.
		sqlRows, err := h.db.QueryContext(ctx,
			`SELECT id, title, page_type, content_status FROM wiki_pages WHERE parent_id IS NULL ORDER BY sort_order, id`)
		if err != nil {
			return 0
		}
		defer sqlRows.Close()
		for sqlRows.Next() {
			var c child
			if err := sqlRows.Scan(&c.ID, &c.Title, &c.PageType, &c.ContentStatus); err != nil {
				continue
			}
			children = append(children, c)
		}
	}

	indent := strings.Repeat("  ", depth)
	count := 0
	for _, c := range children {
		b.WriteString(fmt.Sprintf("%s- [ID=%d] %s (%s, %s)\n", indent, c.ID, c.Title, c.PageType, c.ContentStatus))
		count++
		if depth+1 < maxDepth {
			count += h.renderChildrenTree(ctx, b, c.ID, depth+1, maxDepth)
		}
	}
	return count
}

// executeFindBrokenLinks scans [[X]] patterns in page content and reports
// targets that don't match any existing page title. Use to find dangling
// references after renames/deletes. Without page_id it scans the whole wiki
// (N+1 queries — acceptable at the current page count).
func (h *AIHandler) executeFindBrokenLinks(ctx context.Context, tc ai.ToolCall) string {
	var input struct {
		PageID float64 `json:"page_id"`
	}
	_ = json.Unmarshal([]byte(tc.Input), &input)
	pageID := int64(input.PageID)

	type brokenEntry struct {
		pageID    int64
		pageTitle string
		targets   []string
	}
	var results []brokenEntry

	titleCache := map[string]bool{}
	titleExists := func(title string) bool {
		if v, ok := titleCache[title]; ok {
			return v
		}
		_, err := h.queries.GetWikiPageByTitle(ctx, title)
		v := err == nil
		titleCache[title] = v
		return v
	}

	scanPage := func(id int64, title, content string) {
		if content == "" {
			return
		}
		matches := wikiLinkPattern.FindAllStringSubmatch(content, -1)
		seen := map[string]bool{}
		var broken []string
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			target := strings.TrimSpace(m[1])
			if target == "" || seen[target] {
				continue
			}
			seen[target] = true
			if !titleExists(target) {
				broken = append(broken, target)
			}
		}
		if len(broken) > 0 {
			results = append(results, brokenEntry{id, title, broken})
		}
	}

	if pageID > 0 {
		p, err := h.queries.GetWikiPageByID(ctx, pageID)
		if err != nil {
			return fmt.Sprintf("[系统] find_broken_links 未找到页面 #%d", pageID)
		}
		scanPage(p.ID, p.Title, p.Content)
	} else {
		pages, err := h.queries.GetWikiPageTree(ctx)
		if err != nil {
			return fmt.Sprintf("[系统] find_broken_links 执行失败：%v", err)
		}
		for _, p := range pages {
			full, err := h.queries.GetWikiPageByID(ctx, p.ID)
			if err != nil {
				continue
			}
			scanPage(full.ID, full.Title, full.Content)
		}
	}

	if len(results) == 0 {
		if pageID > 0 {
			return fmt.Sprintf("[系统] 页面 #%d 没有死链", pageID)
		}
		return "[系统] 全库扫描：没有死链"
	}

	var b strings.Builder
	scope := "全库"
	if pageID > 0 {
		scope = fmt.Sprintf("页面 #%d", pageID)
	}
	b.WriteString(fmt.Sprintf("[系统] %s 找到 %d 个含死链的页面：\n\n", scope, len(results)))
	for _, e := range results {
		b.WriteString(fmt.Sprintf("- [ID=%d] %s\n", e.pageID, e.pageTitle))
		for _, t := range e.targets {
			b.WriteString(fmt.Sprintf("  - 死链: [[%s]]\n", t))
		}
	}
	return b.String()
}

// snippetFromContent returns the first non-empty line of content (excluding
// H1 headings, blockquotes, and horizontal rules), truncated to maxRunes.
// Used as a one-line preview in list_backlinks / list_links output.
func snippetFromContent(content string, maxRunes int) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ">") || strings.HasPrefix(line, "---") {
			continue
		}
		runes := []rune(line)
		if len(runes) > maxRunes {
			return string(runes[:maxRunes]) + "..."
		}
		return line
	}
	return "(无内容)"
}
