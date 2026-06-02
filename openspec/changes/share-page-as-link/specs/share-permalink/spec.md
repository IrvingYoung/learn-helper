## ADDED Requirements

### Requirement: Each wiki page has a non-enumerable share token
The system SHALL generate a 32-character `share_token` for every newly created wiki page, using a fixed alphabet of 32 ambiguity-free characters (lowercase letters excluding `i/l/o` and digits excluding `0/1`). The token SHALL be persisted on the `wiki_pages` row and SHALL NOT be derivable from the page slug, title, id, or any combination thereof.

#### Scenario: New page creation
- **WHEN** the AI or user creates a new wiki page via `POST /api/wiki`
- **THEN** the response includes a `share_token` field containing a 32-character string from the defined alphabet
- **AND** the token is stored in the `wiki_pages.share_token` column

#### Scenario: Existing pages at deploy time
- **WHEN** the migration runs on a database with pre-existing pages
- **THEN** those pages have `share_token = ''` (empty string)
- **AND** they are NOT publicly shareable until a token is generated for them

### Requirement: Slug is set once at page creation and never regenerated
The system SHALL compute the `slug` for a wiki page exactly once, at the time of creation, from the initial title. The system SHALL NOT recompute or change the `slug` when the page is later renamed, edited, or have its title changed. Pages whose `slug` was set by a pre-migration system SHALL keep their existing slug indefinitely.

#### Scenario: Rename preserves slug
- **WHEN** a user edits a page's title from "GitHub 热门日报" to "GitHub Trending Daily" via `PUT /api/wiki/{id}`
- **THEN** the row's `slug` field is unchanged
- **AND** the existing public share URL `/share/{slug}?t=...` continues to resolve to the page

#### Scenario: New slug only on creation
- **WHEN** `POST /api/wiki` creates a new page with title "Redis 入门"
- **THEN** the slug is computed from "Redis 入门" and stored
- **AND** subsequent edits to that page never change the stored slug

### Requirement: Public share URL is rendered with Open Graph meta
The system SHALL serve `GET /share/{slug}?t={token}` as an HTML response that contains a fully populated `<head>` with Open Graph meta tags derived from the target page. The system MUST validate that the supplied `token` matches the page's `share_token` exactly; on mismatch, the system MUST respond with HTTP 404 (not 401, to avoid revealing page existence).

#### Scenario: Valid token returns rendered page with og meta
- **WHEN** a public visitor requests `GET /share/{slug}?t={matching-token}` for a page with title "T" and summary "S"
- **THEN** the response is HTTP 200 with `Content-Type: text/html; charset=utf-8`
- **AND** the response body contains `<meta property="og:title" content="T">`
- **AND** the response body contains `<meta property="og:description" content="S">` (or first paragraph if summary empty)
- **AND** the response body contains `<meta property="og:url" content="https://{host}/share/{slug}?t={token}">`
- **AND** the response body contains `<meta property="og:image" content="https://{host}/static/og-default.png">`
- **AND** the response body contains `<meta property="og:type" content="article">`

#### Scenario: Missing or wrong token returns 404
- **WHEN** a public visitor requests `GET /share/{slug}?t=wrong-token` or `GET /share/{slug}` (no token) or `GET /share/{slug}?t=` (empty token)
- **THEN** the response is HTTP 404
- **AND** the response body does not reveal whether the slug exists

#### Scenario: og:description falls back to first paragraph
- **WHEN** a page has empty `summary` and content beginning with "# Title\n\nFirst paragraph text here."
- **THEN** the og:description meta value is "First paragraph text here." (truncated to 200 chars)

### Requirement: Public content API serves page data to anonymous visitors
The system SHALL expose `GET /api/share/{slug}?t={token}` as a JSON endpoint that returns the page's `id`, `title`, `slug`, `content`, `content_status`, `summary`, and `updated_at` when the token is valid. The response MUST NOT include the `share_token` field itself. On invalid or missing token, the system MUST respond with HTTP 404.

#### Scenario: Valid token returns page data without share_token
- **WHEN** a public visitor's browser fetches `/api/share/{slug}?t={valid-token}`
- **THEN** the response is HTTP 200 with JSON containing `id`, `title`, `slug`, `content`, `content_status`, `summary`, `updated_at`
- **AND** the response JSON does NOT contain a `share_token` field

#### Scenario: Invalid token returns 404
- **WHEN** a public visitor's browser fetches `/api/share/{slug}?t=invalid` or `/api/share/{slug}` (no token)
- **THEN** the response is HTTP 404

### Requirement: Owner API returns share_token for menu use
The system SHALL include the `share_token` field in the JSON response of the owner's existing `GET /api/wiki/{slug}` endpoint, so that the owner-facing SPA can construct the public share URL without an extra round-trip.

#### Scenario: Owner fetches own page
- **WHEN** the owner's SPA calls `GET /api/wiki/{slug}` (no token, same-origin trusted)
- **THEN** the response JSON includes the page's `share_token` (or empty string if not yet generated)

### Requirement: Frontend route reflects selected page in URL
The system SHALL provide a `/wiki/{slug}` frontend route that, when accessed directly, opens the page with the given slug. When the user navigates between pages within the owner SPA, the URL SHALL update to `/wiki/{slug}` so that the browser address bar, back/forward buttons, and bookmarks all reflect the current page.

#### Scenario: Direct deep link
- **WHEN** the owner pastes `https://{host}/wiki/my-page` into the address bar and presses Enter
- **THEN** the SPA loads and displays the page with slug `my-page`

#### Scenario: Internal navigation updates URL
- **WHEN** the owner is on `/wiki/page-a` and clicks a tree node for `page-b`
- **THEN** the URL changes to `/wiki/page-b` (via History API, no full page reload)
- **AND** pressing the browser back button returns to `/wiki/page-a`

#### Scenario: Browser reload preserves page
- **WHEN** the owner is viewing `/wiki/my-page` and presses F5 / Cmd-R
- **THEN** the SPA reloads and displays the same page (`my-page`)

### Requirement: Share menu exposes "copy link" action
The system SHALL display a "复制链接" menu item in the PageViewer share dropdown (alongside the existing "用图片分享" item). Clicking the item SHALL copy the full public share URL `https://{window.location.origin}/share/{slug}?t={token}` to the system clipboard, and SHALL provide a visible ✓ confirmation lasting at least 1 second.

#### Scenario: Copy success
- **WHEN** the owner clicks "复制链接" on a page with slug `my-page` and token `abc123`
- **THEN** `navigator.clipboard.writeText` is called with `https://{origin}/share/my-page?t=abc123`
- **AND** the menu item label briefly changes to "✓ 已复制" for at least 1 second before reverting

#### Scenario: Copy on page without token
- **WHEN** the owner clicks "复制链接" on a page whose `share_token` is empty (legacy page, pre-migration)
- **THEN** the menu item is disabled and shows a tooltip "该页面尚未生成分享链接,请刷新或重新生成"
- **AND** no clipboard write is attempted

### Requirement: Public visitor SPA shows read-only view
When a public visitor's browser renders `/share/{slug}?t={token}`, the resulting SPA experience MUST be a read-only view of the page: no edit UI, no AI selection tooltip, no draft confirmation banner, and no owner-only navigation. The page MUST still display mermaid diagrams, code blocks, and other markdown content with the same fidelity as the owner sees.

#### Scenario: Public visitor sees content without owner UI
- **WHEN** a public visitor loads `/share/{slug}?t={token}` in a browser
- **THEN** the page content (including mermaid blocks) is rendered identically to the owner view
- **AND** no "在 AI 中打开" tooltip is available on text selection
- **AND** no draft confirmation banner is shown
- **AND** wiki `[[internal links]]` appear as plain text (not clickable links)

#### Scenario: Public visitor sees "open in app" link
- **WHEN** a public visitor loads `/share/{slug}?t={token}`
- **THEN** a visible link `<a href="/wiki/{slug}">在 LLM Wiki 中打开</a>` is present at the top of the page

### Requirement: No write operations exposed via the public path
The system SHALL NOT expose any state-mutating endpoint (POST, PUT, DELETE) under the `/api/share/...` prefix or via the `/share/...` route. The public read path (`GET /api/share/...` and `GET /share/...`) is the only public surface added by this change.

#### Scenario: Only GET on public path
- **WHEN** a public visitor sends `POST /api/share/{slug}` or `PUT /share/{slug}` or `DELETE /api/share/{slug}`
- **THEN** the response is HTTP 405 Method Not Allowed (or 404)
- **AND** no state change occurs
