import { useState, useEffect, useCallback } from 'react';
import type { WikiPage } from '../types';
import { MarkdownContent } from './MarkdownContent';
import { confirmPageContent } from '../lib/api';

interface PageViewerProps {
  page: WikiPage | null;
  collapsed: boolean;
  breadcrumb?: { title: string; slug: string }[];
  onViewPage?: (slug: string) => void;
  onSelectPage: (slug: string) => void;
  onInternalLink?: (href: string) => void;
  onAskAI?: (text: string, pageTitle: string) => void;
  onContentConfirmed?: (pageId: number) => void;
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

export function PageViewer({ page, collapsed, breadcrumb = [], onSelectPage, onInternalLink, onAskAI, onContentConfirmed }: PageViewerProps) {
  const [confirming, setConfirming] = useState(false);
  const [selectionTooltip, setSelectionTooltip] = useState<{
    text: string;
    x: number;
    y: number;
  } | null>(null);

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
  }, []);

  const handleAskAIClick = useCallback(() => {
    if (!selectionTooltip || !page) return;
    onAskAI?.(selectionTooltip.text, page.title);
    setSelectionTooltip(null);
    window.getSelection()?.removeAllRanges();
  }, [selectionTooltip, page, onAskAI]);

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

  return (
    <div className="h-full flex flex-col bg-th-bg-secondary">
      {isDraft && (
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
            <div className="relative max-w-2xl mx-auto px-6 py-10">

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
              <div className="mb-8">
                <h1 className="font-display text-[34px] leading-[1.15] font-bold text-th-text-primary tracking-tight">
                  {page.title}
                </h1>
                {page.page_type && page.page_type !== 'entity' && (
                  <div className="mt-3 text-sm text-th-text-muted font-mono tracking-wide">
                    {PAGE_TYPE_LABELS[page.page_type] || page.page_type}
                  </div>
                )}
              </div>

              {/* Body */}
              <div onMouseUp={handleContentMouseUp} className="prose-custom">
                <MarkdownContent content={page.content} onInternalLink={onInternalLink ?? ((href) => onSelectPage(href.replace(/^\/+/, '')))} />
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
