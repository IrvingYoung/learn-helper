package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"learn-helper/internal/ai"
	"learn-helper/internal/model"
	_ "modernc.org/sqlite"
)

// setupNavTestDB creates an in-memory SQLite with a tiny wiki tree:
//
//	1  Root         (overview, parent=NULL)
//	2  Child A      (entity,   parent=1, links=[3,4], backlinks=[])
//	3  Child B      (entity,   parent=1, links=[],    backlinks=[2])
//	4  Grandchild   (entity,   parent=2, links=[],    backlinks=[2])
//	5  Lonely       (entity,   parent=NULL, content with [[Missing]])
func setupNavTestDB(t *testing.T) *AIHandler {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := `
CREATE TABLE wiki_pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    page_type TEXT NOT NULL DEFAULT 'entity',
    content TEXT NOT NULL DEFAULT '',
    tags TEXT,
    tags_normalized TEXT NOT NULL DEFAULT '',
    parent_id INTEGER REFERENCES wiki_pages(id),
    path TEXT NOT NULL DEFAULT '',
    links TEXT NOT NULL DEFAULT '[]',
    backlinks TEXT NOT NULL DEFAULT '[]',
    link_count INTEGER NOT NULL DEFAULT 0,
    backlink_count INTEGER NOT NULL DEFAULT 0,
    content_status TEXT NOT NULL DEFAULT 'empty',
    summary TEXT NOT NULL DEFAULT '',
    summary_status TEXT NOT NULL DEFAULT 'empty',
    summary_generated_at DATETIME,
    summary_content_hash TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}

	rows := []struct {
		id        int
		title     string
		slug      string
		ptype     string
		content   string
		parent    sql.NullInt64
		links     string
		backlinks string
	}{
		{1, "Root", "root", "overview", "", sql.NullInt64{}, "[]", "[]"},
		{2, "Child A", "child-a", "entity", "child A body", sql.NullInt64{Int64: 1, Valid: true}, "[3,4]", "[]"},
		{3, "Child B", "child-b", "entity", "child B body", sql.NullInt64{Int64: 1, Valid: true}, "[]", "[2]"},
		{4, "Grandchild", "grandchild", "entity", "grandchild body", sql.NullInt64{Int64: 2, Valid: true}, "[]", "[2]"},
		{5, "Lonely", "lonely", "entity", "see [[Missing]] and [[Child A]]", sql.NullInt64{}, "[2]", "[]"},
	}
	for _, r := range rows {
		_, err := db.Exec(
			`INSERT INTO wiki_pages (id, title, slug, page_type, content, parent_id, links, backlinks, content_status) VALUES (?,?,?,?,?,?,?,?,?)`,
			r.id, r.title, r.slug, r.ptype, r.content, r.parent, r.links, r.backlinks, "published",
		)
		if err != nil {
			t.Fatalf("insert %d: %v", r.id, err)
		}
	}

	return &AIHandler{db: db, queries: model.New(db)}
}

func toolCall(input map[string]any) ai.ToolCall {
	b, _ := json.Marshal(input)
	return ai.ToolCall{Input: string(b)}
}

func TestExecuteListBacklinks_ReturnsReferencingPages(t *testing.T) {
	h := setupNavTestDB(t)
	out := h.executeListBacklinks(context.Background(), toolCall(map[string]any{"page_id": 3}))
	if !strings.Contains(out, "Child A") {
		t.Errorf("expected backlink to mention Child A (which links to Child B), got: %s", out)
	}
	if !strings.Contains(out, "1 条反链") {
		t.Errorf("expected count '1 条反链', got: %s", out)
	}
}

func TestExecuteListBacklinks_NoBacklinks(t *testing.T) {
	h := setupNavTestDB(t)
	out := h.executeListBacklinks(context.Background(), toolCall(map[string]any{"page_id": 2}))
	if !strings.Contains(out, "没有反链") {
		t.Errorf("expected '没有反链', got: %s", out)
	}
}

func TestExecuteListLinks_ReturnsOutgoingPages(t *testing.T) {
	h := setupNavTestDB(t)
	out := h.executeListLinks(context.Background(), toolCall(map[string]any{"page_id": 2}))
	if !strings.Contains(out, "Child B") || !strings.Contains(out, "Grandchild") {
		t.Errorf("expected both Child B and Grandchild in outgoing links, got: %s", out)
	}
}

func TestExecuteListChildren_DepthOne(t *testing.T) {
	h := setupNavTestDB(t)
	out := h.executeListChildren(context.Background(), toolCall(map[string]any{"parent_id": 1}))
	if !strings.Contains(out, "Child A") || !strings.Contains(out, "Child B") {
		t.Errorf("expected Child A and Child B at depth 1, got: %s", out)
	}
	if strings.Contains(out, "Grandchild") {
		t.Errorf("Grandchild should not appear at depth 1, got: %s", out)
	}
}

func TestExecuteListChildren_DepthTwo(t *testing.T) {
	h := setupNavTestDB(t)
	out := h.executeListChildren(context.Background(), toolCall(map[string]any{"parent_id": 1, "depth": 2}))
	if !strings.Contains(out, "Grandchild") {
		t.Errorf("expected Grandchild at depth 2, got: %s", out)
	}
}

func TestExecuteListChildren_TopLevel(t *testing.T) {
	h := setupNavTestDB(t)
	out := h.executeListChildren(context.Background(), toolCall(map[string]any{"parent_id": 0}))
	if !strings.Contains(out, "Root") || !strings.Contains(out, "Lonely") {
		t.Errorf("expected top-level Root and Lonely, got: %s", out)
	}
}

func TestExecuteFindBrokenLinks_DetectsMissingTarget(t *testing.T) {
	h := setupNavTestDB(t)
	out := h.executeFindBrokenLinks(context.Background(), toolCall(map[string]any{"page_id": 5}))
	if !strings.Contains(out, "Missing") {
		t.Errorf("expected 'Missing' to be flagged as broken, got: %s", out)
	}
	if strings.Contains(out, "Child A") && strings.Contains(out, "死链: [[Child A") {
		t.Errorf("Child A exists — should not be flagged as broken, got: %s", out)
	}
}

func TestExecuteFindBrokenLinks_WholeWiki(t *testing.T) {
	h := setupNavTestDB(t)
	out := h.executeFindBrokenLinks(context.Background(), toolCall(map[string]any{}))
	if !strings.Contains(out, "Missing") {
		t.Errorf("whole-wiki scan should find [[Missing]] in Lonely page, got: %s", out)
	}
	if !strings.Contains(out, "Lonely") {
		t.Errorf("expected report to mention Lonely page, got: %s", out)
	}
}

func TestSnippetFromContent_SkipsTitleAndQuote(t *testing.T) {
	in := "# Title\n\n> blockquote\n---\n\n这是正文第一行。\n更多内容。"
	got := snippetFromContent(in, 80)
	if got != "这是正文第一行。" {
		t.Errorf("expected first non-meta line, got: %q", got)
	}
}

// lookup_page used to silently append a full subtree knowledge-map dump
// (~200 to ~2000 tokens depending on the page). That meant any [[X]] link
// validation call carried unpredictable cost. Now it returns only page
// metadata; AI must call list_children for subtree exploration.
func TestExecuteLookupPage_ReturnsMetadataNoSubtree(t *testing.T) {
	h := setupNavTestDB(t)
	out := h.executeLookupPage(context.Background(), toolCall(map[string]any{"title": "Child A"}))

	// Must contain the page ID and basic metadata.
	if !strings.Contains(out, `"id":2`) {
		t.Errorf("expected id=2 in output, got: %s", out)
	}
	if !strings.Contains(out, `"page_type":"entity"`) {
		t.Errorf("expected page_type metadata, got: %s", out)
	}
	if !strings.Contains(out, `"parent_id":1`) {
		t.Errorf("expected parent_id metadata, got: %s", out)
	}

	// Must NOT contain a KnowledgeMap subtree dump. The renderer emits
	// these section headings; their presence would mean the implicit
	// subtree leaked back in.
	for _, marker := range []string{"【知识库概览】", "【知识地图】", "【最近活动】"} {
		if strings.Contains(out, marker) {
			t.Errorf("lookup_page output should not contain subtree marker %q, got:\n%s", marker, out)
		}
	}

	// Bound the output size — anything more than ~500 chars suggests a
	// subtree leaked in.
	if len([]rune(out)) > 500 {
		t.Errorf("lookup_page output unexpectedly long (%d runes) — possible subtree leak:\n%s", len([]rune(out)), out)
	}
}

func TestExecuteLookupPage_NotFound(t *testing.T) {
	h := setupNavTestDB(t)
	out := h.executeLookupPage(context.Background(), toolCall(map[string]any{"title": "Nonexistent"}))
	if !strings.Contains(out, "未找到页面") {
		t.Errorf("expected '未找到页面' in output, got: %s", out)
	}
}
