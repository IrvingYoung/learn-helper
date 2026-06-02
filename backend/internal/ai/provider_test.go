package ai

import (
	"strings"
	"testing"

	"learn-helper/internal/ai/skills"
)

func TestBuildChatSystemPrompt_NilSkillUnchanged(t *testing.T) {
	base := BuildSystemPrompt(RoleWikiMaintainer, "(wiki context)")
	got := BuildChatSystemPrompt(RoleWikiMaintainer, "(wiki context)", nil)
	if got != base {
		t.Errorf("nil skill should produce base prompt unchanged\nbase=%q\ngot =%q", base, got)
	}
}

func TestBuildChatSystemPrompt_AppendsSkillBody(t *testing.T) {
	skill := &skills.Skill{
		Name: "explain-page",
		Body: "你正在用通俗解释模式。",
	}
	got := BuildChatSystemPrompt(RoleWikiMaintainer, "(ctx)", skill)
	if !strings.Contains(got, "你正在用通俗解释模式。") {
		t.Errorf("prompt missing skill body: %q", got)
	}
	if !strings.Contains(got, "## 当前 Skill: explain-page") {
		t.Errorf("prompt missing skill header: %q", got)
	}
	// Wiki context must still be there.
	if !strings.Contains(got, "(ctx)") {
		t.Errorf("prompt missing wiki context: %q", got)
	}
}
