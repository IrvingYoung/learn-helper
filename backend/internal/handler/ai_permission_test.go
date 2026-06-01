package handler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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

func TestPermissionDecisionJSON_OmitsNilEditedInput(t *testing.T) {
	d := PermissionDecision{
		ID:     "toolu_xyz",
		Action: "approve",
		// EditedInput intentionally left nil to exercise omitempty.
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), `"edited_input"`) {
		t.Errorf("expected edited_input to be omitted, got %s", string(b))
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := got["edited_input"]; present {
		t.Errorf("expected edited_input key to be absent, got %v", got)
	}
	if got["id"] != "toolu_xyz" {
		t.Errorf("id = %v, want toolu_xyz", got["id"])
	}
	if got["action"] != "approve" {
		t.Errorf("action = %v, want approve", got["action"])
	}
}

func TestPermissionRequestJSON(t *testing.T) {
	r := PermissionRequest{
		RequestID:      "req_1",
		ConversationID: 42,
		Items: []PermissionRequestItem{
			{
				ID:   "toolu_1",
				Tool: "create_page",
				Input: map[string]any{
					"title": "Hello",
				},
				Preview: "# Hello",
			},
		},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["request_id"] != "req_1" {
		t.Errorf("request_id = %v, want req_1", got["request_id"])
	}
	// JSON numbers decode to float64 by default.
	if got["conversation_id"] != float64(42) {
		t.Errorf("conversation_id = %v, want 42", got["conversation_id"])
	}
	items, ok := got["items"].([]any)
	if !ok {
		t.Fatalf("items not array: %T", got["items"])
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("items[0] not object: %T", items[0])
	}
	if item["id"] != "toolu_1" {
		t.Errorf("items[0].id = %v, want toolu_1", item["id"])
	}
	if item["tool"] != "create_page" {
		t.Errorf("items[0].tool = %v, want create_page", item["tool"])
	}
	input, ok := item["input"].(map[string]any)
	if !ok {
		t.Fatalf("items[0].input not object: %T", item["input"])
	}
	if input["title"] != "Hello" {
		t.Errorf("items[0].input.title = %v, want Hello", input["title"])
	}
	if item["preview"] != "# Hello" {
		t.Errorf("items[0].preview = %v, want # Hello", item["preview"])
	}
}

func TestPermissionRequestItemJSON(t *testing.T) {
	item := PermissionRequestItem{
		ID:   "toolu_item",
		Tool: "patch_page",
		Input: map[string]any{
			"page_id": float64(7),
			"body":    "updated body",
		},
		Preview: "patched body",
	}
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["id"] != "toolu_item" {
		t.Errorf("id = %v, want toolu_item", got["id"])
	}
	if got["tool"] != "patch_page" {
		t.Errorf("tool = %v, want patch_page", got["tool"])
	}
	input, ok := got["input"].(map[string]any)
	if !ok {
		t.Fatalf("input not object: %T", got["input"])
	}
	if input["page_id"] != float64(7) {
		t.Errorf("input.page_id = %v, want 7", input["page_id"])
	}
	if input["body"] != "updated body" {
		t.Errorf("input.body = %v, want updated body", input["body"])
	}
	if got["preview"] != "patched body" {
		t.Errorf("preview = %v, want patched body", got["preview"])
	}
}

func TestPermissionResponseJSON(t *testing.T) {
	resp := PermissionResponse{
		RequestID: "req_42",
		Decisions: []PermissionDecision{
			{
				ID:     "toolu_a",
				Action: "approve",
			},
			{
				ID:     "toolu_b",
				Action: "edit",
				EditedInput: map[string]any{
					"title": "Renamed",
				},
			},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["request_id"] != "req_42" {
		t.Errorf("request_id = %v, want req_42", got["request_id"])
	}
	decisions, ok := got["decisions"].([]any)
	if !ok {
		t.Fatalf("decisions not array: %T", got["decisions"])
	}
	if len(decisions) != 2 {
		t.Fatalf("len(decisions) = %d, want 2", len(decisions))
	}
	first, ok := decisions[0].(map[string]any)
	if !ok {
		t.Fatalf("decisions[0] not object: %T", decisions[0])
	}
	if first["id"] != "toolu_a" {
		t.Errorf("decisions[0].id = %v, want toolu_a", first["id"])
	}
	if first["action"] != "approve" {
		t.Errorf("decisions[0].action = %v, want approve", first["action"])
	}
	second, ok := decisions[1].(map[string]any)
	if !ok {
		t.Fatalf("decisions[1] not object: %T", decisions[1])
	}
	if second["id"] != "toolu_b" {
		t.Errorf("decisions[1].id = %v, want toolu_b", second["id"])
	}
	if second["action"] != "edit" {
		t.Errorf("decisions[1].action = %v, want edit", second["action"])
	}
	edited, ok := second["edited_input"].(map[string]any)
	if !ok {
		t.Fatalf("decisions[1].edited_input not object: %T", second["edited_input"])
	}
	if edited["title"] != "Renamed" {
		t.Errorf("decisions[1].edited_input.title = %v, want Renamed", edited["title"])
	}
}

func TestPermissionRegistry(t *testing.T) {
	r := NewPermissionRegistry()

	ch := r.Register("perm-1", 5)
	if ch == nil {
		t.Fatal("Register returned nil chan")
	}
	if r.Pending() != 1 {
		t.Errorf("Pending = %d, want 1", r.Pending())
	}

	r.Resolve("perm-1", []PermissionDecision{{ID: "x", Action: "approve"}})
	select {
	case got := <-ch:
		if len(got) != 1 || got[0].ID != "x" {
			t.Errorf("got %+v, want one decision for x", got)
		}
	case <-time.After(time.Second):
		t.Fatal("resolve did not unblock Register")
	}

	if r.Pending() != 0 {
		t.Errorf("after resolve, Pending = %d, want 0", r.Pending())
	}

	// Resolve unknown id is a no-op (no panic)
	r.Resolve("perm-unknown", nil)
}

func TestPermissionRegistry_CancelAll(t *testing.T) {
	r := NewPermissionRegistry()
	chA := r.Register("perm-A", 5)
	chB := r.Register("perm-B", 5)
	if r.Pending() != 2 {
		t.Fatalf("Pending = %d, want 2", r.Pending())
	}

	r.CancelAll()
	if r.Pending() != 0 {
		t.Errorf("after CancelAll, Pending = %d, want 0", r.Pending())
	}

	// Both channels should be closed (receive returns zero value with ok=false)
	for i, ch := range []chan []PermissionDecision{chA, chB} {
		select {
		case got, ok := <-ch:
			if ok {
				t.Errorf("ch[%d] received a value after CancelAll: %v", i, got)
			}
		case <-time.After(time.Second):
			t.Errorf("ch[%d] did not return after CancelAll", i)
		}
	}

	// Idempotent: second CancelAll is a no-op
	r.CancelAll()
}
