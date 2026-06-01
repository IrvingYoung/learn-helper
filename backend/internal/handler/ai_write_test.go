package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseWriteInput_CreatePage(t *testing.T) {
	in := json.RawMessage(`{"title":"Go 并发","parent_id":16}`)
	got, err := parseWriteInput("create_page", in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got["title"] != "Go 并发" {
		t.Errorf("title = %v", got["title"])
	}
	if got["page_id"] != nil {
		// create_page has no page_id, but a placeholder should be set
		t.Errorf("page_id should be nil, got %v", got["page_id"])
	}
}

func TestParseWriteInput_UpdatePage(t *testing.T) {
	in := json.RawMessage(`{"page_id":42,"content":"# x"}`)
	got, err := parseWriteInput("update_page", in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got["page_id"] != float64(42) {
		t.Errorf("page_id = %v (%T)", got["page_id"], got["page_id"])
	}
}

func TestParseWriteInput_MissingRequired(t *testing.T) {
	in := json.RawMessage(`{"content":"# x"}`)
	_, err := parseWriteInput("update_page", in)
	if err == nil {
		t.Fatal("expected error for missing page_id")
	}
}

func TestParseWriteInput_RejectsPlaceholder(t *testing.T) {
	in := json.RawMessage(`{"title":"x","parent_id":"{{action:a1.page_id}}"}`)
	_, err := parseWriteInput("create_page", in)
	if err == nil {
		t.Fatal("expected error for placeholder reference")
	}
	if !strings.Contains(err.Error(), "placeholder") {
		t.Errorf("error should mention placeholder, got: %v", err)
	}
}

func TestPreviewWrite(t *testing.T) {
	cases := []struct {
		tool string
		in   map[string]any
		want string
	}{
		{"create_page", map[string]any{"title": "Go 并发", "parent_id": float64(16)}, "在父页 16 下创建页面「Go 并发」"},
		{"update_page", map[string]any{"page_id": float64(42)}, "更新页面 42"},
		{"delete_page", map[string]any{"page_id": float64(42)}, "删除页面 42"},
		{"move_page", map[string]any{"page_id": float64(42), "new_parent_id": float64(16)}, "把页面 42 移到父页 16 下"},
		{"link_pages", map[string]any{"source_page_id": float64(1), "target_page_id": float64(2)}, "在页面 1 添加指向 2 的链接"},
		{"patch_page", map[string]any{"page_id": float64(42)}, "增量编辑页面 42"},
	}
	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			got := previewWrite(c.tool, c.in)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
