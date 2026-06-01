package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"learn-helper/internal/model"
)

// ---------------------------------------------------------------------------
// Standalone test helpers that mirror engine logic without requiring a DB
// ---------------------------------------------------------------------------

// testAction is a simplified action for topo sort testing.
type testAction struct {
	ID        string
	DependsOn []string
}

// testTopoSort performs Kahn's algorithm on testAction slices.
func testTopoSort(actions []testAction) ([]string, error) {
	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	for _, a := range actions {
		inDegree[a.ID] = 0
		adj[a.ID] = nil
	}

	for _, a := range actions {
		for _, dep := range a.DependsOn {
			if _, ok := inDegree[dep]; !ok {
				continue
			}
			adj[dep] = append(adj[dep], a.ID)
			inDegree[a.ID]++
		}
	}

	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		sorted = append(sorted, id)

		for _, dependent := range adj[id] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(sorted) != len(actions) {
		return nil, errCircular
	}

	return sorted, nil
}

var errCircular = &circularError{}

type circularError struct{}

func (e *circularError) Error() string { return "circular dependency detected" }

// testReplacePlaceholders mirrors the engine's placeholder replacement
// using a simple map[string]any result store. It returns an error
// when any placeholder cannot be resolved (mirrors production behavior).
func testReplacePlaceholders(input string, results map[string]any) (string, error) {
	var unresolved []string
	out := placeholderPattern.ReplaceAllStringFunc(input, func(match string) string {
		subs := placeholderPattern.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		actionID := subs[1]
		field := subs[2]

		res, ok := results[actionID]
		if !ok {
			unresolved = append(unresolved, actionID+"."+field)
			return match
		}
		val := resolveField(res, field)
		if val == nil {
			unresolved = append(unresolved, actionID+"."+field)
			return match
		}
		return fmtVal(val)
	})
	if len(unresolved) > 0 {
		return "", fmt.Errorf("unresolved placeholders: %s", strings.Join(unresolved, ", "))
	}
	return out, nil
}

func fmtVal(v any) string {
	switch n := v.(type) {
	case int64:
		return formatInt(n)
	case float64:
		if n == float64(int64(n)) {
			return formatInt(int64(n))
		}
		b, _ := json.Marshal(n)
		return string(b)
	case json.Number:
		return string(n)
	case string:
		return n
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestTopologicalSort_NoDeps(t *testing.T) {
	actions := []testAction{
		{ID: "a1", DependsOn: nil},
		{ID: "a2", DependsOn: nil},
		{ID: "a3", DependsOn: nil},
	}

	sorted, err := testTopoSort(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(sorted))
	}

	// All should appear; order among independent nodes is not guaranteed
	seen := map[string]bool{}
	for _, id := range sorted {
		seen[id] = true
	}
	for _, id := range []string{"a1", "a2", "a3"} {
		if !seen[id] {
			t.Errorf("expected action %s in sorted output", id)
		}
	}
}

func TestTopologicalSort_WithDeps(t *testing.T) {
	// Diamond: a1 -> a2, a1 -> a3, a2 -> a4, a3 -> a4
	actions := []testAction{
		{ID: "a1", DependsOn: nil},
		{ID: "a2", DependsOn: []string{"a1"}},
		{ID: "a3", DependsOn: []string{"a1"}},
		{ID: "a4", DependsOn: []string{"a2", "a3"}},
	}

	sorted, err := testTopoSort(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(sorted))
	}

	// Verify ordering constraints
	pos := make(map[string]int)
	for i, id := range sorted {
		pos[id] = i
	}

	if pos["a1"] >= pos["a2"] {
		t.Error("a1 should come before a2")
	}
	if pos["a1"] >= pos["a3"] {
		t.Error("a1 should come before a3")
	}
	if pos["a2"] >= pos["a4"] {
		t.Error("a2 should come before a4")
	}
	if pos["a3"] >= pos["a4"] {
		t.Error("a3 should come before a4")
	}
}

func TestTopologicalSort_Circular(t *testing.T) {
	// a1 -> a2 -> a3 -> a1
	actions := []testAction{
		{ID: "a1", DependsOn: []string{"a3"}},
		{ID: "a2", DependsOn: []string{"a1"}},
		{ID: "a3", DependsOn: []string{"a2"}},
	}

	_, err := testTopoSort(actions)
	if err == nil {
		t.Fatal("expected error for circular dependency, got nil")
	}
}

func TestReplacePlaceholders(t *testing.T) {
	results := map[string]any{
		"a1": map[string]any{
			"page_id": int64(42),
			"slug":    "my-page",
		},
		"a2": map[string]any{
			"page_id": int64(99),
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
		expectErr string
	}{
		{
			name:     "single replacement",
			input:    `{"parent_id": {{action:a1.page_id}}}`,
			expected: `{"parent_id": 42}`,
		},
		{
			name:     "multiple replacements",
			input:    `{"source_page_id": {{action:a1.page_id}}, "target_page_id": {{action:a2.page_id}}}`,
			expected: `{"source_page_id": 42, "target_page_id": 99}`,
		},
		{
			name:     "string field replacement",
			input:    `{"slug": "{{action:a1.slug}}"}`,
			expected: `{"slug": "my-page"}`,
		},
		{
			name:     "no placeholders",
			input:    `{"title": "hello"}`,
			expected: `{"title": "hello"}`,
		},
		{
			name:     "unresolved placeholder returns error",
			input:    `{"parent_id": {{action:a3.page_id}}}`,
			expectErr: "unresolved placeholders: a3.page_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testReplacePlaceholders(tt.input, results)
			if tt.expectErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectErr)
				}
				if !strings.Contains(err.Error(), tt.expectErr) {
					t.Errorf("expected error to contain %q, got %q", tt.expectErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Additional test: topoSort with model.PlanAction directly
// ---------------------------------------------------------------------------

func TestTopoSort_ModelPlanAction_NoDeps(t *testing.T) {
	actions := []model.PlanAction{
		{ID: "a1", DependsOn: json.RawMessage("[]")},
		{ID: "a2", DependsOn: json.RawMessage("[]")},
		{ID: "a3", DependsOn: json.RawMessage("[]")},
	}

	sorted, err := topoSort(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("expected 3, got %d", len(sorted))
	}

	seen := map[string]bool{}
	for _, a := range sorted {
		seen[a.ID] = true
	}
	for _, id := range []string{"a1", "a2", "a3"} {
		if !seen[id] {
			t.Errorf("expected action %s in output", id)
		}
	}
}

func TestTopoSort_ModelPlanAction_Diamond(t *testing.T) {
	actions := []model.PlanAction{
		{ID: "a1", DependsOn: json.RawMessage("[]")},
		{ID: "a2", DependsOn: json.RawMessage(`["a1"]`)},
		{ID: "a3", DependsOn: json.RawMessage(`["a1"]`)},
		{ID: "a4", DependsOn: json.RawMessage(`["a2","a3"]`)},
	}

	sorted, err := topoSort(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pos := make(map[string]int)
	for i, a := range sorted {
		pos[a.ID] = i
	}

	if pos["a1"] >= pos["a2"] {
		t.Error("a1 should come before a2")
	}
	if pos["a1"] >= pos["a3"] {
		t.Error("a1 should come before a3")
	}
	if pos["a2"] >= pos["a4"] {
		t.Error("a2 should come before a4")
	}
	if pos["a3"] >= pos["a4"] {
		t.Error("a3 should come before a4")
	}
}

func TestTopoSort_ModelPlanAction_Circular(t *testing.T) {
	actions := []model.PlanAction{
		{ID: "a1", DependsOn: json.RawMessage(`["a3"]`)},
		{ID: "a2", DependsOn: json.RawMessage(`["a1"]`)},
		{ID: "a3", DependsOn: json.RawMessage(`["a2"]`)},
	}

	_, err := topoSort(actions)
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error should mention circular dependency, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Slugify tests
// ---------------------------------------------------------------------------

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Go 后端开发", "go-后端开发"},
		{"React & Vue", "react-vue"},
		{"  Multiple   Spaces  ", "multiple-spaces"},
		{"Test--Double", "test-double"},
		{"UPPERCASE", "uppercase"},
		{"数据库索引", "数据库索引"},
		{"C++ Programming", "c-programming"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.expected {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// toInt64 tests
// ---------------------------------------------------------------------------

func TestToInt64(t *testing.T) {
	tests := []struct {
		input    any
		expected int64
		ok       bool
	}{
		{float64(42), 42, true},
		{int64(99), 99, true},
		{int(7), 7, true},
		{json.Number("123"), 123, true},
		{"not a number", 0, false},
		{nil, 0, false},
	}

	for _, tt := range tests {
		got, ok := toInt64(tt.input)
		if ok != tt.ok {
			t.Errorf("toInt64(%v) ok = %v, want %v", tt.input, ok, tt.ok)
		}
		if ok && got != tt.expected {
			t.Errorf("toInt64(%v) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests for fix-plan-parent-id-resolution (tasks 1.2-1.12)
// ---------------------------------------------------------------------------

// isPlaceholderString is defined in engine.go; tests use it directly.

func TestHasParentID_RejectsPlaceholder(t *testing.T) {
	params := json.RawMessage(`{"parent_id": "{{action:a1.page_id}}"}`)
	if hasParentID(params) {
		t.Error("hasParentID should return false for placeholder string value")
	}
}

func TestHasParentID_AcceptsInteger(t *testing.T) {
	params := json.RawMessage(`{"parent_id": 42}`)
	if !hasParentID(params) {
		t.Error("hasParentID should return true for integer parent_id")
	}
}

func TestHasParentID_AcceptsMissing(t *testing.T) {
	params := json.RawMessage(`{"title": "hello"}`)
	if hasParentID(params) {
		t.Error("hasParentID should return false when parent_id absent")
	}
}

func TestInferDependencies(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single placeholder",
			input:    `{"parent_id": "{{action:a1.page_id}}"}`,
			expected: []string{"a1"},
		},
		{
			name:     "multiple placeholders",
			input:    `{"parent_id": "{{action:a1.page_id}}", "title": "x {{action:a2.slug}}"}`,
			expected: []string{"a1", "a2"},
		},
		{
			name:     "no placeholders",
			input:    `{"title": "x"}`,
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferDependencies(tt.input)
			if !stringSlicesEqualUnordered(got, tt.expected) {
				t.Errorf("inferDependencies(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func stringSlicesEqualUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
		if m[s] < 0 {
			return false
		}
	}
	return true
}

func TestInferPlanStage_MainPage(t *testing.T) {
	p := &planProposalLite{
		Actions: []planActionLite{
			{Type: "create_page"},
		},
	}
	stage, err := inferPlanStage(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stage != StageMain {
		t.Errorf("expected stage=main, got %q", stage)
	}
}

func TestInferPlanStage_Outline(t *testing.T) {
	p := &planProposalLite{
		Outline: json.RawMessage(`[{"title": "X", "page_type": "concept"}]`),
	}
	stage, err := inferPlanStage(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stage != StageOutline {
		t.Errorf("expected stage=outline, got %q", stage)
	}
}

func TestInferPlanStage_Content(t *testing.T) {
	p := &planProposalLite{
		Actions: []planActionLite{
			{Type: "update_page"},
			{Type: "patch_page"},
		},
	}
	stage, err := inferPlanStage(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stage != StageContent {
		t.Errorf("expected stage=content, got %q", stage)
	}
}

func TestRejectPlan_MainStageTooManyCreate(t *testing.T) {
	p := &planProposalLite{
		Actions: []planActionLite{
			{Type: "create_page"},
			{Type: "create_page"},
		},
	}
	_, err := inferPlanStage(p)
	if err == nil {
		t.Fatal("expected error for main stage with 2 create_pages")
	}
}

func TestRejectPlan_OutlineTooManyNodes(t *testing.T) {
	// Build outline with 50 leaves
	children := []map[string]any{}
	for i := 0; i < 50; i++ {
		children = append(children, map[string]any{"title": fmt.Sprintf("n%d", i), "page_type": "concept"})
	}
	outline, _ := json.Marshal([]map[string]any{
		{"title": "root", "children": children},
	})
	p := &planProposalLite{Outline: outline}
	_, err := inferPlanStage(p)
	if err == nil {
		t.Fatal("expected error for outline with 50 nodes")
	}
}

func TestRejectPlan_ContentStageHasCreate(t *testing.T) {
	p := &planProposalLite{
		Actions: []planActionLite{
			{Type: "update_page"},
			{Type: "create_page", HasContent: true},
		},
	}
	_, err := inferPlanStage(p)
	if err == nil {
		t.Fatal("expected error for content stage with create_page")
	}
}

func TestRejectPlan_OutlineAndActionsMixed(t *testing.T) {
	p := &planProposalLite{
		Outline: json.RawMessage(`[{"title": "x"}]`),
		Actions: []planActionLite{
			{Type: "update_page"},
		},
	}
	_, err := inferPlanStage(p)
	if err == nil {
		t.Fatal("expected error for outline + actions both non-empty")
	}
}

func TestFocusFallback_PlaceholderAndFocus(t *testing.T) {
	// Simulate the engine's focus fallback: with placeholder parent_id and a focusID,
	// hasParentID should return false so the fallback kicks in.
	params := json.RawMessage(`{"parent_id": "{{action:a1.page_id}}"}`)
	if hasParentID(params) {
		t.Error("placeholder parent_id should be treated as missing so focusPageID fallback fires")
	}
}
