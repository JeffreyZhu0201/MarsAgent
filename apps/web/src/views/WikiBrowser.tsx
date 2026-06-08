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
    console.debug('[MarsAgent:wiki] loading tree')
    fetch('/api/wiki/tree')
      .then(r => r.json())
      .then(d => {
        console.debug('[MarsAgent:wiki] tree loaded', { count: (d.docs || []).length, docs: d.docs || [] })
        setDocs(d.docs || [])
      })
      .catch((err) => console.error('[MarsAgent:wiki] tree load failed', err))
  }, [])

  function handleSelect(doc: WikiDoc) {
    console.debug('[MarsAgent:wiki] select doc', doc)
    setSelected(doc)
    fetch(`/api/wiki/doc/${encodeURIComponent(doc.slug)}`)
      .then(r => r.json())
      .then(d => {
        console.debug('[MarsAgent:wiki] doc loaded', { slug: doc.slug, doc: d })
        const body = d.content || `**Source:** [${d.url}](${d.url})\n\n*(Content from MinIO loads in M3)*\n\n---\n\n## ${d.title}\n\n---\n\n`
        setContent(`# ${d.title}\n\n*Source:* [${d.url}](${d.url})\n\n${body}`)
      })
      .catch((err) => {
        console.error('[MarsAgent:wiki] doc load failed', { doc, err })
        setContent(`# ${doc.title}\n\n*Failed to load content.*`)
      })
  }

  function handleSearch(q: string) {
    console.debug('[MarsAgent:rag] search requested', { q, k: 20 })
    if (!q) {
      fetch('/api/wiki/tree')
        .then(r => r.json())
        .then(d => {
          console.debug('[MarsAgent:rag] empty query, restored tree', { count: (d.docs || []).length })
          setDocs(d.docs || [])
        })
        .catch((err) => console.error('[MarsAgent:rag] restore tree failed', err))
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
        console.debug('[MarsAgent:rag] raw search hits', { q, count: hits.length, hits })
        const unique = hits.filter((h: any) => {
          if (seen.has(h.doc_id)) return false
          seen.add(h.doc_id); return true
        })
        console.debug('[MarsAgent:rag] deduped search hits', { q, count: unique.length, hits: unique })
        setDocs(unique.map((h: any) => ({
          slug: h.doc_id,
          title: h.payload?.text?.slice(0, 60) || h.url || '',
          category: h.payload?.category || 'general',
          source: h.payload?.source || '',
          url: h.payload?.url || '',
          updated_at: '',
        })))
      })
      .catch((err) => console.error('[MarsAgent:rag] search failed', { q, err }))
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
