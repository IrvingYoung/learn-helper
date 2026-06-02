import { useState, useEffect, useCallback, useRef } from 'react';
import type { WikiPage } from '../types';
import { MarkdownContent } from './MarkdownContent';
import { confirmPageContent } from '../lib/api';
import { exportPageAsPng } from '../lib/share-as-image';
import { ShareAsImageModal } from './ShareAsImageModal';

interface PageViewerProps {
  page: WikiPage | null;
  collapsed: boolean;
  breadcrumb?: { title: string; slug: string }[];
  onViewPage?: (slug: string) => void;
  onSelectPage: (slug: string) => void;
  onInternalLink?: (href: string) => void;
  onAskAI?: (text: string, pageTitle: string) => void;
  onContentConfirmed?: (pageId: number) => void;
  /**
   * When true, the viewer renders in read-only public mode: no share menu,
   * no draft banner, no "ask AI" selection tooltip. Used on /share/{slug}
   * routes by anonymous visitors.
   */
  publicMode?: boolean;
}

const STATUS_STYLES: Record<string, { bg: string; text: string; border: string; dot: string; label: string }> = {
  published: {
    bg: 'bg-th-badge-published-bg',
    text: 'text-th-badge-published-text',
    border: 'border-th-badge-published-border',
    dot: 'bg-th-badge-published-dot',
    label: '已发布',
  },
  draft: {
    bg: 'bg-th-badge-draft-bg',
    text: 'text-th-badge-draft-text',
    border: 'border-th-badge-draft-border',
    dot: 'bg-th-badge-draft-dot',
    label: '草稿',
  },
  empty: {
    bg: 'bg-th-badge-empty-bg',
    text: 'text-th-badge-empty-text',
    border: 'border-th-badge-empty-border',
    dot: 'bg-th-badge-empty-dot',
    label: '待填充',
  },
};

const PAGE_TYPE_LABELS: Record<string, string> = {
  overview: '概览',
  concept: '概念',
  entity: '实体',
};

export function PageViewer({ page, collapsed, breadcrumb = [], onSelectPage, onInternalLink, onAskAI, onContentConfirmed, publicMode = false }: PageViewerProps) {
  const [confirming, setConfirming] = useState(false);
  const [selectionTooltip, setSelectionTooltip] = useState<{
    text: string;
    x: number;
    y: number;
  } | null>(null);
  const [shareModalOpen, setShareModalOpen] = useState(false);
  const [shareBlob, setShareBlob] = useState<Blob | null>(null);
  const [shareError, setShareError] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);
  const [shareMenuOpen, setShareMenuOpen] = useState(false);
  const [linkCopied, setLinkCopied] = useState(false);
  const shareMenuRef = useRef<HTMLDivElement | null>(null);
  const articleRef = useRef<HTMLDivElement | null>(null);

  const handleConfirm = useCallback(async () => {
    if (!page) return;
    setConfirming(true);
    try {
      await confirmPageContent(page.id);
      onContentConfirmed?.(page.id);
    } catch (err) {
      console.error("Failed to confirm page content:", err);
    } finally {
      setConfirming(false);
    }
  }, [page, onContentConfirmed]);

  const handleContentMouseUp = useCallback(() => {
    // Public visitors don't get the "ask AI" tooltip — there's no AI chat
    // session for anonymous viewers.
    if (publicMode) return;
    setTimeout(() => {
      const sel = window.getSelection();
      if (!sel || sel.isCollapsed || !sel.toString().trim()) {
        setSelectionTooltip(null);
        return;
      }
      const text = sel.toString().trim();
      const range = sel.getRangeAt(0);
      const rect = range.getBoundingClientRect();
      setSelectionTooltip({
        text,
        x: rect.left + rect.width / 2,
        y: rect.top + window.scrollY - 8,
      });
    }, 10);
  }, [publicMode]);

  const handleAskAIClick = useCallback(() => {
    if (!selectionTooltip || !page) return;
    onAskAI?.(selectionTooltip.text, page.title);
    setSelectionTooltip(null);
    window.getSelection()?.removeAllRanges();
  }, [selectionTooltip, page, onAskAI]);

  const handleShare = useCallback(async () => {
    if (!articleRef.current || !page) return;
    setSelectionTooltip(null);
    window.getSelection()?.removeAllRanges();
    setGenerating(true);
    setShareError(null);
    setShareBlob(null);
    setShareModalOpen(true);
    try {
      const blob = await exportPageAsPng(articleRef.current);
      setShareBlob(blob);
    } catch (err) {
      console.error("Failed to export page as PNG:", err);
      setShareError(err instanceof Error ? err.message : "生成图片失败");
    } finally {
      setGenerating(false);
    }
  }, [page]);

  const handleShareClose = useCallback(() => {
    setShareModalOpen(false);
  }, []);

  const handleShareRetry = useCallback(() => {
    handleShare();
  }, [handleShare]);

  const handleShareAsImage = useCallback(() => {
    setShareMenuOpen(false);
    handleShare();
  }, [handleShare]);

  const handleCopyLink = useCallback(async () => {
    if (!page?.share_token) return;
    const url = `${window.location.origin}/share/${page.slug}?t=${page.share_token}`;

    const fallbackCopy = (text: string) => {
      // navigator.clipboard is only available in secure contexts (HTTPS or
      // localhost). On plain-http deployments (like a bare VPS without a
      // reverse proxy + TLS) we fall back to a hidden textarea + execCommand.
      const ta = document.createElement('textarea');
      ta.value = text;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      ta.style.pointerEvents = 'none';
      document.body.appendChild(ta);
      ta.select();
      let ok = false;
      try {
        ok = document.execCommand('copy');
      } catch (err) {
        console.error('execCommand copy failed:', err);
      }
      document.body.removeChild(ta);
      if (ok) {
        setLinkCopied(true);
        setTimeout(() => setLinkCopied(false), 1500);
      } else {
        console.error('Both navigator.clipboard and execCommand failed');
      }
    };

    if (navigator.clipboard && window.isSecureContext) {
      try {
        await navigator.clipboard.writeText(url);
        setLinkCopied(true);
        setTimeout(() => setLinkCopied(false), 1500);
      } catch (err) {
        // Permission denied, no user gesture, etc. — fall through to fallback.
        console.error('navigator.clipboard failed, falling back:', err);
        fallbackCopy(url);
      }
    } else {
      fallbackCopy(url);
    }
    setShareMenuOpen(false);
  }, [page]);

  // Public visitors should not see the share menu, the draft confirmation
  // banner, or the "ask AI" selection tooltip. The share menu is hidden at
  // the JSX level, the draft banner is gated on `publicMode`, and the
  // selection tooltip short-circuits in `handleContentMouseUp` above.

  useEffect(() => {
    if (!shareMenuOpen) return;
    const handler = (e: MouseEvent) => {
      if (shareMenuRef.current && !shareMenuRef.current.contains(e.target as Node)) {
        setShareMenuOpen(false);
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setShareMenuOpen(false);
    };
    document.addEventListener("mousedown", handler);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", handler);
      document.removeEventListener("keydown", onKey);
    };
  }, [shareMenuOpen]);

  useEffect(() => {
    if (!selectionTooltip) return;
    const handleClickOutside = () => setSelectionTooltip(null);
    const timer = setTimeout(() => {
      document.addEventListener("click", handleClickOutside);
    }, 0);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("click", handleClickOutside);
    };
  }, [selectionTooltip]);

  if (collapsed) return null;

  const status = page ? (STATUS_STYLES[page.content_status] || STATUS_STYLES.empty) : STATUS_STYLES.empty;
  const isDraft = page?.content_status === 'draft' && page.content?.trim().length > 0;
  const showDraftBanner = !publicMode && isDraft;

  return (
    <div className="h-full flex flex-col bg-th-bg-secondary">
      {showDraftBanner && (
        <div className="flex items-center gap-3 px-5 py-2 bg-th-accent-bg border-b border-th-accent-bg-strong shrink-0 text-sm">
          <span className="w-1.5 h-1.5 rounded-full bg-th-accent animate-pulse-dot" />
          <span className="text-th-text-primary font-medium">有 AI 生成的待确认内容</span>
          <div className="flex-1" />
          <button
            onClick={() => onAskAI?.("算了，删掉这段内容", page?.title || "")}
            className="text-xs text-th-text-secondary hover:text-th-text-primary transition-colors"
          >
            放弃
          </button>
          <button
            onClick={handleConfirm}
            disabled={confirming}
            className="px-3 py-1 text-xs font-medium bg-th-accent text-white rounded hover:opacity-90 disabled:opacity-50 transition-all active:scale-[0.97]"
          >
            {confirming ? '确认中…' : '确认发布'}
          </button>
        </div>
      )}

      <div className="flex-1 min-h-0">
        {!page ? (
          <div className="h-full flex items-center justify-center">
            <div className="text-center text-th-text-muted space-y-3 max-w-xs">
              <div className="w-12 h-12 mx-auto rounded-full border border-dashed border-th-border flex items-center justify-center">
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
              </div>
              <p className="text-sm font-medium text-th-text-secondary">选择一个页面查看</p>
              <p className="text-xs leading-relaxed opacity-70">从左侧知识树中点选<br/>或直接在中间和 AI 对话</p>
            </div>
          </div>
        ) : (
          <div className="h-full overflow-y-auto custom-scroll">
            <div ref={articleRef} className="relative max-w-2xl @md:max-w-3xl @xl:max-w-4xl mx-auto px-6 py-10">

              {/* Breadcrumb */}
              {breadcrumb.length > 1 && (
                <nav className="flex items-center gap-1.5 text-xs text-th-text-muted mb-5 flex-wrap">
                  {breadcrumb.slice(0, -1).map((crumb, i) => (
                    <span key={crumb.slug} className="flex items-center gap-1.5">
                      <button
                        onClick={() => onSelectPage(crumb.slug)}
                        className="hover:text-th-text-secondary transition-colors truncate max-w-[140px]"
                      >
                        {crumb.title}
                      </button>
                      {i < breadcrumb.length - 2 && (
                        <svg className="w-2.5 h-2.5 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                        </svg>
                      )}
                    </span>
                  ))}
                </nav>
              )}

              {/* Title block */}
              <div className="mb-8 flex items-start justify-between gap-4">
                <div className="min-w-0">
                  <h1 className="font-display text-[34px] leading-[1.15] font-bold text-th-text-primary tracking-tight">
                    {page.title}
                  </h1>
                  {page.page_type && page.page_type !== 'entity' && (
                    <div className="mt-3 text-sm text-th-text-muted font-mono tracking-wide">
                      {PAGE_TYPE_LABELS[page.page_type] || page.page_type}
                    </div>
                  )}
                </div>
                {!publicMode && (
                <div ref={shareMenuRef} className="relative shrink-0" data-share-ui>
                  <button
                    onClick={() => setShareMenuOpen((v) => !v)}
                    aria-label="分享"
                    aria-haspopup="menu"
                    aria-expanded={shareMenuOpen}
                    className="inline-flex items-center justify-center w-8 h-8 text-th-text-muted hover:text-th-text-primary hover:bg-th-hover rounded transition-colors"
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0 0a2.25 2.25 0 1 0 3.935 2.186 2.25 2.25 0 0 0-3.935-2.186Zm0-12.814a2.25 2.25 0 1 0 3.933-2.185 2.25 2.25 0 0 0-3.933 2.185Z" />
                    </svg>
                  </button>
                  {shareMenuOpen && (
                    <div
                      role="menu"
                      className="absolute right-0 top-full mt-1 min-w-[180px] bg-white rounded-md shadow-lg border border-gray-200 py-1 z-30"
                    >
                      <button
                        role="menuitem"
                        onClick={handleCopyLink}
                        disabled={!page?.share_token}
                        title={!page?.share_token ? "该页面尚未生成分享链接" : undefined}
                        className="w-full text-left px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50 flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                      >
                        <svg className="w-3.5 h-3.5 text-gray-500" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
                        </svg>
                        {linkCopied ? "✓ 已复制" : "复制链接"}
                      </button>
                      <button
                        role="menuitem"
                        onClick={handleShareAsImage}
                        disabled={generating}
                        className="w-full text-left px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50 flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                      >
                        <svg className="w-3.5 h-3.5 text-gray-500" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.5 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
                        </svg>
                        用图片分享
                      </button>
                    </div>
                  )}
                </div>
                )}
              </div>

              {/* Body */}
              <div onMouseUp={handleContentMouseUp} className="prose-custom">
                {publicMode ? (
                  <MarkdownContent content={page.content} />
                ) : (
                  <MarkdownContent content={page.content} onInternalLink={onInternalLink ?? ((href) => onSelectPage(href.replace(/^\/+/, '')))} />
                )}
              </div>

              {/* Backlinks */}
              {page.backlinks && page.backlinks.length > 0 && (
                <section className="mt-12 pt-6 border-t border-th-separator">
                  <h3 className="text-[11px] font-semibold text-th-text-muted tracking-[0.14em] uppercase mb-3">
                    反向链接 · {page.backlinks.length}
                  </h3>
                  <div className="flex flex-wrap gap-2">
                    {page.backlinks.map((blId) => (
                      <BacklinkBadge key={blId} pageId={blId} onSelect={onSelectPage} />
                    ))}
                  </div>
                </section>
              )}
            </div>

            {/* Status ribbon (Bear-style: top-right corner) */}
            <div
              className="absolute top-0 right-0 pointer-events-none"
              aria-hidden="true"
              data-share-ui
            >
              <div
                className={`inline-flex items-center gap-1.5 px-3 py-1 text-[11px] font-medium border-l border-b rounded-bl-md ${status.bg} ${status.text} ${status.border}`}
              >
                <span className={`w-1.5 h-1.5 rounded-full ${status.dot}`} />
                {status.label}
              </div>
            </div>

            {/* Selection tooltip */}
            {selectionTooltip && (
              <button
                className="fixed z-50 inline-flex items-center gap-1.5 px-3 py-1.5 bg-th-text-primary text-th-bg-primary text-xs font-medium rounded-lg shadow-th-float hover:opacity-90 transition-opacity animate-spring-in"
                style={{
                  left: selectionTooltip.x,
                  top: selectionTooltip.y,
                  transform: "translate(-50%, -100%)",
                }}
                onClick={handleAskAIClick}
                onMouseDown={(e) => e.preventDefault()}
              >
                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                </svg>
                询问 AI
              </button>
            )}
          </div>
        )}
      </div>

      <ShareAsImageModal
        open={shareModalOpen}
        blob={shareBlob}
        error={shareError}
        slug={page?.slug ?? ""}
        onClose={handleShareClose}
        onRetry={handleShareRetry}
      />
    </div>
  );
}

function BacklinkBadge({ pageId, onSelect }: { pageId: number; onSelect: (slug: string) => void }) {
  const [info, setInfo] = useState<{ title: string; slug: string } | null>(null);

  useEffect(() => {
    fetch(`/api/wiki/by-id?id=${pageId}`)
      .then((res) => res.json())
      .then((data) => setInfo({ title: data.title, slug: data.slug }))
      .catch(() => {});
  }, [pageId]);

  if (!info) return null;

  return (
    <button
      onClick={() => onSelect(info.slug)}
      className="px-2.5 py-1 text-xs rounded-md border border-th-border text-th-text-secondary hover:text-th-text-primary hover:border-th-border-hover hover:bg-th-hover transition-colors"
    >
      {info.title}
    </button>
  );
}
