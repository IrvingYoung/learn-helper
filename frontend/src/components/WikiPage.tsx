import { useState, useCallback, useRef } from 'react';
import useSWR from 'swr';
import { Group, Panel, Separator, type PanelImperativeHandle } from 'react-resizable-panels';
import { fetchWikiTree, fetchWikiPage, fetchOverviewPage } from '../lib/api';
import { useTheme } from '../contexts/ThemeContext';
import type { WikiPage } from '../types';
import { KnowledgeTree } from './KnowledgeTree';
import { ChatPanel } from './ChatPanel';
import { PageViewer } from './PageViewer';

const LAYOUT_KEY = 'wiki-layout';
const DEFAULT_LAYOUT: Record<string, number> = { left: 20, center: 50, right: 30 };

function loadLayout(): Record<string, number> | undefined {
  try {
    const raw = localStorage.getItem(LAYOUT_KEY);
    if (raw) return JSON.parse(raw);
  } catch { /* ignore */ }
}

function saveLayout(layout: Record<string, number>) {
  try {
    localStorage.setItem(LAYOUT_KEY, JSON.stringify(layout));
  } catch { /* ignore */ }
}

export function WikiPageLayout() {
  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);
  const leftPanelRef = useRef<PanelImperativeHandle>(null);
  const rightPanelRef = useRef<PanelImperativeHandle>(null);
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
      <header className="bg-th-bg-secondary/80 backdrop-blur-sm border-b border-th-border h-12 flex items-center px-4 shrink-0">
        <div className="flex items-center gap-2.5">
          <div className="w-7 h-7 bg-th-accent rounded-lg flex items-center justify-center shadow-sm">
            <svg className="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 19.477 5.754 20 7.5 20s3.332-.477 4.5-1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 19.477 18.247 20 16.5 20a3.5 3.5 0 01-3.5-3.5" />
            </svg>
          </div>
          <h1 className="text-base font-semibold text-th-text-primary font-display tracking-tight">LLM Wiki</h1>
        </div>
        <div className="flex-1" />
        <div className="flex items-center gap-0.5">
          <button
            onClick={toggleTheme}
            className="p-1.5 rounded text-sm text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-tertiary transition-all duration-150 active:scale-90"
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
            className={`p-1.5 rounded text-sm transition-colors ${showSettings ? 'text-th-accent bg-th-accent-bg' : 'text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-tertiary'}`}
            title="设置"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-1.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
          </button>
          <button
            onClick={() => {
              const panel = leftPanelRef.current;
              if (!panel) return;
              if (panel.isCollapsed()) panel.expand();
              else panel.collapse();
            }}
            className={`p-1.5 rounded text-sm transition-colors ${leftCollapsed ? 'text-th-accent bg-th-accent-bg' : 'text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-tertiary'}`}
            title={leftCollapsed ? '展开知识树' : '收起知识树'}
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h7" />
            </svg>
          </button>
          <button
            onClick={() => {
              const panel = rightPanelRef.current;
              if (!panel) return;
              if (panel.isCollapsed()) panel.expand();
              else panel.collapse();
            }}
            className={`p-1.5 rounded text-sm transition-colors ${rightCollapsed ? 'text-th-accent bg-th-accent-bg' : 'text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-tertiary'}`}
            title={rightCollapsed ? '展开页面' : '收起页面'}
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
          </button>
        </div>
      </header>

      {/* Main content - three columns */}
      <Group
        className="flex-1"
        orientation="horizontal"
        defaultLayout={loadLayout() || DEFAULT_LAYOUT}
        onLayoutChanged={saveLayout}
      >
        {/* Left: Knowledge Tree */}
        <Panel
          id="left"
          panelRef={leftPanelRef}
          minSize={150}
          collapsible
          collapsedSize={0}
          onResize={(size) => {
            if (size.asPercentage === 0 && !leftCollapsed) setLeftCollapsed(true);
            else if (size.asPercentage > 0 && leftCollapsed) setLeftCollapsed(false);
          }}
        >
          <div className="h-full overflow-hidden">
            <KnowledgeTree
              tree={tree || []}
              selectedSlug={selectedSlug}
              onSelect={setSelectedSlug}
              collapsed={leftCollapsed}
            />
          </div>
        </Panel>

        <Separator />

        {/* Center: Chat */}
        <Panel id="center" minSize={300}>
          <ChatPanel onPageChanged={handlePageChanged} />
        </Panel>

        <Separator />

        {/* Right: Page Viewer */}
        <Panel
          id="right"
          panelRef={rightPanelRef}
          minSize={200}
          collapsible
          collapsedSize={0}
          onResize={(size) => {
            if (size.asPercentage === 0 && !rightCollapsed) setRightCollapsed(true);
            else if (size.asPercentage > 0 && rightCollapsed) setRightCollapsed(false);
          }}
        >
          <div className="h-full overflow-hidden">
            <PageViewer page={displayPage} collapsed={rightCollapsed} />
          </div>
        </Panel>
      </Group>

      {/* Settings Modal */}
      {showSettings && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowSettings(false)}>
          <div className="bg-th-bg-secondary rounded-lg shadow-th-md w-full max-w-md mx-4 p-6" onClick={(e) => e.stopPropagation()}>
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
                  className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent"
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
                  className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent"
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
                  className="w-full border border-th-input-border bg-th-input-bg text-th-text-primary rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent"
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