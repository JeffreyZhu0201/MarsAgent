import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { WikiTree } from '@/components/WikiTree'
import { WikiSearch } from '@/components/WikiSearch'

interface WikiDoc {
  slug: string
  title: string
  category: string
  source: string
  url: string
  updated_at: string
}

export function WikiBrowser() {
  const [docs, setDocs] = useState<WikiDoc[]>([])
  const [selected, setSelected] = useState<WikiDoc | null>(null)
  const [content, setContent] = useState<string>('')

  useEffect(() => {
    fetch('/api/wiki/tree')
      .then(r => r.json())
      .then(d => setDocs(d.docs || []))
      .catch(console.error)
  }, [])

  function handleSelect(doc: WikiDoc) {
    setSelected(doc)
    fetch(`/api/wiki/doc/${encodeURIComponent(doc.slug)}`)
      .then(r => r.json())
      .then(d => {
        const body = d.content || `**Source:** [${d.url}](${d.url})\n\n*(Content from MinIO loads in M3)*\n\n---\n\n## ${d.title}\n\n---\n\n`
        setContent(`# ${d.title}\n\n*Source:* [${d.url}](${d.url})\n\n${body}`)
      })
      .catch(() => setContent(`# ${doc.title}\n\n*Failed to load content.*`))
  }

  function handleSearch(q: string) {
    if (!q) {
      fetch('/api/wiki/tree')
        .then(r => r.json())
        .then(d => setDocs(d.docs || []))
        .catch(console.error)
      return
    }
    fetch('/api/wiki/search', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ q, k: 20 }),
    })
      .then(r => r.json())
      .then(d => {
        const seen = new Set<string>()
        const hits: any[] = d.hits || []
        const unique = hits.filter((h: any) => {
          if (seen.has(h.doc_id)) return false
          seen.add(h.doc_id); return true
        })
        setDocs(unique.map((h: any) => ({
          slug: h.doc_id,
          title: h.payload?.text?.slice(0, 60) || h.url || '',
          category: h.payload?.category || 'general',
          source: h.payload?.source || '',
          url: h.payload?.url || '',
          updated_at: '',
        })))
      })
      .catch(console.error)
  }

  return (
    <div className="flex h-[calc(100vh-4rem)]">
      <aside className="w-64 border-r overflow-y-auto p-4 flex flex-col">
        <WikiSearch onSearch={handleSearch} />
        <WikiTree docs={docs} selected={selected} onSelect={handleSelect} />
      </aside>
      <main className="flex-1 overflow-y-auto p-8">
        {content ? (
          <article className="prose prose-slate max-w-3xl mx-auto">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
          </article>
        ) : (
          <p className="text-slate-400 text-center mt-20">选择左侧文档开始阅读</p>
        )}
      </main>
    </div>
  )
}
