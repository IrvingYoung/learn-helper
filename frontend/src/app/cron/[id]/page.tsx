"use client";
import { useNavigate, useParams } from "react-router-dom";
import useSWR from "swr";
import { CronTaskForm } from "../../../components/CronTaskForm";
import { CronRunHistory } from "../../../components/CronRunHistory";
import {
  CronTask,
  getCronTask,
  runCronTaskNow,
  updateCronTask,
} from "../../../lib/api-cron";

export default function EditCronTaskPage() {
  const navigate = useNavigate();
  const params = useParams();
  const id = parseInt(params.id as string);

  const { data: task, error } = useSWR<CronTask>(`cron-task-${id}`, () => getCronTask(id));

  if (error) return <p className="p-6 text-red-600">加载失败</p>;
  if (!task) return <p className="p-6 text-gray-500">加载中...</p>;

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-8">
      <nav className="text-sm text-gray-500 flex items-center gap-2">
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
        <span className="text-gray-700 truncate max-w-xs">{task.name}</span>
      </nav>
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{task.name}</h1>
          <p className="text-sm text-gray-500 mt-1">编辑任务配置</p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={async () => {
              if (!confirm(`立即运行 "${task.name}" 吗？`)) return;
              await runCronTaskNow(task.id);
            }}
            className="px-3 py-1.5 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700"
          >
            立即运行
          </button>
          <button
            onClick={() => navigate("/cron")}
            className="px-3 py-1.5 text-sm bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200"
          >
            返回列表
          </button>
        </div>
      </div>

      <section>
        <h2 className="text-lg font-medium mb-3">配置</h2>
        <CronTaskForm
          initialValues={{
            name: task.name,
            description: task.description,
            cron_expr: task.cron_expr,
            prompt: task.prompt,
            max_steps: task.max_steps,
            timeout_sec: task.timeout_sec,
          }}
          submitLabel="保存修改"
          onCancel={() => navigate("/cron")}
          onSubmit={async (values) => {
            await updateCronTask(task.id, values);
            navigate("/cron");
          }}
        />
      </section>

      <section>
        <CronRunHistory taskId={task.id} />
      </section>
    </div>
  );
}
