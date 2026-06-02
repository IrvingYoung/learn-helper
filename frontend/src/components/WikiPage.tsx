import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import useSWR from 'swr';
import { Group, Panel, Separator, type PanelImperativeHandle } from 'react-resizable-panels';
import { fetchWikiTree, fetchWikiPage, fetchOverviewPage, createEmptyWikiPage, renameWikiPage, moveWikiPage, deleteWikiPage, fetchPublicSharePage } from '../lib/api';
import { useTheme } from '../contexts/ThemeContext';
import type { WikiPage, WikiTreeNode, ToolCallInfo } from '../types';
import { KnowledgeTree } from './KnowledgeTree';
import { ChatPanel } from './ChatPanel';
import { PageViewer } from './PageViewer';
import { TabbedPageReview } from './TabbedPageReview';
import { BrandMark } from './BrandMark';
import { useIsMobile } from '../lib/useIsMobile';
import { MobileShell } from './MobileShell';

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

interface WikiPageLayoutProps {
  /**
   * Slug from the URL route param (`/wiki/:slug` or `/share/:slug`).
   * When non-null, takes precedence over the localStorage-restored slug.
   * Drives the initial selected page and the back/forward sync.
   */
  urlSlug: string | null;
  /**
   * Share token from the URL query string on `/share/:slug?t=...`.
   * Ignored on `/wiki` routes.
   */
  shareTokenFromUrl: string | null;
}

export function WikiPageLayout({ urlSlug, shareTokenFromUrl }: WikiPageLayoutProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const isPublicPath = location.pathname.startsWith('/share/');

  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);
  const leftPanelRef = useRef<PanelImperativeHandle>(null);
  const rightPanelRef = useRef<PanelImperativeHandle>(null);
  const chatPanelRef = useRef<{ setSelectedText: (text: string, pageTitle: string) => void; sendMessage: (text: string) => void; continueAfterConfirm: () => void }>(null);
  const [selectedSlug, setSelectedSlug] = useState<string | null>(() => urlSlug ?? localStorage.getItem('wiki-selected-slug'));
  const [newPageNodeId, setNewPageNodeId] = useState<number | null>(null);
  const [reviewSlugs, setReviewSlugs] = useState<string[]>([]);
  const { theme, toggleTheme } = useTheme();
  const [treeVersion, setTreeVersion] = useState(0);
  const isMobile = useIsMobile();

  // Persist selected page across refreshes (owner only — public visitors don't
  // have a stable localStorage contract).
  useEffect(() => {
    if (isPublicPath) return;
    if (selectedSlug) localStorage.setItem('wiki-selected-slug', selectedSlug);
    else localStorage.removeItem('wiki-selected-slug');
  }, [selectedSlug, isPublicPath]);

  // Sync URL on state change so the address bar always reflects the current
  // page. On /share/... paths we never push (the URL is the entry point
  // and contains the token).
  useEffect(() => {
    if (isPublicPath) return;
    const target = selectedSlug ? `/wiki/${selectedSlug}` : '/wiki';
    if (location.pathname !== target) {
      navigate(target, { replace: false });
    }
  }, [selectedSlug, isPublicPath, location.pathname, navigate]);

  // Sync state when the URL slug changes (back/forward, deep link).
  useEffect(() => {
    if (urlSlug && urlSlug !== selectedSlug) {
      setSelectedSlug(urlSlug);
    }
  }, [urlSlug]); // eslint-disable-line react-hooks/exhaustive-deps

  // Tree is owner-only. In public mode we skip fetching it (avoids leaking
  // page metadata to anonymous visitors and saves a network round-trip).
  const { data: tree } = useSWR(
    isPublicPath ? null : ['wiki-tree', treeVersion],
    fetchWikiTree
  );

  // Page fetching — branch on public vs owner path.
  // Public: use the public API with the share token from the URL.
  // Owner: use the standard /api/wiki/{slug} endpoint.
  const pageFetcher = useCallback(() => {
    if (!selectedSlug) return null;
    if (isPublicPath) {
      if (!shareTokenFromUrl) return null;
      return fetchPublicSharePage(selectedSlug, shareTokenFromUrl);
    }
    return fetchWikiPage(selectedSlug);
  }, [selectedSlug, isPublicPath, shareTokenFromUrl]);

  const { data: page, mutate: mutatePage } = useSWR(
    selectedSlug ? `wiki-page-${selectedSlug}-${isPublicPath ? 'public' : 'owner'}` : null,
    pageFetcher
  );

  const mutateCurrentPage = useCallback(() => {
    mutatePage();
  }, [mutatePage]);

  const { data: overviewPage, mutate: mutateOverview } = useSWR(
    !isPublicPath && !selectedSlug ? 'wiki-overview' : null,
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

  const slugIndex = useMemo(() => {
    const titleToSlug = new Map<string, string>();
    const slugSet = new Set<string>();
    const walk = (nodes: WikiTreeNode[]) => {
      for (const n of nodes) {
        titleToSlug.set(n.title, n.slug);
        slugSet.add(n.slug);
        if (n.children) walk(n.children);
      }
    };
    if (tree) walk(tree);
    return { titleToSlug, slugSet };
  }, [tree]);

  // Internal link navigation is owner-only. Public visitors see wiki
  // [[links]] rendered as plain text (the MarkdownContent component already
  // handles this when onInternalLink is undefined).
  const handleInternalLink = useCallback((href: string) => {
    let target = href;
    let lookupByTitle = false;
    if (target.startsWith('wiki:')) {
      target = target.slice(5);
      lookupByTitle = true;
    }
    try {
      target = decodeURIComponent(target);
    } catch { /* keep as-is */ }
    target = target.replace(/^\/+/, '').split(/[?#]/)[0];
    if (!target) return;

    if (lookupByTitle) {
      const slug = slugIndex.titleToSlug.get(target);
      if (slug) { setSelectedSlug(slug); return; }
    }
    if (slugIndex.slugSet.has(target)) {
      setSelectedSlug(target);
      return;
    }
    const slugByTitle = slugIndex.titleToSlug.get(target);
    if (slugByTitle) setSelectedSlug(slugByTitle);
  }, [slugIndex]);

  const handlePageChanged = useCallback((tc?: ToolCallInfo) => {
    if (isPublicPath) return;
    setTreeVersion(v => v + 1);
    mutateOverview();
    if (tc?.name === "create_page" && tc.output) {
      try {
        const result = JSON.parse(tc.output);
        if (typeof result.slug === "string" && result.slug) {
          setSelectedSlug(result.slug);
          return;
        }
      } catch { /* fall through to mutate current page */ }
    }
    mutateCurrentPage();
  }, [mutateCurrentPage, mutateOverview, isPublicPath]);

  const handleContentConfirmed = useCallback((_pageId: number) => {
    if (isPublicPath) return;
    mutateCurrentPage();
    handlePageChanged();
  }, [mutateCurrentPage, handlePageChanged, isPublicPath]);

  const handleReviewDone = useCallback(() => {
    setReviewSlugs([]);
  }, []);


  const handleAskAI = useCallback((text: string, pageTitle: string) => {
    if (isPublicPath) return;
    chatPanelRef.current?.setSelectedText(text, pageTitle);
  }, [isPublicPath]);

  const handleAddChild = async (parentId: number | null) => {
    if (isPublicPath) return;
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
    if (isPublicPath) return;
    try {
      await renameWikiPage(nodeId, newTitle);
      handlePageChanged();
      mutateCurrentPage();
    } catch (err) {
      console.error("Failed to rename page:", err);
    }
  };

  const handleMove = async (nodeId: number, newParentId: number | null) => {
    if (isPublicPath) return;
    try {
      await moveWikiPage(nodeId, newParentId);
      handlePageChanged();
    } catch (err) {
      console.error("Failed to move page:", err);
    }
  };

  const handleAskAIMove = (nodeId: number) => {
    if (isPublicPath) return;
    chatPanelRef.current?.setSelectedText(`请将页面 ID ${nodeId} 移动到合适的位置`, "");
  };

  const handleDelete = async (nodeId: number, _hasChildren: boolean) => {
    if (isPublicPath) return;
    try {
      await deleteWikiPage(nodeId);
      handlePageChanged();
    } catch (err) {
      console.error("Failed to delete wiki page:", err);
    }
  };

  // Public visitors see a stripped layout: header with brand + "open in app"
  // link, then the page viewer takes the full width below. No tree, no chat,
  // no settings/cron/theme buttons (no need to render a full app shell for
  // read-only anonymous viewers).
  if (isPublicPath) {
    return (
      <div className="h-screen flex flex-col bg-th-bg-primary">
        <header className="bg-th-bg-secondary/70 backdrop-blur-md border-b border-th-separator h-12 flex items-center pl-4 pr-4 shrink-0">
          <BrandMark />
          <div className="flex-1" />
          {selectedSlug && (
            <a
              href={`/wiki/${selectedSlug}`}
              className="text-xs text-th-text-secondary hover:text-th-text-primary transition-colors inline-flex items-center gap-1"
              title="Open in the LLM Wiki app"
            >
              在 LLM Wiki 中打开
              <svg className="w-3 h-3" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
              </svg>
            </a>
          )}
        </header>
        <div className="flex-1 min-h-0 overflow-hidden @container">
          <PageViewer
            page={displayPage}
            collapsed={false}
            breadcrumb={[]}
            onSelectPage={() => {}}
            publicMode={true}
          />
        </div>
      </div>
    );
  }

  if (isMobile) {
    return (
      <MobileShell
        tree={tree || []}
        selectedSlug={selectedSlug}
        onSelectSlug={setSelectedSlug}
        displayPage={displayPage}
        breadcrumb={breadcrumb}
        onInternalLink={handleInternalLink}
        onContentConfirmed={handleContentConfirmed}
        onWriteToolComplete={handlePageChanged}
        focusPageId={displayPage?.id ?? null}
        currentSlug={selectedSlug ?? displayPage?.slug ?? undefined}
        currentPageTitle={selectedPageInfo.title ?? displayPage?.title ?? undefined}
      />
    );
  }

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
            onClick={() => navigate('/cron')}
            className="p-2 rounded-md text-th-text-muted hover:text-th-text-primary hover:bg-th-hover transition-all duration-150 active:scale-90"
            title="定时任务"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
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
          <ChatPanel ref={chatPanelRef} focusPageId={displayPage?.id ?? null} currentSlug={selectedSlug ?? displayPage?.slug ?? undefined} currentPageTitle={selectedPageInfo.title ?? displayPage?.title ?? undefined} onWriteToolComplete={handlePageChanged} />
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
          <div className="h-full overflow-hidden @container">
            {reviewSlugs.length > 0 ? (
              <TabbedPageReview
                slugs={reviewSlugs}
                onDone={handleReviewDone}
                onSelectPage={(slug) => setSelectedSlug(slug)}
                onContentConfirmed={handleContentConfirmed}
                onInternalLink={handleInternalLink}
              />
            ) : (
              <PageViewer
                page={displayPage}
                collapsed={rightCollapsed}
                breadcrumb={breadcrumb}
                onViewPage={(slug) => setSelectedSlug(slug)}
                onSelectPage={(slug) => setSelectedSlug(slug)}
                onInternalLink={handleInternalLink}
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
