import { useState, useEffect, useCallback } from 'react';
import type { WikiPage, Plan, ExecutionReport } from '../types';
import { MarkdownContent } from './MarkdownContent';
import { PlanPreview } from './PlanPreview';
import { OperationQueue } from './OperationQueue';

interface PageViewerProps {
  page: WikiPage | null;
  collapsed: boolean;
  plan: Plan | null;
  pendingPlans: Plan[];
  onConfirmPlan: (planId: string) => void;
  onRejectPlan: (planId: string) => void;
  confirmingPlan: boolean;
  onSelectPage: (slug: string) => void;
  onAskAI?: (text: string, pageTitle: string) => void;
  onPlanConfirmed: (planId: string, report: ExecutionReport) => void;
  onPlanRejected: (planId: string) => void;
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

export function PageViewer({ page, collapsed, plan, pendingPlans, onConfirmPlan, onRejectPlan, confirmingPlan, onSelectPage, onAskAI, onPlanConfirmed, onPlanRejected }: PageViewerProps) {
  const [activeTab, setActiveTab] = useState<"content" | "operations">("content");
  const [selectionTooltip, setSelectionTooltip] = useState<{
    text: string;
    x: number;
    y: number;
  } | null>(null);

  useEffect(() => {
    if (pendingPlans.length > 0 && activeTab === "content") {
      setActiveTab("operations");
    }
  }, [pendingPlans.length]);

  const handleContentMouseUp = useCallback((_e: React.MouseEvent) => {
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
        y: rect.top - 10,
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
    const handleClickOutside = () => {
      setSelectionTooltip(null);
    };
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

  return (
    <div className="h-full flex flex-col bg-th-bg-secondary">
      {/* Tab bar */}
      <div className="flex items-center border-b border-th-border shrink-0">
        <button
          onClick={() => setActiveTab("content")}
          className={`px-4 py-2 text-sm font-medium transition-colors ${
            activeTab === "content"
              ? "text-th-accent border-b-2 border-th-accent"
              : "text-th-text-muted hover:text-th-text-secondary"
          }`}
        >
          页面内容
        </button>
        <button
          onClick={() => setActiveTab("operations")}
          className={`px-4 py-2 text-sm font-medium transition-colors flex items-center gap-1.5 ${
            activeTab === "operations"
              ? "text-th-accent border-b-2 border-th-accent"
              : "text-th-text-muted hover:text-th-text-secondary"
          }`}
        >
          待确认操作
          {pendingPlans.length > 0 && (
            <span className="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-bold bg-red-500 text-white rounded-full">
              {pendingPlans.length}
            </span>
          )}
        </button>
      </div>

      {/* Tab content */}
      <div className="flex-1 min-h-0">
        {activeTab === "content" && (
          plan ? (
            <PlanPreview
              plan={plan}
              onConfirm={onConfirmPlan}
              onReject={onRejectPlan}
              confirming={confirmingPlan}
            />
          ) : !page ? (
            <div className="h-full flex items-center justify-center">
              <div className="text-center text-th-text-muted space-y-3">
                <svg className="w-10 h-10 mx-auto opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
                <p className="text-base font-medium">选择一个页面</p>
                <p className="text-sm opacity-60">点击左侧知识树查看内容</p>
              </div>
            </div>
          ) : (
            <div className="h-full overflow-y-auto custom-scroll">
              <div className="p-6 max-w-3xl">
                <div className="mb-6">
                  <h1 className="text-2xl font-bold text-th-text-primary font-display leading-tight">{page.title}</h1>
                  <div className="flex items-center gap-2 mt-3 text-sm">
                    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium border ${status.bg} ${status.text} ${status.border}`}>
                      <span className={`w-1.5 h-1.5 rounded-full mr-1.5 ${status.dot}`} />
                      {status.label}
                    </span>
                    {page.page_type !== 'entity' && (
                      <span className="px-2.5 py-0.5 rounded bg-th-accent-bg text-th-accent text-xs font-medium">
                        {page.page_type}
                      </span>
                    )}
                  </div>
                </div>
                <div className="prose-custom" onMouseUp={handleContentMouseUp}>
                  <MarkdownContent content={page.content} onWikiLinkClick={onSelectPage} />
                </div>
                {page.backlinks && page.backlinks.length > 0 && (
                  <div className="mt-6 pt-4 border-t border-th-separator">
                    <h3 className="text-sm font-medium text-th-muted mb-2">反向链接</h3>
                    <div className="flex flex-wrap gap-2">
                      {page.backlinks.map(blId => (
                        <BacklinkBadge key={blId} pageId={blId} onSelect={onSelectPage} />
                      ))}
                    </div>
                  </div>
                )}
              </div>
              {selectionTooltip && (
                <button
                  className="fixed z-50 px-3 py-1.5 bg-th-accent text-white text-xs rounded-lg shadow-md hover:opacity-90 transition-opacity whitespace-nowrap"
                  style={{
                    left: selectionTooltip.x,
                    top: selectionTooltip.y,
                    transform: "translate(-50%, -100%)",
                  }}
                  onClick={handleAskAIClick}
                  onMouseDown={(e) => e.preventDefault()}
                >
                  💬 询问 AI
                </button>
              )}
            </div>
          )
        )}
        {activeTab === "operations" && (
          <OperationQueue
            plans={pendingPlans}
            onPlanConfirmed={onPlanConfirmed}
            onPlanRejected={onPlanRejected}
          />
        )}
      </div>
    </div>
  );
}

function BacklinkBadge({ pageId, onSelect }: { pageId: number; onSelect: (slug: string) => void }) {
  const [info, setInfo] = useState<{title: string; slug: string} | null>(null);

  useEffect(() => {
    fetch(`/api/wiki/by-id?id=${pageId}`)
      .then(res => res.json())
      .then(data => setInfo({ title: data.title, slug: data.slug }))
      .catch(() => {});
  }, [pageId]);

  if (!info) return null;

  return (
    <button
      onClick={() => onSelect(info.slug)}
      className="px-2 py-1 text-xs rounded-md border border-th-separator
                 text-th-muted hover:bg-th-hover transition-colors"
    >
      {info.title}
    </button>
  );
}