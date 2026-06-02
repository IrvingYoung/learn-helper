import { memo } from "react";
import ReactMarkdown, { defaultUrlTransform } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { PrismLight as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneLight } from 'react-syntax-highlighter/dist/cjs/styles/prism'
import { oneDark } from 'react-syntax-highlighter/dist/cjs/styles/prism'
import python from 'react-syntax-highlighter/dist/cjs/languages/prism/python'
import javascript from 'react-syntax-highlighter/dist/cjs/languages/prism/javascript'
import typescript from 'react-syntax-highlighter/dist/cjs/languages/prism/typescript'
import go from 'react-syntax-highlighter/dist/cjs/languages/prism/go'
import java from 'react-syntax-highlighter/dist/cjs/languages/prism/java'
import cpp from 'react-syntax-highlighter/dist/cjs/languages/prism/cpp'
import sql from 'react-syntax-highlighter/dist/cjs/languages/prism/sql'
import { useTheme } from '../contexts/ThemeContext'
import { MermaidBlock } from './MermaidBlock'

SyntaxHighlighter.registerLanguage('python', python)
SyntaxHighlighter.registerLanguage('javascript', javascript)
SyntaxHighlighter.registerLanguage('typescript', typescript)
SyntaxHighlighter.registerLanguage('go', go)
SyntaxHighlighter.registerLanguage('java', java)
SyntaxHighlighter.registerLanguage('cpp', cpp)
SyntaxHighlighter.registerLanguage('c', cpp)
SyntaxHighlighter.registerLanguage('sql', sql)

interface MarkdownContentProps {
  content: string
  className?: string
  onInternalLink?: (href: string) => void
  compact?: boolean
}

const EXTERNAL_LINK_PATTERN = /^(https?:|mailto:|tel:|ftp:)/i

function processWikiLinks(content: string): string {
  return content.replace(/\[\[([^\]]+)\]\]/g, (_, title) => {
    return `[${title}](wiki:${encodeURIComponent(title)})`
  })
}

const urlTransform = (value: string) =>
  value.startsWith('wiki:') ? value : defaultUrlTransform(value)

export const MarkdownContent = memo(function MarkdownContent({
  content, className = '', onInternalLink, compact = false,
}: MarkdownContentProps) {
  const { theme } = useTheme()
  const syntaxStyle = theme === 'dark' ? oneDark : oneLight
  const processedContent = processWikiLinks(content)

  if (!content) return null
  return (
    <div className={`prose-custom ${compact ? 'prose-custom--compact' : ''} ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        urlTransform={urlTransform}
        components={{
          code({ className, children, ...props }) {
            const match = /language-(\w+)/.exec(className || '')
            if (!match) {
              return (
                <code className="font-mono" {...props}>
                  {children}
                </code>
              )
            }
            const lang = match[1]
            const codeText = String(children).replace(/\n$/, '')
            if (lang === 'mermaid') {
              return <MermaidBlock code={codeText} />
            }
            return (
              <SyntaxHighlighter
                style={syntaxStyle}
                language={lang}
                PreTag="div"
                className="rounded-md"
                customStyle={{
                  fontSize: '0.8125rem',
                  padding: '0.75rem 1rem',
                  margin: 0,
                  background: 'var(--bg-tertiary)',
                  border: '1px solid var(--separator)',
                }}
              >
                {codeText}
              </SyntaxHighlighter>
            )
          },
          table({ children }) {
            return (
              <div className="overflow-x-auto">
                <table>{children}</table>
              </div>
            )
          },
          th({ children }) {
            return <th>{children}</th>
          },
          td({ children }) {
            return <td>{children}</td>
          },
          blockquote({ children }) {
            return <blockquote>{children}</blockquote>
          },
          h1({ children }) {
            return <h1>{children}</h1>
          },
          h2({ children }) {
            return <h2>{children}</h2>
          },
          h3({ children }) {
            return <h3>{children}</h3>
          },
          h4({ children }) {
            return <h4>{children}</h4>
          },
          ul({ children }) {
            return <ul>{children}</ul>
          },
          ol({ children }) {
            return <ol>{children}</ol>
          },
          p({ children }) {
            return <p>{children}</p>
          },
          em({ children }) {
            return <em className="italic text-th-text-primary">{children}</em>
          },
          strong({ children }) {
            return <strong className="font-semibold text-th-text-primary">{children}</strong>
          },
          a({ href, children, ...props }) {
            if (href && !EXTERNAL_LINK_PATTERN.test(href)) {
              // Internal link (e.g. wiki:Title). When the parent doesn't pass
              // an onInternalLink handler — public viewer mode — render as
              // plain text instead of a clickable <a>, since the link target
              // would be inaccessible to anonymous viewers.
              if (!onInternalLink) {
                return <span className="text-th-text-primary">{children}</span>
              }
              return (
                <a
                  href={href}
                  onClick={(e) => {
                    e.preventDefault()
                    onInternalLink?.(href)
                  }}
                  {...props}
                >
                  {children}
                </a>
              )
            }
            return <a href={href} target="_blank" rel="noopener noreferrer" {...props}>{children}</a>
          },
        }}
      >
        {processedContent}
      </ReactMarkdown>
    </div>
  )
})
