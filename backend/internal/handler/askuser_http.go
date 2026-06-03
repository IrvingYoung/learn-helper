package handler

import (
	"encoding/json"
	"net/http"
)

func (h *AIHandler) HandleAskUserResponse(w http.ResponseWriter, r *http.Request) {
	var req AskUserResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.RequestID == "" {
		http.Error(w, "missing request_id", http.StatusBadRequest)
		return
	}
	if h.askUsers != nil {
		h.askUsers.Resolve(req.RequestID, req)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
