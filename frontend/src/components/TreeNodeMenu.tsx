import { useEffect, useRef } from "react";

interface TreeNodeMenuProps {
  x: number;
  y: number;
  nodeId: number;
  nodeTitle: string;
  hasChildren: boolean;
  onAddChild: (parentId: number) => void;
  onRename: (nodeId: number, currentTitle: string) => void;
  onMove: (nodeId: number, newParentId: number | null) => void;
  onDelete: (nodeId: number, hasChildren: boolean) => void;
  onClose: () => void;
}

export function TreeNodeMenu({
  x, y, nodeId, nodeTitle, hasChildren,
  onAddChild, onRename, onMove, onDelete, onClose,
}: TreeNodeMenuProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [onClose]);

  const adjustedX = Math.min(x, window.innerWidth - 180);
  const adjustedY = Math.min(y, window.innerHeight - 200);

  return (
    <div
      ref={ref}
      className="fixed z-50 bg-th-bg-secondary border border-th-border rounded-lg shadow-lg py-1 min-w-[160px]"
      style={{ left: adjustedX, top: adjustedY }}
    >
      <button
        onClick={() => { onAddChild(nodeId); onClose(); }}
        className="w-full text-left px-3 py-1.5 text-sm text-th-text-primary hover:bg-th-bg-tertiary flex items-center gap-2"
      >
        <span className="text-th-text-muted">+</span> 添加子页面
      </button>
      <button
        onClick={() => { onRename(nodeId, nodeTitle); onClose(); }}
        className="w-full text-left px-3 py-1.5 text-sm text-th-text-primary hover:bg-th-bg-tertiary flex items-center gap-2"
      >
        <span className="text-th-text-muted">✎</span> 重命名
      </button>
      <button
        onClick={() => { onMove(nodeId, null); onClose(); }}
        className="w-full text-left px-3 py-1.5 text-sm text-th-text-primary hover:bg-th-bg-tertiary flex items-center gap-2"
      >
        <span className="text-th-text-muted">↗</span> 移动到...
      </button>
      <div className="border-t border-th-border my-1" />
      <button
        onClick={() => { onDelete(nodeId, hasChildren); onClose(); }}
        className="w-full text-left px-3 py-1.5 text-sm text-red-600 hover:bg-red-50 flex items-center gap-2"
      >
        <span>🗑</span> {hasChildren ? "删除子树" : "删除页面"}
      </button>
    </div>
  );
}
