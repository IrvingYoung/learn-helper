import type { WikiPage } from '../types';
import { MarkdownContent } from './MarkdownContent';

interface PageViewerProps {
  page: WikiPage | null;
  collapsed: boolean;
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

export function PageViewer({ page, collapsed }: PageViewerProps) {
  if (collapsed) return null;

  if (!page) {
    return (
      <div className="h-full flex items-center justify-center bg-th-bg-secondary">
        <div className="text-center text-th-text-muted">
          <p className="text-lg mb-2">选择一个页面</p>
          <p className="text-sm">点击左侧知识树查看内容</p>
        </div>
      </div>
    );
  }

  const status = STATUS_STYLES[page.content_status] || STATUS_STYLES.empty;

  return (
    <div className="h-full overflow-y-auto bg-th-bg-secondary custom-scroll">
      <div className="p-6 max-w-3xl">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-th-text-primary font-display">{page.title}</h1>
          <div className="flex items-center gap-2 mt-2 text-sm">
            <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border ${status.bg} ${status.text} ${status.border}`}>
              <span className={`w-1.5 h-1.5 rounded-full mr-1 ${status.dot}`} />
              {status.label}
            </span>
            {page.page_type !== 'entity' && (
              <span className="px-2 py-0.5 rounded bg-th-accent-bg text-th-accent text-xs">
                {page.page_type}
              </span>
            )}
          </div>
        </div>
        <MarkdownContent content={page.content} />
      </div>
    </div>
  );
}