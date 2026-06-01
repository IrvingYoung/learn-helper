import { memo } from "react";
import ReactMarkdown from 'react-markdown'
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
  onWikiLinkClick?: (title: string) => void
  compact?: boolean
}

function processWikiLinks(content: string): string {
  return content.replace(/\[\[([^\]]+)\]\]/g, (_, title) => {
    return `[${title}](wiki:${encodeURIComponent(title)})`
  })
}

export const MarkdownContent = memo(function MarkdownContent({
  content, className = '', onWikiLinkClick, compact = false,
}: MarkdownContentProps) {
  const { theme } = useTheme()
  const syntaxStyle = theme === 'dark' ? oneDark : oneLight
  const processedContent = processWikiLinks(content)

  if (!content) return null
  return (
    <div className={`prose-custom ${compact ? 'prose-custom--compact' : ''} ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
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
            return (
              <SyntaxHighlighter
                style={syntaxStyle}
                language={match[1]}
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
                {String(children).replace(/\n$/, '')}
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
            if (href?.startsWith('wiki:')) {
              return (
                <a
                  href={href}
                  onClick={(e) => {
                    e.preventDefault()
                    const title = decodeURIComponent(href.slice(5))
                    onWikiLinkClick?.(title)
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
