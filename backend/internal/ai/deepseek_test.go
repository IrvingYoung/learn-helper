package ai

import "testing"

// buildDeepSeekMessages must prepend req.SystemPrompt as a role="system"
// message so the OpenAI-compatible API actually sees the system instructions.
// Regression test for the bug where SystemPrompt was silently dropped by
// both OpenCodeProvider and DeepSeekProvider.
func TestBuildDeepSeekMessages_PrependsSystemPrompt(t *testing.T) {
	req := ChatRequest{
		SystemPrompt: "you are wiki_maintainer",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	}

	got := buildDeepSeekMessages(req)

	if len(got) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d: %+v", len(got), got)
	}
	if got[0].Role != "system" {
		t.Errorf("expected got[0].Role == 'system', got %q", got[0].Role)
	}
	if got[0].Content != "you are wiki_maintainer" {
		t.Errorf("expected system content preserved, got %q", got[0].Content)
	}
	if got[1].Role != "user" {
		t.Errorf("expected got[1] to be the user message, got %q", got[1].Role)
	}
}

// When SystemPrompt is empty, no synthetic system message should be added.
func TestBuildDeepSeekMessages_EmptySystemPromptOmitted(t *testing.T) {
	req := ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
	}

	got := buildDeepSeekMessages(req)

	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d: %+v", len(got), got)
	}
	if got[0].Role != "user" {
		t.Errorf("expected user message, got role=%q", got[0].Role)
	}
}
