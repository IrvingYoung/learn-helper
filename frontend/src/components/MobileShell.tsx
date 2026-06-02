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
  onAskAI: _onAskAI,
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
