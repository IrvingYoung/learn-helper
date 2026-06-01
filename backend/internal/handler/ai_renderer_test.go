package handler

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"learn-helper/internal/model"
)

type fakeOverviewDB struct {
	pages []model.WikiPage
}

func (f *fakeOverviewDB) CountWikiPages(ctx context.Context) (int64, error) {
	return int64(len(f.pages)), nil
}

func (f *fakeOverviewDB) CountWikiPagesByStatus(ctx context.Context, status string) (int64, error) {
	var n int64
	for _, p := range f.pages {
		if p.ContentStatus == status {
			n++
		}
	}
	return n, nil
}

func (f *fakeOverviewDB) GetRecentlyUpdatedWikiPages(ctx context.Context) ([]model.GetRecentlyUpdatedWikiPagesRow, error) {
	if len(f.pages) == 0 {
		return nil, nil
	}
	p := f.pages[0]
	return []model.GetRecentlyUpdatedWikiPagesRow{
		{ID: p.ID, Title: p.Title, Slug: p.Slug, PageType: p.PageType, ContentStatus: p.ContentStatus, UpdatedAt: p.UpdatedAt},
	}, nil
}

func TestRenderOverview_EmptyDB(t *testing.T) {
	queries := &fakeOverviewDB{}
	out := renderOverview(context.Background(), queries)
	if !strings.Contains(out, "总页面数: 0") {
		t.Errorf("expected '总页面数: 0', got: %s", out)
	}
}

func TestRenderOverview_WithPages(t *testing.T) {
	queries := &fakeOverviewDB{
		pages: []model.WikiPage{
			{ID: 1, Title: "Go", ContentStatus: "published"},
			{ID: 2, Title: "算法", ContentStatus: "draft"},
			{ID: 3, Title: "树", ContentStatus: "empty"},
		},
	}
	out := renderOverview(context.Background(), queries)
	if !strings.Contains(out, "总页面数: 3") {
		t.Errorf("expected '总页面数: 3', got: %s", out)
	}
	if !strings.Contains(out, "已填充: 2") {
		t.Errorf("expected '已填充: 2', got: %s", out)
	}
	if !strings.Contains(out, "空: 1") {
		t.Errorf("expected '空: 1', got: %s", out)
	}
}

type fakeTreeDB struct {
	pages   []model.GetWikiPageTreeRow
	content map[int64]string
}

func (f *fakeTreeDB) GetWikiPageTree(ctx context.Context) ([]model.GetWikiPageTreeRow, error) {
	return f.pages, nil
}

func (f *fakeTreeDB) GetPageContentForFallback(ctx context.Context, pageID int64) (string, error) {
	if f.content == nil {
		return "", nil
	}
	return f.content[pageID], nil
}

func TestRenderKnowledgeMap_Basic(t *testing.T) {
	tree := []model.GetWikiPageTreeRow{
		{ID: 1, Title: "概览", PageType: "overview", ContentStatus: "published", ParentID: sql.NullInt64{}, Path: "1/", Summary: "知识库总览", SummaryStatus: "ready", SummaryContentHash: sql.NullString{}, BacklinkCount: 0, LinkCount: 2, TagsNormalized: ""},
		{ID: 2, Title: "Go 语言", PageType: "overview", ContentStatus: "published", ParentID: sql.NullInt64{Int64: 1, Valid: true}, Path: "1/2/", Summary: "Go 语法、并发和工程实践", SummaryStatus: "ready", SummaryContentHash: sql.NullString{}, BacklinkCount: 5, LinkCount: 3, TagsNormalized: "go"},
		{ID: 3, Title: "变量声明", PageType: "entity", ContentStatus: "published", ParentID: sql.NullInt64{Int64: 2, Valid: true}, Path: "1/2/3/", Summary: "var vs := 类型推断", SummaryStatus: "ready", SummaryContentHash: sql.NullString{}, BacklinkCount: 2, LinkCount: 0, TagsNormalized: "go,基础"},
		{ID: 4, Title: "context", PageType: "entity", ContentStatus: "empty", ParentID: sql.NullInt64{Int64: 2, Valid: true}, Path: "1/2/4/", Summary: "", SummaryStatus: "empty", SummaryContentHash: sql.NullString{}, BacklinkCount: 0, LinkCount: 0, TagsNormalized: ""},
	}
	db := &fakeTreeDB{pages: tree, content: map[int64]string{4: "context 是 Go 的..."}}
	out := renderKnowledgeMap(context.Background(), db, nil)

	if !strings.Contains(out, "【知识地图】") {
		t.Error("missing 知识地图 header")
	}
	if !strings.Contains(out, "Go 语言") {
		t.Error("missing Go 语言")
	}
	if !strings.Contains(out, "变量声明") {
		t.Error("missing 变量声明")
	}
	if !strings.Contains(out, "var vs := 类型推断") {
		t.Error("missing summary text for 变量声明")
	}
	if !strings.Contains(out, "context") {
		t.Error("missing context page")
	}
	// Backlink count visible
	if !strings.Contains(out, "2 反链") {
		t.Error("expected backlink count for 变量声明")
	}
	// Tag visible
	if !strings.Contains(out, "#go") {
		t.Error("expected tag #go")
	}
}

func TestRenderKnowledgeMap_PendingSummaryFallback(t *testing.T) {
	tree := []model.GetWikiPageTreeRow{
		{ID: 1, Title: "测试页", PageType: "entity", ContentStatus: "published", ParentID: sql.NullInt64{}, Path: "1/", Summary: "", SummaryStatus: "pending", SummaryContentHash: sql.NullString{}, BacklinkCount: 0, LinkCount: 0, TagsNormalized: ""},
	}
	// content not provided — empty fallback
	out := renderKnowledgeMap(context.Background(), &fakeTreeDB{pages: tree}, nil)
	if !strings.Contains(out, "摘要待更新") {
		t.Errorf("expected 摘要待更新 fallback, got: %s", out)
	}
}
