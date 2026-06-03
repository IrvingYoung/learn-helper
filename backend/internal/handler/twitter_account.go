package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// TwitterAccountHandler exposes CRUD for tracked_twitter_accounts and
// the RSSHub Base URL config (stored in ai_configs.config JSON).
type TwitterAccountHandler struct {
	db *sql.DB
}

func NewTwitterAccountHandler(db *sql.DB) *TwitterAccountHandler {
	return &TwitterAccountHandler{db: db}
}

// twitterAccountHandlerDB is a test-only escape hatch to inject a DB
// after construction. Production code should always set h.db via
// NewTwitterAccountHandler. Used by handlers that need a *sql.DB even
// when constructed with nil (e.g. BulkImport in unit tests).
var twitterAccountHandlerDB *sql.DB

var handleRE = regexp.MustCompile(`^[A-Za-z0-9_]{1,15}$`)

type accountJSON struct {
	ID          int64  `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name,omitempty"`
	Enabled     bool   `json:"enabled"`
	Notes       string `json:"notes,omitempty"`
}

func (h *TwitterAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, handle, COALESCE(display_name, ''), enabled, COALESCE(notes, '')
		FROM tracked_twitter_accounts
		ORDER BY id
	`)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []accountJSON{}
	for rows.Next() {
		var a accountJSON
		var enabled int
		if err := rows.Scan(&a.ID, &a.Handle, &a.DisplayName, &enabled, &a.Notes); err != nil {
			writeJSONError(w, 500, err.Error())
			return
		}
		a.Enabled = enabled != 0
		out = append(out, a)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *TwitterAccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Handle string `json:"handle"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !handleRE.MatchString(in.Handle) {
		writeJSONError(w, http.StatusBadRequest, "handle must match ^[A-Za-z0-9_]{1,15}$ (no @)")
		return
	}
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO tracked_twitter_accounts (handle, notes) VALUES (?, ?)`,
		in.Handle, in.Notes,
	)
	if err != nil {
		if isUniqueViolation(err) {
			writeJSONError(w, http.StatusConflict, "handle already exists")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, accountJSON{ID: id, Handle: in.Handle, Enabled: true, Notes: in.Notes})
}

func (h *TwitterAccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad id")
		return
	}
	var in struct {
		Handle  *string `json:"handle"`
		Enabled *bool   `json:"enabled"`
		Notes   *string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Handle != nil {
		if !handleRE.MatchString(*in.Handle) {
			writeJSONError(w, http.StatusBadRequest, "handle must match ^[A-Za-z0-9_]{1,15}$")
			return
		}
	}
	q := `UPDATE tracked_twitter_accounts SET `
	var sets []string
	var args []any
	if in.Handle != nil {
		sets = append(sets, "handle = ?")
		args = append(args, *in.Handle)
	}
	if in.Enabled != nil {
		sets = append(sets, "enabled = ?")
		v := 0
		if *in.Enabled {
			v = 1
		}
		args = append(args, v)
	}
	if in.Notes != nil {
		sets = append(sets, "notes = ?")
		args = append(args, *in.Notes)
	}
	if len(sets) == 0 {
		writeJSONError(w, http.StatusBadRequest, "no fields to update")
		return
	}
	q += strings.Join(sets, ", ") + " WHERE id = ?"
	args = append(args, id)
	if _, err := h.db.ExecContext(r.Context(), q, args...); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *TwitterAccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad id")
		return
	}
	if _, err := h.db.ExecContext(r.Context(),
		`DELETE FROM tracked_twitter_accounts WHERE id = ?`, id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *TwitterAccountHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	url, err := h.loadRSSHubURL(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"rsshub_base_url": url})
}

func (h *TwitterAccountHandler) PutConfig(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BaseURL string `json:"rsshub_base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.BaseURL == "" {
		writeJSONError(w, http.StatusBadRequest, "rsshub_base_url required")
		return
	}
	if err := h.saveRSSHubURL(r.Context(), in.BaseURL); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"rsshub_base_url": in.BaseURL})
}

// LoadRSSHubURL returns the current RSSHub base URL (read from
// ai_configs.config) or "https://rsshub.app" if unset. Safe to call
// from any context, including the twitter.RSSHubClient URL provider
// closure that runs outside an HTTP request.
func (h *TwitterAccountHandler) LoadRSSHubURL(ctx context.Context) (string, error) {
	return h.loadRSSHubURL(ctx)
}

// loadRSSHubURL reads rsshub_base_url from ai_configs.config JSON.
// Defaults to "https://rsshub.app" if unset.
func (h *TwitterAccountHandler) loadRSSHubURL(ctx context.Context) (string, error) {
	row := h.db.QueryRowContext(ctx, `
		SELECT config FROM ai_configs WHERE is_active = 1 LIMIT 1
	`)
	var cfg sql.NullString
	if err := row.Scan(&cfg); err != nil {
		if err == sql.ErrNoRows {
			return "https://rsshub.app", nil
		}
		return "", err
	}
	if !cfg.Valid || cfg.String == "" {
		return "https://rsshub.app", nil
	}
	var parsed struct {
		BaseURL string `json:"rsshub_base_url"`
	}
	if err := json.Unmarshal([]byte(cfg.String), &parsed); err != nil || parsed.BaseURL == "" {
		return "https://rsshub.app", nil
	}
	return parsed.BaseURL, nil
}

// saveRSSHubURL writes rsshub_base_url into the active ai_configs.config JSON.
// If no active config exists, creates a stub one.
func (h *TwitterAccountHandler) saveRSSHubURL(ctx context.Context, baseURL string) error {
	var existing sql.NullString
	row := h.db.QueryRowContext(ctx, `SELECT config FROM ai_configs WHERE is_active = 1 LIMIT 1`)
	_ = row.Scan(&existing)

	merged := map[string]any{}
	if existing.Valid && existing.String != "" {
		_ = json.Unmarshal([]byte(existing.String), &merged)
	}
	merged["rsshub_base_url"] = baseURL
	body, _ := json.Marshal(merged)

	res, err := h.db.ExecContext(ctx,
		`UPDATE ai_configs SET config = ? WHERE is_active = 1`, string(body),
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		_, err = h.db.ExecContext(ctx,
			`INSERT INTO ai_configs (provider, model_name, api_key, is_active, config) VALUES (?, ?, ?, 1, ?)`,
			"", "", "", string(body),
		)
	}
	return err
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
