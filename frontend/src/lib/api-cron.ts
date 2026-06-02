// Cron task API client. Mirrors the backend /api/cron/* endpoints.
// Kept in a separate file from api.ts to avoid bloating the main module.

const BASE = "/api/cron";

export interface CronTask {
  id: number;
  name: string;
  description: string;
  cron_expr: string;
  prompt: string;
  enabled: boolean;
  auto_approve: boolean;
  max_steps: number;
  timeout_sec: number;
  next_run_at?: string | null;
  last_run_at?: string | null;
  last_status?: string | null;
  last_error?: string | null;
  created_at: string;
  updated_at: string;
}

export interface CronRun {
  id: number;
  task_id: number;
  task_name?: string;
  status: string;
  started_at: string;
  finished_at?: string | null;
  duration_ms?: number | null;
  output_summary?: string;
  error?: string;
  write_count: number;
  steps_used: number;
  conversation_id?: number | null;
}

export async function listCronTasks(): Promise<CronTask[]> {
  const res = await fetch(`${BASE}/tasks`);
  if (!res.ok) throw new Error("Failed to list cron tasks");
  const data = await res.json();
  return data.tasks ?? [];
}

export async function getCronTask(id: number): Promise<CronTask> {
  const res = await fetch(`${BASE}/tasks/${id}`);
  if (!res.ok) throw new Error(`Failed to fetch task ${id}`);
  return res.json();
}

export interface CreateCronTaskInput {
  name: string;
  description?: string;
  cron_expr: string;
  prompt: string;
  enabled?: boolean;
  max_steps?: number;
  timeout_sec?: number;
}

export async function createCronTask(input: CreateCronTaskInput): Promise<CronTask> {
  const res = await fetch(`${BASE}/tasks`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Failed to create task (${res.status})`);
  }
  return res.json();
}

export interface PatchCronTaskInput {
  name?: string;
  description?: string;
  cron_expr?: string;
  prompt?: string;
  enabled?: boolean;
  max_steps?: number;
  timeout_sec?: number;
}

export async function updateCronTask(id: number, patch: PatchCronTaskInput): Promise<CronTask> {
  const res = await fetch(`${BASE}/tasks/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(patch),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Failed to update task (${res.status})`);
  }
  return res.json();
}

export async function deleteCronTask(id: number): Promise<void> {
  const res = await fetch(`${BASE}/tasks/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error(`Failed to delete task (${res.status})`);
}

export async function runCronTaskNow(id: number): Promise<void> {
  const res = await fetch(`${BASE}/tasks/${id}/run-now`, { method: "POST" });
  if (!res.ok) throw new Error(`Failed to trigger run (${res.status})`);
}

export async function listCronRuns(taskId: number, limit = 20, offset = 0): Promise<CronRun[]> {
  const res = await fetch(`${BASE}/tasks/${taskId}/runs?limit=${limit}&offset=${offset}`);
  if (!res.ok) throw new Error("Failed to list runs");
  const data = await res.json();
  return data.runs ?? [];
}

export async function listAllCronRuns(limit = 50, offset = 0): Promise<CronRun[]> {
  const res = await fetch(`${BASE}/runs?limit=${limit}&offset=${offset}`);
  if (!res.ok) throw new Error("Failed to list runs");
  const data = await res.json();
  return data.runs ?? [];
}

export async function getCronRun(id: number): Promise<CronRun> {
  const res = await fetch(`${BASE}/runs/${id}`);
  if (!res.ok) throw new Error(`Failed to fetch run ${id}`);
  return res.json();
}

// Common cron expression presets for the form. Insert these into the cron_expr input.
export const CRON_PRESETS: { label: string; expr: string }[] = [
  { label: "每分钟", expr: "* * * * *" },
  { label: "每小时", expr: "0 * * * *" },
  { label: "每天 9 点", expr: "0 9 * * *" },
  { label: "每天 18 点", expr: "0 18 * * *" },
  { label: "每周一 8 点", expr: "0 8 * * 1" },
  { label: "每周一至周五 9 点", expr: "0 9 * * 1-5" },
  { label: "每月 1 号 0 点", expr: "0 0 1 * *" },
];

// Lightweight client-side description for a cron expression. Backend
// validation is authoritative; this is just a UI hint.
export function describeCron(expr: string): string {
  const parts = expr.trim().split(/\s+/);
  if (parts.length !== 5) return expr;

  const [minute, hour, day, month, weekday] = parts;
  if (expr === "* * * * *") return "每分钟";
  if (expr === "0 * * * *") return "每小时整点";
  if (minute === "0" && hour === "*" && day === "*" && month === "*" && weekday === "*") {
    return "每小时整点";
  }
  if (minute !== "*" && hour !== "*" && day === "*" && month === "*" && weekday === "*") {
    return `每天 ${hour.padStart(2, "0")}:${minute.padStart(2, "0")}`;
  }
  if (minute !== "*" && hour !== "*" && weekday !== "*" && day === "*" && month === "*") {
    const days = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
    return `每${days[parseInt(weekday) % 7] || weekday} ${hour.padStart(2, "0")}:${minute.padStart(2, "0")}`;
  }
  return expr;
}
