package handler

import "testing"

func TestClassifyToolCalls(t *testing.T) {
	tcs := []struct {
		name    string
		in      []string // tool names
		wantR   []string
		wantW   []string
		wantAsk []string
	}{
		{
			name:    "all reads",
			in:      []string{"read_page", "search_pages"},
			wantR:   []string{"read_page", "search_pages"},
		},
		{
			name:    "writes batch",
			in:      []string{"create_page", "link_pages"},
			wantW:   []string{"create_page", "link_pages"},
		},
		{
			name:    "ask_user alone",
			in:      []string{"ask_user"},
			wantAsk: []string{"ask_user"},
		},
		{
			name:    "mixed",
			in:      []string{"read_page", "create_page", "ask_user", "update_page"},
			wantR:   []string{"read_page"},
			wantW:   []string{"create_page", "update_page"},
			wantAsk: []string{"ask_user"},
		},
		{
			name:    "unknown tool → writeBatch (treated as write, will fail at execute)",
			in:      []string{"propose_plan"},
			wantW:   []string{"propose_plan"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var calls []aiToolCall
			for _, n := range tc.in {
				calls = append(calls, aiToolCall{Name: n})
			}
			r, w, a := classifyToolCalls(calls)
			gotR := names(r)
			gotW := names(w)
			gotA := names(a)
			if !equal(gotR, tc.wantR) {
				t.Errorf("reads: got %v want %v", gotR, tc.wantR)
			}
			if !equal(gotW, tc.wantW) {
				t.Errorf("writes: got %v want %v", gotW, tc.wantW)
			}
			if !equal(gotA, tc.wantAsk) {
				t.Errorf("asks: got %v want %v", gotA, tc.wantAsk)
			}
		})
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func names(cs []aiToolCall) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}
