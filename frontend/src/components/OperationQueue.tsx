import type { Plan, ExecutionReport } from "../types";
import { confirmPlan, rejectPlan } from "../lib/api";

interface OperationQueueProps {
  plans: Plan[];
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

export function OperationQueue({ plans, onPlanConfirmed, onPlanRejected }: OperationQueueProps) {
  if (plans.length === 0) {
    return (
      <div className="h-full flex items-center justify-center text-th-text-muted">
        <p className="text-sm">没有待确认的操作</p>
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto p-4 space-y-3">
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
  const isUserPlan = plan.conversation_id === 0;
  const sourceLabel = isUserPlan ? "用户操作" : "AI 计划";
  const sourceColor = isUserPlan ? "bg-gray-100 text-gray-600" : "bg-amber-100 text-amber-700";

  return (
    <div className="border border-th-border rounded-lg p-3 space-y-2">
      <div className="flex items-center gap-2">
        <span className={`text-xs font-medium px-1.5 py-0.5 rounded ${sourceColor}`}>
          {sourceLabel}
        </span>
        {plan.actions.length > 1 && (
          <span className="text-xs text-th-text-muted">{plan.actions.length} 个操作</span>
        )}
      </div>
      <div className="text-sm text-th-text-primary">{plan.reasoning}</div>
      {plan.actions.length > 0 && (
        <div className="text-xs text-th-text-muted pl-2 border-l-2 border-th-border space-y-0.5">
          {plan.actions.map((action) => {
            const typeInfo = ACTION_TYPE_LABELS[action.type] || { label: action.type, color: "bg-gray-100 text-gray-600" };
            const title = (action.params.title as string) || (action.params.page_id ? `页面 #${action.params.page_id}` : action.type);
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
