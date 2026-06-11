import { useEffect, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { listOJSubmissions, type OJSubmission } from '@/lib/api'

const statusColors: Record<string, string> = {
  accepted: 'bg-emerald-100 text-emerald-700',
  pending: 'bg-slate-100 text-slate-600',
  judging: 'bg-amber-100 text-amber-700',
  wrong_answer: 'bg-red-100 text-red-700',
  tle: 'bg-orange-100 text-orange-700',
  mle: 'bg-orange-100 text-orange-700',
  re: 'bg-red-100 text-red-700',
  ce: 'bg-purple-100 text-purple-700',
}

const statusLabels: Record<string, string> = {
  accepted: 'Accepted',
  pending: 'Pending',
  judging: 'Judging',
  wrong_answer: 'Wrong Answer',
  tle: 'Time Limit Exceeded',
  mle: 'Memory Limit Exceeded',
  re: 'Runtime Error',
  ce: 'Compilation Error',
}

export function SubmissionHistory() {
  const navigate = useNavigate()
  const [submissions, setSubmissions] = useState<OJSubmission[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    listOJSubmissions({})
      .then((r) => setSubmissions(r.submissions))
      .catch((err) => console.error('load submissions failed', err))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-slate-800">Submissions</h1>
      </div>

      {loading && <p className="text-slate-400 text-sm">Loading...</p>}

      <div className="space-y-2">
        {submissions.map((sub) => (
          <div
            key={sub.id}
            className="glass-card p-4 flex items-center justify-between"
          >
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-3">
                <span className="font-mono text-xs text-slate-500">{sub.id.slice(0, 8)}</span>
                <span className="text-sm text-slate-700">
                  {sub.lang}
                </span>
                <span className="text-xs text-slate-400 hidden sm:inline">
                  {new Date(sub.created_at).toLocaleString()}
                </span>
              </div>
              <div className="flex gap-4 mt-1 text-xs text-slate-500">
                <span>Score: {sub.score}</span>
                <span>Time: {sub.duration_ms}ms</span>
                {sub.memory_kb > 0 && <span>Memory: {sub.memory_kb}KB</span>}
              </div>
            </div>
            <span
              className={`ml-4 px-2.5 py-1 rounded text-xs font-semibold whitespace-nowrap ${
                statusColors[sub.status] ?? 'bg-slate-100'
              }`}
            >
              {statusLabels[sub.status] ?? sub.status}
            </span>
          </div>
        ))}
      </div>

      {!loading && submissions.length === 0 && (
        <p className="text-slate-400 text-sm text-center mt-8">
          No submissions yet.{' '}
          <button
            className="text-blue-600 underline"
            onClick={() => navigate({ to: '/problems' })}
          >
            Browse problems
          </button>
        </p>
      )}
    </div>
  )
}
