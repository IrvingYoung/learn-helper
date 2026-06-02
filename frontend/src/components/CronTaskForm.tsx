import { useState } from "react";
import { CRON_PRESETS, describeCron } from "../lib/api-cron";

export interface CronTaskFormValues {
  name: string;
  description: string;
  cron_expr: string;
  prompt: string;
  max_steps: number;
  timeout_sec: number;
}

interface Props {
  initialValues?: Partial<CronTaskFormValues>;
  onSubmit: (values: CronTaskFormValues) => Promise<void>;
  submitLabel: string;
  onCancel: () => void;
}

const DEFAULTS: CronTaskFormValues = {
  name: "",
  description: "",
  cron_expr: "0 9 * * *",
  prompt: "",
  max_steps: 10,
  timeout_sec: 300,
};

export function CronTaskForm({ initialValues, onSubmit, submitLabel, onCancel }: Props) {
  const [values, setValues] = useState<CronTaskFormValues>({ ...DEFAULTS, ...initialValues });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleChange = (field: keyof CronTaskFormValues, value: string | number) => {
    setValues((prev) => ({ ...prev, [field]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit(values);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4 max-w-2xl">
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">名称 *</label>
        <input
          type="text"
          value={values.name}
          onChange={(e) => handleChange("name", e.target.value)}
          required
          className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
          placeholder="例：GitHub 每日趋势"
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">描述</label>
        <input
          type="text"
          value={values.description}
          onChange={(e) => handleChange("description", e.target.value)}
          className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
          placeholder="可选，简单说明这个任务做什么"
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Cron 表达式 *</label>
        <input
          type="text"
          value={values.cron_expr}
          onChange={(e) => handleChange("cron_expr", e.target.value)}
          required
          className="w-full px-3 py-2 border border-gray-300 rounded-md font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
          placeholder="0 9 * * *"
        />
        <p className="mt-1 text-sm text-gray-500">
          {values.cron_expr ? describeCron(values.cron_expr) : "请输入标准 5 字段 cron 表达式"}
        </p>
        <div className="mt-2 flex flex-wrap gap-2">
          {CRON_PRESETS.map((p) => (
            <button
              key={p.expr}
              type="button"
              onClick={() => handleChange("cron_expr", p.expr)}
              className="text-xs px-2 py-1 bg-gray-100 hover:bg-gray-200 rounded border border-gray-300"
            >
              {p.label}
            </button>
          ))}
        </div>
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">任务 Prompt *</label>
        <textarea
          value={values.prompt}
          onChange={(e) => handleChange("prompt", e.target.value)}
          required
          rows={6}
          className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
          placeholder={"例：每天抓取 https://github.com/trending 的前 10 个 repo, 写一段中文趋势点评, 然后创建一个 wiki 页面 'GitHub 趋势 YYYY-MM-DD' 保存结果。"}
        />
        <p className="mt-1 text-xs text-gray-500">{values.prompt.length} / 4000 字</p>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">最大步数</label>
          <input
            type="number"
            value={values.max_steps}
            onChange={(e) => handleChange("max_steps", parseInt(e.target.value) || 10)}
            min={1}
            max={50}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <p className="mt-1 text-xs text-gray-500">AI 推理轮数上限,默认 10</p>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">超时 (秒)</label>
          <input
            type="number"
            value={values.timeout_sec}
            onChange={(e) => handleChange("timeout_sec", parseInt(e.target.value) || 300)}
            min={10}
            max={3600}
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <p className="mt-1 text-xs text-gray-500">单次运行最大时长,默认 300s</p>
        </div>
      </div>

      <div className="rounded-md border-2 border-amber-300 bg-amber-50 p-4">
        <p className="text-sm text-amber-900">
          <strong>⚠️ Auto-approve 模式：</strong>
          此任务运行时, AI 调用的写操作 (create_page / update_page / patch_page / delete_page / link_pages / move_page)
          将<strong>直接生效</strong>,不需要确认。请确保 prompt 写得明确。
        </p>
      </div>

      {error && (
        <div className="rounded-md border border-red-300 bg-red-50 p-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <div className="flex gap-3">
        <button
          type="submit"
          disabled={submitting}
          className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
        >
          {submitting ? "提交中..." : submitLabel}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-2 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200"
        >
          取消
        </button>
      </div>
    </form>
  );
}
