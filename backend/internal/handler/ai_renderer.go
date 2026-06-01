package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"learn-helper/internal/model"
)

// OverviewDB is the minimal DB interface for the overview section.
type OverviewDB interface {
	CountWikiPages(ctx context.Context) (int64, error)
	CountWikiPagesByStatus(ctx context.Context, status string) (int64, error)
	GetRecentlyUpdatedWikiPages(ctx context.Context) ([]model.GetRecentlyUpdatedWikiPagesRow, error)
}

// renderOverview builds the high-level stats section.
func renderOverview(ctx context.Context, db OverviewDB) string {
	var b strings.Builder
	b.WriteString("【知识库概览】\n")

	total, _ := db.CountWikiPages(ctx)
	published, _ := db.CountWikiPagesByStatus(ctx, "published")
	draft, _ := db.CountWikiPagesByStatus(ctx, "draft")
	filled := published + draft
	empty := total - filled

	b.WriteString(fmt.Sprintf("总页面数: %d | 已填充: %d (%.0f%%) | 空: %d (%.0f%%)\n",
		total, filled, pct(filled, total), empty, pct(empty, total)))

	recent, _ := db.GetRecentlyUpdatedWikiPages(ctx)
	if len(recent) > 0 {
		b.WriteString(fmt.Sprintf("最近更新: %s (ID=%d)\n", recent[0].Title, recent[0].ID))
	}
	b.WriteString("\n")
	return b.String()
}

func pct(num, denom int64) float64 {
	if denom == 0 {
		return 0
	}
	return float64(num) / float64(denom) * 100
}

// KnowledgeMapDB is the minimal interface for rendering the knowledge map.
type KnowledgeMapDB interface {
	GetWikiPageTree(ctx context.Context) ([]model.GetWikiPageTreeRow, error)
	// For fallback when summary is pending, we may need to fetch content.
	// Optional: if not implemented, fallback shows "(content unavailable)".
	GetPageContentForFallback(ctx context.Context, pageID int64) (string, error)
}

// renderKnowledgeMap builds the categorized tree with per-page summaries.
// focusPageID, if set, only renders the subtree (existing behavior).
func renderKnowledgeMap(ctx context.Context, db KnowledgeMapDB, focusPageID *int64) string {
	var b strings.Builder
	b.WriteString("【知识地图】\n\n")

	pages, err := db.GetWikiPageTree(ctx)
	if err != nil || len(pages) == 0 {
		b.WriteString("（知识库为空）\n")
		return b.String()
	}

	// Build parent-children index
	children := make(map[int64][]model.GetWikiPageTreeRow)
	var roots []model.GetWikiPageTreeRow
	for _, p := range pages {
		if !p.ParentID.Valid || p.ParentID.Int64 == 0 {
			roots = append(roots, p)
		} else {
			children[p.ParentID.Int64] = append(children[p.ParentID.Int64], p)
		}
	}

	// If focus is set, find that node as the root
	if focusPageID != nil {
		var focus *model.GetWikiPageTreeRow
		for i, p := range roots {
			if p.ID == *focusPageID {
				focus = &roots[i]
				break
			}
		}
		if focus == nil {
			b.WriteString(fmt.Sprintf("（未找到页面 ID=%d）\n", *focusPageID))
			return b.String()
		}
		roots = []model.GetWikiPageTreeRow{*focus}
	}

	// Render each root and its descendants
	for _, root := range roots {
		renderNodeWithChildren(ctx, &b, db, root, children, 0)
	}

	// Render global tag index
	b.WriteString("\n")
	b.WriteString(renderTagIndex(ctx, db))

	return b.String()
}

func renderNodeWithChildren(ctx context.Context, b *strings.Builder, db KnowledgeMapDB, node model.GetWikiPageTreeRow, children map[int64][]model.GetWikiPageTreeRow, depth int) {
	indent := strings.Repeat("  ", depth)
	icon := "📄"
	if node.PageType == "overview" {
		icon = "📁"
	}

	// Build status suffix
	status := "空"
	if node.ContentStatus == "published" {
		status = "有内容"
	} else if node.ContentStatus == "draft" {
		status = "草稿"
	}

	// Build summary line
	summaryLine := renderSummaryLine(ctx, db, node)

	// Build metadata
	meta := fmt.Sprintf("[ID=%d]", node.ID)
	if node.BacklinkCount > 0 {
		meta += fmt.Sprintf(", %d 反链", node.BacklinkCount)
	}
	if node.TagsNormalized != "" {
		meta += fmt.Sprintf(", 标签: %s", node.TagsNormalized)
	}

	// Coverage for overview nodes (X/Y 已建)
	if node.PageType == "overview" {
		all := collectDescendants(node, children)
		filled := 0
		for _, d := range all {
			if d.ContentStatus == "published" || d.ContentStatus == "draft" {
				filled++
			}
		}
		meta = fmt.Sprintf("%d/%d 已建, %s", filled, len(all), meta)
	}

	b.WriteString(fmt.Sprintf("%s%s %s (%s) %s\n", indent, icon, node.Title, status, meta))
	if summaryLine != "" {
		b.WriteString(fmt.Sprintf("%s  %s\n", indent, summaryLine))
	}

	// Recurse into children
	for _, child := range children[node.ID] {
		renderNodeWithChildren(ctx, b, db, child, children, depth+1)
	}
}

func collectDescendants(root model.GetWikiPageTreeRow, children map[int64][]model.GetWikiPageTreeRow) []model.GetWikiPageTreeRow {
	var result []model.GetWikiPageTreeRow
	var walk func(id int64)
	walk = func(id int64) {
		for _, c := range children[id] {
			result = append(result, c)
			walk(c.ID)
		}
	}
	walk(root.ID)
	return result
}

// renderSummaryLine returns the summary for a page, with fallback handling.
func renderSummaryLine(ctx context.Context, db KnowledgeMapDB, node model.GetWikiPageTreeRow) string {
	status := node.SummaryStatus

	switch status {
	case "ready":
		if node.Summary != "" {
			return node.Summary
		}
		fallthrough
	case "pending":
		if content, err := db.GetPageContentForFallback(ctx, node.ID); err == nil && content != "" {
			return truncateForDisplay(content, 80) + " (摘要待更新)"
		}
		return "(摘要待更新)"
	case "failed":
		if content, err := db.GetPageContentForFallback(ctx, node.ID); err == nil && content != "" {
			return truncateForDisplay(content, 80) + " (摘要生成失败)"
		}
		return "(摘要生成失败)"
	case "empty":
		if content, err := db.GetPageContentForFallback(ctx, node.ID); err == nil && content != "" {
			return truncateForDisplay(content, 80) + " (暂无摘要)"
		}
		return ""
	default:
		return ""
	}
}

func truncateForDisplay(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

// renderTagIndex builds the global tag summary.
func renderTagIndex(ctx context.Context, db KnowledgeMapDB) string {
	pages, _ := db.GetWikiPageTree(ctx)
	tagCounts := make(map[string]int)
	for _, p := range pages {
		if p.TagsNormalized == "" {
			continue
		}
		for _, tag := range strings.Split(p.TagsNormalized, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagCounts[tag]++
			}
		}
	}
	if len(tagCounts) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("【全局标签索引】\n")
	type tagCount struct {
		tag   string
		count int
	}
	var sorted []tagCount
	for k, v := range tagCounts {
		sorted = append(sorted, tagCount{k, v})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	for i, tc := range sorted {
		if i > 0 {
			b.WriteString(" · ")
		}
		b.WriteString(fmt.Sprintf("#%s (%d 页)", tc.tag, tc.count))
	}
	b.WriteString("\n")
	return b.String()
}

// RecentLogDB is the minimal interface for log queries.
type RecentLogDB interface {
	GetRecentWikiLog(ctx context.Context, arg model.GetRecentWikiLogParams) ([]model.WikiLog, error)
}

// renderRecentLog builds the "recent activity" timeline.
func renderRecentLog(ctx context.Context, db RecentLogDB, window time.Duration, limit int) string {
	since := time.Now().Add(-window)
	entries, err := db.GetRecentWikiLog(ctx, model.GetRecentWikiLogParams{CreatedAt: since, Limit: int64(limit)})
	if err != nil || len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("【最近活动】(过去 %s，共 %d 条操作)\n", window, len(entries)))
	for _, e := range entries {
		ts := e.CreatedAt.Format("2006-01-02 15:04")
		line := fmt.Sprintf("  - %s [%s] %s", ts, e.Action, e.PageTitle)
		if e.PageID.Valid {
			line += fmt.Sprintf(" (ID=%d)", e.PageID.Int64)
		}
		if e.Summary.Valid && e.Summary.String != "" {
			line += " · " + e.Summary.String
		}
		b.WriteString(line + "\n")
	}
	return b.String() + "\n"
}
