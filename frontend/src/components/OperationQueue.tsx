import type { Plan, ExecutionReport } from "../types";
import { confirmPlan, rejectPlan } from "../lib/api";

interface OperationQueueProps {
  plans: Plan[];
  executionResults: Map<string, ExecutionReport>;
  onViewPage?: (slug: string) => void;
  onPlanConfirmed: (planId: string, report: ExecutionReport) => void;
  onPlanRejected: (planId: string) => void;
}

const ACTION_TYPE_LABELS: Record<string, { label: string; color: string }> = {
  create_page: { label: "创建", color: "bg-green-100 text-green-700" },
  update_page: { label: "更新", color: "bg-amber-100 text-amber-700" },
  delete_page: { label: "删除", color: "bg-red-100 text-red-700" },
  link_pages: { label: "链接", color: "bg-blue-100 text-blue-700" },
  move_page: { label: "移动", color: "bg-purple-100 text-purple-700" },
};

export function OperationQueue({ plans, executionResults, onViewPage, onPlanConfirmed, onPlanRejected }: OperationQueueProps) {
  if (plans.length === 0 && executionResults.size === 0) {
    return (
      <div className="h-full flex items-center justify-center text-th-text-muted">
        <p className="text-sm">没有待确认的操作</p>
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto p-4 space-y-3">
      {Array.from(executionResults.entries()).map(([planId, report]) => (
        <ExecutionResultCard
          key={planId}
          report={report}
          onViewPage={onViewPage}
        />
      ))}
      {plans.map((plan) => (
        <OperationCard
          key={plan.id}
          plan={plan}
          onConfirm={async (planId) => {
            const report = await confirmPlan(planId);
            onPlanConfirmed(planId, report);
          }}
          onReject={async (planId) => {
            await rejectPlan(planId);
            onPlanRejected(planId);
          }}
        />
      ))}
    </div>
  );
}

function OperationCard({
  plan,
  onConfirm,
  onReject,
}: {
  plan: Plan;
  onConfirm: (planId: string) => void;
  onReject: (planId: string) => void;
}) {
  const isUserPlan = !plan.conversation_id;
  const sourceLabel = isUserPlan ? "用户操作" : "AI 计划";
  const sourceColor = isUserPlan ? "bg-gray-100 text-gray-600" : "bg-amber-100 text-amber-700";
  const actions = plan.actions ?? [];

  return (
    <div className="border border-th-border rounded-lg p-3 space-y-2">
      <div className="flex items-center gap-2">
        <span className={`text-xs font-medium px-1.5 py-0.5 rounded ${sourceColor}`}>
          {sourceLabel}
        </span>
        {actions.length > 1 && (
          <span className="text-xs text-th-text-muted">{actions.length} 个操作</span>
        )}
      </div>
      <div className="text-sm text-th-text-primary">{plan.reasoning}</div>
      {actions.length > 0 && (
        <div className="text-xs text-th-text-muted pl-2 border-l-2 border-th-border space-y-0.5">
          {actions.map((action) => {
            const typeInfo = ACTION_TYPE_LABELS[action.type] || { label: action.type, color: "bg-gray-100 text-gray-600" };
            const title = (action.params?.title as string) || (action.params?.page_id ? `页面 #${action.params.page_id}` : action.type);
            return (
              <div key={action.id} className="flex items-center gap-1.5">
                <span className={`text-xs font-medium px-1 py-0 rounded ${typeInfo.color}`}>{typeInfo.label}</span>
                <span>{title}</span>
              </div>
            );
          })}
        </div>
      )}
      <div className="flex gap-2 pt-1">
        <button
          onClick={() => onConfirm(plan.id)}
          disabled={plan.status !== "pending"}
          className="px-3 py-1 text-xs font-medium bg-th-accent text-white rounded hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          确认执行
        </button>
        <button
          onClick={() => onReject(plan.id)}
          disabled={plan.status !== "pending"}
          className="px-3 py-1 text-xs font-medium bg-th-bg-tertiary text-th-text-secondary rounded hover:bg-th-bg-primary disabled:opacity-50 disabled:cursor-not-allowed"
        >
          拒绝
        </button>
      </div>
    </div>
  );
}

function ExecutionResultCard({
  report,
  onViewPage,
}: {
  report: ExecutionReport;
  onViewPage?: (slug: string) => void;
}) {
  const actions = report.actions ?? [];
  const failedActions = actions.filter(a => a.status === "failed");
  const succeededActions = actions.filter(a => a.status === "completed");
  const hasFailures = failedActions.length > 0;
  const allFailed = succeededActions.length === 0 && failedActions.length > 0;

  const slugFromResult = (action: typeof actions[number]) => {
    const result = action.result;
    if (result && typeof result === "object" && "slug" in result) {
      return result.slug as string;
    }
    return null;
  };

  const firstSlug = actions
    .map(a => slugFromResult(a))
    .find((s): s is string => s !== null);

  if (allFailed) {
    return (
      <div className="border border-red-200 rounded-lg p-3 space-y-2 bg-red-50">
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-red-100 text-red-700">
            执行失败
          </span>
        </div>
        {failedActions.map(action => (
          <div key={action.id} className="text-xs text-red-700 pl-2 border-l-2 border-red-200">
            <span className="font-medium">{action.type}</span>
            {action.error && <span className="ml-1">: {action.error}</span>}
          </div>
        ))}
      </div>
    );
  }

  if (hasFailures) {
    return (
      <div className="border border-amber-200 rounded-lg p-3 space-y-2 bg-amber-50">
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-amber-100 text-amber-700">
            部分失败
          </span>
          <span className="text-xs text-amber-600">{failedActions.length}/{report.actions.length} 个操作失败</span>
        </div>
        {failedActions.map(action => (
          <div key={action.id} className="text-xs text-amber-700 pl-2 border-l-2 border-amber-200">
            <span className="font-medium">{action.type}</span>
            {action.error && <span className="ml-1">: {action.error}</span>}
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="border border-green-200 rounded-lg p-3 space-y-1 bg-green-50">
      <div className="flex items-center gap-2">
        <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-green-100 text-green-700">
          执行成功
        </span>
        {firstSlug && onViewPage && (
          <button
            onClick={() => onViewPage(firstSlug)}
            className="text-xs text-green-700 underline hover:text-green-900"
          >
            查看新页面
          </button>
        )}
      </div>
    </div>
  );
}
