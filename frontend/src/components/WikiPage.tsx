import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import useSWR from 'swr';
import { Group, Panel, Separator, type PanelImperativeHandle } from 'react-resizable-panels';
import { fetchWikiTree, fetchWikiPage, fetchOverviewPage, createEmptyWikiPage, renameWikiPage, moveWikiPage, deleteWikiPage } from '../lib/api';
import { useTheme } from '../contexts/ThemeContext';
import type { WikiPage, WikiTreeNode } from '../types';
import { KnowledgeTree } from './KnowledgeTree';
import { ChatPanel } from './ChatPanel';
import { PageViewer } from './PageViewer';
import { TabbedPageReview } from './TabbedPageReview';
import { BrandMark } from './BrandMark';

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
  const navigate = useNavigate();
  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);
  const leftPanelRef = useRef<PanelImperativeHandle>(null);
  const rightPanelRef = useRef<PanelImperativeHandle>(null);
  const chatPanelRef = useRef<{ setSelectedText: (text: string, pageTitle: string) => void; sendMessage: (text: string) => void; continueAfterConfirm: () => void }>(null);
  const [selectedSlug, setSelectedSlug] = useState<string | null>(() => localStorage.getItem('wiki-selected-slug'));
  const [newPageNodeId, setNewPageNodeId] = useState<number | null>(null);
  const [reviewSlugs, setReviewSlugs] = useState<string[]>([]);
  const { theme, toggleTheme } = useTheme();
  const [treeVersion, setTreeVersion] = useState(0);

  // Persist selected page across refreshes
  useEffect(() => {
    if (selectedSlug) localStorage.setItem('wiki-selected-slug', selectedSlug);
    else localStorage.removeItem('wiki-selected-slug');
  }, [selectedSlug]);

  const { data: tree } = useSWR(['wiki-tree', treeVersion], fetchWikiTree);
  const { data: page, mutate: mutatePage } = useSWR(
    selectedSlug ? `wiki-page-${selectedSlug}` : null,
    () => selectedSlug ? fetchWikiPage(selectedSlug) : null
  );

  const mutateCurrentPage = useCallback(() => {
    mutatePage();
  }, [mutatePage]);

  const { data: overviewPage, mutate: mutateOverview } = useSWR(
    !selectedSlug ? 'wiki-overview' : null,
    fetchOverviewPage
  );

  const displayPage: WikiPage | null = page || overviewPage || null;

  const selectedPageInfo = useMemo(() => {
    if (!selectedSlug || !tree) return { id: undefined, title: undefined };
    const findNode = (nodes: WikiTreeNode[]): { id: number; title: string } | undefined => {
      for (const n of nodes) {
        if (n.slug === selectedSlug) return { id: n.id, title: n.title };
        if (n.children) {
          const found = findNode(n.children);
          if (found) return found;
        }
      }
      return undefined;
    };
    const info = findNode(tree);
    return { id: info?.id, title: info?.title };
  }, [selectedSlug, tree]);

  const breadcrumb = useMemo(() => {
    if (!tree) return [] as { title: string; slug: string }[];
    const path: { title: string; slug: string }[] = [];
    const walk = (nodes: WikiTreeNode[], trail: { title: string; slug: string }[]): boolean => {
      for (const n of nodes) {
        const next = [...trail, { title: n.title, slug: n.slug }];
        if (n.slug === selectedSlug) {
          path.push(...next);
          return true;
        }
        if (n.children && walk(n.children, next)) return true;
      }
      return false;
    };
    walk(tree, []);
    return path;
  }, [selectedSlug, tree]);

  const handlePageChanged = useCallback(() => {
    setTreeVersion(v => v + 1);
    mutateOverview();
  }, [mutateOverview]);

  const handleContentConfirmed = useCallback((_pageId: number) => {
    mutateCurrentPage();
    handlePageChanged();
  }, [mutateCurrentPage, handlePageChanged]);

  const handleReviewDone = useCallback(() => {
    setReviewSlugs([]);
  }, []);


  const handleAskAI = useCallback((text: string, pageTitle: string) => {
    chatPanelRef.current?.setSelectedText(text, pageTitle);
  }, []);

  const handleAddChild = async (parentId: number | null) => {
    try {
      const page = await createEmptyWikiPage("新页面", parentId);
      setSelectedSlug(page.slug);
      setNewPageNodeId(page.id);
      handlePageChanged();
    } catch (err) {
      console.error("Failed to add child page:", err);
    }
  };

  const handleRename = async (nodeId: number, newTitle: string) => {
    try {
      await renameWikiPage(nodeId, newTitle);
      handlePageChanged();
      mutateCurrentPage();
    } catch (err) {
      console.error("Failed to rename page:", err);
    }
  };

  const handleMove = async (nodeId: number, newParentId: number | null) => {
    try {
      await moveWikiPage(nodeId, newParentId);
      handlePageChanged();
    } catch (err) {
      console.error("Failed to move page:", err);
    }
  };

  const handleAskAIMove = (nodeId: number) => {
    chatPanelRef.current?.setSelectedText(`请将页面 ID ${nodeId} 移动到合适的位置`, "");
  };

  const handleDelete = async (nodeId: number, _hasChildren: boolean) => {
    try {
      await deleteWikiPage(nodeId);
      handlePageChanged();
    } catch (err) {
      console.error("Failed to delete page:", err);
    }
  };

  return (
    <div className="h-screen flex flex-col bg-th-bg-primary">
      {/* Header */}
      <header className="bg-th-bg-secondary/70 backdrop-blur-md border-b border-th-separator h-14 flex items-center pl-4 pr-2 shrink-0">
        <BrandMark />
        <div className="flex-1" />
        <div className="flex items-center gap-0.5">
          <button
            onClick={() => {
              const panel = leftPanelRef.current;
              if (!panel) return;
              if (panel.isCollapsed()) panel.expand();
              else panel.collapse();
            }}
            className={`p-2 rounded-md transition-all duration-150 active:scale-90 ${
              leftCollapsed
                ? 'text-th-text-muted hover:text-th-text-primary hover:bg-th-hover'
                : 'text-th-accent bg-th-accent-bg'
            }`}
            title={leftCollapsed ? '展开知识树' : '收起知识树'}
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M3 7h18M3 12h12M3 17h7" />
            </svg>
          </button>
          <button
            onClick={() => {
              const panel = rightPanelRef.current;
              if (!panel) return;
              if (panel.isCollapsed()) panel.expand();
              else panel.collapse();
            }}
            className={`p-2 rounded-md transition-all duration-150 active:scale-90 ${
              rightCollapsed
                ? 'text-th-text-muted hover:text-th-text-primary hover:bg-th-hover'
                : 'text-th-accent bg-th-accent-bg'
            }`}
            title={rightCollapsed ? '展开页面' : '收起页面'}
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
          </button>
          <div className="w-px h-5 bg-th-separator mx-1.5" />
          <button
            onClick={toggleTheme}
            className="p-2 rounded-md text-th-text-muted hover:text-th-text-primary hover:bg-th-hover transition-all duration-150 active:scale-90"
            title={theme === 'warm' ? '切换深色主题' : '切换暖色主题'}
          >
            {theme === 'warm' ? (
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
              </svg>
            ) : (
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
              </svg>
            )}
          </button>
          <button
            onClick={() => navigate('/settings')}
            className="p-2 rounded-md text-th-text-muted hover:text-th-text-primary hover:bg-th-hover transition-all duration-150 active:scale-90"
            title="设置"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-1.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
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
              onAddChild={handleAddChild}
              onRename={handleRename}
              onMove={handleMove}
              onAskAIMove={handleAskAIMove}
              onDelete={handleDelete}
              newNodeId={newPageNodeId}
            />
          </div>
        </Panel>

        <Separator />

        {/* Center: Chat */}
        <Panel id="center" minSize={300}>
          <ChatPanel ref={chatPanelRef} focusPageId={displayPage?.id ?? null} currentSlug={selectedSlug ?? displayPage?.slug ?? undefined} currentPageTitle={selectedPageInfo.title ?? displayPage?.title ?? undefined} />
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
            {reviewSlugs.length > 0 ? (
              <TabbedPageReview
                slugs={reviewSlugs}
                onDone={handleReviewDone}
                onSelectPage={(slug) => setSelectedSlug(slug)}
                onContentConfirmed={handleContentConfirmed}
              />
            ) : (
              <PageViewer
                page={displayPage}
                collapsed={rightCollapsed}
                breadcrumb={breadcrumb}
                onViewPage={(slug) => setSelectedSlug(slug)}
                onSelectPage={(slug) => setSelectedSlug(slug)}
                onAskAI={handleAskAI}
                onContentConfirmed={handleContentConfirmed}
              />
            )}
          </div>
        </Panel>
      </Group>
    </div>
  );
}
