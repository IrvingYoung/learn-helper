import { useState } from "react";
import { inlineDiff, type DiffLine } from "../lib/diff";
import { MarkdownContent } from "./MarkdownContent";
import type { AskUserContext as AskUserContextT } from "../types";

interface OutlineNode {
  id?: string;
  title: string;
  page_type?: string;
  children?: OutlineNode[];
}

interface DiffEntry {
  page_id: number;
  before: string;
  after: string;
  label?: string;
}

interface Props {
  context: AskUserContextT;
}

function OutlineTree({ nodes, depth = 0 }: { nodes: OutlineNode[]; depth?: number }) {
  return (
    <ul className="text-sm">
      {nodes.map((n, i) => (
        <li key={n.id ?? `${depth}-${i}`} className="py-0.5" style={{ paddingLeft: depth * 16 }}>
          <span className="font-medium">{n.title}</span>
          {n.page_type && <span className="ml-2 text-xs text-th-text-muted">({n.page_type})</span>}
          {n.children && n.children.length > 0 && <OutlineTree nodes={n.children} depth={depth + 1} />}
        </li>
      ))}
    </ul>
  );
}

function DiffView({ diffs }: { diffs: DiffEntry[] }) {
  const [active, setActive] = useState(0);
  const d = diffs[active];
  const lines: DiffLine[] = inlineDiff(d.before, d.after);
  return (
    <div>
      {diffs.length > 1 && (
        <div className="flex gap-1 mb-2 border-b border-th-border">
          {diffs.map((x, i) => (
            <button
              key={x.page_id}
              onClick={() => setActive(i)}
              className={`text-xs px-2 py-1 ${
                i === active ? "border-b-2 border-th-accent" : "text-th-text-muted"
              }`}
            >
              {x.label ?? `Page ${x.page_id}`}
            </button>
          ))}
        </div>
      )}
      <pre className="text-xs font-mono whitespace-pre-wrap max-h-96 overflow-y-auto">
        {lines.map((l, i) => (
          <div
            key={i}
            className={
              l.type === "add"
                ? "bg-green-100 dark:bg-green-950 text-green-900 dark:text-green-100"
                : l.type === "del"
                ? "bg-red-100 dark:bg-red-950 text-red-900 dark:text-red-100 line-through"
                : "text-th-text-muted"
            }
          >
            {l.type === "add" ? "+ " : l.type === "del" ? "- " : "  "}{l.text || " "}
          </div>
        ))}
      </pre>
    </div>
  );
}

function PagePreview({ pageId }: { pageId: number }) {
  // Lazy fetch: simple SWR-style. For single-user, a useEffect is fine.
  const [content, setContent] = useState<string | null>(null);
  if (content === null) {
    fetch(`/api/pages/${pageId}`)
      .then(r => r.ok ? r.json() : Promise.reject(r.status))
      .then((p: any) => setContent(p.content ?? ""))
      .catch(() => setContent("(加载失败)"));
  }
  return (
    <div className="text-sm">
      {content === null ? <span className="text-th-text-muted">加载中...</span> : (
        <MarkdownContent content={content.slice(0, 500) + (content.length > 500 ? "..." : "")} />
      )}
    </div>
  );
}

export function AskUserContextView({ context }: Props) {
  switch (context.kind) {
    case "outline":
      return <OutlineTree nodes={context.data as OutlineNode[]} />;
    case "markdown":
      return <MarkdownContent content={context.data as string} />;
    case "diff":
      return <DiffView diffs={context.data as DiffEntry[]} />;
    case "page":
      return <PagePreview pageId={(context.data as { page_id: number }).page_id} />;
    default:
      return <pre className="text-xs">{JSON.stringify(context, null, 2)}</pre>;
  }
}
