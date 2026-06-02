package ai

import (
	"strings"
	"testing"

	"learn-helper/internal/ai/skills"
)

// With nil registry, the system prompt has no skill catalog.
func TestBuildSystemPrompt_NilRegistryNoCatalog(t *testing.T) {
	got := BuildSystemPrompt(RoleWikiMaintainer, "(wiki context)", nil)
	if strings.Contains(got, "## 可用 Skill") {
		t.Errorf("expected no skill catalog when registry is nil, got: %q", got)
	}
}

// With a populated registry, the system prompt includes the skill catalog
// (name + description), but NOT any skill body.
func TestBuildSystemPrompt_IncludesCatalogButNotBodies(t *testing.T) {
	reg := skills.NewRegistry()
	_ = reg.Add(&skills.Skill{
		Name:        "explain-page",
		Description: "把当前页面讲给非专家",
		Body:        "BODY_SHOULD_NOT_APPEAR",
	})

	got := BuildSystemPrompt(RoleWikiMaintainer, "(ctx)", reg)

	if !strings.Contains(got, "## 可用 Skill") {
		t.Errorf("expected catalog section, got: %q", got)
	}
	if !strings.Contains(got, "explain-page") {
		t.Errorf("expected explain-page in catalog, got: %q", got)
	}
	if !strings.Contains(got, "把当前页面讲给非专家") {
		t.Errorf("expected description in catalog, got: %q", got)
	}
	if strings.Contains(got, "BODY_SHOULD_NOT_APPEAR") {
		t.Errorf("skill body leaked into system prompt: %q", got)
	}
}

// BuildChatSystemPrompt is now a thin wrapper; the body is NOT in the prompt.
func TestBuildChatSystemPrompt_NoBodyInPrompt(t *testing.T) {
	reg := skills.NewRegistry()
	_ = reg.Add(&skills.Skill{
		Name:        "explain-page",
		Description: "d",
		Body:        "BODY_SHOULD_NOT_APPEAR",
	})

	got := BuildChatSystemPrompt(RoleWikiMaintainer, "(ctx)", reg)
	if strings.Contains(got, "BODY_SHOULD_NOT_APPEAR") {
		t.Errorf("skill body leaked into chat system prompt: %q", got)
	}
	if !strings.Contains(got, "## 可用 Skill") {
		t.Errorf("expected catalog, got: %q", got)
	}
}
