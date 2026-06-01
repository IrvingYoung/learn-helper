package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAskUserAnswerJSON(t *testing.T) {
	cases := []struct {
		name string
		in   AskUserAnswer
		want string
	}{
		{
			name: "single option",
			in:   AskUserAnswer{Answer: "底层原理"},
			want: `{"answer":"底层原理"}`,
		},
		{
			name: "multi select",
			in:   AskUserAnswer{Answer: []any{"opt A", "opt C"}},
			want: `{"answer":["opt A","opt C"]}`,
		},
		{
			name: "no answer",
			in:   AskUserAnswer{Answer: "no_answer"},
			want: `{"answer":"no_answer"}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(c.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(b) != c.want {
				t.Errorf("got %s, want %s", b, c.want)
			}
		})
	}
}

func TestAskUserRequestJSON(t *testing.T) {
	r := AskUserRequest{
		RequestID:      "ask_1",
		ConversationID: 42,
		Question:       "Pick a topic?",
		Options:        []string{"底层原理", "应用技巧", "对比分析"},
		Context: &AskUserContext{
			Kind: "outline",
			Data: map[string]any{
				"root": "Go",
				"children": []any{
					map[string]any{"id": "1", "title": "goroutine"},
					map[string]any{"id": "2", "title": "channel"},
				},
			},
		},
		MultiSelect:   true,
		AllowFreeText: false,
		Header:        "Topic",
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["request_id"] != "ask_1" {
		t.Errorf("request_id = %v, want ask_1", got["request_id"])
	}
	if got["conversation_id"] != float64(42) {
		t.Errorf("conversation_id = %v, want 42", got["conversation_id"])
	}
	if got["question"] != "Pick a topic?" {
		t.Errorf("question = %v, want Pick a topic?", got["question"])
	}
	if got["header"] != "Topic" {
		t.Errorf("header = %v, want Topic", got["header"])
	}
	if got["multi_select"] != true {
		t.Errorf("multi_select = %v, want true", got["multi_select"])
	}
	if got["allow_free_text"] != false {
		t.Errorf("allow_free_text = %v, want false", got["allow_free_text"])
	}
	options, ok := got["options"].([]any)
	if !ok {
		t.Fatalf("options not array: %T", got["options"])
	}
	if len(options) != 3 {
		t.Fatalf("len(options) = %d, want 3", len(options))
	}
	if options[0] != "底层原理" || options[1] != "应用技巧" || options[2] != "对比分析" {
		t.Errorf("options = %v, want [底层原理 应用技巧 对比分析]", options)
	}
	ctx, ok := got["context"].(map[string]any)
	if !ok {
		t.Fatalf("context not object: %T", got["context"])
	}
	if ctx["kind"] != "outline" {
		t.Errorf("context.kind = %v, want outline", ctx["kind"])
	}
	data, ok := ctx["data"].(map[string]any)
	if !ok {
		t.Fatalf("context.data not object: %T", ctx["data"])
	}
	if data["root"] != "Go" {
		t.Errorf("context.data.root = %v, want Go", data["root"])
	}
}

func TestAskUserContextJSON(t *testing.T) {
	cases := []struct {
		name     string
		in       AskUserContext
		wantKind string
		check    func(t *testing.T, data map[string]any)
	}{
		{
			name: "page kind",
			in: AskUserContext{
				Kind: "page",
				Data: map[string]any{"page_id": float64(5)},
			},
			wantKind: "page",
			check: func(t *testing.T, data map[string]any) {
				if data["page_id"] != float64(5) {
					t.Errorf("data.page_id = %v, want 5", data["page_id"])
				}
			},
		},
		{
			name: "outline kind",
			in: AskUserContext{
				Kind: "outline",
				Data: map[string]any{
					"root": "Go",
					"children": []any{
						map[string]any{"id": "1", "title": "goroutine"},
					},
				},
			},
			wantKind: "outline",
			check: func(t *testing.T, data map[string]any) {
				if data["root"] != "Go" {
					t.Errorf("data.root = %v, want Go", data["root"])
				}
				children, ok := data["children"].([]any)
				if !ok {
					t.Fatalf("data.children not array: %T", data["children"])
				}
				if len(children) != 1 {
					t.Fatalf("len(children) = %d, want 1", len(children))
				}
			},
		},
		{
			name: "markdown kind",
			in: AskUserContext{
				Kind: "markdown",
				Data: map[string]any{"body": "# Hello\nWorld"},
			},
			wantKind: "markdown",
			check: func(t *testing.T, data map[string]any) {
				if data["body"] != "# Hello\nWorld" {
					t.Errorf("data.body = %v, want # Hello\\nWorld", data["body"])
				}
			},
		},
		{
			name: "diff kind",
			in: AskUserContext{
				Kind: "diff",
				Data: map[string]any{"patch": "@@ -1 +1 @@\n-old\n+new"},
			},
			wantKind: "diff",
			check: func(t *testing.T, data map[string]any) {
				if data["patch"] != "@@ -1 +1 @@\n-old\n+new" {
					t.Errorf("data.patch = %v", data["patch"])
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(c.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got["kind"] != c.wantKind {
				t.Errorf("kind = %v, want %s", got["kind"], c.wantKind)
			}
			data, ok := got["data"].(map[string]any)
			if !ok {
				t.Fatalf("data not object: %T", got["data"])
			}
			c.check(t, data)
		})
	}
}

func TestAskUserResponseJSON(t *testing.T) {
	cases := []struct {
		name    string
		in      AskUserResponse
		wantAns any
		isArr   bool
	}{
		{
			name:    "string answer",
			in:      AskUserResponse{RequestID: "ask_1", Answer: "底层原理"},
			wantAns: "底层原理",
			isArr:   false,
		},
		{
			name:    "[]string answer",
			in:      AskUserResponse{RequestID: "ask_2", Answer: []string{"opt A", "opt C"}},
			wantAns: []any{"opt A", "opt C"},
			isArr:   true,
		},
		{
			name:    "no_answer",
			in:      AskUserResponse{RequestID: "ask_3", Answer: "no_answer"},
			wantAns: "no_answer",
			isArr:   false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(c.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got["request_id"] != c.in.RequestID {
				t.Errorf("request_id = %v, want %s", got["request_id"], c.in.RequestID)
			}
			if c.isArr {
				arr, ok := got["answer"].([]any)
				if !ok {
					t.Fatalf("answer not array: %T", got["answer"])
				}
				wantArr, ok := c.wantAns.([]any)
				if !ok || len(arr) != len(wantArr) {
					t.Fatalf("answer length mismatch: got %v, want %v", arr, wantArr)
				}
				for i, v := range arr {
					if v != wantArr[i] {
						t.Errorf("answer[%d] = %v, want %v", i, v, wantArr[i])
					}
				}
			} else {
				if got["answer"] != c.wantAns {
					t.Errorf("answer = %v, want %v", got["answer"], c.wantAns)
				}
			}
		})
	}
}

func TestAskUserRequestJSON_OmitsNilContextAndEmptyHeader(t *testing.T) {
	r := AskUserRequest{
		RequestID:      "ask_omitzero",
		ConversationID: 1,
		Question:       "Continue?",
		Options:        []string{"yes", "no"},
		// Context and Header intentionally zero-valued to exercise omitempty.
		MultiSelect:   false,
		AllowFreeText: true,
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), `"context"`) {
		t.Errorf("expected context to be omitted, got %s", string(b))
	}
	if strings.Contains(string(b), `"header"`) {
		t.Errorf("expected header to be omitted, got %s", string(b))
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := got["context"]; present {
		t.Errorf("expected context key to be absent, got %v", got)
	}
	if _, present := got["header"]; present {
		t.Errorf("expected header key to be absent, got %v", got)
	}
}
