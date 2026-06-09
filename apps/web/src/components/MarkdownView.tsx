import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

export function MarkdownView({ content }: { content: string }) {
  return (
    <article className="markdown-glass mx-auto max-w-4xl">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h1: ({ children }) => <h1 className="markdown-h1">{children}</h1>,
          h2: ({ children }) => <h2 className="markdown-h2">{children}</h2>,
          h3: ({ children }) => <h3 className="markdown-h3">{children}</h3>,
          p: ({ children }) => <p className="markdown-p">{children}</p>,
          ul: ({ children }) => <ul className="markdown-ul">{children}</ul>,
          ol: ({ children }) => <ol className="markdown-ol">{children}</ol>,
          li: ({ children }) => <li className="markdown-li">{children}</li>,
          blockquote: ({ children }) => <blockquote className="markdown-quote">{children}</blockquote>,
          a: ({ children, href }) => (
            <a className="markdown-link" href={href} target="_blank" rel="noreferrer">
              {children}
            </a>
          ),
          table: ({ children }) => (
            <div className="my-6 overflow-x-auto rounded-3xl ring-1 ring-white/70">
              <table className="w-full border-collapse bg-white/55 text-sm">{children}</table>
            </div>
          ),
          th: ({ children }) => <th className="border-b border-white/70 bg-white/70 px-4 py-3 text-left font-semibold text-slate-900">{children}</th>,
          td: ({ children }) => <td className="border-b border-white/50 px-4 py-3 text-slate-700">{children}</td>,
          code: ({ children }) => <code className="markdown-inline-code">{children}</code>,
          pre: ({ children }) => <pre className="markdown-pre">{children}</pre>,
        }}
      >
        {content}
      </ReactMarkdown>
    </article>
  )
}
