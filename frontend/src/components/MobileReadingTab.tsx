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
