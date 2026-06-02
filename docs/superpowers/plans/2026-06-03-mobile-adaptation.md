# Mobile Adaptation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a mobile-responsive layout to the LLM Wiki frontend so phones can read wiki pages and chat with AI.

**Architecture:** A `useIsMobile` hook detects `<768px` screens. `WikiPage` conditionally renders either the existing desktop three-column layout or a new `MobileShell` with two bottom tabs (阅读/对话). The 阅读 tab contains a tree-to-page navigation stack. The 对话 tab reuses the existing `ChatPanel`. Desktop logic is untouched.

**Tech Stack:** React 19, TypeScript, Tailwind CSS, existing `react-resizable-panels` (desktop only), `matchMedia` API

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `src/lib/useIsMobile.ts` | `matchMedia` hook, SSR-safe |
| Create | `src/components/MobileShell.tsx` | Bottom tab container, simplified header |
| Create | `src/components/MobileReadingTab.tsx` | Tree view + page view stack with push/pop |
| Modify | `src/components/KnowledgeTree.tsx` | Accept `readOnly` prop to hide management UI |
| Modify | `src/components/WikiPage.tsx` | Conditional render: mobile vs desktop |
| Modify | `src/index.css` | Mobile spacing/font adjustments |

---

### Task 1: `useIsMobile` Hook

**Files:**
- Create: `src/lib/useIsMobile.ts`

- [ ] **Step 1: Create the hook**

```ts
// src/lib/useIsMobile.ts
import { useState, useEffect } from 'react';

const MQ = '(max-width: 767px)';

export function useIsMobile(): boolean {
  const [isMobile, setIsMobile] = useState(() => {
    if (typeof window === 'undefined') return false;
    return window.matchMedia(MQ).matches;
  });

  useEffect(() => {
    const mql = window.matchMedia(MQ);
    const handler = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    mql.addEventListener('change', handler);
    return () => mql.removeEventListener('change', handler);
  }, []);

  return isMobile;
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add src/lib/useIsMobile.ts
git commit -m "feat: add useIsMobile hook"
```

---

### Task 2: `KnowledgeTree` Read-Only Mode

**Files:**
- Modify: `src/components/KnowledgeTree.tsx`

The tree already hides the "新建" button when `onAddChild` is not passed, and hides the context menu when `onContextMenu`/`onRename`/etc. are not passed. But it still shows the "⋯" hover button on each node when `onContextMenu` is provided. We need a `readOnly` prop to also suppress the hover "⋯" button and the "新建顶层页面" header button.

- [ ] **Step 1: Add `readOnly` prop to `KnowledgeTreeProps`**

In `KnowledgeTree.tsx`, update the interface and destructure:

```tsx
interface KnowledgeTreeProps {
  tree: WikiTreeNode[];
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  collapsed: boolean;
  onAddChild?: (parentId: number | null) => void;
  onRename?: (nodeId: number, newTitle: string) => void;
  onMove?: (nodeId: number, newParentId: number | null) => void;
  onAskAIMove?: (nodeId: number) => void;
  onDelete?: (nodeId: number, hasChildren: boolean) => void;
  newNodeId?: number | null;
  readOnly?: boolean;  // <-- ADD THIS
}
```

Update the function signature to include `readOnly = false`:

```tsx
export function KnowledgeTree({
  tree, selectedSlug, onSelect, collapsed,
  onAddChild, onRename, onMove, onAskAIMove, onDelete, newNodeId,
  readOnly = false,  // <-- ADD THIS
}: KnowledgeTreeProps) {
```

- [ ] **Step 2: Suppress "新建" button when readOnly**

Change the "新建顶层页面" button condition (around line 226):

```tsx
{onAddChild && !readOnly && (
  <button ...>
```

- [ ] **Step 3: Pass `readOnly` to TreeNode to suppress hover "⋯" button**

In the `TreeNode` rendering (around line 268), add `readOnly` prop:

```tsx
<TreeNode
  key={node.id}
  node={node}
  selectedSlug={selectedSlug}
  onSelect={onSelect}
  depth={0}
  expandedIds={expandedIds}
  onToggle={handleToggle}
  onContextMenu={searchQuery || readOnly ? undefined : handleContextMenu}  // <-- CHANGE
  onAddChild={readOnly ? undefined : onAddChild}  // <-- CHANGE
  onRename={readOnly ? undefined : onRename}  // <-- CHANGE
  onMove={readOnly ? undefined : onMove}  // <-- CHANGE
  renameNodeId={readOnly ? undefined : renameNodeId}  // <-- CHANGE
  onRenameStarted={readOnly ? undefined : () => setRenameNodeId(null)}  // <-- CHANGE
  draggedId={readOnly ? undefined : draggedId}  // <-- CHANGE
  draggedDescendants={readOnly ? undefined : draggedDescendants}  // <-- CHANGE
  onDragStart={readOnly ? undefined : (id) => setDraggedId(id)}  // <-- CHANGE
  onDragEnd={readOnly ? undefined : () => { setDraggedId(null); setDropHoverId(null); }}  // <-- CHANGE
  dropHoverId={readOnly ? undefined : dropHoverId}  // <-- CHANGE
  onDropHover={readOnly ? undefined : setDropHoverId}  // <-- CHANGE
  disabled={!!searchQuery || readOnly}  // <-- CHANGE
/>
```

- [ ] **Step 4: Verify it compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add src/components/KnowledgeTree.tsx
git commit -m "feat: add readOnly prop to KnowledgeTree"
```

---

### Task 3: `MobileShell` Component

**Files:**
- Create: `src/components/MobileShell.tsx`

This is the top-level mobile container: simplified header + bottom tab bar + tab content area.

- [ ] **Step 1: Create `MobileShell.tsx`**

```tsx
// src/components/MobileShell.tsx
import { useState, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTheme } from '../contexts/ThemeContext';
import { ChatPanel } from './ChatPanel';
import { MobileReadingTab } from './MobileReadingTab';
import type { WikiPage, WikiTreeNode, ToolCallInfo } from '../types';

interface MobileShellProps {
  tree: WikiTreeNode[];
  selectedSlug: string | null;
  onSelectSlug: (slug: string) => void;
  displayPage: WikiPage | null;
  breadcrumb: { title: string; slug: string }[];
  onInternalLink: (href: string) => void;
  onAskAI: (text: string, pageTitle: string) => void;
  onContentConfirmed: (pageId: number) => void;
  onWriteToolComplete: (tc?: ToolCallInfo) => void;
  focusPageId: number | null;
  currentSlug: string | undefined;
  currentPageTitle: string | undefined;
}

type Tab = 'reading' | 'chat';

export function MobileShell({
  tree,
  selectedSlug,
  onSelectSlug,
  displayPage,
  breadcrumb,
  onInternalLink,
  onAskAI,
  onContentConfirmed,
  onWriteToolComplete,
  focusPageId,
  currentSlug,
  currentPageTitle,
}: MobileShellProps) {
  const [activeTab, setActiveTab] = useState<Tab>('reading');
  const [showOverflow, setShowOverflow] = useState(false);
  const navigate = useNavigate();
  const { theme, toggleTheme } = useTheme();
  const chatPanelRef = useRef<{
    setSelectedText: (text: string, pageTitle: string) => void;
    sendMessage: (text: string) => void;
    continueAfterConfirm: () => void;
  }>(null);

  const handleAskAIFromReading = useCallback((text: string, pageTitle: string) => {
    setActiveTab('chat');
    setTimeout(() => {
      chatPanelRef.current?.setSelectedText(text, pageTitle);
    }, 100);
  }, []);

  return (
    <div className="h-screen flex flex-col bg-th-bg-primary">
      {/* Header */}
      <header className="bg-th-bg-secondary/70 backdrop-blur-md border-b border-th-separator h-12 flex items-center px-4 shrink-0">
        <span className="font-display text-sm font-bold text-th-text-primary tracking-tight">LLM Wiki</span>
        <div className="flex-1" />
        <div className="relative">
          <button
            onClick={() => setShowOverflow(!showOverflow)}
            className="p-2 rounded-md text-th-text-muted hover:text-th-text-primary hover:bg-th-hover transition-all duration-150 active:scale-90"
            title="更多"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M12 6.75a.75.75 0 110-1.5.75.75 0 010 1.5zM12 12.75a.75.75 0 110-1.5.75.75 0 010 1.5zM12 18.75a.75.75 0 110-1.5.75.75 0 010 1.5z" />
            </svg>
          </button>
          {showOverflow && (
            <div className="absolute right-0 top-full mt-1 w-40 bg-th-bg-secondary border border-th-border rounded-lg shadow-th-lg z-20 py-1">
              <button
                onClick={() => { toggleTheme(); setShowOverflow(false); }}
                className="w-full text-left px-3 py-2 text-sm text-th-text-primary hover:bg-th-bg-tertiary flex items-center gap-2"
              >
                {theme === 'warm' ? '深色主题' : '暖色主题'}
              </button>
              <button
                onClick={() => { navigate('/settings'); setShowOverflow(false); }}
                className="w-full text-left px-3 py-2 text-sm text-th-text-primary hover:bg-th-bg-tertiary flex items-center gap-2"
              >
                设置
              </button>
              <button
                onClick={() => { navigate('/cron'); setShowOverflow(false); }}
                className="w-full text-left px-3 py-2 text-sm text-th-text-primary hover:bg-th-bg-tertiary flex items-center gap-2"
              >
                定时任务
              </button>
            </div>
          )}
        </div>
      </header>

      {/* Tab content */}
      <div className="flex-1 min-h-0 overflow-hidden">
        {activeTab === 'reading' ? (
          <MobileReadingTab
            tree={tree}
            selectedSlug={selectedSlug}
            onSelectSlug={onSelectSlug}
            displayPage={displayPage}
            breadcrumb={breadcrumb}
            onInternalLink={onInternalLink}
            onAskAI={handleAskAIFromReading}
            onContentConfirmed={onContentConfirmed}
          />
        ) : (
          <ChatPanel
            ref={chatPanelRef}
            focusPageId={focusPageId}
            currentSlug={currentSlug}
            currentPageTitle={currentPageTitle}
            onWriteToolComplete={onWriteToolComplete}
          />
        )}
      </div>

      {/* Bottom tab bar */}
      <nav className="bg-th-bg-secondary/70 backdrop-blur-md border-t border-th-separator h-14 flex shrink-0">
        <button
          onClick={() => setActiveTab('reading')}
          className={`flex-1 flex flex-col items-center justify-center gap-0.5 transition-colors ${
            activeTab === 'reading'
              ? 'text-th-accent'
              : 'text-th-text-muted hover:text-th-text-primary'
          }`}
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 19.477 5.754 20 7.5 20s3.332-.477 4.5-1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 19.477 18.247 20 16.5 20a3.5 3.5 0 01-3.5-3.5" />
          </svg>
          <span className="text-[10px] font-medium">阅读</span>
        </button>
        <button
          onClick={() => setActiveTab('chat')}
          className={`flex-1 flex flex-col items-center justify-center gap-0.5 transition-colors ${
            activeTab === 'chat'
              ? 'text-th-accent'
              : 'text-th-text-muted hover:text-th-text-primary'
          }`}
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
          <span className="text-[10px] font-medium">对话</span>
        </button>
      </nav>
    </div>
  );
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: errors about missing `MobileReadingTab` import — that's expected, we'll create it next.

- [ ] **Step 3: Commit**

```bash
git add src/components/MobileShell.tsx
git commit -m "feat: add MobileShell with bottom tab navigation"
```

---

### Task 4: `MobileReadingTab` Component

**Files:**
- Create: `src/components/MobileReadingTab.tsx`

This component manages a navigation stack: tree view (default) → page view (pushed). Uses a simple state-based stack, not routing.

- [ ] **Step 1: Create `MobileReadingTab.tsx`**

```tsx
// src/components/MobileReadingTab.tsx
import { useState, useCallback } from 'react';
import type { WikiPage, WikiTreeNode } from '../types';
import { KnowledgeTree } from './KnowledgeTree';
import { PageViewer } from './PageViewer';

interface MobileReadingTabProps {
  tree: WikiTreeNode[];
  selectedSlug: string | null;
  onSelectSlug: (slug: string) => void;
  displayPage: WikiPage | null;
  breadcrumb: { title: string; slug: string }[];
  onInternalLink: (href: string) => void;
  onAskAI: (text: string, pageTitle: string) => void;
  onContentConfirmed: (pageId: number) => void;
}

type View =
  | { type: 'tree' }
  | { type: 'page'; slug: string; title: string };

export function MobileReadingTab({
  tree,
  selectedSlug,
  onSelectSlug,
  displayPage,
  breadcrumb,
  onInternalLink,
  onAskAI,
  onContentConfirmed,
}: MobileReadingTabProps) {
  const [viewStack, setViewStack] = useState<View[]>([{ type: 'tree' }]);

  const currentView = viewStack[viewStack.length - 1];

  const pushPage = useCallback((slug: string) => {
    // Find title from tree
    const findTitle = (nodes: WikiTreeNode[]): string | undefined => {
      for (const n of nodes) {
        if (n.slug === slug) return n.title;
        if (n.children) {
          const found = findTitle(n.children);
          if (found) return found;
        }
      }
      return undefined;
    };
    const title = findTitle(tree) ?? slug;
    onSelectSlug(slug);
    setViewStack(prev => [...prev, { type: 'page', slug, title }]);
  }, [tree, onSelectSlug]);

  const goBack = useCallback(() => {
    setViewStack(prev => {
      if (prev.length <= 1) return prev;
      return prev.slice(0, -1);
    });
  }, []);

  const handleSelectFromTree = useCallback((slug: string) => {
    pushPage(slug);
  }, [pushPage]);

  if (currentView.type === 'tree') {
    return (
      <div className="h-full overflow-y-auto">
        <KnowledgeTree
          tree={tree}
          selectedSlug={selectedSlug}
          onSelect={handleSelectFromTree}
          collapsed={false}
          readOnly
        />
      </div>
    );
  }

  // Page view
  return (
    <div className="h-full flex flex-col">
      {/* Back header */}
      <div className="flex items-center gap-2 px-3 py-2 border-b border-th-separator shrink-0">
        <button
          onClick={goBack}
          className="p-1.5 rounded-md text-th-text-muted hover:text-th-text-primary hover:bg-th-hover transition-all duration-150 active:scale-90"
        >
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        <span className="text-sm font-medium text-th-text-primary truncate flex-1">
          {currentView.title}
        </span>
      </div>
      {/* Page content */}
      <div className="flex-1 min-h-0 overflow-hidden">
        <PageViewer
          page={displayPage}
          collapsed={false}
          breadcrumb={breadcrumb}
          onSelectPage={(slug) => pushPage(slug)}
          onInternalLink={onInternalLink}
          onAskAI={onAskAI}
          onContentConfirmed={onContentConfirmed}
        />
      </div>
      {/* Ask AI FAB */}
      <button
        onClick={() => {
          const sel = window.getSelection();
          const selectedText = sel && !sel.isCollapsed ? sel.toString().trim() : '';
          onAskAI(selectedText || currentView.title, currentView.title);
        }}
        className="fixed bottom-20 right-4 w-12 h-12 rounded-full bg-th-accent text-white shadow-th-lg flex items-center justify-center active:scale-90 transition-transform z-10"
        title="问 AI"
      >
        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" />
        </svg>
      </button>
    </div>
  );
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors (all imports resolve now)

- [ ] **Step 3: Commit**

```bash
git add src/components/MobileReadingTab.tsx
git commit -m "feat: add MobileReadingTab with tree-to-page stack"
```

---

### Task 5: Wire Up `WikiPage` Conditional Rendering

**Files:**
- Modify: `src/components/WikiPage.tsx`

In `WikiPageLayout`, add `useIsMobile()` and conditionally render `MobileShell` vs the existing desktop layout.

- [ ] **Step 1: Add imports**

Add at the top of `WikiPage.tsx`:

```tsx
import { useIsMobile } from '../lib/useIsMobile';
import { MobileShell } from './MobileShell';
```

- [ ] **Step 2: Add `useIsMobile` call inside `WikiPageLayout`**

After the existing state declarations (around line 57), add:

```tsx
const isMobile = useIsMobile();
```

- [ ] **Step 3: Extract shared logic into variables**

The mobile shell needs the same data the desktop layout uses. Before the `if (isPublicPath)` block (around line 281), ensure these variables are already available (they are — `displayPage`, `breadcrumb`, `tree`, `selectedSlug`, etc.).

- [ ] **Step 4: Add mobile rendering for public path**

In the public path block (line 281), the existing single-column layout is already mobile-friendly. No changes needed there.

- [ ] **Step 5: Add mobile rendering for owner path**

Before the existing `return (` with the three-column layout (around line 313), add the mobile branch:

```tsx
if (isMobile) {
  return (
    <MobileShell
      tree={tree || []}
      selectedSlug={selectedSlug}
      onSelectSlug={setSelectedSlug}
      displayPage={displayPage}
      breadcrumb={breadcrumb}
      onInternalLink={handleInternalLink}
      onAskAI={handleAskAI}
      onContentConfirmed={handleContentConfirmed}
      onWriteToolComplete={handlePageChanged}
      focusPageId={displayPage?.id ?? null}
      currentSlug={selectedSlug ?? displayPage?.slug ?? undefined}
      currentPageTitle={selectedPageInfo.title ?? displayPage?.title ?? undefined}
    />
  );
}
```

- [ ] **Step 6: Verify it compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add src/components/WikiPage.tsx
git commit -m "feat: wire up mobile shell in WikiPage"
```

---

### Task 6: Mobile CSS Adjustments

**Files:**
- Modify: `src/index.css`

- [ ] **Step 1: Add mobile-specific CSS rules**

Add at the end of `index.css`:

```css
/* === Mobile adjustments === */
@media (max-width: 767px) {
  /* Larger touch targets for tree nodes */
  .knowledge-tree-node {
    min-height: 44px;
  }

  /* PageViewer: reduce padding on mobile */
  .page-viewer-article {
    padding-left: 1rem;
    padding-right: 1rem;
  }

  /* ChatPanel input: full width, taller */
  .chat-input-mobile {
    min-height: 48px;
    font-size: 16px; /* prevents iOS zoom on focus */
  }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add src/index.css
git commit -m "feat: add mobile CSS adjustments"
```

---

### Task 7: Manual Testing & Verification

- [ ] **Step 1: Start dev servers**

```bash
cd backend && go run ./cmd/server &
cd frontend && npm run dev &
```

- [ ] **Step 2: Test in Chrome DevTools mobile simulator (375px)**

Open `http://localhost:3000/wiki` in Chrome, toggle device toolbar (Cmd+Shift+M), set to iPhone SE (375px).

Verify:
- Two bottom tabs visible: 阅读 / 对话
- Header shows "LLM Wiki" + "⋯" overflow menu
- Overflow menu has: theme toggle, settings, cron tasks

- [ ] **Step 3: Test 阅读 tab - tree view**

- Knowledge tree renders with all nodes
- Search bar works
- No "新建" button visible
- No "⋯" hover button on nodes
- Tap a node → page view pushes in

- [ ] **Step 4: Test 阅读 tab - page view**

- "←" back button + page title at top
- Page content renders correctly
- "问 AI" floating button visible (bottom-right)
- Internal links work (pushes to new page)
- "←" pops back to tree

- [ ] **Step 5: Test "问 AI" flow**

- On a page, tap "问 AI" → switches to 对话 tab
- Chat input has context about the page
- Send a message → AI responds
- Approve a write operation → page created → tree refreshes

- [ ] **Step 6: Test 对话 tab**

- Conversation list dropdown works
- Send/receive messages
- Permission queue shows when AI requests approval
- Conversation menu (rename/delete) works

- [ ] **Step 7: Test desktop is unaffected**

Resize browser to > 768px. Verify:
- Three-column layout renders as before
- All desktop functionality works (drag/drop, context menu, etc.)

- [ ] **Step 8: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: mobile testing adjustments"
```
