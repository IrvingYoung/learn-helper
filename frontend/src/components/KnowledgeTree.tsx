import { useState } from 'react';
import type { WikiTreeNode } from '../types';

interface KnowledgeTreeProps {
  tree: WikiTreeNode[];
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  collapsed: boolean;
}

export function KnowledgeTree({ tree, selectedSlug, onSelect, collapsed }: KnowledgeTreeProps) {
  if (collapsed) return null;

  return (
    <div className="h-full overflow-y-auto p-4 bg-th-bg-tertiary custom-scroll">
      <div className="text-sm font-medium text-th-text-muted mb-3">知识库</div>
      <div className="space-y-0.5">
        {tree.map((node) => (
          <TreeNode
            key={node.id}
            node={node}
            selectedSlug={selectedSlug}
            onSelect={onSelect}
            depth={0}
          />
        ))}
      </div>
      {tree.length === 0 && (
        <div className="text-center text-th-text-muted mt-8 text-sm">
          知识库为空，开始和 AI 对话吧
        </div>
      )}
    </div>
  );
}

interface TreeNodeProps {
  node: WikiTreeNode;
  selectedSlug: string | null;
  onSelect: (slug: string) => void;
  depth: number;
}

function TreeNode({ node, selectedSlug, onSelect, depth }: TreeNodeProps) {
  const [expanded, setExpanded] = useState(depth < 2);
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

  return (
    <div>
      <div
        className={`flex items-center gap-1.5 px-2 py-1.5 rounded-md cursor-pointer hover:bg-th-bg-tertiary transition-colors ${
          isSelected ? 'bg-th-accent-bg text-th-accent' : ''
        }`}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        onClick={handleClick}
      >
        {hasChildren ? (
          <button
            onClick={handleToggle}
            className="w-4 h-4 flex items-center justify-center text-th-text-muted hover:text-th-text-secondary shrink-0"
          >
            <svg className={`w-3 h-3 transition-transform ${expanded ? 'rotate-90' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          </button>
        ) : (
          <span className="w-4 shrink-0" />
        )}
        <span className={`w-2 h-2 rounded-full ${statusColor} shrink-0`} />
        <span className={`flex-1 text-sm truncate ${isSelected ? 'font-medium' : 'text-th-text-primary'}`}>
          {node.title}
        </span>
        {node.page_type === 'overview' && (
          <span className="text-xs text-th-text-muted shrink-0">概览</span>
        )}
      </div>
      {expanded && hasChildren && (
        <div>
          {node.children!.map((child) => (
            <TreeNode
              key={child.id}
              node={child}
              selectedSlug={selectedSlug}
              onSelect={onSelect}
              depth={depth + 1}
            />
          ))}
        </div>
      )}
    </div>
  );
}