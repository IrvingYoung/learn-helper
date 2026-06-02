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

// Provider prompt caching (OpenAI / DeepSeek) hits on the prefix of the
// system prompt. The wikiContext changes on every wiki write, so it must
// live at the *end* — otherwise it invalidates the prefix that comes after
// it. This test asserts that two calls with different wikiContext but the
// same skill registry share an identical leading section large enough to
// be worth caching.
func TestBuildSystemPrompt_StaticPrefixStableAcrossWikiContext(t *testing.T) {
	reg := skills.NewRegistry()
	_ = reg.Add(&skills.Skill{Name: "explain-page", Description: "d", Body: "b"})

	a := BuildSystemPrompt(RoleWikiMaintainer, "knowledge map A", reg)
	b := BuildSystemPrompt(RoleWikiMaintainer, "knowledge map B (very different content)", reg)

	// Find the longest common prefix.
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}

	if n < 1500 {
		t.Errorf("static prefix too short: only %d bytes match. Provider prompt cache will not see meaningful savings. First difference at byte %d:\n  a: %q\n  b: %q",
			n, n, snippet(a, n), snippet(b, n))
	}
	t.Logf("static prefix length: %d bytes (total a=%d, b=%d)", n, len(a), len(b))

	// The shared prefix must cover the knowledgeMapUsageGuide so the guide
	// (which explains how to read the map) is cached alongside the rules.
	prefix := a[:n]
	if !strings.Contains(prefix, "知识地图使用") {
		t.Errorf("static prefix does not cover knowledgeMapUsageGuide — guide is being placed inside the dynamic tail")
	}

	// And it must NOT include either the dateStr or the wikiContext, since
	// both are per-request.
	if strings.Contains(prefix, "knowledge map A") || strings.Contains(prefix, "knowledge map B") {
		t.Errorf("static prefix contains wikiContext fragment — prefix is not actually static")
	}
}

func snippet(s string, start int) string {
	end := start + 60
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}

// wikiMaintainerStaticPrompt is the cacheable prefix. It must contain no
// per-request data — specifically no current date, no wiki context.
// Otherwise the prompt prefix changes day-to-day or write-to-write and the
// provider cache can never hit on it.
func TestWikiMaintainerStaticPrompt_ContainsNoDynamicFields(t *testing.T) {
	p := wikiMaintainerStaticPrompt

	// Should not contain any 4-digit year (typically 20xx in dates).
	for year := 2020; year < 2040; year++ {
		if strings.Contains(p, intToYearString(year)) {
			t.Errorf("static prompt contains hard-coded year %d — likely a date leaked in", year)
		}
	}
	// Should not contain the "## 当前日期" heading — that's the dynamic section.
	if strings.Contains(p, "## 当前日期") {
		t.Error("static prompt contains '## 当前日期' heading — current date section should live in buildWikiMaintainerDynamic")
	}
}

func intToYearString(y int) string {
	return string(rune('0'+y/1000)) +
		string(rune('0'+(y/100)%10)) +
		string(rune('0'+(y/10)%10)) +
		string(rune('0'+y%10))
}
