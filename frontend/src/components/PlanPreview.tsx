import { useState } from "react";
import type { Plan, PlanAction, ActionType, OutlineNode } from "../types";

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

const PAGE_TYPE_ICONS: Record<string, string> = {
  entity: '📄',
  concept: '💡',
  overview: '📋',
};

// Resolve {{action:ID.prop}} placeholders in param values to human-readable labels
function resolveRef(value: unknown, actions: PlanAction[]): string {
  if (typeof value === 'number') {
    return `Page #${value}`;
  }
  if (typeof value !== 'string') {
    return String(value ?? '');
  }
  return value.replace(/\{\{action:([^}]+)\.([^}]+)\}\}/g, (_match, refId, _prop) => {
    const ref = actions.find(a => a.id === refId);
    if (!ref) return '?';
    const title = (ref.params.title as string) || '新页面';
    return `「${title}」`;
  });
}

// Resolve a depends_on action ID to the referenced action's title
function depLabel(depId: string, actions: PlanAction[]): string {
  const dep = actions.find(a => a.id === depId);
  if (!dep) return depId;
  return (dep.params.title as string) || dep.type;
}

function ActionPreview({ action, actions }: { action: PlanAction; actions: PlanAction[] }) {
  const params = action.params;

  switch (action.type) {
    case 'create_page': {
      const title = (params.title as string) || '未命名';
      const parentId = params.parent_id as number | undefined;
      const content = (params.content as string) || '';
      const contentPreview = content.slice(0, 200);
      return (
        <div>
          <div className="font-medium text-th-text">{resolveRef(title, actions)}</div>
          {parentId !== undefined && parentId !== null && (
            <div className="text-xs text-th-muted">→ parent: {resolveRef(parentId, actions)}</div>
          )}
          {contentPreview && (
            <div className="text-sm text-th-muted line-clamp-3 mt-1">{resolveRef(contentPreview, actions)}</div>
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
            {pageId !== undefined ? resolveRef(pageId, actions) : '?'}
            {title && <span> → {resolveRef(title, actions)}</span>}
          </div>
          {contentPreview && (
            <div className="text-sm text-th-muted line-clamp-3 mt-1">{resolveRef(contentPreview, actions)}</div>
          )}
        </div>
      );
    }

    case 'delete_page': {
      const pageId = params.page_id as number | undefined;
      return (
        <div>
          <span className="font-medium text-red-600">{pageId !== undefined ? resolveRef(pageId, actions) : '?'}</span>
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
          {sourceId !== undefined ? resolveRef(sourceId, actions) : '?'} → {targetId !== undefined ? resolveRef(targetId, actions) : '?'}
          {linkText && <span className="text-th-muted"> [{resolveRef(linkText, actions)}]</span>}
        </div>
      );
    }

    case 'move_page': {
      const pageId = params.page_id as number | undefined;
      const newParentId = params.new_parent_id as number | undefined;
      return (
        <div className="text-th-text">
          {pageId !== undefined ? resolveRef(pageId, actions) : '?'} → {newParentId !== undefined ? resolveRef(newParentId, actions) : '?'}
        </div>
      );
    }

    default:
      return <div className="text-th-muted">未知操作类型</div>;
  }
}

// ── OutlineTree component ────────────────────────────────────────

function OutlineNodeRow({ node, depth }: { node: OutlineNode; depth: number }) {
  const [collapsed, setCollapsed] = useState(false);
  const hasChildren = node.children && node.children.length > 0;

  return (
    <div>
      <div
        className="flex items-center gap-2 py-1.5 px-1 rounded hover:bg-th-hover cursor-pointer transition-colors"
        style={{ paddingLeft: `${depth * 20 + 4}px` }}
        onClick={() => hasChildren && setCollapsed(!collapsed)}
      >
        {hasChildren ? (
          <span className="text-xs text-th-muted w-4 text-center flex-shrink-0">
            {collapsed ? '▶' : '▼'}
          </span>
        ) : (
          <span className="w-4 flex-shrink-0" />
        )}
        <span className="text-sm flex-shrink-0">{PAGE_TYPE_ICONS[node.page_type] || '📄'}</span>
        <span className="text-sm text-th-text font-medium">{node.title}</span>
        {node.page_type && (
          <span className="text-xs text-th-muted ml-1">{node.page_type}</span>
        )}
      </div>
      {hasChildren && !collapsed && (
        <div>
          {node.children!.map((child, i) => (
            <OutlineNodeRow key={child.id || i} node={child} depth={depth + 1} />
          ))}
        </div>
      )}
    </div>
  );
}

function OutlineTree({ outline }: { outline: OutlineNode[] }) {
  return (
    <div className="py-2">
      {outline.map((node, i) => (
        <OutlineNodeRow key={node.id || i} node={node} depth={0} />
      ))}
    </div>
  );
}

// ── ActionList component ─────────────────────────────────────────

function ActionList({ plan }: { plan: Plan }) {
  return (
    <div className="space-y-3">
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
                  {action.depends_on && action.depends_on.length > 0 && (
                    <span> · 依赖 {action.depends_on.map(id => depLabel(id, plan.actions)).join('、')}</span>
                  )}
                </div>
                <ActionPreview action={action} actions={plan.actions} />
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ── PhaseProgress component ──────────────────────────────────────

function PhaseProgress({ plan }: { plan: Plan }) {
  if (!plan.phases || plan.phases.length === 0) return null;

  const current = plan.phase_index ?? 0;
  const total = plan.total_phases ?? plan.phases.length;

  return (
    <div className="mb-4 p-3 rounded-lg bg-th-surface border border-th-separator">
      <div className="flex items-center gap-2 mb-2">
        <span className="text-xs text-th-muted">阶段进度</span>
        <div className="flex-1 h-1.5 bg-th-bg-tertiary rounded-full overflow-hidden">
          <div
            className="h-full bg-th-accent rounded-full transition-all"
            style={{ width: `${((current + 1) / total) * 100}%` }}
          />
        </div>
        <span className="text-xs text-th-muted font-mono">{current + 1}/{total}</span>
      </div>
      <div className="space-y-1">
        {plan.phases.map((phase, i) => (
          <div key={i} className="flex items-center gap-2 text-xs">
            {i < current ? (
              <span className="text-th-accent">✓</span>
            ) : i === current ? (
              <span className="text-th-accent font-bold">●</span>
            ) : (
              <span className="text-th-muted">○</span>
            )}
            <span className={i === current ? 'text-th-text font-medium' : i < current ? 'text-th-muted' : 'text-th-muted'}>
              {phase.title}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Main PlanPreview ─────────────────────────────────────────────

export function PlanPreview({ plan, onConfirm, onReject, confirming }: PlanPreviewProps) {
  const isOutlineOnly = plan.outline && plan.outline.length > 0 && plan.actions.length === 0;

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="p-6 border-b border-th-separator">
        <h2 className="text-xl font-display text-th-text">
          {isOutlineOnly ? '知识大纲' : '操作计划'}
        </h2>
        <p className="text-th-muted text-sm leading-relaxed mt-2">{plan.reasoning}</p>
        <div className="text-xs text-th-muted mt-2">
          {isOutlineOnly ? (
            <span>大纲模式 · 确认后将创建所有骨架页面</span>
          ) : (
            <>
              {plan.actions.length} 个操作 · 状态: {STATUS_LABELS[plan.status] || plan.status}
              {plan.phases && plan.phases.length > 0 && ' · 多阶段计划'}
            </>
          )}
        </div>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto p-4">
        {isOutlineOnly ? (
          <OutlineTree outline={plan.outline || []} />
        ) : (
          <>
            <PhaseProgress plan={plan} />
            <ActionList plan={plan} />
          </>
        )}
      </div>

      {/* Footer */}
      <div className="p-4 border-t border-th-separator flex gap-3">
        <button
          onClick={() => onConfirm(plan.id)}
          disabled={confirming || plan.status !== 'pending'}
          className="flex-1 px-4 py-2.5 rounded-lg bg-th-accent text-white font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-opacity"
        >
          {confirming ? '执行中...' : (isOutlineOnly ? '确认大纲' : '确认执行')}
        </button>
        {!isOutlineOnly && (
          <button
            onClick={() => onReject(plan.id)}
            disabled={confirming}
            className="px-4 py-2.5 rounded-lg border border-th-separator text-th-muted hover:bg-th-hover disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            拒绝
          </button>
        )}
      </div>
    </div>
  );
}
