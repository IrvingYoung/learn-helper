## 1. Database

- [x] 1.1 Add migration in `backend/cmd/server/main.go` migrations block: `ALTER TABLE wiki_pages ADD COLUMN share_token TEXT NOT NULL DEFAULT ''` + `CREATE INDEX idx_wiki_pages_share_token ON wiki_pages(share_token)`
- [x] 1.2 Verify migration is idempotent (wrapped in `tryExec` or similar pattern used by existing migrations)

## 2. Backend — share_token lifecycle

- [x] 2.1 Add `shareTokenAlphabet` constant and `newShareToken()` function in `backend/internal/handler/wiki.go` (32 chars from `23456789abcdefghjkmnpqrstuvwxyz`, 32-byte output)
- [x] 2.2 Modify `CreateWikiPage` to generate a `share_token` via `newShareToken()` and include it in the response JSON
- [x] 2.3 Modify `UpdateWikiPage` to NOT recompute or update the `slug` column on rename — title edits leave slug untouched
- [x] 2.4 Modify `GetWikiPageBySlug` (owner API) to include the page's `share_token` in the response JSON

## 3. Backend — public content API

- [x] 3.1 Add `GetPublicSharePage` handler at `GET /api/share/{slug}` (note: token comes from `?t=` query param): look up `wiki_pages WHERE slug=?`, verify `share_token=?`, return JSON with `id, title, slug, content, content_status, summary, updated_at` (omitting `share_token`); return 404 on miss
- [x] 3.2 Register the new route in `backend/cmd/server/main.go` router (alongside existing `/api/wiki/...`)

## 4. Backend — SSR share page

- [x] 4.1 Implement `getDistIndexHTML()` helper in backend: read the SPA's `index.html` from disk (path configurable, defaults to `./dist/index.html` relative to working dir, with `go:embed` fallback for production binary)
- [x] 4.2 Implement `buildOgMeta(page *WikiPage, fullShareURL string) string` returning the `<title>` + 5 `<meta property="og:*">` tags as a single HTML string
- [x] 4.3 Implement `extractFirstParagraph(content string) string` helper that skips fenced code blocks and markdown headings, returns the first non-empty line truncated to 200 chars
- [x] 4.4 Add `GetShareSSRPage` handler at `GET /share/{slug}`: validate token (same as public API), call `getDistIndexHTML`, inject `buildOgMeta` output before `</head>`, return 200 with `Content-Type: text/html; charset=utf-8`; return 404 on token miss
- [x] 4.5 Register the new route in `backend/cmd/server/main.go` router

## 5. Frontend — routing & URL sync

- [x] 5.1 Add `<Route path="/wiki/:slug" element={<WikiPage />} />` in `frontend/src/App.tsx` (keep existing `/wiki` route)
- [x] 5.2 In `frontend/src/app/wiki/page.tsx` (`WikiPage` component): read `const { slug: urlSlug } = useParams<{slug: string}>()`, set as initial `selectedSlug` state
- [x] 5.3 Add `useEffect` in `WikiPage` that calls `setSelectedSlug(urlSlug)` when `urlSlug` changes (handles in-SPA navigation back/forward)
- [x] 5.4 Modify tree click handler: `onSelectPage` should call `useNavigate()('/wiki/' + slug)` instead of just setting state
- [x] 5.5 In `PageViewer` (or `WikiPage`), detect `useLocation().pathname.startsWith('/share/')` and suppress owner-only props (`onAskAI`, draft confirmation, selection tooltip) — public path is read-only

## 6. Frontend — types & data

- [x] 6.1 Add `share_token?: string` to the `WikiPage` interface in `frontend/src/types/index.ts`
- [x] 6.2 Verify the owner API call site for `GET /api/wiki/{slug}` (`frontend/src/lib/api.ts` or equivalent) does not strip unknown fields, so `share_token` flows through to the page state

## 7. Frontend — share menu

- [x] 7.1 Add a "🔗 复制链接" `<button role="menuitem">` inside the existing share dropdown in `frontend/src/components/PageViewer.tsx` (after the "用图片分享" item)
- [x] 7.2 Wire onClick: build URL `window.location.origin + '/share/' + page.slug + '?t=' + page.share_token`, call `navigator.clipboard.writeText(url)`, on success set local state `copied=true` for 1.5s and replace label with "✓ 已复制"
- [x] 7.3 If `!page.share_token`, disable the button and add `title="该页面尚未生成分享链接"`; do not call clipboard API
- [x] 7.4 Handle clipboard API unavailable (`!navigator.clipboard?.writeText`): disable with `title="当前浏览器不支持剪贴板写入"`

## 8. Deployment

- [x] 8.1 Add a deployment section to `README.md` documenting the reverse-proxy rules table: `/api/*` → :8080, `/share/*` → :8080, other → SPA static
- [x] 8.2 Update `frontend/vite.config.ts` (or wherever dev proxy is configured) to add a proxy rule: `/share` → `http://localhost:8080` (and `/api` rule should already exist)
- [x] 8.3 Document the security caveat in README: backend :8080 MUST NOT be directly exposed to the public internet (only via the proxy that maps `/api` and `/share` prefixes) — otherwise `/api/wiki/...` write endpoints are reachable

## 9. Verification

- [ ] 9.1 Manual: create a new wiki page, verify response includes a 32-char `share_token`; verify token is stored by restarting backend and refetching
- [ ] 9.2 Manual: rename an existing page, verify the row's `slug` is unchanged after the PUT
- [ ] 9.3 Manual: in dev (with Vite proxy), visit `http://localhost:3000/share/{slug}?t={token}` — verify HTML response has all 5 og:* meta tags in `<head>`
- [ ] 9.4 Manual: visit `/share/{slug}?t=wrong` — verify 404 response
- [ ] 9.5 Manual: visit `/share/{slug}?t={token}` with valid token, wait for JS, verify page content (including mermaid) renders
- [ ] 9.6 Manual: paste `http://localhost:3000/share/{slug}?t={token}` into WeChat (or any IM with link preview); verify preview card shows title/description/image
- [ ] 9.7 Manual: in owner SPA, click "复制链接" menu item — verify clipboard contains the full URL and the label flashes "✓ 已复制"
- [ ] 9.8 Manual: in owner SPA, click tree nodes — verify URL updates to `/wiki/{slug}` and browser back/forward navigates between pages
- [ ] 9.9 Manual: in owner SPA, reload on `/wiki/{slug}` — verify the same page reopens
- [ ] 9.10 Manual: as public visitor on `/share/...`, verify no "在 AI 中打开" tooltip appears on text selection, no draft banner, `[[links]]` render as plain text
