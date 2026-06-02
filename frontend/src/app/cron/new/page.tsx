"use client";
import { useNavigate } from "react-router-dom";
import { CronTaskForm } from "../../../components/CronTaskForm";
import { createCronTask } from "../../../lib/api-cron";

export default function NewCronTaskPage() {
  const navigate = useNavigate();

  return (
    <div className="p-6 max-w-4xl mx-auto">
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
        <span className="text-gray-700">新建</span>
      </nav>
      <h1 className="text-2xl font-semibold mb-6">新建定时任务</h1>
      <CronTaskForm
        submitLabel="创建"
        onCancel={() => navigate("/cron")}
        onSubmit={async (values) => {
          await createCronTask(values);
          navigate("/cron");
        }}
      />
    </div>
  );
}
