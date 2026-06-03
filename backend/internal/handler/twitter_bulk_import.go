package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"learn-helper/internal/twitter"
)

// BuiltInBulkImportURL is the default source for "import all" when the
// request body has no URL. Points to the maintained follow-builders
// "x_accounts" list (raw GitHub).
const BuiltInBulkImportURL = "https://raw.githubusercontent.com/zarazhangrui/follow-builders/main/config/default-sources.json"

// bulkImportRequest is the request body for BulkImport.
type bulkImportRequest struct {
	URL string `json:"url"`
}

// bulkImportResponse is the response body.
type bulkImportResponse struct {
	Source          string   `json:"source"`
	TotalFound      int      `json:"total_found"`
	Added           int      `json:"added"`
	SkippedExisting int      `json:"skipped_existing"`
	AddedHandles    []string `json:"added_handles,omitempty"`
	Error           string   `json:"error,omitempty"`
}

// BulkImport fetches a remote handle list (JSON or plain text) and inserts
// each handle into tracked_twitter_accounts idempotently. Supports:
//   - JSON with `x_accounts`/`handles`/`accounts` key (array of {handle, name} or strings)
//   - JSON array of strings
//   - Plain text, one handle per line (lines starting with # are comments)
//
// Request body: {"url": "https://..."} (optional; defaults to BuiltInBulkImportURL)
func (h *TwitterAccountHandler) BulkImport(w http.ResponseWriter, r *http.Request) {
	db := h.db
	if db == nil {
		db = twitterAccountHandlerDB
	}
	if db == nil {
		writeJSONError(w, 500, "db not configured")
		return
	}

	var req bulkImportRequest
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	source := req.URL
	if source == "" {
		source = BuiltInBulkImportURL
	}
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		writeJSONError(w, 400, "url must start with http:// or https://")
		return
	}

	handles, err := fetchAndParseHandleList(r.Context(), source)
	if err != nil {
		writeJSON(w, 500, bulkImportResponse{Source: source, Error: err.Error()})
		return
	}

	store := twitter.NewStore(db)
	added, err := store.BulkInsertAccounts(r.Context(), handles)
	if err != nil {
		writeJSON(w, 500, bulkImportResponse{Source: source, TotalFound: len(handles), Error: err.Error()})
		return
	}

	resp := bulkImportResponse{
		Source:          source,
		TotalFound:      len(handles),
		Added:           added,
		SkippedExisting: len(handles) - added,
	}
	// Build added_handles list (best-effort, for the response; limited to
	// the first 100 to keep the payload small).
	if added > 0 && len(handles) <= 100 {
		inputSet := make(map[string]bool, len(handles))
		for _, x := range handles {
			inputSet[strings.TrimPrefix(strings.TrimSpace(x), "@")] = true
		}
		rows, qerr := db.QueryContext(r.Context(),
			`SELECT handle FROM tracked_twitter_accounts WHERE handle IN (`+placeholders(len(handles))+`)`,
			handlesToArgs(handles)...)
		if qerr == nil {
			defer rows.Close()
			for rows.Next() {
				var hh string
				if err := rows.Scan(&hh); err == nil {
					if inputSet[hh] {
						resp.AddedHandles = append(resp.AddedHandles, hh)
					}
				}
			}
		}
	}
	writeJSON(w, 200, resp)
}

// fetchAndParseHandleList fetches a URL and returns a deduped list of
// handles. Tries JSON first, falls back to text.
func fetchAndParseHandleList(ctx context.Context, source string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "learn-helper/1.0")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("upstream returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5MB cap
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parseHandleList(body), nil
}

// parseHandleList detects JSON vs text and extracts handles.
func parseHandleList(body []byte) []string {
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) == 0 {
		return nil
	}
	// Try JSON first
	if trimmed[0] == '{' || trimmed[0] == '[' {
		if handles := parseHandleListJSON(trimmed); len(handles) > 0 {
			return dedupe(handles)
		}
		// JSON but no recognizable shape — fall through to text parsing
	}
	return dedupe(parseHandleListText(string(body)))
}

// parseHandleListJSON tries common shapes:
//   - {"x_accounts": [...]}, {"handles": [...]}, {"accounts": [...]}
//   - [{...}, ...] (array of {handle})
//   - ["a", "b"] (array of strings)
func parseHandleListJSON(s string) []string {
	var top any
	if err := json.Unmarshal([]byte(s), &top); err != nil {
		return nil
	}
	var out []string
	switch v := top.(type) {
	case map[string]any:
		for _, key := range []string{"x_accounts", "handles", "accounts", "users"} {
			if arr, ok := v[key]; ok {
				out = append(out, extractHandlesFromArray(arr)...)
			}
		}
	case []any:
		out = append(out, extractHandlesFromArray(v)...)
	}
	return out
}

func extractHandlesFromArray(arr any) []string {
	list, ok := arr.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range list {
		switch v := item.(type) {
		case string:
			out = append(out, v)
		case map[string]any:
			if h, ok := v["handle"].(string); ok {
				out = append(out, h)
			} else if h, ok := v["username"].(string); ok {
				out = append(out, h)
			}
		}
	}
	return out
}

// parseHandleListText extracts handles from one-per-line text. Lines
// starting with # are comments. Blank lines ignored. Each line is
// trimmed; "handle" or "handle,Display Name" both accepted.
func parseHandleListText(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip optional ",Display Name" suffix
		if idx := strings.Index(line, ","); idx > 0 {
			line = strings.TrimSpace(line[:idx])
		}
		out = append(out, line)
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, h := range in {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	return out
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func handlesToArgs(h []string) []any {
	out := make([]any, len(h))
	for i, v := range h {
		out[i] = v
	}
	return out
}
