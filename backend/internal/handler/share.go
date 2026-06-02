package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

// --- Public content API ---

// GetPublicSharePage serves the public read-only API for a wiki page.
// Validates the supplied share_token from ?t= and returns page data WITHOUT
// the share_token itself (so a public visitor cannot extract the token from
// the API response and continue to access the page after the URL is rotated).
//
//	GET /api/share/{slug}?t={token}
//
// 404 on: missing slug, missing/empty/wrong token, page not found.
func (h *WikiHandler) GetPublicSharePage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	token := r.URL.Query().Get("t")
	if !validateShareAccess(h.db, slug, token) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var (
		id            int64
		title         string
		content       string
		contentStatus string
		summary       string
		updatedAt     string
	)
	// Use a single SELECT that filters on share_token so the query plan stays
	// simple and we don't leak whether a slug exists on token miss.
	err := h.db.QueryRow(`
		SELECT id, title, content, content_status, summary, updated_at
		FROM wiki_pages
		WHERE slug = ? AND share_token != ''
	`, slug).Scan(&id, &title, &content, &contentStatus, &summary, &updatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("public share: query failed for slug=%q: %v", slug, err)
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":             id,
		"title":          title,
		"slug":           slug,
		"content":        content,
		"content_status": contentStatus,
		"summary":        summary,
		"updated_at":     updatedAt,
		// share_token intentionally NOT included.
	})
}

// --- SSR share page (og: meta injection) ---

// GetShareSSRPage returns the SPA's index.html with og:* meta tags injected
// into <head>, so IM crawlers (WeChat, Xiaohongshu, Twitter) that don't run
// JavaScript can still render a link preview card with title/description/image.
//
//	GET /share/{slug}?t={token}
func (h *WikiHandler) GetShareSSRPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	token := r.URL.Query().Get("t")

	// Load page fields we need for og: meta.
	var title, summary, content, shareToken string
	err := h.db.QueryRow(`
		SELECT title, summary, content, share_token
		FROM wiki_pages
		WHERE slug = ?
	`, slug).Scan(&title, &summary, &content, &shareToken)
	if err == sql.ErrNoRows {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("ssr share: query failed for slug=%q: %v", slug, err)
		http.Error(w, "Failed", http.StatusInternalServerError)
		return
	}

	// Token check (same rule as the public API).
	if shareToken == "" || shareToken != token {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	indexHTML, err := getDistIndexHTML()
	if err != nil {
		log.Printf("ssr share: cannot load index.html: %v", err)
		// Surface a useful error: in dev with no dist/ built, the page
		// won't be servable. Caller should `pnpm build` first.
		http.Error(w, "Server misconfigured: SPA bundle not found", http.StatusInternalServerError)
		return
	}

	shareURL := buildAbsoluteShareURL(r, slug, token)
	meta := buildOgMeta(title, summary, content, shareURL)

	// Inject meta right before </head>. This is brittle if Vite ever changes
	// the closing tag, but it's the simplest and most portable approach for v1.
	// If Vite upgrades break this, switch to a `<!--OG-META-->` placeholder
	// (see design.md D9 for the upgrade path).
	out := strings.Replace(string(indexHTML), "</head>", meta+"\n  </head>", 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(out))
}

// validateShareAccess returns true iff a row exists for slug AND its
// share_token exactly matches the supplied token. Returns false on any
// miss (no row, empty token, wrong token) — uniformly, so we don't reveal
// whether a slug exists.
func validateShareAccess(db *sql.DB, slug, token string) bool {
	if slug == "" || token == "" {
		return false
	}
	var stored string
	err := db.QueryRow(`SELECT share_token FROM wiki_pages WHERE slug = ?`, slug).Scan(&stored)
	if err != nil {
		return false
	}
	return stored != "" && stored == token
}

// --- og: meta helpers ---

// buildAbsoluteShareURL reconstructs the public share URL using the request's
// scheme + host. We trust the reverse proxy's Host header in production;
// in dev this gives `http://localhost:3000` via the Vite proxy.
func buildAbsoluteShareURL(r *http.Request, slug, token string) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	q := url.Values{"t": {token}}
	return scheme + "://" + host + "/share/" + slug + "?" + q.Encode()
}

// buildOgMeta returns the HTML string of <title> + 5 <meta property="og:*">
// tags, ready to inject before </head>. og:description prefers the page's
// summary field, falling back to the first non-empty paragraph in content.
func buildOgMeta(title, summary, content, shareURL string) string {
	description := summary
	if description == "" {
		description = extractFirstParagraph(content)
	}
	description = truncateRunes(description, 200)
	titleEsc := htmlEscape(title)
	descEsc := htmlEscape(description)
	urlEsc := htmlEscape(shareURL)
	return "  <title>" + titleEsc + "</title>\n" +
		`  <meta property="og:title" content="` + titleEsc + `">` + "\n" +
		`  <meta property="og:description" content="` + descEsc + `">` + "\n" +
		`  <meta property="og:url" content="` + urlEsc + `">` + "\n" +
		`  <meta property="og:image" content="` + htmlEscape(ogImageURL(shareURL)) + `">` + "\n" +
		`  <meta property="og:type" content="article">`
}

// ogImageURL is the absolute URL of the static og-default.png logo.
// Resolved relative to the share URL's host so crawlers can fetch it
// without following relative paths.
//
// The file itself is NOT shipped with the app — drop a 1200x630 PNG
// (or use the project's brand mark) at frontend/public/og-default.png
// (Vite serves public/* at the root in dev, and the Go binary serves
// the same path in production). If the file is absent, the og:image
// meta will reference a 404 path; IM clients fall back to no thumbnail,
// which is acceptable for v1.
func ogImageURL(shareURL string) string {
	// shareURL looks like https://host/share/slug?t=...; we strip the path
	// and re-attach /og-default.png.
	if i := strings.Index(shareURL, "/share/"); i >= 0 {
		return shareURL[:i] + "/og-default.png"
	}
	return shareURL + "/og-default.png"
}

// extractFirstParagraph returns the first non-empty, non-heading line of
// markdown content, skipping fenced code blocks. Falls back to "" if no
// usable line is found.
func extractFirstParagraph(content string) string {
	inFence := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Track fenced code block boundaries (``` or ~~~)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if trimmed == "" {
			continue
		}
		// Skip headings and blockquote markers
		stripped := trimmed
		stripped = strings.TrimLeft(stripped, "#>")
		stripped = strings.TrimSpace(stripped)
		// Skip list bullets at the start
		stripped = regexp.MustCompile(`^[-*+]\s+`).ReplaceAllString(stripped, "")
		// Skip inline markdown emphasis for the description
		stripped = strings.TrimSpace(stripped)
		if stripped == "" {
			continue
		}
		return stripped
	}
	return ""
}

// truncateRunes shortens s to at most n runes, appending an ellipsis if cut.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

// htmlEscape escapes the few characters that matter inside HTML attribute
// values. We avoid pulling in html/template for this trivial use.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
