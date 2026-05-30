package engine

import (
	"encoding/json"
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
// using a simple map[string]any result store.
func testReplacePlaceholders(input string, results map[string]any) string {
	return placeholderPattern.ReplaceAllStringFunc(input, func(match string) string {
		subs := placeholderPattern.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		actionID := subs[1]
		field := subs[2]

		res, ok := results[actionID]
		if !ok {
			return match
		}
		val := resolveField(res, field)
		if val == nil {
			return match
		}
		return fmtVal(val)
	})
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
	}{
		{
			name:     "single replacement",
			input:    `{"parent_id": {{action:a1.page_id}}}`,
			expected: `{"parent_id": 42}`,
		},
		{
			name:     "multiple replacements",
			input:    `{"source_id": {{action:a1.page_id}}, "target_id": {{action:a2.page_id}}}`,
			expected: `{"source_id": 42, "target_id": 99}`,
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
			name:     "unresolved placeholder left as-is",
			input:    `{"parent_id": {{action:a3.page_id}}}`,
			expected: `{"parent_id": {{action:a3.page_id}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testReplacePlaceholders(tt.input, results)
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
		{ID: "a1", DependsOn: "[]"},
		{ID: "a2", DependsOn: "[]"},
		{ID: "a3", DependsOn: "[]"},
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
		{ID: "a1", DependsOn: "[]"},
		{ID: "a2", DependsOn: `["a1"]`},
		{ID: "a3", DependsOn: `["a1"]`},
		{ID: "a4", DependsOn: `["a2","a3"]`},
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
		{ID: "a1", DependsOn: `["a3"]`},
		{ID: "a2", DependsOn: `["a1"]`},
		{ID: "a3", DependsOn: `["a2"]`},
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
