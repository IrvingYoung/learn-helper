import type { Plan, PlanAction, ActionType } from "../types";

interface PlanPreviewProps {
  plan: Plan;
  onConfirm: (planId: string) => void;
  onReject: (planId: string) => void;
  confirming: boolean;
}

const ACTION_ICONS: Record<ActionType, { char: string; color: string }> = {
  create_page: { char: '+', color: '#d97706' },
  update_page: { char: '~', color: '#d97706' },
  delete_page: { char: '×', color: '#dc2626' },
  link_pages: { char: '→', color: '#d97706' },
  move_page: { char: '↗', color: '#d97706' },
};

const STATUS_LABELS: Record<string, string> = {
  pending: '待确认',
  confirmed: '已确认',
  executing: '执行中',
  completed: '已完成',
  rejected: '已拒绝',
  completed_with_failures: '部分失败',
};

function ActionPreview({ action }: { action: PlanAction }) {
  const params = action.params;

  switch (action.type) {
    case 'create_page': {
      const title = (params.title as string) || '未命名';
      const parentId = params.parent_id as number | undefined;
      const content = (params.content as string) || '';
      const contentPreview = content.slice(0, 200);
      return (
        <div>
          <div className="font-medium text-th-text">{title}</div>
          {parentId !== undefined && parentId !== null && (
            <div className="text-xs text-th-muted">→ parent: {parentId}</div>
          )}
          {contentPreview && (
            <div className="text-sm text-th-muted line-clamp-3 mt-1">{contentPreview}</div>
          )}
        </div>
      );
    }

    case 'update_page': {
      const pageId = params.page_id as number | undefined;
      const title = params.title as string | undefined;
      const content = (params.content as string) || '';
      const contentPreview = content.slice(0, 200);
      return (
        <div>
          <div className="font-medium text-th-text">
            Page #{pageId}
            {title && <span> → {title}</span>}
          </div>
          {contentPreview && (
            <div className="text-sm text-th-muted line-clamp-3 mt-1">{contentPreview}</div>
          )}
        </div>
      );
    }

    case 'delete_page': {
      const pageId = params.page_id as number | undefined;
      return (
        <div>
          <span className="font-medium text-red-600">Page #{pageId}</span>
          <span className="text-th-muted"> 将被删除</span>
        </div>
      );
    }

    case 'link_pages': {
      const sourceId = params.source_page_id as number | undefined;
      const targetId = params.target_page_id as number | undefined;
      const linkText = params.link_text as string | undefined;
      return (
        <div className="text-th-text">
          Page #{sourceId} → Page #{targetId}
          {linkText && <span className="text-th-muted"> [{linkText}]</span>}
        </div>
      );
    }

    case 'move_page': {
      const pageId = params.page_id as number | undefined;
      const newParentId = params.new_parent_id as number | undefined;
      return (
        <div className="text-th-text">
          Page #{pageId} → Parent #{newParentId}
        </div>
      );
    }

    default:
      return <div className="text-th-muted">未知操作类型</div>;
  }
}

export function PlanPreview({ plan, onConfirm, onReject, confirming }: PlanPreviewProps) {
  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="p-6 border-b border-th-separator">
        <h2 className="text-xl font-display text-th-text">操作计划</h2>
        <p className="text-th-muted text-sm leading-relaxed mt-2">{plan.reasoning}</p>
        <div className="text-xs text-th-muted mt-2">
          {plan.actions.length} 个操作 · 状态: {STATUS_LABELS[plan.status] || plan.status}
        </div>
      </div>

      {/* Actions list */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {plan.actions.map((action) => {
          const actionIcon = ACTION_ICONS[action.type];
          return (
            <div
              key={action.id}
              className="p-3 rounded-lg border border-th-separator bg-th-surface"
            >
              <div className="flex items-start gap-2">
                <div
                  className="text-lg font-mono w-6 text-center flex-shrink-0"
                  style={{ color: actionIcon.color }}
                >
                  {actionIcon.char}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-xs text-th-muted mb-1">
                    {action.type}
                    {action.depends_on.length > 0 && (
                      <span> · depends: {action.depends_on.join(', ')}</span>
                    )}
                  </div>
                  <ActionPreview action={action} />
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {/* Footer */}
      <div className="p-4 border-t border-th-separator flex gap-3">
        <button
          onClick={() => onConfirm(plan.id)}
          disabled={confirming || plan.status !== 'pending'}
          className="flex-1 px-4 py-2.5 rounded-lg bg-th-accent text-white font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-opacity"
        >
          {confirming ? '执行中...' : '确认执行'}
        </button>
        <button
          onClick={() => onReject(plan.id)}
          disabled={confirming}
          className="px-4 py-2.5 rounded-lg border border-th-separator text-th-muted hover:bg-th-hover disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          拒绝
        </button>
      </div>
    </div>
  );
}
