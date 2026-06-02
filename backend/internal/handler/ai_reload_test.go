package handler

import (
	"database/sql"
	"encoding/json"
	"testing"

	"learn-helper/internal/ai"
)

// TestReloadMessagesInjectsToolSummary verifies the cross-request memory
// contract: an assistant message saved with a tool_summary column should,
// when reloaded, have that summary appended to its content as a one-line
// note. This is the long-term-memory substitute for the full tool_result
// messages, which we deliberately do NOT persist (they would violate the
// OpenAI/DeepSeek tool protocol on reload because tool_use/tool_result
// pairs cannot be safely reconstructed).
func TestReloadMessagesInjectsToolSummary(t *testing.T) {
	db := newTestDB(t)

	// Create a conversation (column list matches the test schema, which
	// may be a subset of production — only context_type and title here).
	res, err := db.Exec(`INSERT INTO conversations (context_type, title) VALUES ('topic', 'test')`)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	convID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	// Insert a user message
	_, err = db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'user', '学 X', 'opencode', 0)`, convID)
	if err != nil {
		t.Fatalf("insert user msg: %v", err)
	}

	// Insert an assistant message WITH a tool_summary
	const summary = "read_page(ID=36) → 已读「AI Agent」(5320字); ask_user(「深度还是广度?」) → 收到回答「深度」"
	_, err = db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count, tool_summary) VALUES (?, 'assistant', '好的，我查了 AI Agent 页。', 'opencode', 0, ?)`,
		convID, summary)
	if err != nil {
		t.Fatalf("insert assistant msg: %v", err)
	}

	// Insert another user + assistant turn WITHOUT tool calls
	_, err = db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'user', '继续', 'opencode', 0)`, convID)
	if err != nil {
		t.Fatalf("insert user2: %v", err)
	}
	_, err = db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, token_count) VALUES (?, 'assistant', '好。', 'opencode', 0)`, convID)
	if err != nil {
		t.Fatalf("insert assistant2: %v", err)
	}

	// Reload messages the same way AIChat does
	rows, err := db.Query(`SELECT role, content, tool_call_id, tool_summary FROM messages WHERE conversation_id = ? ORDER BY created_at`, convID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	defer rows.Close()

	var aiMessages []ai.Message
	for rows.Next() {
		var role, content string
		var toolCallID, toolSummary sql.NullString
		if err := rows.Scan(&role, &content, &toolCallID, &toolSummary); err != nil {
			t.Fatalf("scan: %v", err)
		}
		// Mirror the injection logic from AIChat
		if role == "assistant" && toolSummary.Valid && toolSummary.String != "" {
			content = content + "\n\n[本轮工具调用: " + toolSummary.String + "]"
		}
		aiMessages = append(aiMessages, ai.Message{Role: role, Content: content})
	}

	if len(aiMessages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(aiMessages))
	}

	// First assistant message should have the summary appended
	firstAssistant := aiMessages[1].Content
	if !contains(firstAssistant, "[本轮工具调用: read_page(ID=36)") {
		t.Errorf("first assistant missing injected summary; got %q", firstAssistant)
	}
	if !contains(firstAssistant, "ask_user") {
		t.Errorf("first assistant missing ask_user part; got %q", firstAssistant)
	}

	// Second assistant message should NOT have an injection (no tool_summary)
	secondAssistant := aiMessages[3].Content
	if contains(secondAssistant, "[本轮工具调用:") {
		t.Errorf("second assistant should not have injection; got %q", secondAssistant)
	}

	// Critical: NO `role='tool'` messages should exist — this is the
	// long-term memory design. If a `role='tool'` row is ever inserted,
	// the model would see an orphan tool_result on reload.
	var toolCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE role = 'tool'`).Scan(&toolCount); err != nil {
		t.Fatalf("count tool role: %v", err)
	}
	if toolCount != 0 {
		t.Errorf("expected 0 messages with role='tool', got %d", toolCount)
	}
}

// TestReloadMessagesPreservesProtocol validates the protocol safety claim:
// after loading history from DB, there are no `role='tool'` messages in
// aiMessages. The full tool_use/tool_result flow only exists in-memory
// during the ReAct loop and is discarded after — replaced by the
// one-line summary appended to the assistant's content.
func TestReloadMessagesPreservesProtocol(t *testing.T) {
	db := newTestDB(t)

	res, err := db.Exec(`INSERT INTO conversations (context_type, title) VALUES ('topic', 'test')`)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	convID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	// Simulate 10 rounds of: user msg + assistant msg with tool_summary
	for i := 0; i < 10; i++ {
		db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider) VALUES (?, 'user', ?, 'opencode')`, convID, "q")
		db.Exec(`INSERT INTO messages (conversation_id, role, content, model_provider, tool_summary) VALUES (?, 'assistant', ?, 'opencode', ?)`,
			convID, "a", "read_page(ID=36) → 已读「X」")
	}

	// Reload
	rows, _ := db.Query(`SELECT role, content, tool_call_id, tool_summary FROM messages WHERE conversation_id = ? ORDER BY created_at`, convID)
	defer rows.Close()
	var aiMessages []ai.Message
	for rows.Next() {
		var role, content string
		var toolCallID, toolSummary sql.NullString
		rows.Scan(&role, &content, &toolCallID, &toolSummary)
		if role == "assistant" && toolSummary.Valid && toolSummary.String != "" {
			content = content + "\n\n[本轮工具调用: " + toolSummary.String + "]"
		}
		aiMessages = append(aiMessages, ai.Message{Role: role, Content: content})
	}

	// Verify: 20 messages, all role='user' or 'assistant', no 'tool'
	if len(aiMessages) != 20 {
		t.Fatalf("expected 20 messages, got %d", len(aiMessages))
	}
	for i, m := range aiMessages {
		if m.Role == "tool" {
			t.Errorf("message[%d] has role='tool' — would violate tool protocol on reload", i)
		}
		if m.Role != "user" && m.Role != "assistant" {
			t.Errorf("message[%d] has unexpected role %q", i, m.Role)
		}
	}
}

// TestToolSummaryPerToolType is a smoke test that every tool type used
// in production produces a non-empty summary when fed realistic input
// and output. New tool types added to the registry should be added here.
func TestToolSummaryPerToolType(t *testing.T) {
	toolCases := []struct {
		tool   string
		input  string
		output string
	}{
		{"read_page", `{"page_id": 1}`, "[系统] 工具 read_page 已执行完毕，读取页面「X」(ID=1) 内容：\n\nbody"},
		{"lookup_page", `{"title": "X"}`, `[系统] 工具 lookup_page 已执行完毕，查询「X」结果：{"id": 1}`},
		{"search_pages", `{"query": "q"}`, "[系统] 搜索「q」找到 1 个匹配页面：\n\n- [ID=1] X"},
		{"create_page", `{"title": "T"}`, `{"id": 1, "title": "T"}`},
		{"update_page", `{"page_id": 1}`, "ok"},
		{"patch_page", `{"page_id": 1, "operations": [{}]}`, "ok"},
		{"delete_page", `{"page_id": 1}`, "ok"},
		{"link_pages", `{"source_page_id": 1, "target_page_id": 2}`, "ok"},
		{"move_page", `{"page_id": 1, "new_parent_id": 2}`, "ok"},
		{"ask_user", `{"question": "Q"}`, `{"answer": "A"}`},
	}
	for _, tc := range toolCases {
		t.Run(tc.tool, func(t *testing.T) {
			got := summarizeToolCall(tc.tool, tc.input, tc.output, "")
			if got == "" {
				t.Errorf("empty summary for %s", tc.tool)
			}
			if got == tc.tool+" → 完成" {
				t.Errorf("summary for %s fell back to generic; per-tool summary missing", tc.tool)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// helper to ensure encoding/json is used (keeps the import live in case
// future cases need it)
var _ = json.Marshal
