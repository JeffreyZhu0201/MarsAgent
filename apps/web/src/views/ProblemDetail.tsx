import { useEffect, useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import {
  getOJProblem,
  createOJSubmission,
  getOJSubmission,
  type OJProblem,
  type OJTestCase,
  type OJSubmission,
} from '@/lib/api'
import { MarkdownView } from '@/components/MarkdownView'

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

export function ProblemDetail() {
  const { problemId } = useParams({ from: '/problems/$problemId' })
  const navigate = useNavigate()
  const [problem, setProblem] = useState<OJProblem | null>(null)
  const [samples, setSamples] = useState<OJTestCase[]>([])
  const [code, setCode] = useState('')
  const [lang, setLang] = useState<'python' | 'node' | 'go'>('python')
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<OJSubmission | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    getOJProblem(problemId)
      .then((r) => {
        setProblem(r.problem)
        setSamples(r.sample_test_cases)
      })
      .catch((err) => console.error('load problem failed', err))
      .finally(() => setLoading(false))
  }, [problemId])

  async function handleSubmit() {
    if (!code.trim()) return
    setSubmitting(true)
    setResult(null)
    try {
      const r = await createOJSubmission({ problem_id: problemId, code, lang })
      pollSubmission(r.submission_id)
    } catch (err) {
      console.error('submit failed', err)
    } finally {
      setSubmitting(false)
    }
  }

  async function pollSubmission(id: string) {
    try {
      const r = await getOJSubmission(id)
      setResult(r.submission)
      if (r.submission.status === 'pending' || r.submission.status === 'judging') {
        setTimeout(() => pollSubmission(id), 1000)
      }
    } catch {
      // ignore poll errors
    }
  }

  if (loading) {
    return <p className="p-6 text-slate-400 text-sm">Loading...</p>
  }

  if (!problem) {
    return <p className="p-6 text-red-400 text-sm">Problem not found.</p>
  }

  return (
    <div className="flex flex-col lg:flex-row h-[calc(100vh-4rem)]">
      {/* Left: description */}
      <div className="flex-1 overflow-y-auto p-6">
        <button
          className="text-sm text-slate-500 hover:text-slate-700 mb-4 flex items-center gap-1"
          onClick={() => navigate({ to: '/problems' })}
        >
          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
          Back to Problems
        </button>

        <h1 className="text-2xl font-bold text-slate-800 mb-2">{problem.title}</h1>
        <div className="flex flex-wrap gap-3 mb-6 text-sm text-slate-500">
          <span>TL: {problem.time_limit_ms}ms</span>
          <span>ML: {problem.memory_limit_mb}MB</span>
          <span
            className={`px-2 py-0.5 rounded text-xs font-medium ${
              problem.difficulty === 'easy'
                ? 'bg-emerald-100 text-emerald-700'
                : problem.difficulty === 'medium'
                ? 'bg-amber-100 text-amber-700'
                : 'bg-red-100 text-red-700'
            }`}
          >
            {problem.difficulty}
          </span>
        </div>

        <MarkdownView content={problem.description_md} />

        {samples.length > 0 && (
          <div className="mt-8">
            <h2 className="text-lg font-semibold text-slate-800 mb-3">Sample Input/Output</h2>
            {samples.map((tc, i) => (
              <div key={tc.id} className="glass-card p-4 mb-3">
                <div className="mb-2">
                  <span className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Sample {i + 1}</span>
                </div>
                <div>
                  <div className="text-xs font-medium text-slate-600 mb-1">Input:</div>
                  <pre className="bg-slate-100 rounded p-2 text-sm font-mono whitespace-pre-wrap">{tc.input || '(empty)'}</pre>
                </div>
                <div className="mt-2">
                  <div className="text-xs font-medium text-slate-600 mb-1">Output:</div>
                  <pre className="bg-slate-100 rounded p-2 text-sm font-mono whitespace-pre-wrap">{tc.expected_output}</pre>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Right: submit panel */}
      <div className="w-full lg:w-[420px] flex flex-col glass-card rounded-none border-l border-white/20">
        <div className="p-4 border-b border-white/10">
          <h2 className="font-semibold text-slate-800">Submit Solution</h2>
        </div>
        <div className="flex-1 flex flex-col p-4 gap-3">
          <select
            className="glass-input"
            value={lang}
            onChange={(e) => setLang(e.target.value as typeof lang)}
          >
            <option value="python">Python 3</option>
            <option value="node">Node.js</option>
            <option value="go">Go</option>
          </select>
          <textarea
            className="glass-input flex-1 font-mono text-sm resize-none"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            placeholder="Enter your code..."
            spellCheck={false}
          />
          <button
            className="glass-button w-full"
            onClick={handleSubmit}
            disabled={submitting || !code.trim()}
          >
            {submitting ? 'Submitting...' : 'Submit'}
          </button>

          {result && (
            <div
              className={`mt-2 p-3 rounded-xl ${
                result.status === 'accepted' ? 'bg-emerald-50 border border-emerald-200' : 'bg-red-50 border border-red-200'
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="font-semibold text-sm">
                  {result.status === 'accepted' ? '✅ Accepted' : `❌ ${result.status.toUpperCase()}`}
                </span>
                <span className={`text-xs px-2 py-0.5 rounded ${statusColors[result.status] ?? 'bg-slate-100'}`}>
                  {result.status}
                </span>
              </div>
              <div className="text-xs text-slate-600 mt-1 space-y-0.5">
                <div>Score: {result.score}</div>
                <div>Time: {result.duration_ms}ms</div>
                {result.error_msg && <div className="text-red-600 truncate">Error: {result.error_msg}</div>}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
