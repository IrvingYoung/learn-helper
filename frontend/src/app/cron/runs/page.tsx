import { useState } from "react";
import { useNavigate } from "react-router-dom";
import useSWR from "swr";
import { CronRun, getCronRun, listAllCronRuns } from "../../../lib/api-cron";

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

function StatusBadge({ status }: { status: string }) {
  const color = STATUS_COLORS[status] || "bg-gray-100 text-gray-800 border-gray-300";
  const label = STATUS_LABELS[status] || status;
  return (
    <span className={`inline-block px-2 py-0.5 text-xs rounded border ${color}`}>{label}</span>
  );
}

function formatDuration(ms?: number | null) {
  if (ms == null) return "-";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function formatTime(s?: string | null) {
  if (!s) return "-";
  return new Date(s).toLocaleString("zh-CN", { hour12: false });
}

export default function AllCronRunsPage() {
  const navigate = useNavigate();
  const { data: runs, mutate } = useSWR<CronRun[]>(
    "cron-runs-all",
    () => listAllCronRuns(50),
    { refreshInterval: 5000 }
  );
  const [selectedRun, setSelectedRun] = useState<CronRun | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const handleRowClick = async (id: number) => {
    setDetailLoading(true);
    setSelectedRun(null);
    try {
      const run = await getCronRun(id);
      setSelectedRun(run);
    } finally {
      setDetailLoading(false);
    }
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
        <button
          onClick={() => navigate("/cron")}
          className="hover:text-gray-700 hover:underline"
        >
          定时任务
        </button>
        <span>/</span>
        <span className="text-gray-700">所有运行历史</span>
      </nav>

      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-semibold">所有运行历史</h1>
        <button
          onClick={() => mutate()}
          className="text-sm text-gray-500 hover:text-gray-700"
        >
          刷新
        </button>
      </div>

      {!runs ? (
        <p className="text-sm text-gray-500">加载中...</p>
      ) : runs.length === 0 ? (
        <p className="text-sm text-gray-500">还没有任何运行记录</p>
      ) : (
        <div className="border border-gray-200 rounded-md overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 text-gray-600 text-left">
              <tr>
                <th className="px-3 py-2">状态</th>
                <th className="px-3 py-2">任务</th>
                <th className="px-3 py-2">开始时间</th>
                <th className="px-3 py-2">耗时</th>
                <th className="px-3 py-2">步数</th>
                <th className="px-3 py-2">写入</th>
                <th className="px-3 py-2">摘要</th>
              </tr>
            </thead>
            <tbody>
              {runs.map((r) => (
                <tr
                  key={r.id}
                  onClick={() => handleRowClick(r.id)}
                  className="border-t border-gray-100 hover:bg-gray-50 cursor-pointer"
                >
                  <td className="px-3 py-2">
                    <StatusBadge status={r.status} />
                  </td>
                  <td className="px-3 py-2">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        navigate(`/cron/${r.task_id}`);
                      }}
                      className="text-blue-600 hover:underline truncate max-w-xs block"
                    >
                      {r.task_name || `task #${r.task_id}`}
                    </button>
                  </td>
                  <td className="px-3 py-2 text-gray-600">{formatTime(r.started_at)}</td>
                  <td className="px-3 py-2 text-gray-600">{formatDuration(r.duration_ms)}</td>
                  <td className="px-3 py-2 text-gray-600">{r.steps_used}</td>
                  <td className="px-3 py-2 text-gray-600">{r.write_count}</td>
                  <td className="px-3 py-2 text-gray-700 max-w-md truncate">
                    {r.output_summary || r.error || "-"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selectedRun && (
        <RunDetailDialog run={selectedRun} onClose={() => setSelectedRun(null)} />
      )}
      {detailLoading && <p className="text-sm text-gray-500 mt-2">加载详情...</p>}
    </div>
  );
}

function RunDetailDialog({ run, onClose }: { run: CronRun; onClose: () => void }) {
  return (
    <div
      className="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[80vh] overflow-y-auto p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between mb-4">
          <h2 className="text-xl font-semibold">Run #{run.id}</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 text-2xl leading-none">
            ×
          </button>
        </div>

        <dl className="grid grid-cols-2 gap-3 text-sm mb-4">
          <dt className="text-gray-500">状态</dt>
          <dd>
            <StatusBadge status={run.status} />
          </dd>
          <dt className="text-gray-500">任务</dt>
          <dd>{run.task_name || `#${run.task_id}`}</dd>
          <dt className="text-gray-500">开始</dt>
          <dd>{formatTime(run.started_at)}</dd>
          <dt className="text-gray-500">结束</dt>
          <dd>{formatTime(run.finished_at)}</dd>
          <dt className="text-gray-500">耗时</dt>
          <dd>{formatDuration(run.duration_ms)}</dd>
          <dt className="text-gray-500">步数</dt>
          <dd>{run.steps_used}</dd>
          <dt className="text-gray-500">写入</dt>
          <dd>{run.write_count} 个页面</dd>
          {run.conversation_id != null && (
            <>
              <dt className="text-gray-500">对话</dt>
              <dd>
                <a
                  href={`/conversations/${run.conversation_id}`}
                  className="text-blue-600 hover:underline"
                >
                  查看完整对话 #{run.conversation_id}
                </a>
              </dd>
            </>
          )}
        </dl>

        {run.output_summary && (
          <div className="mb-4">
            <h3 className="font-medium text-sm mb-1">输出摘要</h3>
            <p className="text-sm text-gray-700 bg-gray-50 p-3 rounded border border-gray-200">
              {run.output_summary}
            </p>
          </div>
        )}

        {run.error && (
          <div
            className={`text-sm p-3 rounded border ${
              run.status === "timeout"
                ? "bg-orange-50 text-orange-800 border-orange-200"
                : "bg-red-50 text-red-800 border-red-200"
            }`}
          >
            <strong>错误：</strong>
            <pre className="mt-1 whitespace-pre-wrap text-xs">{run.error}</pre>
          </div>
        )}
      </div>
    </div>
  );
}
