package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"learn-helper/internal/ai/skills"
)

func TestSkillsHandler_ListSorted(t *testing.T) {
	r := skills.NewRegistry()
	_ = r.Add(&skills.Skill{Name: "zebra", Description: "Z"})
	_ = r.Add(&skills.Skill{Name: "alpha", Description: "A"})

	h := &SkillsHandler{Registry: r}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/skills", h.HandleList)

	req := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var got []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "zebra" {
		t.Errorf("unexpected order/content: %+v", got)
	}
	// Body / Extra / SourcePath must NOT be exposed.
	raw := w.Body.String()
	for _, leaked := range []string{"body", "Body", "Extra", "SourcePath"} {
		if containsAny(raw, leaked) {
			t.Errorf("response leaked field %q: %s", leaked, raw)
		}
	}
}

func containsAny(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
