package handler

import "testing"

func TestExtractFirstJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple object", `{"a": 1}`, `{"a": 1}`},
		{"nested object", `{"a": {"b": 1}}`, `{"a": {"b": 1}}`},
		{"close brace in string", `{"a": "use } end"}`, `{"a": "use } end"}`},
		{"open brace in string", `{"a": "use { end"}`, `{"a": "use { end"}`},
		{"unbalanced close in value", `{"content": "x } y", "b": 1}`, `{"content": "x } y", "b": 1}`},
		{"prefix wrapper", `f({"a": 1})`, `{"a": 1}`},
		{"no opening brace", `hello`, `hello`},
		{"unclosed object", `{"a": 1`, `{"a": 1`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFirstJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractFirstJSON(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractFirstJSON_CodeBlockInContent(t *testing.T) {
	// Simulates the actual bug: propose_plan JSON whose content field
	// contains a code snippet with curly braces.
	input := `f({"reasoning":"test","actions":[{"id":"a1","type":"update_page","params":{"page_id":107,"content":"# Title\n\n` + "```" + `c\nint main() {\n  return 0;\n}\n` + "```" + `"}}]})`
	want := `{"reasoning":"test","actions":[{"id":"a1","type":"update_page","params":{"page_id":107,"content":"# Title\n\n` + "```" + `c\nint main() {\n  return 0;\n}\n` + "```" + `"}}]}`
	got := extractFirstJSON(input)
	if got != want {
		t.Errorf("extractFirstJSON did not correctly extract JSON with braces in string values\ngot:  %s\nwant: %s", got, want)
	}
}
