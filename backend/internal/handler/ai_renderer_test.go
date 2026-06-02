package handler

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

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

func (f *fakeTreeDB) GetPageContentsForFallback(ctx context.Context) (map[int64]string, error) {
	if f.content == nil {
		return map[int64]string{}, nil
	}
	return f.content, nil
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

// renderSummaryLine — fallback should pick a meaningful first line, not the
// H1 / blockquote / horizontal-rule that every page tends to start with.
// Regression test for audit item #8 (前 80 字降级常常全是 markdown 装饰).
func TestRenderSummaryLine_FallbackSkipsTitleAndQuote(t *testing.T) {
	node := model.GetWikiPageTreeRow{ID: 1, Title: "Go", SummaryStatus: "failed"}
	content := "# Go\n\n> 简洁、可靠、高效的编程语言。\n\n---\n\n本页讲 Go 的核心特性和工程实践。"
	got := renderSummaryLine(node, map[int64]string{1: content})

	if strings.Contains(got, "# Go") || strings.Contains(got, ">") || strings.Contains(got, "---") {
		t.Errorf("fallback leaked H1/quote/HR into the summary line: %q", got)
	}
	if !strings.Contains(got, "本页讲 Go") {
		t.Errorf("expected first real content line, got: %q", got)
	}
	if !strings.Contains(got, "(摘要生成失败)") {
		t.Errorf("expected failure tag, got: %q", got)
	}
}

func TestRenderSummaryLine_NoContentNoTag(t *testing.T) {
	node := model.GetWikiPageTreeRow{ID: 1, SummaryStatus: "empty"}
	got := renderSummaryLine(node, map[int64]string{}) // no fallback content
	if got != "" {
		t.Errorf("empty status with no content should yield empty string, got: %q", got)
	}
}

type fakeLogDB struct {
	entries []model.WikiLog
}

func (f *fakeLogDB) GetRecentWikiLog(ctx context.Context, arg model.GetRecentWikiLogParams) ([]model.WikiLog, error) {
	return f.entries, nil
}

func TestRenderRecentLog_Empty(t *testing.T) {
	db := &fakeLogDB{}
	out := renderRecentLog(context.Background(), db, 7*24*time.Hour, 20)
	if out != "" {
		t.Errorf("expected empty output for empty log, got: %s", out)
	}
}

func TestRenderRecentLog_WithEntries(t *testing.T) {
	now := time.Now()
	db := &fakeLogDB{
		entries: []model.WikiLog{
			{ID: 1, Action: "create", PageTitle: "Go 语言", PageID: sql.NullInt64{Int64: 2, Valid: true}, CreatedAt: now.Add(-1 * time.Hour), Summary: sql.NullString{String: "新建主分类", Valid: true}},
			{ID: 2, Action: "update", PageTitle: "channel", PageID: sql.NullInt64{Int64: 3, Valid: true}, CreatedAt: now.Add(-2 * time.Hour), Summary: sql.NullString{}},
		},
	}
	out := renderRecentLog(context.Background(), db, 7*24*time.Hour, 20)
	if !strings.Contains(out, "【最近活动】") {
		t.Error("missing 最近活动 header")
	}
	if !strings.Contains(out, "Go 语言") {
		t.Error("missing Go 语言 entry")
	}
	if !strings.Contains(out, "[create]") {
		t.Error("missing action tag")
	}
}

func TestRenderHealthCheck_DeadPage(t *testing.T) {
	// Page with content but 0 backlinks and 0 links → "dead page"
	tree := []model.GetWikiPageTreeRow{
		{ID: 1, Title: "孤岛", PageType: "entity", ContentStatus: "published", LinkCount: 0, BacklinkCount: 0},
	}
	db := &fakeTreeDB{pages: tree}
	out := renderHealthCheck(context.Background(), db)
	if !strings.Contains(out, "死页") {
		t.Errorf("expected 死页 warning, got: %s", out)
	}
	if !strings.Contains(out, "孤岛") {
		t.Errorf("expected page title in warning, got: %s", out)
	}
}

func TestRenderKnowledgeGaps_GroupedByCategory(t *testing.T) {
	tree := []model.GetWikiPageTreeRow{
		{ID: 1, Title: "Go 语言", PageType: "overview", ParentID: sql.NullInt64{}, Path: "1/"},
		{ID: 2, Title: "channel", PageType: "entity", ContentStatus: "empty", ParentID: sql.NullInt64{Int64: 1, Valid: true}, Path: "1/2/"},
		{ID: 3, Title: "context", PageType: "entity", ContentStatus: "empty", ParentID: sql.NullInt64{Int64: 1, Valid: true}, Path: "1/3/"},
		{ID: 4, Title: "算法", PageType: "overview", ParentID: sql.NullInt64{}, Path: "4/"},
		{ID: 5, Title: "树", PageType: "entity", ContentStatus: "empty", ParentID: sql.NullInt64{Int64: 4, Valid: true}, Path: "4/5/"},
	}
	db := &fakeTreeDB{pages: tree} // reuse fakeTreeDB from Task 2.2
	out := renderKnowledgeGaps(context.Background(), db)
	if !strings.Contains(out, "Go 语言") {
		t.Error("expected Go 语言 in gaps")
	}
	if !strings.Contains(out, "channel") {
		t.Error("expected channel in gaps")
	}
	if !strings.Contains(out, "算法") {
		t.Error("expected 算法 in gaps")
	}
	// Verify grouping: Go 语言的空页应该一起列
	idx := strings.Index(out, "Go 语言")
	idxAlg := strings.Index(out, "算法")
	if idx > idxAlg {
		t.Error("expected Go 语言 to appear before 算法")
	}
}

type fullMockDB struct {
	*fakeOverviewDB
	*fakeTreeDB
	*fakeLogDB
}

func TestBuildKnowledgeMap_Integration(t *testing.T) {
	db := &fullMockDB{
		fakeOverviewDB: &fakeOverviewDB{pages: []model.WikiPage{
			{ID: 1, Title: "Go", ContentStatus: "published"},
			{ID: 2, Title: "channel", ContentStatus: "empty"},
		}},
		fakeTreeDB: &fakeTreeDB{pages: []model.GetWikiPageTreeRow{
			{ID: 1, Title: "Go", PageType: "overview", ContentStatus: "published", Path: "1/", Summary: "Go 入门", SummaryStatus: "ready"},
			{ID: 2, Title: "channel", PageType: "entity", ContentStatus: "empty", ParentID: sql.NullInt64{Int64: 1, Valid: true}, Path: "1/2/", SummaryStatus: "empty"},
		}},
		fakeLogDB: &fakeLogDB{entries: []model.WikiLog{
			{ID: 1, Action: "create", PageTitle: "Go", CreatedAt: time.Now(), PageID: sql.NullInt64{Int64: 1, Valid: true}},
		}},
	}

	out := buildKnowledgeMap(context.Background(), db, nil)
	// Should contain all sections
	for _, section := range []string{"【知识库概览】", "【知识地图】", "【最近活动】", "【结构健康检查】", "【知识缺口】"} {
		if !strings.Contains(out, section) {
			t.Errorf("missing section: %s", section)
		}
	}
}
