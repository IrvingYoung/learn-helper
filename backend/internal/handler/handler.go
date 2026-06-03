package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"learn-helper/internal/model"
)

type Handler struct {
	db      *sql.DB
	queries *model.Queries
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{
		db:      db,
		queries: model.New(db),
	}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// writeJSON sets the Content-Type header, writes the status, and encodes v to the response writer.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}