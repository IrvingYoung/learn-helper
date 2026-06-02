# Mobile Adaptation Design

## Scope

Mobile adaptation for LLM Wiki, focusing on **read + AI chat**. No tree management (drag/drop, rename, move, delete) on mobile — those stay desktop-only.

## Target

- Phones (< 768px). Tablets not in scope for this round.
- Breakpoint: `768px` (Tailwind `md:`)

## Core Decisions

1. **2-tab bottom navigation**: 阅读 / 对话
2. **阅读 tab**: full knowledge tree → push to full-page view (iOS-style navigation)
3. **对话 tab**: reuse existing `ChatPanel` with minor mobile tweaks
4. **Page creation**: only through AI conversation (no manual "+" button on mobile)
5. **"Ask AI" button**: on page view, carries context to chat tab
6. **Detection**: CSS media queries + `window.matchMedia` for the few JS cases

---

## Section 1: Detection & Architecture

**Breakpoint**: `768px` (`md:` in Tailwind)

- Layout switch via CSS: desktop uses `react-resizable-panels` three-column layout, mobile uses two-tab shell
- JS detection only where needed (e.g., `ChatPanel` drawer behavior): `window.matchMedia('(max-width: 767px)')`
- No new state management libraries; native CSS + matchMedia is sufficient

**Routes unchanged**: `/wiki`, `/share/:slug`, `/settings`, `/cron` keep their route structure. Only `/wiki` renders a different shell on mobile.

---

## Section 2: Mobile Shell

When `< 768px`, `WikiPage` renders `MobileShell` instead of the three-column `Group`:

```
┌─────────────────────────┐
│  LLM Wiki        [⋯]   │  ← simplified header
├─────────────────────────┤
│                         │
│   current tab content   │
│                         │
├─────────────────────────┤
│  📖 阅读    💬 对话      │  ← bottom tab bar, fixed 56px
└─────────────────────────┘
```

**Header**:
- Brand name + overflow menu ("⋯") containing: theme toggle, settings, cron tasks
- Removes the 6 desktop header buttons (tree collapse, page collapse, etc.)

**Tab behavior**:
- Each tab preserves its own scroll position (switching away and back doesn't lose position)
- 阅读 tab has internal stack (tree view ↔ page view), managed via push/pop state, not routing
- 对话 tab is the existing `ChatPanel`, reused as-is

---

## Section 3: 阅读 Tab

Two sub-views managed by a stack (push page, "←" pops back to tree):

### Tree View (default)

```
┌─────────────────────────┐
│  🔍 搜索页面……           │  ← search bar, real-time filter
├─────────────────────────┤
│  ▾ 编程                  │  ← collapsible parent
│    ○ React Hooks         │  ← tap → push page view
│    ○ Next.js 路由
│    ○ CSS Grid
│  ▾ 设计模式
│    ○ 单例模式
│  ▾ 阅读笔记
│    …
└─────────────────────────┘
```

- Reuse existing `KnowledgeTree` component with **drag-and-drop and context menu disabled** (via props)
- Search bar with real-time filtering (existing logic)
- Node touch targets at least 44px tall
- No "add child" button on mobile (creation is AI-only)

### Page View (pushed)

```
┌─────────────────────────┐
│  ← React Hooks           │  ← back button + page title
├─────────────────────────┤
│                         │
│  [page content,         │  ← reuse PageViewer
│   full-screen reading]  │
│                         │
│                         │
│            [问 AI] 🤖   │  ← floating action button, bottom-right
└─────────────────────────┘
```

- "←" button pops back to tree view
- Page content reuses existing `PageViewer` component
- **"Ask AI" floating button**:
  - Taps → switches to 对话 tab
  - Auto-carries current page title as context
  - If text is selected (`window.getSelection()`), carries selected text too
  - Shows a dismissible context strip in 对话 tab: "正在讨论: React Hooks"
- Share functionality (`ShareAsImageModal`) accessible via a small icon in the page header

---

## Section 4: 对话 Tab

Reuses `ChatPanel` with minor mobile adjustments:

**Unchanged (already works)**:
- Conversation list dropdown drawer (`showList`)
- Conversation menu (rename, delete)
- Message stream, SSE streaming
- Input box + skill commands (`/skill`)
- Permission approval queue (`PermissionQueue`)
- AskUser card (`AskUserCard`)

**Mobile adjustments**:
- **Input box**: fixed at bottom, 100% width, min-height 48px (touch-friendly)
- **Conversation list drawer**: change from dropdown (`max-h-80`) to bottom sheet (slides up from bottom, 70% screen height) — standard mobile sheet pattern
- **Context from 阅读 tab**: when user taps "Ask AI" and switches here:
  - Input pre-filled with `[关于页面: React Hooks]` prefix
  - Dismissible context strip shown above input: "正在讨论: React Hooks"
  - Dismissing clears the context

**Page creation flow** (unchanged): user says "帮我写个 XX 页面" → AI calls `create_page` → `PermissionQueue` shows approval → user confirms → page created → tree refreshes. Existing logic, no changes needed.

---

## Section 5: Other Pages

### `/share/:slug` (public share)
- Already single-column (header + PageViewer), no structural changes needed
- Only adjustment: "在 LLM Wiki 中打开" button touch target enlarged to 44px

### `/settings`
- Already `max-w-2xl` single-column layout
- Mobile: change `px-6` to `px-4`, ensure buttons are 44px touch targets

### `/cron`, `/cron/new`, `/cron/:id`
- List page `md:grid-cols-2` already falls back to single-column on mobile — OK
- Form pages: ensure input fields and buttons have adequate touch targets

---

## Implementation Strategy

### New files
- `src/components/MobileShell.tsx` — bottom tab container, header simplification
- `src/components/MobileReadingTab.tsx` — tree view + page view stack
- `src/components/MobilePageView.tsx` — full-screen page with "Ask AI" FAB

### New hook
- `src/lib/useIsMobile.ts` — `matchMedia('(max-width: 767px)')` with SSR-safe default

### Modified files
- `src/components/WikiPage.tsx` — conditional render: `useIsMobile()` → `MobileShell` vs desktop three-column `Group`
- `src/components/KnowledgeTree.tsx` — accept `disableDrag` and `disableContextMenu` props
- `src/components/ChatPanel.tsx` — accept optional `contextFromReading` prop for the context strip; adjust conversation list drawer on mobile

### CSS
- `@media (max-width: 767px)` rules in `index.css` for spacing/font-size micro-adjustments
- Touch target minimums enforced via Tailwind utilities

### What stays untouched
- All desktop logic — `MobileShell` and the three-column layout are mutually exclusive renders
- Backend — zero backend changes
- Routes — unchanged
- `/share/:slug`, `/settings`, `/cron` — minor CSS tweaks only

---

## Testing

- Chrome DevTools mobile simulator: verify all pages at 375px and 390px widths
- Manual test critical path: tree → page → "Ask AI" → chat → approve → page created → tree refreshes
- Verify desktop layout is unaffected (no regressions)
- Test conversation list bottom sheet on mobile
- Test permission queue in mobile chat view
