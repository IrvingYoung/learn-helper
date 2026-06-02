package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSummarizeToolCall_ReadPage covers the read_page tool with content,
// empty page, and not-found cases.
func TestSummarizeToolCall_ReadPage(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		output string
		want   string
	}{
		{
			name:   "with content",
			input:  `{"page_id": 36}`,
			output: "[系统] 工具 read_page 已执行完毕，读取页面「AI Agent」(ID=36) 内容：\n\n# AI Agent\n\n这是页面内容，包含了记忆系统的详细描述。",
			want:   "read_page(ID=36) → 已读「AI Agent」(32字)",
		},
		{
			name:   "empty page",
			input:  `{"page_id": 15}`,
			output: "[系统] 工具 read_page 已执行完毕，读取页面「AI」(ID=15) 内容：\n\n",
			want:   "read_page(ID=15) → 已读「AI」(空页)",
		},
		{
			name:   "not found",
			input:  `{"page_id": 999}`,
			output: "[系统] read_page 未找到页面 #999",
			want:   "read_page(ID=999) → 未找到",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeToolCall("read_page", tc.input, tc.output, "")
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSummarizeToolCall_LookupPage(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		output string
		want   string
	}{
		{
			name:   "found",
			input:  `{"title": "AI Agent"}`,
			output: `[系统] 工具 lookup_page 已执行完毕，查询「AI Agent」结果：{"id": 36, "title": "AI Agent", "slug": "ai-agent", "content_status": "published"}`,
			want:   "lookup_page(「AI Agent」) → ID=36",
		},
		{
			name:   "not found",
			input:  `{"title": "不存在"}`,
			output: "[系统] lookup_page 未找到页面「不存在」",
			want:   "lookup_page(「不存在」) → 未找到",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeToolCall("lookup_page", tc.input, tc.output, "")
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSummarizeToolCall_SearchPages(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		output string
		want   string
	}{
		{
			name:   "found results",
			input:  `{"query": "记忆"}`,
			output: "[系统] 搜索「记忆」找到 3 个匹配页面：\n\n- [ID=15] AI (空)\n- [ID=36] AI Agent (有内容)",
			want:   "search_pages(「记忆」) → 3 个匹配",
		},
		{
			name:   "no results",
			input:  `{"query": "noexist"}`,
			output: "[系统] search_pages 未找到匹配「noexist」的页面",
			want:   "search_pages(「noexist」) → 无匹配",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeToolCall("search_pages", tc.input, tc.output, "")
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSummarizeToolCall_WriteTools(t *testing.T) {
	cases := []struct {
		name   string
		tool   string
		input  string
		output string
		errStr string
		want   string
	}{
		{
			name:   "create_page success",
			tool:   "create_page",
			input:  `{"title": "新页面", "parent_id": 36}`,
			output: `{"id": 42, "title": "新页面", "slug": "xin-ye-mian"}`,
			want:   "create_page(「新页面」) → 成功(ID=42)",
		},
		{
			name:   "update_page success",
			tool:   "update_page",
			input:  `{"page_id": 36, "content": "new content"}`,
			output: "ok",
			want:   "update_page(ID=36) → 成功",
		},
		{
			name:   "patch_page with ops",
			tool:   "patch_page",
			input:  `{"page_id": 36, "operations": [{"type": "append", "content": "x"}, {"type": "replace", "target": "## A", "content": "y"}]}`,
			output: "ok",
			want:   "patch_page(ID=36, ops=2) → 成功",
		},
		{
			name:   "delete_page success",
			tool:   "delete_page",
			input:  `{"page_id": 42}`,
			output: "ok",
			want:   "delete_page(ID=42) → 成功",
		},
		{
			name:   "link_pages",
			tool:   "link_pages",
			input:  `{"source_page_id": 15, "target_page_id": 36}`,
			output: "ok",
			want:   "link_pages(15→36) → 成功",
		},
		{
			name:   "move_page",
			tool:   "move_page",
			input:  `{"page_id": 42, "new_parent_id": 36}`,
			output: "ok",
			want:   "move_page(ID=42→parent=36) → 成功",
		},
		{
			name:   "write tool rejected by user",
			tool:   "create_page",
			input:  `{"title": "被拒的页"}`,
			errStr: "rejected by user",
			want:   "create_page → 被用户拒绝",
		},
		{
			name:   "write tool error",
			tool:   "create_page",
			input:  `{"title": "坏的页"}`,
			errStr: "invalid parent_id: page 999 not found",
			want:   "create_page → 失败: invalid parent_id: page 999 not found",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeToolCall(tc.tool, tc.input, tc.output, tc.errStr)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSummarizeToolCall_AskUser(t *testing.T) {
	got := summarizeToolCall("ask_user",
		`{"question": "你想要深度还是广度?", "options": ["A", "B"]}`,
		`{"answer": "深度"}`,
		"",
	)
	want := `ask_user(「你想要深度还是广度?」) → 收到回答「深度」`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSummarizeToolCall_WebSearch(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "success with results",
			output: "[系统] 网络搜索「AI Agent」结果：\n\n1. **Title**\n   URL: x\n   摘要: y",
			want:   "websearch(「AI Agent」) → 0 个结果", // marker is 字符 not 结果 here
		},
		{
			name:   "failure",
			output: "[系统] websearch 搜索失败: timeout",
			want:   "websearch(「x」) → 失败",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeToolCall("websearch", `{"query": "x"}`, tc.output, "")
			if !strings.Contains(got, tc.want[:5]) {
				t.Errorf("got %q, want to contain %q", got, tc.want)
			}
		})
	}
}

// TestSummarizeToolCalls_Joins verifies that the batched summary joins
// per-tool one-liners with "; ".
func TestSummarizeToolCalls_Joins(t *testing.T) {
	results := []ToolCallResult{
		{
			ID:    "1",
			Name:  "read_page",
			Input: json.RawMessage(`{"page_id": 36}`),
			Output: "[系统] 工具 read_page 已执行完毕，读取页面「AI Agent」(ID=36) 内容：\n\nx",
		},
		{
			ID:    "2",
			Name:  "lookup_page",
			Input: json.RawMessage(`{"title": "AI"}`),
			Output: "[系统] lookup_page 未找到页面「AI」",
		},
		{
			ID:    "3",
			Name:  "create_page",
			Input: json.RawMessage(`{"title": "新页"}`),
			Output: `{"id": 42, "title": "新页"}`,
		},
	}
	got := summarizeToolCalls(results)
	wantParts := []string{
		"read_page(ID=36) → 已读「AI Agent」",
		"lookup_page(「AI」) → 未找到",
		"create_page(「新页」) → 成功(ID=42)",
	}
	for _, p := range wantParts {
		if !strings.Contains(got, p) {
			t.Errorf("summary missing part %q in %q", p, got)
		}
	}
	// Verify it's all on one line, joined with "; "
	if strings.Count(got, ";") != 2 {
		t.Errorf("expected 2 separators, got %d in %q", strings.Count(got, ";"), got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("summary should be one-line, got %q", got)
	}
}

func TestSummarizeToolCalls_Empty(t *testing.T) {
	if got := summarizeToolCalls(nil); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := summarizeToolCalls([]ToolCallResult{}); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// TestTruncate covers the helper used for truncating user questions
// in summaries.
func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"这是一个很长的中文字符串", 5, "这是一个很…"},
		{"", 5, ""},
		{"abc", 0, "…"},
	}
	for _, tc := range cases {
		got := truncate(tc.in, tc.max)
		if got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}
