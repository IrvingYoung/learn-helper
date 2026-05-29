import { useState, useCallback } from 'react';
import useSWR from 'swr';
import { fetchWikiTree, fetchWikiPage, fetchOverviewPage } from '../lib/api';
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

  return (
    <div className="h-screen flex flex-col bg-gray-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 h-12 flex items-center px-4 shrink-0">
        <h1 className="text-base font-semibold text-gray-900">LLM Wiki</h1>
        <div className="flex-1" />
        <div className="flex items-center gap-1">
          <button
            onClick={() => setLeftCollapsed(!leftCollapsed)}
            className={`p-1.5 rounded text-sm ${leftCollapsed ? 'text-blue-600 bg-blue-50' : 'text-gray-500 hover:text-gray-700 hover:bg-gray-100'}`}
            title={leftCollapsed ? '展开知识树' : '收起知识树'}
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h7" />
            </svg>
          </button>
          <button
            onClick={() => setRightCollapsed(!rightCollapsed)}
            className={`p-1.5 rounded text-sm ${rightCollapsed ? 'text-blue-600 bg-blue-50' : 'text-gray-500 hover:text-gray-700 hover:bg-gray-100'}`}
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
          className="shrink-0 overflow-hidden border-r border-gray-200 transition-all duration-200"
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
          className="shrink-0 overflow-hidden border-l border-gray-200 transition-all duration-200"
        >
          <PageViewer page={displayPage} collapsed={rightCollapsed} />
        </div>
      </div>
    </div>
  );
}
