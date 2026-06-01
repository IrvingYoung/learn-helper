package handler

import (
	"context"
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
