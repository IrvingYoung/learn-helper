import type { Plan, OutlineNode, PlanAction } from "../types";
import { MarkdownContent } from "./MarkdownContent";

interface PlanPreviewProps {
  plan: Plan;
  onConfirm: (planId: string) => void;
  confirming: boolean;
  onCalibrationAnswer?: (answer: string) => void;
}

function OutlineTree({ node, depth }: { node: OutlineNode; depth: number }) {
  const typeLabel: Record<string, string> = {
    entity: "实体",
    concept: "概念",
    overview: "概览",
  };
  return (
    <li className="select-none">
      <div
        className="flex items-center gap-1.5 py-1 text-sm"
        style={{ paddingLeft: depth * 16 }}
      >
        <span className="text-th-text-muted shrink-0">
          {node.children && node.children.length > 0 ? (
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          ) : (
            <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 24 24">
              <circle cx="12" cy="12" r="3" />
            </svg>
          )}
        </span>
        <span className="text-th-text-primary font-medium truncate">{node.title}</span>
        {node.page_type && (
          <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-th-bg-tertiary text-th-text-muted ml-1 shrink-0">
            {typeLabel[node.page_type] || node.page_type}
          </span>
        )}
      </div>
      {node.children && node.children.length > 0 && (
        <ul className="list-none">
          {node.children.map((child, i) => (
            <OutlineTree key={child.id || i} node={child} depth={depth + 1} />
          ))}
        </ul>
      )}
    </li>
  );
}

// Extract a display label from action params, falling back gracefully
function getActionLabel(action: PlanAction): string {
  const p = action.params ?? {};
  switch (action.type) {
    case "create_page":
      return (p.title as string) || (p.page_type as string) || "新页面";
    case "update_page":
      return (p.title as string) || `页面 #${p.page_id ?? "?"}`;
    case "delete_page":
      return (p.title as string) || `页面 #${p.page_id ?? "?"}`;
    case "link_pages":
      return `${p.source_title || p.source_page_id || "?"} → ${p.target_title || p.target_page_id || "?"}`;
    case "move_page":
      return (p.title as string) || `页面 #${p.page_id ?? "?"}`;
    default:
      return action.type;
  }
}

function ActionRow({ action }: { action: PlanAction }) {
  const typeColors: Record<string, string> = {
    create_page: "text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-900/20",
    update_page: "text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20",
    delete_page: "text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20",
    link_pages: "text-purple-600 dark:text-purple-400 bg-purple-50 dark:bg-purple-900/20",
    move_page: "text-orange-600 dark:text-orange-400 bg-orange-50 dark:bg-orange-900/20",
  };
  const typeLabels: Record<string, string> = {
    create_page: "创建",
    update_page: "更新",
    delete_page: "删除",
    link_pages: "链接",
    move_page: "移动",
  };
  const statusIcons: Record<string, string> = {
    pending: "○",
    running: "◌",
    completed: "✓",
    failed: "✗",
    skipped: "–",
  };

  const label = getActionLabel(action);
  const content = (action.params?.content as string) || undefined;
  const showContent = (action.type === "create_page" || action.type === "update_page") && !!content;

  return (
    <div className="py-1.5 text-sm">
      <div className="flex items-center gap-2">
        <span className="text-th-text-muted w-4 text-center shrink-0">
          {statusIcons[action.status] || "○"}
        </span>
        <span
          className={`text-[11px] px-1.5 py-0.5 rounded font-medium shrink-0 ${
            typeColors[action.type] || "text-th-text-muted bg-th-bg-tertiary"
          }`}
        >
          {typeLabels[action.type] || action.type}
        </span>
        <span className="text-th-text-primary font-medium truncate flex-1 min-w-0">
          {label}
        </span>
      </div>
      {showContent && (
        <div className="mt-2">
          <MarkdownContent content={content} />
        </div>
      )}
    </div>
  );
}

function ActionList({ actions }: { actions: PlanAction[] }) {
  if (!actions || actions.length === 0) return null;
  return (
    <div className="divide-y divide-th-border/50">
      {actions.map((action) => (
        <ActionRow key={action.id} action={action} />
      ))}
    </div>
  );
}

export function PlanPreview({ plan, onConfirm, confirming, onCalibrationAnswer }: PlanPreviewProps) {
  const hasOutline = plan.outline && plan.outline.length > 0;
  const hasActions = plan.actions && plan.actions.length > 0;

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="px-4 py-3 border-b border-th-border bg-th-bg-secondary/50 shrink-0">
        <div className="flex items-center gap-2">
          <div className="w-1.5 h-1.5 rounded-full bg-th-accent animate-pulse" />
          <h2 className="text-sm font-semibold text-th-text-primary">
            {hasOutline ? "知识大纲" : "操作计划"}
          </h2>
        </div>
        {plan.reasoning && (
          <p className="text-xs text-th-text-muted mt-2 leading-relaxed line-clamp-3">
            {plan.reasoning}
          </p>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {plan.phases && plan.phases.length > 0 && (
          <div className="px-4 py-3 border-b border-th-border/50">
            <h3 className="text-xs font-semibold text-th-text-secondary uppercase tracking-wider mb-2">阶段</h3>
            <div className="space-y-2">
              {plan.phases.map((phase, i) => (
                <div
                  key={i}
                  className={`p-2 rounded text-sm ${
                    plan.phase_index === i
                      ? "bg-th-accent-bg border border-th-accent/30"
                      : "bg-th-bg-tertiary"
                  }`}
                >
                  <div className="flex items-center gap-1.5">
                    <span className="text-[10px] font-bold text-th-text-muted">#{i + 1}</span>
                    <span className="font-medium text-th-text-primary text-xs">{phase.title}</span>
                  </div>
                  {phase.description && (
                    <p className="text-[11px] text-th-text-muted mt-1">{phase.description}</p>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {hasOutline && (
          <div className="px-4 py-3">
            <h3 className="text-xs font-semibold text-th-text-secondary uppercase tracking-wider mb-2">
              大纲 ({plan.outline!.length} 个顶级节点)
            </h3>
            <ul className="list-none">
              {plan.outline!.map((node, i) => (
                <OutlineTree key={node.id || i} node={node} depth={0} />
              ))}
            </ul>
          </div>
        )}

        {/* Show actions when there's no outline OR alongside outline */}
        {hasActions && !hasOutline && (
          <div className="px-4 py-3">
            <h3 className="text-xs font-semibold text-th-text-secondary uppercase tracking-wider mb-2">
              操作 ({plan.actions.length} 项)
            </h3>
            <ActionList actions={plan.actions} />
          </div>
        )}

        {plan.calibration_question && (
          <div className="px-4 py-4">
            <h3 className="text-xs font-semibold text-th-text-secondary uppercase tracking-wider mb-3">校准问题</h3>
            <p className="text-sm text-th-text-primary leading-relaxed mb-4">{plan.calibration_question.question}</p>
            {plan.calibration_question.options && plan.calibration_question.options.length > 0 && (
              <div className="space-y-2">
                {plan.calibration_question.options.map((option, i) => (
                  <button
                    key={i}
                    onClick={() => onCalibrationAnswer?.(option)}
                    className="w-full text-left px-3 py-2.5 rounded-lg border border-th-border hover:border-th-accent hover:bg-th-accent-bg text-sm text-th-text-primary transition-colors"
                  >
                    <span className="text-th-text-muted mr-2">{i + 1}.</span>
                    {option}
                  </button>
                ))}
              </div>
            )}
            <p className="text-[11px] text-th-text-muted mt-3">也可以在聊天中自由回复</p>
          </div>
        )}

        {!hasOutline && !hasActions && !plan.calibration_question && (
          <div className="flex-1 flex items-center justify-center p-6">
            <div className="text-center">
              <svg className="w-8 h-8 mx-auto text-th-text-muted mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <p className="text-sm text-th-text-muted">暂无可预览的内容</p>
            </div>
          </div>
        )}

        {hasActions && hasOutline && (
          <div className="px-4 py-3 border-t border-th-border/50">
            <h3 className="text-xs font-semibold text-th-text-secondary uppercase tracking-wider mb-2">
              执行操作 ({plan.actions.length} 项)
            </h3>
            <ActionList actions={plan.actions} />
          </div>
        )}
      </div>

      {/* Footer with confirm button - hidden for calibration questions */}
      {!plan.calibration_question && <div className="px-4 py-3 border-t border-th-border bg-th-bg-secondary/50 shrink-0">
        <button
          onClick={() => onConfirm(plan.id)}
          disabled={confirming}
          className="w-full px-4 py-2 bg-th-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-150 active:scale-[0.98] flex items-center justify-center gap-2"
        >
          {confirming ? (
            <>
              <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
              执行中...
            </>
          ) : (
            <>
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
              {hasOutline ? "确认大纲" : "确认执行"}
            </>
          )}
        </button>
      </div>}
    </div>
  );
}
