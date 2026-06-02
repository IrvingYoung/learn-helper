import { useNavigate } from "react-router-dom";
import useSWR from "swr";
import {
  CronTask,
  deleteCronTask,
  listCronTasks,
  runCronTaskNow,
  updateCronTask,
  describeCron,
} from "../../lib/api-cron";

const STATUS_COLORS: Record<string, string> = {
  success: "bg-green-100 text-green-800 border-green-300",
  failed: "bg-red-100 text-red-800 border-red-300",
  running: "bg-yellow-100 text-yellow-800 border-yellow-300",
  timeout: "bg-orange-100 text-orange-800 border-orange-300",
};

const STATUS_LABELS: Record<string, string> = {
  success: "成功",
  failed: "失败",
  running: "运行中",
  timeout: "超时",
};

function StatusBadge({ status }: { status: string | null | undefined }) {
  if (!status) return <span className="text-xs text-gray-400">—</span>;
  const color = STATUS_COLORS[status] || "bg-gray-100 text-gray-800 border-gray-300";
  const label = STATUS_LABELS[status] || status;
  return (
    <span className={`inline-block px-2 py-0.5 text-xs rounded border ${color}`}>{label}</span>
  );
}

function formatTime(s?: string | null) {
  if (!s) return "—";
  return new Date(s).toLocaleString("zh-CN", { hour12: false });
}

export default function CronListPage() {
  const navigate = useNavigate();
  const { data: tasks, mutate } = useSWR<CronTask[]>("cron-tasks", listCronTasks, {
    refreshInterval: 10000,
  });

  const handleToggle = async (task: CronTask, e: React.MouseEvent) => {
    e.stopPropagation();
    await updateCronTask(task.id, { enabled: !task.enabled });
    mutate();
  };

  const handleDelete = async (task: CronTask, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirm(`确定删除任务 "${task.name}" 吗？所有运行历史会一起删除。`)) return;
    await deleteCronTask(task.id);
    mutate();
  };

  const handleRunNow = async (task: CronTask, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirm(`立即运行 "${task.name}" 吗？`)) return;
    await runCronTaskNow(task.id);
    setTimeout(() => mutate(), 1000);
  };

  const handleCardClick = (task: CronTask) => {
    navigate(`/cron/${task.id}`);
  };

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <nav className="text-sm text-gray-500 mb-4 flex items-center gap-2">
        <button
          onClick={() => navigate("/wiki")}
          className="hover:text-gray-700 hover:underline"
        >
          ← 知识库
        </button>
        <span>/</span>
        <span className="text-gray-700">定时任务</span>
      </nav>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">定时任务</h1>
          <p className="text-sm text-gray-500 mt-1">
            配置自动运行的 AI 任务,到点 AI 自主执行,无人工确认 (auto-approve)。
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => navigate("/cron/runs")}
            className="px-3 py-2 text-sm text-gray-700 border border-gray-300 rounded-md hover:bg-gray-50"
          >
            全部运行历史
          </button>
          <button
            onClick={() => navigate("/cron/new")}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
          >
            + 新建任务
          </button>
        </div>
      </div>

      {!tasks ? (
        <p className="text-sm text-gray-500">加载中...</p>
      ) : tasks.length === 0 ? (
        <div className="text-center py-12 border-2 border-dashed border-gray-200 rounded-md">
          <p className="text-gray-500">还没有定时任务</p>
          <button
            onClick={() => navigate("/cron/new")}
            className="mt-3 inline-block text-blue-600 hover:underline"
          >
            创建第一个任务
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {tasks.map((task) => (
            <div
              key={task.id}
              role="button"
              tabIndex={0}
              onClick={() => handleCardClick(task)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  handleCardClick(task);
                }
              }}
              className="border border-gray-200 rounded-md p-4 bg-white hover:shadow-md hover:border-blue-300 transition cursor-pointer"
            >
              <div className="flex items-start justify-between">
                <div className="flex-1 min-w-0">
                  <h3 className="font-medium truncate">{task.name}</h3>
                  {task.description && (
                    <p className="text-xs text-gray-500 mt-0.5 line-clamp-2">
                      {task.description}
                    </p>
                  )}
                </div>
                <StatusBadge status={task.last_status} />
              </div>

              <div className="mt-3 text-xs text-gray-500 space-y-1">
                <div>
                  <span className="font-mono bg-gray-50 px-1.5 py-0.5 rounded">
                    {task.cron_expr}
                  </span>
                  <span className="ml-2">{describeCron(task.cron_expr)}</span>
                </div>
                {task.enabled && task.next_run_at && (
                  <div>下次运行：{formatTime(task.next_run_at)}</div>
                )}
                {task.last_run_at && (
                  <div>上次运行：{formatTime(task.last_run_at)}</div>
                )}
                {task.last_error && (
                  <div className="text-red-600 truncate">错误：{task.last_error}</div>
                )}
              </div>

              <div className="mt-3 flex items-center gap-2 text-xs" onClick={(e) => e.stopPropagation()}>
                <label className="flex items-center gap-1 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={task.enabled}
                    onChange={(e) => handleToggle(task, e as unknown as React.MouseEvent)}
                    className="rounded"
                  />
                  <span>{task.enabled ? "启用" : "已停用"}</span>
                </label>
                <span className="text-gray-300">|</span>
                <button
                  onClick={() => navigate(`/cron/${task.id}`)}
                  className="text-blue-600 hover:underline"
                >
                  编辑
                </button>
                <span className="text-gray-300">|</span>
                <button
                  onClick={(e) => handleRunNow(task, e)}
                  className="text-blue-600 hover:underline"
                >
                  立即运行
                </button>
                <span className="text-gray-300">|</span>
                <button
                  onClick={(e) => handleDelete(task, e)}
                  className="text-red-600 hover:underline"
                >
                  删除
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
