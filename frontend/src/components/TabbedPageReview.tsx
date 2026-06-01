import { useState, useEffect, useCallback } from "react";
import type { WikiPage } from "../types";
import { fetchWikiPage, confirmPageContent } from "../lib/api";
import { MarkdownContent } from "./MarkdownContent";

interface TabbedPageReviewProps {
  slugs: string[];
  onDone: () => void;
  onSelectPage: (slug: string) => void;
  onContentConfirmed: (pageId: number) => void;
}

const STATUS_STYLES: Record<string, { dot: string }> = {
  published: { dot: "bg-[var(--badge-published-dot)]" },
  draft: { dot: "bg-[var(--badge-draft-dot)]" },
  empty: { dot: "bg-[var(--badge-empty-dot)]" },
};

export function TabbedPageReview({ slugs, onDone, onSelectPage, onContentConfirmed }: TabbedPageReviewProps) {
  const [pages, setPages] = useState<WikiPage[]>([]);
  const [activeSlug, setActiveSlug] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [confirmingPageId, setConfirmingPageId] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    Promise.all(slugs.map((slug) => fetchWikiPage(slug).catch(() => null)))
      .then((results) => {
        if (cancelled) return;
        const valid = results.filter((p): p is WikiPage => p !== null);
        setPages(valid);
        if (valid.length > 0 && !activeSlug) {
          setActiveSlug(valid[0].slug);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => { cancelled = true; };
  }, [slugs.join(",")]);

  const activePage = pages.find((p) => p.slug === activeSlug) || null;

  const handleConfirm = useCallback(async (page: WikiPage) => {
    setConfirmingPageId(page.id);
    try {
      await confirmPageContent(page.id);
      onContentConfirmed(page.id);
      setPages((prev) =>
        prev.map((p) =>
          p.id === page.id ? { ...p, content_status: "published" as const } : p
        )
      );
    } catch (err) {
      console.error("Failed to confirm page content:", err);
    } finally {
      setConfirmingPageId(null);
    }
  }, [onContentConfirmed]);

  if (slugs.length === 0) return null;

  const isDraft = activePage?.content_status === "draft" && activePage.content?.trim().length > 0;

  return (
    <div className="h-full flex flex-col bg-th-bg-secondary">
      {/* Tab bar */}
      <div className="shrink-0 border-b border-th-border bg-th-bg-primary">
        <div className="flex items-center overflow-x-auto custom-scroll">
          {pages.map((page) => {
            const isActive = page.slug === activeSlug;
            const ss = STATUS_STYLES[page.content_status] || STATUS_STYLES.empty;
            return (
              <button
                key={page.slug}
                onClick={() => setActiveSlug(page.slug)}
                className={`flex items-center gap-1.5 px-3 py-2 text-xs font-medium border-b-2 transition-colors shrink-0 ${
                  isActive
                    ? "border-th-accent text-th-text-primary bg-th-bg-secondary"
                    : "border-transparent text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-tertiary"
                }`}
              >
                <span className={`w-1.5 h-1.5 rounded-full ${ss.dot}`} />
                <span className="truncate max-w-[120px]">{page.title}</span>
              </button>
            );
          })}
          <div className="flex-1" />
          <button
            onClick={onDone}
            className="px-3 py-2 text-xs font-medium text-th-accent hover:text-th-accent/80 hover:bg-th-accent-bg transition-colors shrink-0"
          >
            完成审阅
          </button>
        </div>
      </div>

      {/* Draft confirmation bar */}
      {isDraft && activePage && (
        <div className="flex items-center gap-3 px-4 py-2 bg-amber-50 border-b border-amber-200 shrink-0">
          <span className="text-sm text-amber-700">{activePage.title} 有未确认的内容</span>
          <button
            onClick={() => handleConfirm(activePage)}
            disabled={confirmingPageId === activePage.id}
            className="px-3 py-1 text-xs font-medium bg-amber-600 text-white rounded hover:bg-amber-700 disabled:opacity-50 transition-colors"
          >
            {confirmingPageId === activePage.id ? "确认中..." : "确认"}
          </button>
        </div>
      )}

      {/* Content */}
      <div className="flex-1 min-h-0">
        {loading ? (
          <div className="h-full flex items-center justify-center">
            <div className="flex items-center gap-1.5 text-th-text-muted">
              <span className="w-2 h-2 bg-th-accent rounded-full animate-bounce" style={{ animationDelay: "0ms" }} />
              <span className="w-2 h-2 bg-th-accent rounded-full animate-bounce" style={{ animationDelay: "150ms" }} />
              <span className="w-2 h-2 bg-th-accent rounded-full animate-bounce" style={{ animationDelay: "300ms" }} />
            </div>
          </div>
        ) : !activePage ? (
          <div className="h-full flex items-center justify-center">
            <div className="text-center text-th-text-muted space-y-3">
              <svg className="w-10 h-10 mx-auto opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <p className="text-base font-medium">加载失败</p>
            </div>
          </div>
        ) : (
          <div className="h-full overflow-y-auto custom-scroll">
            <div className="p-6 max-w-3xl">
              <div className="mb-6">
                <h1 className="text-2xl font-bold text-th-text-primary font-display leading-tight">{activePage.title}</h1>
              </div>
              <div className="prose-custom">
                <MarkdownContent content={activePage.content} onWikiLinkClick={(slug) => {
                  if (pages.find((p) => p.slug === slug)) {
                    setActiveSlug(slug);
                  } else {
                    onSelectPage(slug);
                  }
                }} />
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
