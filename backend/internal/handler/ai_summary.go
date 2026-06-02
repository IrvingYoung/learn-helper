package handler

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// summarizeToolCall produces a short human-readable one-liner describing
// a tool call's outcome. Heuristic per tool type — no AI call. Output is
// appended to assistant messages (via the messages.tool_summary column)
// so the model can recall what tools it used in previous turns without
// re-loading full tool_result content (which would violate the
// OpenAI/DeepSeek tool protocol because tool_use/tool_result pairs
// cannot be safely split across requests).
func summarizeToolCall(name, input, output, errStr string) string {
	if errStr != "" {
		short := errStr
		if len([]rune(short)) > 60 {
			short = string([]rune(short)[:60]) + "…"
		}
		// "rejected by user" is a normal user action, not a real error
		if strings.Contains(errStr, "rejected") {
			return fmt.Sprintf("%s → 被用户拒绝", name)
		}
		return fmt.Sprintf("%s → 失败: %s", name, short)
	}

	switch name {
	case "read_page":
		var p struct {
			PageID float64 `json:"page_id"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		title, n := parseReadPageResult(output)
		if title == "" {
			return fmt.Sprintf("read_page(ID=%d) → 未找到", int(p.PageID))
		}
		if n == 0 {
			return fmt.Sprintf("read_page(ID=%d) → 已读「%s」(空页)", int(p.PageID), title)
		}
		return fmt.Sprintf("read_page(ID=%d) → 已读「%s」(%d字)", int(p.PageID), title, n)

	case "lookup_page":
		var p struct {
			Title string `json:"title"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		id, found := parseLookupResult(output)
		if !found {
			return fmt.Sprintf("lookup_page(「%s」) → 未找到", p.Title)
		}
		return fmt.Sprintf("lookup_page(「%s」) → ID=%d", p.Title, id)

	case "search_pages":
		var p struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		n := parseIntPrefix(output, "搜索")
		if n == 0 && strings.Contains(output, "未找到") {
			return fmt.Sprintf("search_pages(「%s」) → 无匹配", p.Query)
		}
		return fmt.Sprintf("search_pages(「%s」) → %d 个匹配", p.Query, n)

	case "websearch":
		var p struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		if strings.Contains(output, "失败") {
			return fmt.Sprintf("websearch(「%s」) → 失败", p.Query)
		}
		n := parseIntPrefix(output, "结果")
		return fmt.Sprintf("websearch(「%s」) → %d 个结果", p.Query, n)

	case "webfetch":
		var p struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		if strings.Contains(output, "失败") {
			return fmt.Sprintf("webfetch → 失败")
		}
		n := parseIntPrefix(output, "字符")
		return fmt.Sprintf("webfetch → 成功(%d字)", n)

	case "create_page":
		var p struct {
			Title    string  `json:"title"`
			ParentID float64 `json:"parent_id"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		id, ok := parseIDFromJSON(output, `"id":`)
		if ok {
			return fmt.Sprintf("create_page(「%s」) → 成功(ID=%d)", p.Title, id)
		}
		return fmt.Sprintf("create_page(「%s」) → 成功", p.Title)

	case "update_page":
		var p struct {
			PageID float64 `json:"page_id"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		return fmt.Sprintf("update_page(ID=%d) → 成功", int(p.PageID))

	case "patch_page":
		var p struct {
			PageID     float64 `json:"page_id"`
			Operations []any   `json:"operations"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		return fmt.Sprintf("patch_page(ID=%d, ops=%d) → 成功", int(p.PageID), len(p.Operations))

	case "delete_page":
		var p struct {
			PageID float64 `json:"page_id"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		return fmt.Sprintf("delete_page(ID=%d) → 成功", int(p.PageID))

	case "link_pages":
		var p struct {
			SourcePageID float64 `json:"source_page_id"`
			TargetPageID float64 `json:"target_page_id"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		return fmt.Sprintf("link_pages(%d→%d) → 成功", int(p.SourcePageID), int(p.TargetPageID))

	case "move_page":
		var p struct {
			PageID     float64 `json:"page_id"`
			NewParent  float64 `json:"new_parent_id"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		return fmt.Sprintf("move_page(ID=%d→parent=%d) → 成功", int(p.PageID), int(p.NewParent))

	case "ask_user":
		var p struct {
			Question string `json:"question"`
		}
		_ = json.Unmarshal([]byte(input), &p)
		q := truncate(p.Question, 40)
		answer := extractAnswerFromJSON(output)
		return fmt.Sprintf("ask_user(「%s」) → 收到回答「%s」", q, truncate(answer, 40))
	}

	return fmt.Sprintf("%s → 完成", name)
}

// summarizeToolCalls builds a one-line summary of a batch of tool calls,
// joined by "; ". Empty if no calls. Used as the assistant message's
// tool_summary field for cross-request context.
func summarizeToolCalls(results []ToolCallResult) string {
	if len(results) == 0 {
		return ""
	}
	parts := make([]string, 0, len(results))
	for _, r := range results {
		parts = append(parts, summarizeToolCall(r.Name, string(r.Input), r.Output, r.Error))
	}
	return strings.Join(parts, "; ")
}

// --- result parsers ---

var (
	readPageHeaderRe = regexp.MustCompile(`读取页面「([^」]+)」`)
	readPageContentRe = regexp.MustCompile(`内容：\n\n`)
	lookupIDRe        = regexp.MustCompile(`"id":\s*(\d+)`)
	intPrefixRe       = regexp.MustCompile(`找到\s*(\d+)\s*个匹配页面`)
	intResultRe       = regexp.MustCompile(`(\d+)\s*个结果`)
	intCharRe         = regexp.MustCompile(`共\s*(\d+)\s*字符`)
	searchCountRe     = regexp.MustCompile(`找到\s*(\d+)\s*个匹配页面`)
	searchNotFoundRe  = regexp.MustCompile(`未找到匹配`)
	answerJSONRe      = regexp.MustCompile(`"answer":\s*"([^"]*)"`)
)

func parseReadPageResult(s string) (title string, charCount int) {
	m := readPageHeaderRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return "", 0
	}
	title = m[1]
	if loc := readPageContentRe.FindStringIndex(s); loc != nil {
		content := s[loc[1]:]
		charCount = len([]rune(content))
	}
	return
}

func parseLookupResult(s string) (id int64, found bool) {
	if strings.Contains(s, "未找到页面") {
		return 0, false
	}
	m := lookupIDRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0, false
	}
	var n int64
	if _, err := fmt.Sscanf(m[1], "%d", &n); err != nil {
		return 0, false
	}
	return n, true
}

func parseIntPrefix(s, marker string) int {
	switch marker {
	case "搜索":
		if m := searchCountRe.FindStringSubmatch(s); len(m) >= 2 {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			return n
		}
	case "结果":
		if m := intResultRe.FindStringSubmatch(s); len(m) >= 2 {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			return n
		}
	case "字符":
		if m := intCharRe.FindStringSubmatch(s); len(m) >= 2 {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			return n
		}
	}
	return 0
}

func parseIDFromJSON(s, marker string) (int64, bool) {
	pattern := regexp.MustCompile(regexp.QuoteMeta(marker) + `\s*(\d+)`)
	m := pattern.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0, false
	}
	var n int64
	if _, err := fmt.Sscanf(m[1], "%d", &n); err != nil {
		return 0, false
	}
	return n, true
}

func extractAnswerFromJSON(s string) string {
	m := answerJSONRe.FindStringSubmatch(s)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
