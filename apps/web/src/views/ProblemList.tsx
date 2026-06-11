import { useEffect, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  listOJProblems,
  type OJProblem,
} from '@/lib/api'

const difficultyColors: Record<string, string> = {
  easy: 'bg-emerald-100 text-emerald-700',
  medium: 'bg-amber-100 text-amber-700',
  hard: 'bg-red-100 text-red-700',
}

export function ProblemList() {
  const navigate = useNavigate()
  const [problems, setProblems] = useState<OJProblem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [difficulty, setDifficulty] = useState('')
  const [tag, setTag] = useState('')

  useEffect(() => {
    setLoading(true)
    listOJProblems({ difficulty: difficulty || undefined, tag: tag || undefined })
      .then((r) => {
        setProblems(r.problems)
        setTotal(r.total)
      })
      .catch((err) => console.error('load problems failed', err))
      .finally(() => setLoading(false))
  }, [difficulty, tag])

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-slate-800">Problems</h1>
        <span className="text-sm text-slate-500">{total} total</span>
      </div>

      <div className="flex flex-wrap gap-3 mb-6">
        <select
          className="glass-input"
          value={difficulty}
          onChange={(e) => setDifficulty(e.target.value)}
        >
          <option value="">All Difficulties</option>
          <option value="easy">Easy</option>
          <option value="medium">Medium</option>
          <option value="hard">Hard</option>
        </select>
        <input
          className="glass-input"
          placeholder="Filter by tag..."
          value={tag}
          onChange={(e) => setTag(e.target.value)}
        />
      </div>

      {loading && <p className="text-slate-400 text-sm">Loading...</p>}

      <div className="space-y-2">
        {problems.map((p) => (
          <button
            key={p.id}
            className="glass-card w-full text-left p-4 hover:bg-white/40 transition cursor-pointer"
            onClick={() => navigate({ to: '/problems/$problemId', params: { problemId: p.id } })}
          >
            <div className="flex items-center justify-between">
              <span className="font-semibold text-slate-800">{p.title}</span>
              <span
                className={`text-xs px-2 py-0.5 rounded font-medium ${difficultyColors[p.difficulty] ?? 'bg-slate-100 text-slate-600'}`}
              >
                {p.difficulty}
              </span>
            </div>
            {p.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-2">
                {p.tags.map((t) => (
                  <span
                    key={t}
                    className="text-[11px] bg-blue-50 text-blue-600 px-2 py-0.5 rounded"
                  >
                    {t}
                  </span>
                ))}
              </div>
            )}
          </button>
        ))}
      </div>

      {!loading && problems.length === 0 && (
        <p className="text-slate-400 text-sm text-center mt-8">No problems found.</p>
      )}
    </div>
  )
}
