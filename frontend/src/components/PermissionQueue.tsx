import { useState } from "react";
import type { PermissionRequestEvent, PermissionDecisionInput } from "../types";

interface Props {
  request: PermissionRequestEvent | null;
  onResolve: (decisions: PermissionDecisionInput[]) => void;
}

export function PermissionQueue({ request, onResolve }: Props) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<string | null>(null);
  const [editText, setEditText] = useState("");

  if (!request) {
    return (
      <div className="text-sm text-th-text-muted p-3">
        当前没有待批准的操作
      </div>
    );
  }

  const items = request.items;
  const allSelected = selected.size === items.length;
  const noneSelected = selected.size === 0;

  function toggle(id: string) {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setSelected(next);
  }

  function submit(action: "approve" | "reject") {
    const targets = noneSelected ? items.map(i => i.id) : Array.from(selected);
    onResolve(targets.map(id => ({ id, action })));
    setSelected(new Set());
    setEditing(null);
  }

  function startEdit(id: string) {
    const item = items.find(i => i.id === id);
    if (!item) return;
    setEditing(id);
    setEditText(JSON.stringify(item.input, null, 2));
  }

  function saveEdit(id: string) {
    let parsed: any;
    try {
      parsed = JSON.parse(editText);
    } catch {
      alert("JSON 解析失败");
      return;
    }
    onResolve([{ id, action: "edit", edited_input: parsed }]);
    setEditing(null);
    setEditText("");
  }

  return (
    <div className="rounded-lg border border-th-border bg-th-bg-secondary p-3">
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-medium text-th-text-primary">
          待批准 ({items.length})
        </h3>
        <div className="flex gap-2">
          <button
            onClick={() => setSelected(allSelected ? new Set() : new Set(items.map(i => i.id)))}
            className="text-xs px-2 py-1 rounded border border-th-border text-th-text-secondary hover:bg-th-bg-tertiary"
          >
            {allSelected ? "全不选" : "全选"}
          </button>
          <button
            onClick={() => submit("approve")}
            className="text-xs px-2 py-1 rounded bg-th-accent text-white hover:opacity-90"
          >
            全部批准
          </button>
          <button
            onClick={() => submit("reject")}
            className="text-xs px-2 py-1 rounded border border-th-border text-th-text-secondary hover:bg-th-bg-tertiary"
          >
            全部拒绝
          </button>
        </div>
      </div>

      <ul className="space-y-2">
        {items.map(item => (
          <li key={item.id} className="text-sm border border-th-border rounded p-2">
            {editing === item.id ? (
              <div>
                <textarea
                  className="w-full h-32 font-mono text-xs p-2 rounded border border-th-border bg-th-bg-primary"
                  value={editText}
                  onChange={e => setEditText(e.target.value)}
                />
                <div className="flex gap-2 mt-2">
                  <button onClick={() => saveEdit(item.id)} className="text-xs px-2 py-1 rounded bg-th-accent text-white">
                    保存并批准
                  </button>
                  <button onClick={() => setEditing(null)} className="text-xs px-2 py-1 rounded border border-th-border">
                    取消
                  </button>
                </div>
              </div>
            ) : (
              <div className="flex items-start gap-2">
                <input
                  type="checkbox"
                  checked={selected.has(item.id)}
                  onChange={() => toggle(item.id)}
                  className="mt-1"
                />
                <div className="flex-1">
                  <div className="font-mono text-xs text-th-text-muted">{item.tool}</div>
                  <div className="text-th-text-primary">{item.preview}</div>
                </div>
                <button
                  onClick={() => startEdit(item.id)}
                  className="text-xs text-th-text-muted hover:text-th-text-primary"
                >
                  编辑
                </button>
              </div>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
