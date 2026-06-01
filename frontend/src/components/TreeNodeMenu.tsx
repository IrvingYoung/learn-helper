import { useEffect, useRef } from "react";

interface TreeNodeMenuProps {
  x: number;
  y: number;
  nodeId: number;
  hasChildren: boolean;
  onAddChild: (parentId: number) => void;
  onStartRename: (nodeId: number) => void;
  onAskAIMove: (nodeId: number) => void;
  onDelete: (nodeId: number, hasChildren: boolean) => void;
  onClose: () => void;
}

export function TreeNodeMenu({
  x, y, nodeId, hasChildren,
  onAddChild, onStartRename, onAskAIMove, onDelete, onClose,
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

  const adjustedX = Math.min(x, window.innerWidth - 200);
  const adjustedY = Math.min(y, window.innerHeight - 220);

  return (
    <div
      ref={ref}
      className="fixed z-50 bg-th-bg-elevated border border-th-border rounded-lg shadow-th-float py-1 min-w-[180px] animate-spring-in"
      style={{ left: adjustedX, top: adjustedY }}
    >
      <MenuButton
        onClick={() => { onAddChild(nodeId); onClose(); }}
        icon={
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M12 4v16m8-8H4" />
        }
        label="添加子页面"
      />
      <MenuButton
        onClick={() => { onStartRename(nodeId); onClose(); }}
        icon={
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
        }
        label="重命名"
      />
      <MenuButton
        onClick={() => { onAskAIMove(nodeId); onClose(); }}
        icon={
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
        }
        label="移动到…"
      />
      <div className="border-t border-th-separator my-1" />
      <MenuButton
        onClick={() => { onDelete(nodeId, hasChildren); onClose(); }}
        icon={
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        }
        label={hasChildren ? "删除子树" : "删除页面"}
        danger
      />
    </div>
  );
}

function MenuButton({
  onClick, icon, label, danger = false,
}: {
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
  danger?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      className={`w-full text-left px-3 py-1.5 text-[13px] flex items-center gap-2.5 transition-colors ${
        danger
          ? 'text-th-danger hover:bg-th-danger-bg'
          : 'text-th-text-primary hover:bg-th-hover'
      }`}
    >
      <svg
        className={`w-3.5 h-3.5 shrink-0 ${danger ? 'text-th-danger' : 'text-th-text-muted'}`}
        fill="none" stroke="currentColor" viewBox="0 0 24 24"
      >
        {icon}
      </svg>
      {label}
    </button>
  );
}
