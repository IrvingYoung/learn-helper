import { useState } from "react";
import type { AskUserRequestEvent } from "../types";

interface Props {
  request: AskUserRequestEvent;
  onAnswer: (answer: string | string[] | "no_answer") => void;
}

export function AskUserCard({ request, onAnswer }: Props) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [freeText, setFreeText] = useState("");

  function toggle(opt: string) {
    if (request.multi_select) {
      const next = new Set(selected);
      if (next.has(opt)) next.delete(opt);
      else next.add(opt);
      setSelected(next);
    } else {
      onAnswer(opt);
    }
  }

  function submitMulti() {
    if (selected.size > 0) {
      onAnswer(Array.from(selected));
    }
  }

  function submitFreeText() {
    const t = freeText.trim();
    if (t) onAnswer(t);
  }

  return (
    <div className="rounded-lg border-l-4 border-blue-500 bg-blue-50 dark:bg-blue-950 p-3 my-2">
      {request.header && (
        <div className="text-[10px] uppercase tracking-wide text-blue-700 dark:text-blue-300 mb-1">
          {request.header}
        </div>
      )}
      <div className="text-sm text-th-text-primary font-medium mb-2">
        {request.question}
      </div>
      <div className="flex flex-wrap gap-2">
        {request.options.map(opt => (
          <button
            key={opt}
            onClick={() => toggle(opt)}
            className={`text-sm px-3 py-1 rounded border ${
              selected.has(opt)
                ? "border-blue-500 bg-blue-100 dark:bg-blue-900"
                : "border-th-border hover:bg-th-bg-tertiary"
            }`}
          >
            {opt}
          </button>
        ))}
      </div>
      {request.multi_select && selected.size > 0 && (
        <button
          onClick={submitMulti}
          className="mt-2 text-xs px-2 py-1 rounded bg-blue-500 text-white"
        >
          确认 ({selected.size})
        </button>
      )}
      {request.allow_free_text && (
        <div className="mt-2 flex gap-2">
          <input
            type="text"
            value={freeText}
            onChange={e => setFreeText(e.target.value)}
            placeholder="其它想法..."
            className="flex-1 text-sm px-2 py-1 rounded border border-th-border bg-th-bg-primary"
            onKeyDown={e => e.key === "Enter" && submitFreeText()}
          />
          <button
            onClick={submitFreeText}
            disabled={!freeText.trim()}
            className="text-xs px-2 py-1 rounded border border-th-border disabled:opacity-50"
          >
            发送
          </button>
        </div>
      )}
      <button
        onClick={() => onAnswer("no_answer")}
        className="mt-2 text-xs text-th-text-muted hover:text-th-text-primary"
      >
        跳过
      </button>
    </div>
  );
}
