import { useState } from "react";
import type { ToolCallInfo } from "../types";

interface ToolCallCardProps {
  toolCall: ToolCallInfo;
  defaultExpanded?: boolean;
}

const toolIcons: Record<string, string> = {
  websearch: "🔍",
  webfetch: "🌐",
  lookup_page: "📋",
  read_page: "📄",
  search_pages: "🔎",
};

function getInputSummary(tc: ToolCallInfo): string {
  if (tc.name === "websearch" && tc.input?.query) {
    return `搜索: ${String(tc.input.query).slice(0, 60)}`;
  }
  if (tc.name === "webfetch" && tc.input?.url) {
    return `读取: ${String(tc.input.url).slice(0, 60)}`;
  }
  if (tc.name === "lookup_page" && tc.input?.title) {
    return `查找: ${String(tc.input.title)}`;
  }
  if (tc.name === "read_page" && tc.input?.id) {
    return `读取页面 #${tc.input.id}`;
  }
  if (tc.name === "search_pages" && tc.input?.query) {
    return `搜索: ${String(tc.input.query).slice(0, 60)}`;
  }
  return tc.name;
}

export function ToolCallCard({ toolCall, defaultExpanded = false }: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  // Prefer explicit status when present; otherwise derive from output/error.
  // - "pending" : write tool awaiting user permission approval (semi-transparent, no expand)
  // - "running" : tool currently executing (spinner, no expand)
  // - "done"    : tool completed successfully (icon, expandable)
  // - "error"   : tool failed (✕, expandable)
  const explicitStatus = toolCall.status;
  const isPending = explicitStatus === "pending";
  const isRunning =
    explicitStatus === "running" ||
    (explicitStatus === undefined && !toolCall.output && !toolCall.error);
  const isError = !!toolCall.error;

  const icon = toolIcons[toolCall.name] || "🛠";
  const summary = getInputSummary(toolCall);

  // Truncate long output for display
  const displayOutput = toolCall.output
    ? toolCall.output.length > 2000
      ? toolCall.output.slice(0, 2000) + "\n...（内容过长已截断）"
      : toolCall.output
    : "";

  return (
    <div className={`text-xs leading-relaxed ${isPending ? "opacity-50" : ""}`}>
      {/* One-line collapsed state */}
      <button
        onClick={() => !isRunning && !isPending && setExpanded(!expanded)}
        className="inline-flex items-center gap-1 text-th-text-muted hover:text-th-text-primary transition-colors cursor-pointer"
      >
        {isPending ? (
          <span className="inline-block h-3 w-3 shrink-0 animate-spin rounded-full border-2 border-th-text-muted border-t-transparent" />
        ) : isRunning ? (
          <span className="w-2.5 h-2.5 shrink-0 border-2 border-th-accent border-t-transparent rounded-full animate-spin inline-block" />
        ) : isError ? (
          <span className="text-red-500">✕</span>
        ) : (
          <span>{icon}</span>
        )}
        <span>{summary}</span>
        {!isRunning && !isPending && (
          <svg
            className={`w-2.5 h-2.5 transition-transform ${expanded ? "rotate-180" : ""}`}
            fill="none" stroke="currentColor" viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        )}
      </button>

      {/* Expanded content */}
      {expanded && !isRunning && !isPending && (
        <div className="mt-1 space-y-1">
          {isError && (
            <div className="p-1.5 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-red-700 dark:text-red-300 whitespace-pre-wrap break-words">
              {toolCall.error}
            </div>
          )}

          {displayOutput && (
            <div className="p-1.5 bg-th-bg-primary border border-th-border rounded text-th-text-primary leading-relaxed whitespace-pre-wrap break-words max-h-48 overflow-y-auto">
              {displayOutput}
            </div>
          )}

          {toolCall.input && Object.keys(toolCall.input).length > 0 && (
            <details>
              <summary className="text-th-text-muted cursor-pointer hover:text-th-text-primary">
                参数详情
              </summary>
              <pre className="mt-1 p-1.5 bg-th-bg-primary border border-th-border rounded text-th-text-muted overflow-x-auto">
                {JSON.stringify(toolCall.input, null, 2)}
              </pre>
            </details>
          )}
        </div>
      )}
    </div>
  );
}
