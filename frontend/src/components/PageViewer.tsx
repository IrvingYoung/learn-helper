import type { WikiPage } from '../types';
import { MarkdownContent } from './MarkdownContent';

interface PageViewerProps {
  page: WikiPage | null;
  collapsed: boolean;
}

export function PageViewer({ page, collapsed }: PageViewerProps) {
  if (collapsed) return null;

  if (!page) {
    return (
      <div className="h-full flex items-center justify-center bg-white">
        <div className="text-center text-gray-400">
          <p className="text-lg mb-2">选择一个页面</p>
          <p className="text-sm">点击左侧知识树查看内容</p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto bg-white">
      <div className="p-6 max-w-3xl">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900">{page.title}</h1>
          <div className="flex items-center gap-2 mt-2 text-sm">
            <span className={`px-2 py-0.5 rounded ${
              page.content_status === 'published' ? 'bg-green-100 text-green-700' :
              page.content_status === 'draft' ? 'bg-yellow-100 text-yellow-700' :
              'bg-gray-100 text-gray-500'
            }`}>
              {page.content_status === 'published' ? '已填充' :
               page.content_status === 'draft' ? '草稿' : '空'}
            </span>
            {page.page_type !== 'entity' && (
              <span className="px-2 py-0.5 rounded bg-blue-100 text-blue-700">
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
