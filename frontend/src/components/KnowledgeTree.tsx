import { useState, useEffect, useRef } from 'react';
import type { WikiTreeNode } from '../types';
import { TreeNodeMenu } from './TreeNodeMenu';

interface KnowledgeTreeProps {
  tree: WikiTreeNode[];
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  collapsed: boolean;
  onAddChild?: (parentId: number) => void;
  onRename?: (nodeId: number, currentTitle: string) => void;
  onMove?: (nodeId: number, newParentId: number | null) => void;
  onDelete?: (nodeId: number, hasChildren: boolean) => void;
}

export function KnowledgeTree({ tree, selectedSlug, onSelect, collapsed, onAddChild, onRename, onMove, onDelete }: KnowledgeTreeProps) {
  const [menuState, setMenuState] = useState<{
    x: number; y: number; nodeId: number; nodeTitle: string; hasChildren: boolean;
  } | null>(null);

  const handleContextMenu = (e: React.MouseEvent, node: WikiTreeNode) => {
    e.preventDefault();
    e.stopPropagation();
    setMenuState({
      x: e.clientX,
      y: e.clientY,
      nodeId: node.id,
      nodeTitle: node.title,
      hasChildren: !!(node.children && node.children.length > 0),
    });
  };

  if (collapsed) return null;

  return (
    <div className="h-full overflow-y-auto p-4 bg-th-bg-tertiary custom-scroll">
      <div className="text-sm font-medium text-th-text-muted mb-3 tracking-wide uppercase text-xs">知识库</div>
      <div className="space-y-0.5">
        {tree.map((node) => (
          <TreeNode
            key={node.id}
            node={node}
            selectedSlug={selectedSlug}
            onSelect={onSelect}
            depth={0}
            onContextMenu={handleContextMenu}
            onAddChild={onAddChild}
            onRename={onRename}
          />
        ))}
      </div>
      {tree.length === 0 && (
        <div className="text-center text-th-text-muted mt-12 space-y-2">
          <svg className="w-8 h-8 mx-auto opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 19.477 5.754 20 7.5 20s3.332-.477 4.5-1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 19.477 18.247 20 16.5 20a3.5 3.5 0 01-3.5-3.5" />
          </svg>
          <p className="text-sm">知识库为空</p>
          <p className="text-xs opacity-60">和 AI 对话创建知识树</p>
        </div>
      )}
      {menuState && onAddChild && onRename && onMove && onDelete && (
        <TreeNodeMenu
          x={menuState.x}
          y={menuState.y}
          nodeId={menuState.nodeId}
          nodeTitle={menuState.nodeTitle}
          hasChildren={menuState.hasChildren}
          onAddChild={onAddChild}
          onRename={onRename}
          onMove={onMove}
          onDelete={onDelete}
          onClose={() => setMenuState(null)}
        />
      )}
    </div>
  );
}

interface TreeNodeProps {
  node: WikiTreeNode;
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  depth: number;
  onContextMenu?: (e: React.MouseEvent, node: WikiTreeNode) => void;
  onAddChild?: (parentId: number) => void;
  onRename?: (nodeId: number, currentTitle: string) => void;
}

function TreeNode({ node, selectedSlug, onSelect, depth, onContextMenu, onAddChild, onRename }: TreeNodeProps) {
  const [expanded, setExpanded] = useState(depth < 2);
  const [hovered, setHovered] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState(node.title);
  const inputRef = useRef<HTMLInputElement>(null);
  const hasChildren = node.children && node.children.length > 0;
  const isSelected = node.slug === selectedSlug;

  const statusColor = {
    published: 'bg-th-node-filled',
    draft: 'bg-th-node-partial',
    empty: 'bg-th-node-empty',
  }[node.content_status] || 'bg-th-node-empty';

  const handleClick = () => {
    onSelect(node.slug);
  };

  const handleToggle = (e: React.MouseEvent) => {
    e.stopPropagation();
    setExpanded(!expanded);
  };

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editing]);

  const handleDoubleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (node.page_type === 'overview') return;
    setEditing(true);
    setEditTitle(node.title);
  };

  return (
    <div>
      <div
        className={`flex items-center gap-1.5 px-2 py-1.5 rounded-md cursor-pointer transition-all duration-150 ${
          isSelected
            ? 'bg-th-accent-bg text-th-accent font-medium shadow-sm'
            : 'text-th-text-primary hover:bg-th-bg-primary hover:text-th-text-primary'
        }`}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        onClick={handleClick}
        onContextMenu={(e) => onContextMenu?.(e, node)}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        {hasChildren ? (
          <button
            onClick={handleToggle}
            className="w-4 h-4 flex items-center justify-center text-th-text-muted hover:text-th-text-secondary shrink-0 transition-transform duration-200"
            style={{ transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)' }}
          >
            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </button>
        ) : (
          <span className="w-4 shrink-0" />
        )}
        <span className={`w-2 h-2 rounded-full ${statusColor} shrink-0 transition-transform duration-150 ${isSelected ? 'scale-125' : ''}`} />
        {editing ? (
          <input
            ref={inputRef}
            type="text"
            value={editTitle}
            onChange={(e) => setEditTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && editTitle.trim() && editTitle.trim() !== node.title) {
                onRename?.(node.id, editTitle.trim());
                setEditing(false);
              }
              if (e.key === 'Escape') {
                setEditing(false);
                setEditTitle(node.title);
              }
            }}
            onBlur={() => {
              setEditing(false);
              setEditTitle(node.title);
            }}
            className="flex-1 text-sm bg-th-bg-primary border border-th-accent rounded px-1 py-0 outline-none"
            onClick={(e) => e.stopPropagation()}
          />
        ) : (
          <span className={`flex-1 text-sm truncate ${isSelected ? 'font-medium' : ''}`} onDoubleClick={handleDoubleClick}>
            {node.title}
          </span>
        )}
        {node.page_type === 'overview' && (
          <span className="text-xs text-th-text-muted shrink-0 font-mono tracking-tight">概览</span>
        )}
        {hovered && onContextMenu && (
          <button
            onClick={(e) => { e.stopPropagation(); onContextMenu(e, node); }}
            className="w-4 h-4 flex items-center justify-center text-th-text-muted hover:text-th-text-secondary shrink-0"
          >
            ⋯
          </button>
        )}
      </div>
      {expanded && hasChildren && (
        <div className="transition-all duration-200">
          {node.children!.map((child) => (
            <TreeNode
              key={child.id}
              node={child}
              selectedSlug={selectedSlug}
              onSelect={onSelect}
              depth={depth + 1}
              onContextMenu={onContextMenu}
              onAddChild={onAddChild}
              onRename={onRename}
            />
          ))}
        </div>
      )}
      {expanded && onAddChild && (
        <button
          onClick={() => onAddChild(node.id)}
          className="flex items-center gap-1.5 px-2 py-1 rounded-md text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-primary transition-colors border border-dashed border-transparent hover:border-th-border"
          style={{ paddingLeft: `${(depth + 1) * 16 + 8}px` }}
        >
          <span className="w-4 shrink-0" />
          <span className="text-xs">+ 添加子页面</span>
        </button>
      )}
    </div>
  );
}