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
    <div className="h-full overflow-y-auto p-4 bg-gray-50">
      <div className="text-sm font-medium text-gray-500 mb-3">知识库</div>
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
        <div className="text-center text-gray-400 mt-8 text-sm">
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
    published: 'text-green-600',
    draft: 'text-yellow-600',
    empty: 'text-gray-400',
  }[node.content_status] || 'text-gray-400';

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
        className={`flex items-center gap-1 px-2 py-1.5 rounded cursor-pointer hover:bg-gray-100 transition-colors ${
          isSelected ? 'bg-blue-100 text-blue-700' : ''
        }`}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        onClick={handleClick}
      >
        {hasChildren ? (
          <button
            onClick={handleToggle}
            className="w-4 h-4 flex items-center justify-center text-gray-400 hover:text-gray-600 shrink-0"
          >
            {expanded ? '−' : '+'}
          </button>
        ) : (
          <span className="w-4 shrink-0" />
        )}
        <span className={`flex-1 text-sm truncate ${isSelected ? '' : statusColor}`}>
          {node.title}
        </span>
        {node.page_type === 'overview' && (
          <span className="text-xs text-gray-400 shrink-0">概览</span>
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
