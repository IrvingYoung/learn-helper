package handler

import (
	"encoding/json"
	"net/http"
)

func (h *AIHandler) HandlePermissionResponse(w http.ResponseWriter, r *http.Request) {
	var req PermissionResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.RequestID == "" {
		http.Error(w, "missing request_id", http.StatusBadRequest)
		return
	}
	if h.permissions != nil {
		h.permissions.Resolve(req.RequestID, req.Decisions)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
