package handler

import (
	"encoding/json"
	"testing"
)

func TestPermissionDecisionJSON(t *testing.T) {
	d := PermissionDecision{
		ID:     "toolu_abc",
		Action: "edit",
		EditedInput: map[string]any{
			"title": "Edited",
		},
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["id"] != "toolu_abc" {
		t.Errorf("id = %v, want toolu_abc", got["id"])
	}
	if got["action"] != "edit" {
		t.Errorf("action = %v, want edit", got["action"])
	}
	edited, ok := got["edited_input"].(map[string]any)
	if !ok {
		t.Fatalf("edited_input not object: %T", got["edited_input"])
	}
	if edited["title"] != "Edited" {
		t.Errorf("edited_input.title = %v, want Edited", edited["title"])
	}
}
