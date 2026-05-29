# Theme Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor LLM Wiki frontend to support Warm and Dark Tech themes with runtime switching via CSS custom properties.

**Architecture:** CSS custom properties on `:root`/`[data-theme="dark"]` define all colors and fonts. A React ThemeContext provides `theme`/`setTheme`/`toggleTheme` and persists to localStorage. All components replace hardcoded colors with semantic Tailwind classes mapped to CSS variables.

**Tech Stack:** React 19, Tailwind CSS v3, CSS Custom Properties, react-syntax-highlighter

---

### Task 1: Create theme CSS variables file

**Files:**
- Create: `frontend/src/styles/themes.css`

- [ ] **Step 1: Create the themes.css file with both theme variable sets**

```css
/* Warm theme (default) */
:root,
[data-theme="warm"] {
  --bg-primary: #faf9f7;
  --bg-secondary: #ffffff;
  --bg-tertiary: #f5f3ef;
  --border: #eae7e0;
  --border-hover: #d9d5cc;
  --text-primary: #1a1815;
  --text-secondary: #4a4640;
  --text-muted: #8a857a;
  --accent: #c45c26;
  --accent-light: #e87d3a;
  --accent-bg: #fdf6f0;
  --success: #2d7a3e;
  --warning: #c78a2a;
  --error: #dc2626;
  --node-filled: #2d7a3e;
  --node-partial: #c78a2a;
  --node-empty: #b5b0a6;
  --font-display: 'Playfair Display', serif;
  --font-body: 'Source Sans 3', sans-serif;
  --font-mono: 'Source Sans 3', sans-serif;
  --scrollbar-thumb: #d9d5cc;
  --scrollbar-thumb-hover: #b5b0a6;
  --shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
  --input-bg: #ffffff;
  --input-border: #d9d5cc;
  --user-bubble-bg: #c45c26;
  --user-bubble-text: #ffffff;
  --assistant-bubble-bg: #f5f3ef;
  --assistant-bubble-text: #1a1815;
  --badge-published-bg: #f0fdf4;
  --badge-published-text: #166534;
  --badge-published-border: #bbf7d0;
  --badge-published-dot: #22c55e;
  --badge-draft-bg: #fffbeb;
  --badge-draft-text: #92400e;
  --badge-draft-border: #fde68a;
  --badge-draft-dot: #f59e0b;
  --badge-empty-bg: #f5f3ef;
  --badge-empty-text: #8a857a;
  --badge-empty-border: #eae7e0;
  --badge-empty-dot: #b5b0a6;
}

/* Dark tech theme */
[data-theme="dark"] {
  --bg-primary: #0a0a0f;
  --bg-secondary: #111118;
  --bg-tertiary: #16161e;
  --border: #2a2a35;
  --border-hover: #3a3a4a;
  --text-primary: #e2e2e8;
  --text-secondary: #a0a0b0;
  --text-muted: #6a6a7a;
  --accent: #7c3aed;
  --accent-light: #a78bfa;
  --accent-bg: rgba(124, 58, 237, 0.1);
  --success: #22c55e;
  --warning: #f59e0b;
  --error: #ef4444;
  --node-filled: #22c55e;
  --node-partial: #f59e0b;
  --node-empty: #6a6a7a;
  --font-display: 'Inter', sans-serif;
  --font-body: 'Inter', sans-serif;
  --font-mono: 'JetBrains Mono', monospace;
  --scrollbar-thumb: #2a2a35;
  --scrollbar-thumb-hover: #3a3a4a;
  --shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
  --input-bg: #16161e;
  --input-border: #2a2a35;
  --user-bubble-bg: #7c3aed;
  --user-bubble-text: #ffffff;
  --assistant-bubble-bg: #16161e;
  --assistant-bubble-text: #e2e2e8;
  --badge-published-bg: rgba(34, 197, 94, 0.1);
  --badge-published-text: #4ade80;
  --badge-published-border: rgba(34, 197, 94, 0.2);
  --badge-published-dot: #22c55e;
  --badge-draft-bg: rgba(245, 158, 11, 0.1);
  --badge-draft-text: #fbbf24;
  --badge-draft-border: rgba(245, 158, 11, 0.2);
  --badge-draft-dot: #f59e0b;
  --badge-empty-bg: rgba(106, 106, 122, 0.1);
  --badge-empty-text: #a0a0b0;
  --badge-empty-border: rgba(106, 106, 122, 0.2);
  --badge-empty-dot: #6a6a7a;
}

/* Custom scrollbar */
.custom-scroll::-webkit-scrollbar { width: 5px; }
.custom-scroll::-webkit-scrollbar-track { background: transparent; }
.custom-scroll::-webkit-scrollbar-thumb {
  background: var(--scrollbar-thumb);
  border-radius: 3px;
}
.custom-scroll::-webkit-scrollbar-thumb:hover {
  background: var(--scrollbar-thumb-hover);
}

/* Dark theme glow effect */
[data-theme="dark"] .glow {
  box-shadow: 0 0 20px rgba(124, 58, 237, 0.15);
}
[data-theme="dark"] .glow-strong {
  box-shadow: 0 0 30px rgba(124, 58, 237, 0.25), 0 0 60px rgba(124, 58, 237, 0.1);
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/styles/themes.css
git commit -m "feat: add theme CSS variables for warm and dark themes"
```

---

### Task 2: Create fonts CSS file

**Files:**
- Create: `frontend/src/styles/fonts.css`

- [ ] **Step 1: Create fonts.css with Google Fonts imports**

```css
/* Warm theme fonts */
@import url('https://fonts.googleapis.com/css2?family=Playfair+Display:wght@400;600;700&family=Source+Sans+3:wght@300;400;500;600&display=swap');

/* Dark theme fonts */
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@300;400;500;600&display=swap');

body {
  font-family: var(--font-body);
  color: var(--text-primary);
  background: var(--bg-primary);
}

.font-display {
  font-family: var(--font-display);
}

.font-mono {
  font-family: var(--font-mono);
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/styles/fonts.css
git commit -m "feat: add Google Fonts imports for both themes"
```

---

### Task 3: Update tailwind.config.js with theme colors

**Files:**
- Modify: `frontend/tailwind.config.js`

- [ ] **Step 1: Replace the empty tailwind config with theme-aware configuration**

```js
/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        display: ['var(--font-display)', 'serif'],
        body: ['var(--font-body)', 'sans-serif'],
        mono: ['var(--font-mono)', 'monospace'],
      },
      colors: {
        'th-bg-primary': 'var(--bg-primary)',
        'th-bg-secondary': 'var(--bg-secondary)',
        'th-bg-tertiary': 'var(--bg-tertiary)',
        'th-border': 'var(--border)',
        'th-border-hover': 'var(--border-hover)',
        'th-text-primary': 'var(--text-primary)',
        'th-text-secondary': 'var(--text-secondary)',
        'th-text-muted': 'var(--text-muted)',
        'th-accent': 'var(--accent)',
        'th-accent-light': 'var(--accent-light)',
        'th-accent-bg': 'var(--accent-bg)',
        'th-success': 'var(--success)',
        'th-warning': 'var(--warning)',
        'th-error': 'var(--error)',
        'th-node-filled': 'var(--node-filled)',
        'th-node-partial': 'var(--node-partial)',
        'th-node-empty': 'var(--node-empty)',
        'th-input-bg': 'var(--input-bg)',
        'th-input-border': 'var(--input-border)',
        'th-user-bubble': 'var(--user-bubble-bg)',
        'th-user-bubble-text': 'var(--user-bubble-text)',
        'th-assistant-bubble': 'var(--assistant-bubble-bg)',
        'th-assistant-bubble-text': 'var(--assistant-bubble-text)',
      },
      boxShadow: {
        'th': 'var(--shadow)',
      },
    },
  },
  plugins: [],
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/tailwind.config.js
git commit -m "feat: extend Tailwind config with theme CSS variable colors"
```

---

### Task 4: Update index.css to import theme files

**Files:**
- Modify: `frontend/src/index.css`

- [ ] **Step 1: Replace index.css content**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@import './styles/fonts.css';
@import './styles/themes.css';
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/index.css
git commit -m "feat: import theme CSS files in index.css"
```

---

### Task 5: Create ThemeContext

**Files:**
- Create: `frontend/src/contexts/ThemeContext.tsx`

- [ ] **Step 1: Create ThemeContext.tsx**

```tsx
import { createContext, useContext, useState, useEffect, type ReactNode } from 'react';

type Theme = 'warm' | 'dark';

interface ThemeContextType {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
}

const ThemeContext = createContext<ThemeContextType | null>(null);

const STORAGE_KEY = 'llm-wiki-theme';

function getInitialTheme(): Theme {
  if (typeof window === 'undefined') return 'warm';
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === 'warm' || stored === 'dark') return stored;
  return 'warm';
}

function applyTheme(theme: Theme) {
  document.documentElement.setAttribute('data-theme', theme);
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(getInitialTheme);

  useEffect(() => {
    applyTheme(theme);
    localStorage.setItem(STORAGE_KEY, theme);
  }, [theme]);

  const setTheme = (t: Theme) => setThemeState(t);

  const toggleTheme = () => setThemeState((prev) => (prev === 'warm' ? 'dark' : 'warm'));

  return (
    <ThemeContext.Provider value={{ theme, setTheme, toggleTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider');
  return ctx;
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/contexts/ThemeContext.tsx
git commit -m "feat: add ThemeContext with localStorage persistence"
```

---

### Task 6: Wire ThemeProvider into app

**Files:**
- Modify: `frontend/src/main.tsx`

- [ ] **Step 1: Wrap App with ThemeProvider in main.tsx**

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { ThemeProvider } from './contexts/ThemeContext'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider>
      <App />
    </ThemeProvider>
  </React.StrictMode>,
)
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/main.tsx
git commit -m "feat: wrap App with ThemeProvider"
```

---

### Task 7: Refactor WikiPage.tsx to use theme classes

**Files:**
- Modify: `frontend/src/components/WikiPage.tsx`

- [ ] **Step 1: Replace all hardcoded color classes with theme classes**

Replace in WikiPage.tsx:
- `bg-gray-50` → `bg-th-bg-primary`
- `bg-white` → `bg-th-bg-secondary`
- `border-gray-200` → `border-th-border`
- `text-gray-900` → `text-th-text-primary`
- `text-gray-500` → `text-th-text-muted`
- `text-blue-600` → `text-th-accent`
- `bg-blue-50` → `bg-th-accent-bg`
- `hover:text-gray-700` → `hover:text-th-text-secondary`
- `hover:bg-gray-100` → `hover:bg-th-bg-tertiary`
- `bg-black/30` → `bg-black/50`
- Settings modal `bg-white` → `bg-th-bg-secondary`
- Settings form elements borders → `border-th-input-border`
- Settings save button `bg-blue-600` → `bg-th-accent`

Also add the theme toggle button in the header. Import `useTheme` from `../contexts/ThemeContext`.

Full updated component:

```tsx
import { useState, useCallback } from 'react';
import useSWR from 'swr';
import { fetchWikiTree, fetchWikiPage, fetchOverviewPage } from '../lib/api';
import { useTheme } from '../contexts/ThemeContext';
import type { WikiPage } from '../types';
import { KnowledgeTree } from './KnowledgeTree';
import { ChatPanel } from './ChatPanel';
import { PageViewer } from './PageViewer';

export function WikiPageLayout() {
  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);
  const [leftWidth] = useState(280);
  const [rightWidth] = useState(400);
  const [selectedSlug, setSelectedSlug] = useState<string | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [provider, setProvider] = useState('claude');
  const [model, setModel] = useState('claude-sonnet-4-7-20250514');
  const [apiKey, setApiKey] = useState('');
  const [saved, setSaved] = useState(false);
  const { theme, toggleTheme } = useTheme();

  const { data: tree, mutate: mutateTree } = useSWR('wiki-tree', fetchWikiTree);
  const { data: page } = useSWR(
    selectedSlug ? `wiki-page-${selectedSlug}` : null,
    () => selectedSlug ? fetchWikiPage(selectedSlug) : null
  );

  const { data: overviewPage } = useSWR(
    !selectedSlug ? 'wiki-overview' : null,
    fetchOverviewPage
  );

  const displayPage: WikiPage | null = page || overviewPage || null;

  const handlePageChanged = useCallback(() => {
    mutateTree();
  }, [mutateTree]);

  const handleSaveConfig = async () => {
    const resp = await fetch('/api/ai/configs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ provider, model_name: model, api_key: apiKey, is_active: true }),
    });
    if (resp.ok) {
      setSaved(true);
      setApiKey('');
      setTimeout(() => setSaved(false), 3000);
    }
  };

  return (
    <div className="h-screen flex flex-col bg-th-bg-primary">
      {/* Header */}
      <header className="bg-th-bg-secondary border-b border-th-border h-12 flex items-center px-4 shrink-0">
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 bg-th-accent rounded-md flex items-center justify-center">
            <svg className="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 19.477 5.754 20 7.5 20s3.332-.477 4.5-1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 19.477 18.247 20 16.5 20a3.5 3.5 0 01-3.5-3.5" />
            </svg>
          </div>
          <h1 className="text-base font-semibold text-th-text-primary font-display">LLM Wiki</h1>
        </div>
        <div className="flex-1" />
        <div className="flex items-center gap-1">
          <button
            onClick={toggleTheme}
            className="p-1.5 rounded text-sm text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-tertiary transition-colors"
            title={theme === 'warm' ? '切换深色主题' : '切换暖色主题'}
          >
            {theme === 'warm' ? (
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
              </svg>
            ) : (
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
              </svg>
            )}
          </button>
          <button
            onClick={() => setShowSettings(!showSettings)}
            className={`p-1.5 rounded text-sm ${showSettings ? 'text-th-accent bg-th-accent-bg' : 'text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-tertiary'}`}
            title="设置"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-1.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
          </button>
          <button
            onClick={() => setLeftCollapsed(!leftCollapsed)}
            className={`p-1.5 rounded text-sm ${leftCollapsed ? 'text-th-accent bg-th-accent-bg' : 'text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-tertiary'}`}
            title={leftCollapsed ? '展开知识树' : '收起知识树'}
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h7" />
            </svg>
          </button>
          <button
            onClick={() => setRightCollapsed(!rightCollapsed)}
            className={`p-1.5 rounded text-sm ${rightCollapsed ? 'text-th-accent bg-th-accent-bg' : 'text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-tertiary'}`}
            title={rightCollapsed ? '展开页面' : '收起页面'}
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
          </button>
        </div>
      </header>

      {/* Main content - three columns */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left: Knowledge Tree */}
        <div
          style={{ width: leftCollapsed ? 0 : leftWidth, minWidth: leftCollapsed ? 0 : leftWidth }}
          className="shrink-0 overflow-hidden border-r border-th-border transition-all duration-200"
        >
          <KnowledgeTree
            tree={tree || []}
            selectedSlug={selectedSlug}
            onSelect={setSelectedSlug}
            collapsed={leftCollapsed}
          />
        </div>

        {/* Center: Chat */}
        <div className="flex-1 min-w-0">
          <ChatPanel onPageChanged={handlePageChanged} />
        </div>

        {/* Right: Page Viewer */}
        <div
          style={{ width: rightCollapsed ? 0 : rightWidth, minWidth: rightCollapsed ? 0 : rightWidth }}
          className="shrink-0 overflow-hidden border-l border-th-border transition-all duration-200"
        >
          <PageViewer page={displayPage} collapsed={rightCollapsed} />
        </div>
      </div>

      {/* Settings Modal */}
      {showSettings && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowSettings(false)}>
          <div className="bg-th-bg-secondary rounded-lg shadow-th shadow-xl w-full max-w-md mx-4 p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-th-text-primary">AI 模型配置</h2>
              <button onClick={() => setShowSettings(false)} className="text-th-text-muted hover:text-th-text-secondary">
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-th-text-secondary mb-1">Provider</label>
                <select
                  value={provider}
                  onChange={(e) => {
                    setProvider(e.target.value);
                    setModel(e.target.value === 'deepseek' ? 'deepseek-v4-flash' : 'claude-sonnet-4-7-20250514');
                  }}
                  className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm"
                >
                  <option value="claude">Claude</option>
                  <option value="deepseek">DeepSeek</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-th-text-secondary mb-1">Model</label>
                <select
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm"
                >
                  {provider === 'claude' ? (
                    <>
                      <option value="claude-sonnet-4-7-20250514">Claude Sonnet 4.7</option>
                      <option value="claude-opus-4-7-20250514">Claude Opus 4.7</option>
                      <option value="claude-haiku-4-5-20250501">Claude Haiku 4.5</option>
                    </>
                  ) : (
                    <>
                      <option value="deepseek-v4-flash">DeepSeek V4 Flash</option>
                      <option value="deepseek-chat">DeepSeek Chat</option>
                    </>
                  )}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-th-text-secondary mb-1">API Key</label>
                <input
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder={provider === 'deepseek' ? 'sk-...' : 'sk-ant-...'}
                  className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm"
                />
              </div>
              <button
                onClick={handleSaveConfig}
                disabled={!apiKey.trim()}
                className="w-full px-4 py-2 bg-th-accent text-white rounded-md hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
              >
                保存配置
              </button>
              {saved && <p className="text-th-success text-sm text-center">配置已保存</p>}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/WikiPage.tsx
git commit -m "feat: refactor WikiPage with theme-aware classes and toggle button"
```

---

### Task 8: Refactor KnowledgeTree.tsx to use theme classes

**Files:**
- Modify: `frontend/src/components/KnowledgeTree.tsx`

- [ ] **Step 1: Replace hardcoded color classes with theme classes**

Class mappings:
- `bg-gray-50` → `bg-th-bg-tertiary`
- `text-gray-500` → `text-th-text-muted`
- `text-gray-400` → `text-th-text-muted`
- `hover:bg-gray-100` → `hover:bg-th-bg-tertiary`
- `text-green-600` → `text-th-success`
- `text-yellow-600` → `text-th-warning`
- `bg-blue-100 text-blue-700` → `bg-th-accent-bg text-th-accent`
- `text-gray-600` → `text-th-text-secondary`

Full updated component:

```tsx
import { useState } from 'react';
import type { WikiTreeNode } from '../types';

interface KnowledgeTreeProps {
  tree: WikiTreeNode[];
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  collapsed: boolean;
}

export function KnowledgeTree({ tree, selectedSlug, onSelect, collapsed }: KnowledgeTreeProps) {
  if (collapsed) return null;

  return (
    <div className="h-full overflow-y-auto p-4 bg-th-bg-tertiary custom-scroll">
      <div className="text-sm font-medium text-th-text-muted mb-3">知识库</div>
      <div className="space-y-0.5">
        {tree.map((node) => (
          <TreeNode
            key={node.id}
            node={node}
            selectedSlug={selectedSlug}
            onSelect={onSelect}
            depth={0}
          />
        ))}
      </div>
      {tree.length === 0 && (
        <div className="text-center text-th-text-muted mt-8 text-sm">
          知识库为空，开始和 AI 对话吧
        </div>
      )}
    </div>
  );
}

interface TreeNodeProps {
  node: WikiTreeNode;
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  depth: number;
}

function TreeNode({ node, selectedSlug, onSelect, depth }: TreeNodeProps) {
  const [expanded, setExpanded] = useState(depth < 2);
  const hasChildren = node.children && node.children.length > 0;
  const isSelected = node.slug === selectedSlug;

  const statusColor = {
    published: 'bg-th-node-filled',
    draft: 'bg-th-node-partial',
    empty: 'bg-th-node-empty',
  }[node.content_status] || 'bg-th-node-empty';

  const handleClick = () => {
    onSelect(node.slug);
  };

  const handleToggle = (e: React.MouseEvent) => {
    e.stopPropagation();
    setExpanded(!expanded);
  };

  return (
    <div>
      <div
        className={`flex items-center gap-1.5 px-2 py-1.5 rounded-md cursor-pointer hover:bg-th-bg-tertiary transition-colors ${
          isSelected ? 'bg-th-accent-bg text-th-accent' : ''
        }`}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        onClick={handleClick}
      >
        {hasChildren ? (
          <button
            onClick={handleToggle}
            className="w-4 h-4 flex items-center justify-center text-th-text-muted hover:text-th-text-secondary shrink-0"
          >
            <svg className={`w-3 h-3 transition-transform ${expanded ? 'rotate-90' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </button>
        ) : (
          <span className="w-4 shrink-0" />
        )}
        <span className={`w-2 h-2 rounded-full ${statusColor} shrink-0`} />
        <span className={`flex-1 text-sm truncate ${isSelected ? 'font-medium' : 'text-th-text-primary'}`}>
          {node.title}
        </span>
        {node.page_type === 'overview' && (
          <span className="text-xs text-th-text-muted shrink-0">概览</span>
        )}
      </div>
      {expanded && hasChildren && (
        <div>
          {node.children!.map((child) => (
            <TreeNode
              key={child.id}
              node={child}
              selectedSlug={selectedSlug}
              onSelect={onSelect}
              depth={depth + 1}
            />
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/KnowledgeTree.tsx
git commit -m "feat: refactor KnowledgeTree with theme-aware classes and status dots"
```

---

### Task 9: Refactor ChatPanel.tsx to use theme classes

**Files:**
- Modify: `frontend/src/components/ChatPanel.tsx`

- [ ] **Step 1: Replace hardcoded color classes with theme classes**

Class mappings:
- `bg-white` → `bg-th-bg-secondary`
- `border-gray-200` → `border-th-border`
- `bg-gray-50` → `bg-th-bg-tertiary`
- `bg-blue-50` → `bg-th-accent-bg`
- `text-gray-500` → `text-th-text-muted`
- `text-gray-800` → `text-th-text-primary`
- `bg-blue-500 text-white` → `bg-th-user-bubble text-th-user-bubble-text`
- `bg-gray-100 text-gray-800` → `bg-th-assistant-bubble text-th-assistant-bubble-text`
- `border-gray-300` → `border-th-input-border`
- `focus:ring-blue-400` → `focus:ring-th-accent`
- `bg-blue-500 hover:bg-blue-600` → `bg-th-accent hover:opacity-90`
- `bg-yellow-50 border-yellow-200` → `bg-th-accent-bg border-th-accent`
- `bg-green-500 hover:bg-green-600` → `bg-th-success hover:opacity-90`
- Select/inputs `bg-white` → `bg-th-input-bg`
- `text-gray-400` → `text-th-text-muted`

Full updated component — replace entire file content, keeping all logic identical:

```tsx
import { useState, useEffect, useRef, useCallback } from "react";
import type { Conversation, ConversationMessage, PendingAction } from "../types";
import {
  listConversations,
  createConversation,
  updateConversationTitle,
  deleteConversation,
  getConversationMessages,
  streamChat,
} from "../lib/api";
import { MarkdownContent } from "./MarkdownContent";

const STORAGE_KEY = "llm-wiki-active-conversation-id";

interface ChatPanelProps {
  onPageChanged?: () => void;
}

export function ChatPanel({ onPageChanged }: ChatPanelProps) {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConv, setActiveConv] = useState<Conversation | null>(null);
  const [messages, setMessages] = useState<ConversationMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [showNewDialog, setShowNewDialog] = useState(false);
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState("");

  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  useEffect(() => {
    loadConversations();
  }, []);

  useEffect(() => {
    if (conversations.length === 0) return;
    const storedId = localStorage.getItem(STORAGE_KEY);
    if (storedId) {
      const id = parseInt(storedId, 10);
      const conv = conversations.find((c) => c.id === id);
      if (conv) {
        switchToConversation(conv);
      } else {
        localStorage.removeItem(STORAGE_KEY);
      }
    }
  }, [conversations.length > 0]);

  async function loadConversations() {
    try {
      const convs = await listConversations();
      setConversations(convs || []);
    } catch {
      // ignore
    }
  }

  async function switchToConversation(conv: Conversation) {
    setActiveConv(conv);
    localStorage.setItem(STORAGE_KEY, String(conv.id));
    try {
      const msgs = await getConversationMessages(conv.id);
      setMessages(msgs || []);
    } catch {
      setMessages([]);
    }
  }

  async function handleCreateConversation(title?: string) {
    try {
      const conv = await createConversation(title);
      await loadConversations();
      await switchToConversation(conv);
      setShowNewDialog(false);
      setTitleDraft("");
    } catch (e) {
      console.error("Failed to create conversation:", e);
    }
  }

  async function handleDeleteConversation() {
    if (!activeConv) return;
    try {
      await deleteConversation(activeConv.id);
      localStorage.removeItem(STORAGE_KEY);
      setActiveConv(null);
      setMessages([]);
      await loadConversations();
    } catch (e) {
      console.error("Failed to delete conversation:", e);
    }
  }

  async function handleRenameTitle() {
    if (!activeConv || !titleDraft.trim()) return;
    try {
      await updateConversationTitle(activeConv.id, titleDraft.trim());
      setActiveConv({ ...activeConv, title: titleDraft.trim() });
      setEditingTitle(false);
      await loadConversations();
    } catch (e) {
      console.error("Failed to update title:", e);
    }
  }

  async function handleSend(confirmedActions?: PendingAction[]) {
    if (!activeConv || loading) return;

    const userContent = input.trim();
    if (!userContent && !confirmedActions) return;

    if (userContent) {
      const userMsg: ConversationMessage = {
        id: Date.now(),
        role: "user",
        content: userContent,
        model_provider: null,
        token_count: null,
        created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, userMsg]);
      setInput("");
    }

    setLoading(true);

    const assistantMsg: ConversationMessage = {
      id: Date.now() + 1,
      role: "assistant",
      content: "",
      model_provider: null,
      token_count: null,
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, assistantMsg]);

    try {
      let fullContent = "";
      let newConvId: number | undefined;

      await streamChat(
        {
          conversation_id: activeConv.id,
          message: userContent,
          role: "wiki_maintainer",
          context_type: "wiki",
          confirmed_actions: confirmedActions,
        },
        (content) => {
          fullContent += content;
          setMessages((prev) => {
            const updated = [...prev];
            const last = updated[updated.length - 1];
            if (last.role === "assistant") {
              updated[updated.length - 1] = { ...last, content: fullContent };
            }
            return updated;
          });
        },
        (meta) => {
          if (meta.conversation_id) {
            newConvId = meta.conversation_id;
          }
          if (meta.pending_actions) {
            setMessages((prev) => {
              const updated = [...prev];
              const last = updated[updated.length - 1];
              if (last.role === "assistant") {
                updated[updated.length - 1] = { ...last, pending_actions: meta.pending_actions };
              }
              return updated;
            });
          }
        },
      );

      if (newConvId && newConvId !== activeConv.id) {
        setActiveConv((prev) => (prev ? { ...prev, id: newConvId! } : prev));
        localStorage.setItem(STORAGE_KEY, String(newConvId));
        await loadConversations();
      }

      if (confirmedActions && confirmedActions.length > 0) {
        onPageChanged?.();
      }
    } catch (e) {
      setMessages((prev) => {
        const updated = [...prev];
        updated[updated.length - 1] = {
          ...updated[updated.length - 1],
          content: `Error: ${e}`,
        };
        return updated;
      });
    } finally {
      setLoading(false);
    }
  }

  function handleConfirm(actions: PendingAction[]) {
    handleSend(actions);
  }

  return (
    <div className="flex flex-col h-full bg-th-bg-secondary">
      {/* Header */}
      <div className="flex items-center gap-2 p-3 border-b border-th-border bg-th-bg-tertiary shrink-0">
        <select
          className="flex-1 text-sm border border-th-input-border rounded px-2 py-1.5 bg-th-input-bg text-th-text-primary"
          value={activeConv?.id ?? ""}
          onChange={(e) => {
            const conv = conversations.find((c) => c.id === Number(e.target.value));
            if (conv) switchToConversation(conv);
          }}
        >
          <option value="" disabled>
            选择会话...
          </option>
          {conversations.map((c) => (
            <option key={c.id} value={c.id}>
              {c.title || `会话 ${c.id}`} ({c.message_count})
            </option>
          ))}
        </select>
        <button
          onClick={() => setShowNewDialog(true)}
          className="p-1.5 text-th-text-muted hover:text-th-accent hover:bg-th-accent-bg rounded"
          title="新建会话"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
        </button>
        {activeConv && (
          <>
            <button
              onClick={() => {
                setTitleDraft(activeConv.title || "");
                setEditingTitle(true);
              }}
              className="p-1.5 text-th-text-muted hover:text-th-success hover:bg-green-50 rounded"
              title="重命名"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
              </svg>
            </button>
            <button
              onClick={handleDeleteConversation}
              className="p-1.5 text-th-text-muted hover:text-th-error hover:bg-red-50 rounded"
              title="删除会话"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </>
        )}
      </div>

      {/* Title edit */}
      {editingTitle && activeConv && (
        <div className="flex items-center gap-2 px-3 py-2 border-b border-th-border bg-th-accent-bg shrink-0">
          <input
            className="flex-1 text-sm border border-th-input-border bg-th-input-bg text-th-text-primary rounded px-2 py-1"
            value={titleDraft}
            onChange={(e) => setTitleDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleRenameTitle();
              if (e.key === "Escape") setEditingTitle(false);
            }}
            autoFocus
          />
          <button onClick={handleRenameTitle} className="text-xs bg-th-accent text-white px-2 py-1 rounded">
            保存
          </button>
          <button onClick={() => setEditingTitle(false)} className="text-xs bg-th-bg-tertiary text-th-text-secondary px-2 py-1 rounded">
            取消
          </button>
        </div>
      )}

      {/* New conversation dialog */}
      {showNewDialog && (
        <div className="p-4 border-b border-th-border bg-th-accent-bg space-y-3 shrink-0">
          <div className="text-sm font-medium text-th-text-secondary">新建会话</div>
          <input
            className="w-full text-sm border border-th-input-border bg-th-input-bg text-th-text-primary rounded px-2 py-1.5"
            placeholder="给会话起个名字（可选）"
            value={titleDraft}
            onChange={(e) => setTitleDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleCreateConversation(titleDraft || undefined);
              if (e.key === "Escape") { setShowNewDialog(false); setTitleDraft(""); }
            }}
            autoFocus
          />
          <div className="flex justify-end gap-2">
            <button
              onClick={() => { setShowNewDialog(false); setTitleDraft(""); }}
              className="text-sm px-3 py-1.5 bg-th-bg-tertiary text-th-text-secondary rounded hover:bg-th-bg-primary"
            >
              取消
            </button>
            <button
              onClick={() => handleCreateConversation(titleDraft || undefined)}
              className="text-sm px-3 py-1.5 bg-th-accent text-white rounded hover:opacity-90"
            >
              创建
            </button>
          </div>
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4 custom-scroll">
        {!activeConv && (
          <div className="text-center text-th-text-muted mt-10">
            <p className="text-lg mb-2">没有活动会话</p>
            <p className="text-sm">点击 + 新建一个 AI 对话</p>
          </div>
        )}
        {messages.map((msg, i) => (
          <div key={msg.id || i} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
            <div
              className={`max-w-[85%] rounded-lg px-3 py-2 text-sm ${
                msg.role === "user"
                  ? "bg-th-user-bubble text-th-user-bubble-text"
                  : "bg-th-assistant-bubble text-th-assistant-bubble-text"
              }`}
            >
              {msg.role === "assistant" ? (
                <MarkdownContent content={msg.content} />
              ) : (
                <span className="whitespace-pre-wrap">{msg.content}</span>
              )}
              {msg.pending_actions && msg.pending_actions.length > 0 && (
                <div className="mt-2 pt-2 border-t border-th-border space-y-1">
                  <div className="text-xs text-th-text-muted font-medium">待确认操作：</div>
                  {msg.pending_actions.map((action, j) => (
                    <div key={j} className="text-xs bg-th-accent-bg border border-th-accent p-2 rounded">
                      {action.preview}
                    </div>
                  ))}
                  <button
                    onClick={() => handleConfirm(msg.pending_actions!)}
                    className="mt-1 text-xs bg-th-success text-white px-3 py-1 rounded hover:opacity-90"
                  >
                    确认执行
                  </button>
                </div>
              )}
            </div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="p-3 border-t border-th-border shrink-0">
        <div className="flex gap-2">
          <input
            type="text"
            className="flex-1 border border-th-input-border bg-th-input-bg text-th-text-primary rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent"
            placeholder={activeConv ? "输入消息..." : "请先选择或新建会话"}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                handleSend();
              }
            }}
            disabled={!activeConv || loading}
          />
          <button
            onClick={() => handleSend()}
            disabled={!activeConv || loading || !input.trim()}
            className="px-4 py-2 rounded-lg text-sm font-medium text-white bg-th-accent hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            发送
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/ChatPanel.tsx
git commit -m "feat: refactor ChatPanel with theme-aware classes"
```

---

### Task 10: Refactor PageViewer.tsx to use theme classes

**Files:**
- Modify: `frontend/src/components/PageViewer.tsx`

- [ ] **Step 1: Replace hardcoded color classes with theme classes**

Full updated component:

```tsx
import type { WikiPage } from '../types';
import { MarkdownContent } from './MarkdownContent';

interface PageViewerProps {
  page: WikiPage | null;
  collapsed: boolean;
}

const STATUS_STYLES: Record<string, { bg: string; text: string; border: string; dot: string; label: string }> = {
  published: {
    bg: 'bg-[var(--badge-published-bg)]',
    text: 'text-[var(--badge-published-text)]',
    border: 'border-[var(--badge-published-border)]',
    dot: 'bg-[var(--badge-published-dot)]',
    label: '已填充',
  },
  draft: {
    bg: 'bg-[var(--badge-draft-bg)]',
    text: 'text-[var(--badge-draft-text)]',
    border: 'border-[var(--badge-draft-border)]',
    dot: 'bg-[var(--badge-draft-dot)]',
    label: '草稿',
  },
  empty: {
    bg: 'bg-[var(--badge-empty-bg)]',
    text: 'text-[var(--badge-empty-text)]',
    border: 'border-[var(--badge-empty-border)]',
    dot: 'bg-[var(--badge-empty-dot)]',
    label: '空',
  },
};

export function PageViewer({ page, collapsed }: PageViewerProps) {
  if (collapsed) return null;

  if (!page) {
    return (
      <div className="h-full flex items-center justify-center bg-th-bg-secondary">
        <div className="text-center text-th-text-muted">
          <p className="text-lg mb-2">选择一个页面</p>
          <p className="text-sm">点击左侧知识树查看内容</p>
        </div>
      </div>
    );
  }

  const status = STATUS_STYLES[page.content_status] || STATUS_STYLES.empty;

  return (
    <div className="h-full overflow-y-auto bg-th-bg-secondary custom-scroll">
      <div className="p-6 max-w-3xl">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-th-text-primary font-display">{page.title}</h1>
          <div className="flex items-center gap-2 mt-2 text-sm">
            <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border ${status.bg} ${status.text} ${status.border}`}>
              <span className={`w-1.5 h-1.5 rounded-full mr-1 ${status.dot}`} />
              {status.label}
            </span>
            {page.page_type !== 'entity' && (
              <span className="px-2 py-0.5 rounded bg-th-accent-bg text-th-accent text-xs">
                {page.page_type}
              </span>
            )}
          </div>
        </div>
        <MarkdownContent content={page.content} />
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/PageViewer.tsx
git commit -m "feat: refactor PageViewer with theme-aware classes and status badges"
```

---

### Task 11: Update MarkdownContent.tsx for dark theme code highlighting

**Files:**
- Modify: `frontend/src/components/MarkdownContent.tsx`

- [ ] **Step 1: Add dark theme syntax highlighting support**

Add `oneDark` import and use `useTheme` to switch styles. Also update inline hardcoded colors to use CSS variables.

Full updated component:

```tsx
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { PrismLight as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneLight } from 'react-syntax-highlighter/dist/cjs/styles/prism'
import { oneDark } from 'react-syntax-highlighter/dist/cjs/styles/prism'
import python from 'react-syntax-highlighter/dist/cjs/languages/prism/python'
import javascript from 'react-syntax-highlighter/dist/cjs/languages/prism/javascript'
import typescript from 'react-syntax-highlighter/dist/cjs/languages/prism/typescript'
import go from 'react-syntax-highlighter/dist/cjs/languages/prism/go'
import java from 'react-syntax-highlighter/dist/cjs/languages/prism/java'
import cpp from 'react-syntax-highlighter/dist/cjs/languages/prism/cpp'
import sql from 'react-syntax-highlighter/dist/cjs/languages/prism/sql'
import { useTheme } from '../contexts/ThemeContext'

SyntaxHighlighter.registerLanguage('python', python)
SyntaxHighlighter.registerLanguage('javascript', javascript)
SyntaxHighlighter.registerLanguage('typescript', typescript)
SyntaxHighlighter.registerLanguage('go', go)
SyntaxHighlighter.registerLanguage('java', java)
SyntaxHighlighter.registerLanguage('cpp', cpp)
SyntaxHighlighter.registerLanguage('c', cpp)
SyntaxHighlighter.registerLanguage('sql', sql)

interface MarkdownContentProps {
  content: string
  className?: string
}

export function MarkdownContent({ content, className = '' }: MarkdownContentProps) {
  const { theme } = useTheme()
  const syntaxStyle = theme === 'dark' ? oneDark : oneLight

  if (!content) return null
  return (
    <div className={`prose prose-sm max-w-none ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          code({ className, children, ...props }) {
            const match = /language-(\w+)/.exec(className || '')
            if (!match) {
              return (
                <code className="bg-th-bg-tertiary text-th-accent px-1 py-0.5 rounded text-sm" {...props}>
                  {children}
                </code>
              )
            }
            return (
              <SyntaxHighlighter
                style={syntaxStyle}
                language={match[1]}
                PreTag="div"
                className="rounded-md !mt-2 !mb-2"
              >
                {String(children).replace(/\n$/, '')}
              </SyntaxHighlighter>
            )
          },
          table({ children }) {
            return (
              <div className="overflow-x-auto">
                <table className="min-w-full border-collapse border border-th-border">{children}</table>
              </div>
            )
          },
          th({ children }) {
            return <th className="border border-th-border bg-th-bg-tertiary px-3 py-1.5 text-left text-sm font-medium text-th-text-secondary">{children}</th>
          },
          td({ children }) {
            return <td className="border border-th-border px-3 py-1.5 text-sm text-th-text-primary">{children}</td>
          },
          blockquote({ children }) {
            return <blockquote className="border-l-4 border-th-accent bg-th-accent-bg pl-4 py-1 my-2 text-sm text-th-text-secondary">{children}</blockquote>
          },
          h1({ children }) {
            return <h1 className="text-xl font-bold mt-4 mb-2 text-th-text-primary">{children}</h1>
          },
          h2({ children }) {
            return <h2 className="text-lg font-bold mt-3 mb-1.5 text-th-text-primary">{children}</h2>
          },
          h3({ children }) {
            return <h3 className="text-base font-semibold mt-2 mb-1 text-th-text-primary">{children}</h3>
          },
          ul({ children }) {
            return <ul className="list-disc list-inside space-y-0.5 my-1 text-th-text-primary">{children}</ul>
          },
          ol({ children }) {
            return <ol className="list-decimal list-inside space-y-0.5 my-1 text-th-text-primary">{children}</ol>
          },
          p({ children }) {
            return <p className="my-1.5 leading-relaxed text-th-text-secondary">{children}</p>
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/MarkdownContent.tsx
git commit -m "feat: add dark theme syntax highlighting to MarkdownContent"
```

---

### Task 12: Verify and fix any remaining issues

**Files:**
- All modified files

- [ ] **Step 1: Start the dev server and verify**

Run: `cd frontend && npm run dev`

Verify:
1. Page loads with Warm theme (default)
2. Theme toggle button in header switches to Dark theme
3. All components respond to theme change
4. Refresh preserves selected theme
5. Code blocks use correct highlighting per theme

- [ ] **Step 2: Fix any visual issues found**

Check for:
- Hardcoded colors that were missed
- Select/option elements that don't inherit theme colors
- Any remaining `bg-white`, `text-gray-*`, `border-gray-*` classes in modified files

- [ ] **Step 3: Final commit**

```bash
git add -A
git commit -m "fix: address remaining theme issues after refactor"
```
