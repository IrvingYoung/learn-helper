package cron

import (
	"strings"
	"testing"
	"time"
)

// TestBuildUserMessage_TaskInMessage verifies the task name and prompt appear
// in the user message (not just the system prompt). This is the regression
// test for the bug where the AI didn't know what task to execute because the
// task prompt was only in the system prompt's `## 本次任务` line.
func TestBuildUserMessage_TaskInMessage(t *testing.T) {
	task := &Task{
		Name:   "GitHub 每日趋势",
		Prompt: "抓取 github.com/trending 前 10 个 repo，写中文摘要，存到 wiki",
	}
	now := time.Date(2026, 6, 2, 15, 58, 3, 0, time.Local)
	msg := buildUserMessage(task, now)

	if !strings.Contains(msg, task.Name) {
		t.Errorf("user message must contain task name %q, got:\n%s", task.Name, msg)
	}
	if !strings.Contains(msg, task.Prompt) {
		t.Errorf("user message must contain task prompt %q, got:\n%s", task.Prompt, msg)
	}
	if !strings.Contains(msg, "请执行") {
		t.Errorf("user message should start with an instruction, got:\n%s", msg)
	}
	if !strings.Contains(msg, "1-2 句") {
		t.Errorf("user message should ask for a summary at the end, got:\n%s", msg)
	}
}

// TestBuildUserMessage_EmptyPrompt — when prompt is empty, the message still
// has structure (this is a degenerate case; the handler should reject empty
// prompts at creation time, but the runner is defensive).
func TestBuildUserMessage_EmptyPrompt(t *testing.T) {
	task := &Task{Name: "X", Prompt: ""}
	msg := buildUserMessage(task, time.Now())
	if !strings.Contains(msg, "X") {
		t.Errorf("name should be in message")
	}
}
