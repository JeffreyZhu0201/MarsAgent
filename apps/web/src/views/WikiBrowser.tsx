import { useEffect, useMemo, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { ProgressFeed } from '@/components/ProgressFeed'
import { WikiTree } from '@/components/WikiTree'
import { WikiSearch } from '@/components/WikiSearch'
import { collectWiki, searchWiki, type RagHit } from '@/lib/api'
import { useSse, type ProgressEvent } from '@/lib/useSse'

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
  const [collectTopic, setCollectTopic] = useState('Python')
  const [collectTaskId, setCollectTaskId] = useState<string | null>(null)
  const [collecting, setCollecting] = useState(false)
  const [ragQuery, setRagQuery] = useState('Python')
  const [ragHits, setRagHits] = useState<RagHit[]>([])
  const [ragChecking, setRagChecking] = useState(false)
  const { events: collectEvents, connected: collectConnected, closed: collectClosed } = useSse(collectTaskId)

  const discovered = useMemo(() => extractDocs(collectEvents, 'discover'), [collectEvents])
  const written = useMemo(() => extractDocs(collectEvents, 'write_wiki'), [collectEvents])

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

  useEffect(() => {
    if (!collectClosed) return
    fetch('/api/wiki/tree')
      .then(r => r.json())
      .then(d => setDocs(d.docs || []))
      .catch((err) => console.error('[MarsAgent:wiki] post-collect refresh failed', err))
  }, [collectClosed])

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

  async function handleCollect() {
    console.debug('[MarsAgent:collect] network search requested', { topic: collectTopic })
    setCollecting(true)
    try {
      const resp = await collectWiki({
        topic: collectTopic,
        sources: ['tavily', 'doc', 'arxiv', 'github'],
        max_per_source: 5,
      })
      console.debug('[MarsAgent:collect] task accepted', resp)
      setCollectTaskId(resp.task_id)
    } finally {
      setCollecting(false)
    }
  }

  async function handleRagCheck() {
    console.debug('[MarsAgent:rag] check requested', { q: ragQuery, k: 10 })
    setRagChecking(true)
    try {
      const hits = await searchWiki(ragQuery, 10)
      console.debug('[MarsAgent:rag] check result', { count: hits.length, hits })
      setRagHits(hits)
    } finally {
      setRagChecking(false)
    }
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
    searchWiki(q, 20)
      .then(hits => {
        const seen = new Set<string>()
        console.debug('[MarsAgent:rag] raw search hits', { q, count: hits.length, hits })
        const unique = hits.filter((h) => {
          if (seen.has(h.doc_id)) return false
          seen.add(h.doc_id); return true
        })
        console.debug('[MarsAgent:rag] deduped search hits', { q, count: unique.length, hits: unique })
        setRagHits(unique)
        setDocs(unique.map((h) => ({
          slug: h.doc_id,
          title: h.title || h.text?.slice(0, 60) || h.url || h.doc_id,
          category: 'rag',
          source: h.source || '',
          url: h.url || '',
          updated_at: '',
        })))
      })
      .catch((err) => console.error('[MarsAgent:rag] search failed', { q, err }))
  }

  return (
    <div className="grid grid-cols-[20rem_1fr_24rem] gap-4 h-[calc(100vh-4rem)]">
      <aside className="glass-card overflow-y-auto p-4 flex flex-col">
        <div className="mb-4">
          <div className="text-xs uppercase tracking-[0.2em] text-slate-400">Existing Wiki</div>
          <h2 className="text-lg font-semibold">已有 Wiki</h2>
        </div>
        <WikiSearch onSearch={handleSearch} />
        <WikiTree docs={docs} selected={selected} onSelect={handleSelect} />
      </aside>

      <main className="glass-card overflow-y-auto p-8">
        {content ? (
          <article className="prose prose-slate max-w-3xl mx-auto">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
          </article>
        ) : (
          <p className="text-slate-400 text-center mt-20">选择左侧文档开始阅读，或在右侧启动网络搜索 Agent。</p>
        )}
      </main>

      <aside className="space-y-4 overflow-y-auto pr-1">
        <section className="glass-card p-4 space-y-3">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-blue-500">Search Agent</div>
            <h2 className="text-lg font-semibold">网络搜索 / 信息收集</h2>
            <p className="text-xs text-slate-500 mt-1">启动 collector，从网络发现资料并写入 Wiki。</p>
          </div>
          <input
            className="glass-input w-full"
            value={collectTopic}
            onChange={(e) => setCollectTopic(e.target.value)}
            placeholder="输入主题，如 Python asyncio"
          />
          <button className="glass-button w-full" onClick={handleCollect} disabled={collecting || !collectTopic.trim()}>
            {collecting ? '启动中…' : '启动搜索 Agent'}
          </button>
          {collectTaskId && (
            <div className="text-xs text-slate-500">
              task: <code>{collectTaskId}</code> · {collectConnected ? '已连接' : '连接中'} {collectClosed && '· 已结束'}
            </div>
          )}
          <ProgressFeed events={collectEvents} />
        </section>

        <section className="glass-card p-4 space-y-3">
          <h3 className="font-semibold">新发现的信息</h3>
          <InfoList items={discovered} empty="等待搜索 Agent 返回发现结果。" />
        </section>

        <section className="glass-card p-4 space-y-3">
          <h3 className="font-semibold">新写入 Wiki</h3>
          <InfoList items={written} empty="暂无新写入。" />
        </section>

        <section className="glass-card p-4 space-y-3">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-emerald-500">RAG Check</div>
            <h3 className="font-semibold">检查 RAG 是否有效</h3>
          </div>
          <input
            className="glass-input w-full"
            value={ragQuery}
            onChange={(e) => setRagQuery(e.target.value)}
            placeholder="输入检索问题"
          />
          <button className="glass-button w-full" onClick={handleRagCheck} disabled={ragChecking || !ragQuery.trim()}>
            {ragChecking ? '检索中…' : '检查 RAG'}
          </button>
          <div className="text-xs text-slate-500">命中数：{ragHits.length}</div>
          <ul className="space-y-2">
            {ragHits.map((hit, idx) => (
              <li key={`${hit.doc_id}-${hit.chunk_id}-${idx}`} className="rounded-2xl bg-white/50 p-3 text-xs shadow-sm ring-1 ring-white/60">
                <div className="font-medium text-slate-800">{hit.title || hit.url || hit.doc_id}</div>
                <div className="mt-1 text-slate-500">score={Number(hit.score || 0).toFixed(3)} · {hit.source}</div>
                <p className="mt-2 line-clamp-4 text-slate-600">{hit.text}</p>
              </li>
            ))}
          </ul>
        </section>
      </aside>
    </div>
  )
}

interface InfoItem {
  title: string
  url: string
  source: string
}

function extractDocs(events: ProgressEvent[], stage: 'discover' | 'write_wiki'): InfoItem[] {
  const items: InfoItem[] = []
  for (const ev of events) {
    if (ev.extra?.stage !== stage) continue
    if (stage === 'discover' && Array.isArray(ev.extra.docs)) {
      for (const raw of ev.extra.docs as any[]) {
        items.push({ title: raw.title || raw.url || '(untitled)', url: raw.url || '', source: raw.source || '' })
      }
    }
    if (stage === 'write_wiki' && ev.extra.doc && typeof ev.extra.doc === 'object') {
      const raw = ev.extra.doc as any
      items.push({ title: raw.title || raw.url || raw.doc_id || '(untitled)', url: raw.url || '', source: raw.source || '' })
    }
  }
  return items
}

function InfoList({ items, empty }: { items: InfoItem[]; empty: string }) {
  if (!items.length) return <p className="text-sm text-slate-400">{empty}</p>
  return (
    <ul className="space-y-2">
      {items.map((item, idx) => (
        <li key={`${item.url}-${idx}`} className="rounded-2xl bg-white/50 p-3 text-xs shadow-sm ring-1 ring-white/60">
          <div className="font-medium text-slate-800">{item.title}</div>
          <div className="mt-1 text-slate-500">{item.source}</div>
          {item.url && <a className="mt-1 block truncate text-blue-600 underline" href={item.url} target="_blank" rel="noreferrer">{item.url}</a>}
        </li>
      ))}
    </ul>
  )
}
