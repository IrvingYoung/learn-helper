package handler

import (
	"encoding/json"
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
