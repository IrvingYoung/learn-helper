package handler

import (
	"context"
	"fmt"
	"strings"

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
