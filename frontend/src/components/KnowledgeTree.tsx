import { useState, useEffect, useRef, useMemo } from 'react';
import type { WikiTreeNode } from '../types';
import { TreeNodeMenu } from './TreeNodeMenu';

function filterTreeByTitle(nodes: WikiTreeNode[], query: string): WikiTreeNode[] {
  const lower = query.toLowerCase();
  const match = (node: WikiTreeNode) => node.title.toLowerCase().includes(lower);

  function walk(list: WikiTreeNode[]): WikiTreeNode[] {
    return list.reduce<WikiTreeNode[]>((acc, node) => {
      const nodeMatches = match(node);
      const filteredChildren = node.children ? walk(node.children) : undefined;

      if (nodeMatches) {
        // Keep entire subtree
        acc.push(node);
      } else if (filteredChildren && filteredChildren.length > 0) {
        // Keep as ancestor of matching descendants
        acc.push({ ...node, children: filteredChildren });
      }
      return acc;
    }, []);
  }

  return walk(nodes);
}

interface KnowledgeTreeProps {
  tree: WikiTreeNode[];
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  collapsed: boolean;
  onAddChild?: (parentId: number | null) => void;
  onRename?: (nodeId: number, newTitle: string) => void;
  onMove?: (nodeId: number, newParentId: number | null) => void;
  onAskAIMove?: (nodeId: number) => void;
  onDelete?: (nodeId: number, hasChildren: boolean) => void;
  newNodeId?: number | null;
}

export function KnowledgeTree({
  tree, selectedSlug, onSelect, collapsed,
  onAddChild, onRename, onMove, onAskAIMove, onDelete, newNodeId,
}: KnowledgeTreeProps) {
  const [menuState, setMenuState] = useState<{
    x: number; y: number; nodeId: number; nodeTitle: string; hasChildren: boolean;
  } | null>(null);
  const [renameNodeId, setRenameNodeId] = useState<number | null>(null);
  const [draggedId, setDraggedId] = useState<number | null>(null);
  const [dropHoverId, setDropHoverId] = useState<number | null>(null);
  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set());
  const [searchQuery, setSearchQuery] = useState('');
  const savedExpandedIds = useRef<Set<number> | null>(null);
  const prevNewNodeId = useRef<number | null>(null);
  const treeRef = useRef(tree);
  treeRef.current = tree;
  const selectedSlugRef = useRef(selectedSlug);
  selectedSlugRef.current = selectedSlug;
  const effectiveTree = searchQuery ? filterTreeByTitle(tree, searchQuery) : tree;

  // Auto-expand ancestors of the selected page
  useEffect(() => {
    if (!selectedSlug || !tree) return;
    const ancestors: number[] = [];
    const findPath = (nodes: WikiTreeNode[]): boolean => {
      for (const n of nodes) {
        if (n.slug === selectedSlug) return true;
        if (n.children && findPath(n.children)) {
          ancestors.push(n.id);
          return true;
        }
      }
      return false;
    };
    findPath(tree);
    if (ancestors.length > 0) {
      setExpandedIds(prev => {
        const next = new Set(prev);
        ancestors.forEach(id => next.add(id));
        return next;
      });
    }
  }, [selectedSlug, tree]);

  // Save/restore expandedIds for search
  useEffect(() => {
    if (searchQuery) {
      if (!savedExpandedIds.current) {
        savedExpandedIds.current = new Set(expandedIds);
      }
      const ancestors = new Set<number>();
      const collectAncestors = (nodes: WikiTreeNode[], parentIds: number[]) => {
        for (const node of nodes) {
          if (node.title.toLowerCase().includes(searchQuery.toLowerCase())) {
            parentIds.forEach(id => ancestors.add(id));
          }
          if (node.children) {
            collectAncestors(node.children, [...parentIds, node.id]);
          }
        }
      };
      collectAncestors(treeRef.current, []);
      if (ancestors.size > 0) {
        setExpandedIds(prev => {
          const next = new Set(prev);
          ancestors.forEach(id => next.add(id));
          return next;
        });
      }
    } else {
      if (savedExpandedIds.current) {
        const restored = savedExpandedIds.current;
        savedExpandedIds.current = null;

        // Also expand ancestors of the currently selected page
        const slug = selectedSlugRef.current;
        if (slug) {
          const findAncestors = (nodes: WikiTreeNode[], acc: number[]): number[] => {
            for (const n of nodes) {
              if (n.slug === slug) return acc;
              if (n.children) {
                const found = findAncestors(n.children, [...acc, n.id]);
                if (found.length > 0) return found;
              }
            }
            return [];
          };
          const ancestors = findAncestors(treeRef.current, []);
          ancestors.forEach(id => restored.add(id));
        }

        setExpandedIds(restored);
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchQuery]);

  // When a new node is created from parent, trigger rename mode
  useEffect(() => {
    if (newNodeId != null && newNodeId !== prevNewNodeId.current) {
      prevNewNodeId.current = newNodeId;
      setRenameNodeId(newNodeId);
    }
  }, [newNodeId]);

  const handleToggle = (nodeId: number) => {
    setExpandedIds(prev => {
      const next = new Set(prev);
      if (next.has(nodeId)) next.delete(nodeId);
      else next.add(nodeId);
      return next;
    });
  };

  const getDescendantIds = (parentId: number, nodes: WikiTreeNode[]): Set<number> => {
    const ids = new Set<number>();
    const findAndCollect = (ns: WikiTreeNode[]): void => {
      for (const n of ns) {
        if (n.id === parentId) {
          const collect = (node: WikiTreeNode): void => {
            ids.add(node.id);
            node.children?.forEach(collect);
          };
          n.children?.forEach(collect);
          return;
        }
        if (n.children) findAndCollect(n.children);
      }
    };
    findAndCollect(nodes);
    return ids;
  };

  const draggedDescendants = useMemo(
    () => (draggedId ? getDescendantIds(draggedId, tree) : new Set<number>()),
    [draggedId, tree]
  );

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

  const handleRootDragOver = (e: React.DragEvent) => {
    if (draggedId === null) return;
    e.preventDefault();
    e.stopPropagation();
    e.dataTransfer.dropEffect = "move";
    setDropHoverId(-1);
  };

  const handleRootDragLeave = (e: React.DragEvent) => {
    // Only clear if actually leaving the container (not entering a child)
    if (e.currentTarget === e.target || !e.currentTarget.contains(e.relatedTarget as Node)) {
      setDropHoverId(null);
    }
  };

  const handleRootDrop = (e: React.DragEvent) => {
    if (draggedId === null) return;
    e.preventDefault();
    e.stopPropagation();
    setDropHoverId(null);
    const draggedNodeId = parseInt(e.dataTransfer.getData("text/plain"), 10);
    if (onMove) {
      onMove(draggedNodeId, null);
    }
    setDraggedId(null);
  };

  if (collapsed) return null;

  const isDropRoot = draggedId !== null && dropHoverId === -1;

  return (
    <div className="h-full overflow-y-auto p-4 bg-th-bg-tertiary custom-scroll">
      <div className="flex items-center justify-between mb-3">
        <div className="text-sm font-medium text-th-text-muted tracking-wide uppercase text-xs">知识库</div>
        {onAddChild && (
          <button
            onClick={() => onAddChild(null)}
            className="w-5 h-5 flex items-center justify-center text-th-text-muted hover:text-th-text-primary hover:bg-th-hover rounded transition-colors"
            title="新建顶层页面"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
          </button>
        )}
      </div>
      <div className="relative mb-2">
        <svg className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-th-text-muted pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-4.35-4.35M11 19a8 8 0 100-16 8 8 0 000 16z" />
        </svg>
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="搜索页面名称..."
          className="w-full pl-8 pr-7 py-1.5 text-sm bg-th-bg-primary border border-th-border rounded-md placeholder-th-text-muted/60 text-th-text-primary outline-none focus:border-th-accent transition-colors"
        />
        {searchQuery && (
          <button
            onClick={() => setSearchQuery('')}
            className="absolute right-2 top-1/2 -translate-y-1/2 w-4 h-4 flex items-center justify-center text-th-text-muted hover:text-th-text-secondary"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        )}
      </div>
      <div
        className={`space-y-0.5 min-h-[60px] rounded-md transition-all duration-150 ${
          isDropRoot ? 'bg-th-bg-primary outline-2 outline-dashed outline-th-accent/40 -outline-offset-2' : ''
        }`}
        onDragOver={handleRootDragOver}
        onDragLeave={handleRootDragLeave}
        onDrop={handleRootDrop}
      >
        {effectiveTree.map((node) => (
          <TreeNode
            key={node.id}
            node={node}
            selectedSlug={selectedSlug}
            onSelect={onSelect}
            depth={0}
            expandedIds={expandedIds}
            onToggle={handleToggle}
            onContextMenu={searchQuery ? undefined : handleContextMenu}
            onAddChild={onAddChild}
            onRename={onRename}
            onMove={onMove}
            renameNodeId={renameNodeId}
            onRenameStarted={() => setRenameNodeId(null)}
            draggedId={draggedId}
            draggedDescendants={draggedDescendants}
            onDragStart={(id) => setDraggedId(id)}
            onDragEnd={() => { setDraggedId(null); setDropHoverId(null); }}
            dropHoverId={dropHoverId}
            onDropHover={setDropHoverId}
            disabled={!!searchQuery}
          />
        ))}
        {effectiveTree.length === 0 && (
          <div className="text-center text-th-text-muted mt-10 px-4 space-y-3">
            <div className="w-10 h-10 mx-auto rounded-full border border-dashed border-th-border flex items-center justify-center">
              <svg className="w-4 h-4 text-th-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 19.477 5.754 20 7.5 20s3.332-.477 4.5-1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 19.477 18.247 20 16.5 20a3.5 3.5 0 01-3.5-3.5" />
              </svg>
            </div>
            <div className="space-y-1">
              <p className="text-sm font-medium text-th-text-secondary">{searchQuery ? '未找到匹配的页面' : '知识库还是空的'}</p>
              <p className="text-xs leading-relaxed">{searchQuery ? '尝试其他关键词' : '和 AI 聊聊你想学的领域，它会帮你把知识整理成一棵树'}</p>
            </div>
          </div>
        )}
      </div>
      {!searchQuery && menuState && onAddChild && onRename && onMove && onAskAIMove && onDelete && (
        <TreeNodeMenu
          x={menuState.x}
          y={menuState.y}
          nodeId={menuState.nodeId}
          hasChildren={menuState.hasChildren}
          onAddChild={onAddChild}
          onStartRename={(nodeId) => setRenameNodeId(nodeId)}
          onAskAIMove={onAskAIMove}
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
  expandedIds: Set<number>;
  onToggle: (nodeId: number) => void;
  onContextMenu?: (e: React.MouseEvent, node: WikiTreeNode) => void;
  onAddChild?: (parentId: number | null) => void;
  onRename?: (nodeId: number, newTitle: string) => void;
  onMove?: (nodeId: number, newParentId: number | null) => void;
  renameNodeId?: number | null;
  onRenameStarted?: () => void;
  draggedId?: number | null;
  draggedDescendants?: Set<number>;
  onDragStart?: (id: number) => void;
  onDragEnd?: () => void;
  dropHoverId?: number | null;
  onDropHover?: (id: number | null) => void;
  disabled?: boolean;
}


function TreeNode({ node, selectedSlug, onSelect, depth, expandedIds, onToggle, onContextMenu, onAddChild, onRename, onMove, renameNodeId, onRenameStarted, draggedId, draggedDescendants, onDragStart, onDragEnd, dropHoverId, onDropHover, disabled = false }: TreeNodeProps) {
  const [hovered, setHovered] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState(node.title);
  const inputRef = useRef<HTMLInputElement>(null);
  const hasChildren = node.children && node.children.length > 0;
  const isSelected = node.slug === selectedSlug;
  const expanded = expandedIds.has(node.id);
  const isDropTarget = draggedId !== null && dropHoverId === node.id && draggedId !== node.id;

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
    onToggle(node.id);
  };

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [editing]);

  useEffect(() => {
    if (renameNodeId === node.id && !editing) {
      setEditing(true);
      setEditTitle(node.title);
      onRenameStarted?.();
    }
  }, [renameNodeId, node.id, node.title, editing, onRenameStarted]);

  const handleDoubleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (node.page_type === 'overview') return;
    setEditing(true);
    setEditTitle(node.title);
  };

  const handleDragStart = (e: React.DragEvent) => {
    if (disabled) return;
    e.dataTransfer.setData("text/plain", String(node.id));
    e.dataTransfer.effectAllowed = "move";
    onDragStart?.(node.id);
  };

  const handleDragOver = (e: React.DragEvent) => {
    if (disabled) return;
    if (draggedId === node.id || draggedDescendants?.has(node.id)) {
      e.dataTransfer.dropEffect = "none";
      return;
    }
    e.preventDefault();
    e.stopPropagation();
    e.dataTransfer.dropEffect = "move";
    onDropHover?.(node.id);
  };

  const handleDragLeave = () => {
    onDropHover?.(null);
  };

  const handleDrop = (e: React.DragEvent) => {
    if (disabled) return;
    e.preventDefault();
    e.stopPropagation();
    onDropHover?.(null);
    const draggedNodeId = parseInt(e.dataTransfer.getData("text/plain"), 10);
    if (draggedNodeId !== node.id && !draggedDescendants?.has(node.id) && onMove) {
      onMove(draggedNodeId, node.id);
    }
    onDragEnd?.();
  };

  const handleDragEnd = () => {
    onDragEnd?.();
  };

  return (
    <div className="relative">
      <div
        className={`group flex items-center gap-1.5 px-2 py-[5px] rounded-md cursor-pointer transition-colors duration-150 ${
          isSelected
            ? 'text-th-text-primary'
            : isDropTarget
            ? 'bg-th-bg-primary text-th-text-primary ring-2 ring-th-accent/50'
            : 'text-th-text-secondary hover:text-th-text-primary hover:bg-th-hover'
        }`}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        onClick={handleClick}
        onContextMenu={(e) => onContextMenu?.(e, node)}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        draggable={!disabled}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onDragEnd={handleDragEnd}
      >
        {/* Left accent bar for selected state */}
        {isSelected && (
          <span className="absolute left-0 top-1.5 bottom-1.5 w-[2px] bg-th-accent rounded-r" />
        )}
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
        <span className={`w-1.5 h-1.5 rotate-45 ${statusColor} shrink-0 transition-transform duration-200 ${isSelected ? 'scale-125' : ''}`} />
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
          <span className={`flex-1 text-[13.5px] truncate leading-snug ${isSelected ? 'font-medium text-th-text-primary' : ''}`} onDoubleClick={handleDoubleClick}>
            {node.title}
          </span>
        )}
        {node.page_type === 'overview' && (
          <span className="text-[10px] text-th-text-muted shrink-0 font-mono tracking-tight opacity-70">home</span>
        )}
        {hovered && onContextMenu && (
          <button
            onClick={(e) => { e.stopPropagation(); onContextMenu(e, node); }}
            className="w-5 h-5 flex items-center justify-center text-th-text-muted hover:text-th-text-primary shrink-0 rounded transition-colors"
          >
            <svg className="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24">
              <circle cx="5" cy="12" r="1.5" />
              <circle cx="12" cy="12" r="1.5" />
              <circle cx="19" cy="12" r="1.5" />
            </svg>
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
              expandedIds={expandedIds}
              onToggle={onToggle}
              onContextMenu={onContextMenu}
              onAddChild={onAddChild}
              onRename={onRename}
              onMove={onMove}
              renameNodeId={renameNodeId}
              onRenameStarted={onRenameStarted}
              draggedId={draggedId}
              draggedDescendants={draggedDescendants}
              onDragStart={onDragStart}
              onDragEnd={onDragEnd}
              dropHoverId={dropHoverId}
              onDropHover={onDropHover}
              disabled={disabled}
            />
          ))}
        </div>
      )}
    </div>
  );
}
