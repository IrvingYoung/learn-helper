package handler

import (
	"encoding/json"
	"net/http"

	"learn-helper/internal/ai/skills"
)

// SkillsHandler exposes the catalog of available skills to the frontend.
type SkillsHandler struct {
	Registry *skills.Registry
}

type skillListItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// HandleList responds with the sorted list of skills (name + description only).
// Body, Extra, and SourcePath are intentionally NOT exposed.
func (h *SkillsHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	all := h.Registry.List()
	out := make([]skillListItem, 0, len(all))
	for _, s := range all {
		out = append(out, skillListItem{
			Name:        s.Name,
			Description: s.Description,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
