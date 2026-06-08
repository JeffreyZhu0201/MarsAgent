import { useState } from 'react'

export function WikiSearch({ onSearch }: { onSearch: (q: string) => void }) {
  const [q, setQ] = useState('')

  return (
    <div className="flex gap-1">
      <input
        className="border rounded px-2 py-1 text-sm flex-1"
        placeholder="搜索 Wiki…"
        value={q}
        onChange={e => setQ(e.target.value)}
        onKeyDown={e => e.key === 'Enter' && onSearch(q)}
      />
      <button
        className="text-sm bg-slate-900 text-white px-2 py-1 rounded"
        onClick={() => onSearch(q)}
      >
        搜索
      </button>
      {q && (
        <button
          className="text-sm text-slate-500 px-1"
          onClick={() => { setQ(''); onSearch('') }}
        >
          ×
        </button>
      )}
    </div>
  )
}
