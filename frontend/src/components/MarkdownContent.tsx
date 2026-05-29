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
}

export function MarkdownContent({ content, className = '' }: MarkdownContentProps) {
  const { theme } = useTheme()
  const syntaxStyle = theme === 'dark' ? oneDark : oneLight

  if (!content) return null
  return (
    <div className={`prose prose-sm max-w-none ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          code({ className, children, ...props }) {
            const match = /language-(\w+)/.exec(className || '')
            if (!match) {
              return (
                <code className="bg-th-bg-tertiary text-th-accent px-1 py-0.5 rounded text-sm" {...props}>
                  {children}
                </code>
              )
            }
            return (
              <SyntaxHighlighter
                style={syntaxStyle}
                language={match[1]}
                PreTag="div"
                className="rounded-md !mt-2 !mb-2"
              >
                {String(children).replace(/\n$/, '')}
              </SyntaxHighlighter>
            )
          },
          table({ children }) {
            return (
              <div className="overflow-x-auto">
                <table className="min-w-full border-collapse border border-th-border">{children}</table>
              </div>
            )
          },
          th({ children }) {
            return <th className="border border-th-border bg-th-bg-tertiary px-3 py-1.5 text-left text-sm font-medium text-th-text-secondary">{children}</th>
          },
          td({ children }) {
            return <td className="border border-th-border px-3 py-1.5 text-sm text-th-text-primary">{children}</td>
          },
          blockquote({ children }) {
            return <blockquote className="border-l-4 border-th-accent bg-th-accent-bg pl-4 py-1 my-2 text-sm text-th-text-secondary">{children}</blockquote>
          },
          h1({ children }) {
            return <h1 className="text-xl font-bold mt-4 mb-2 text-th-text-primary">{children}</h1>
          },
          h2({ children }) {
            return <h2 className="text-lg font-bold mt-3 mb-1.5 text-th-text-primary">{children}</h2>
          },
          h3({ children }) {
            return <h3 className="text-base font-semibold mt-2 mb-1 text-th-text-primary">{children}</h3>
          },
          ul({ children }) {
            return <ul className="list-disc list-inside space-y-0.5 my-1 text-th-text-primary">{children}</ul>
          },
          ol({ children }) {
            return <ol className="list-decimal list-inside space-y-0.5 my-1 text-th-text-primary">{children}</ol>
          },
          p({ children }) {
            return <p className="my-1.5 leading-relaxed text-th-text-secondary">{children}</p>
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}