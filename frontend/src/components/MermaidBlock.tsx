import { useEffect, useId, useState } from 'react';
import mermaid from 'mermaid';
import { useTheme } from '../contexts/ThemeContext';

type MermaidTheme = 'default' | 'dark';

const THEME_MAP: Record<'warm' | 'dark', MermaidTheme> = {
  warm: 'default',
  dark: 'dark',
};

function configureMermaid(theme: 'warm' | 'dark') {
  mermaid.initialize({
    startOnLoad: false,
    theme: THEME_MAP[theme],
    securityLevel: 'strict',
    fontFamily: 'inherit',
  });
}

interface MermaidBlockProps {
  code: string;
}

export function MermaidBlock({ code }: MermaidBlockProps) {
  const rawId = useId();
  const renderId = `mermaid-${rawId.replace(/:/g, '')}`;
  const { theme } = useTheme();
  const [svg, setSvg] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    configureMermaid(theme);
    mermaid
      .render(renderId, code)
      .then(({ svg }) => {
        if (!cancelled) {
          setSvg(svg);
          setError(null);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [code, renderId, theme]);

  if (error) {
    return (
      <div className="rounded-md border border-dashed border-th-border bg-th-bg-tertiary p-3 text-xs">
        <div className="mb-2 font-medium text-th-text-primary">
          Mermaid 渲染失败
        </div>
        <pre className="mermaid-error-detail font-mono whitespace-pre-wrap text-[0.75rem] text-th-text-muted">
          {error}
        </pre>
        <pre className="mt-2 font-mono whitespace-pre-wrap text-[0.75rem] text-th-text-secondary border-t border-th-separator pt-2">
          {code}
        </pre>
      </div>
    );
  }

  if (!svg) {
    return (
      <div className="mermaid-loading rounded-md border border-th-separator bg-th-bg-tertiary p-6 text-center text-xs text-th-text-muted">
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-th-accent animate-pulse-dot mr-2 align-middle" />
        正在渲染图表…
      </div>
    );
  }

  return (
    <div
      className="mermaid-block rounded-md border border-th-separator bg-th-bg-tertiary p-4 overflow-x-auto flex justify-center"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
