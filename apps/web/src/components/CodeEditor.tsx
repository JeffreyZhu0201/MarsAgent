import { useState } from 'react'
import { runSandbox, type CodeExample, type SandboxResult } from '@/lib/api'

function normalizeLang(lang: string) {
  const l = lang.toLowerCase()
  if (l.includes('javascript') || l.includes('node')) return 'node'
  if (l.includes('go')) return 'go'
  return 'python'
}

export function CodeEditor({ example }: { example: CodeExample }) {
  const [code, setCode] = useState(example.code || '')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<SandboxResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function onRun() {
    const lang = normalizeLang(example.lang)
    console.debug('[MarsAgent:sandbox] run requested', {
      title: example.title,
      lang,
      codeBytes: new Blob([code]).size,
    })
    setRunning(true)
    setError(null)
    try {
      const r = await runSandbox({ lang, code, timeout: 15 })
      console.debug('[MarsAgent:sandbox] run finished', { title: example.title, result: r })
      setResult(r)
    } catch (e) {
      console.error('[MarsAgent:sandbox] run failed', { title: example.title, error: e })
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setRunning(false)
    }
  }

  return (
    <div className="border rounded bg-slate-950 text-slate-100 overflow-hidden my-4">
      <div className="flex items-center justify-between px-3 py-2 bg-slate-900 text-xs">
        <span>{example.title || 'Code'} · {example.lang || 'python'}</span>
        <button onClick={onRun} disabled={running} className="bg-emerald-600 text-white px-2 py-1 rounded disabled:opacity-50">
          {running ? 'Running…' : 'Run'}
        </button>
      </div>
      <textarea
        className="w-full min-h-48 bg-slate-950 text-slate-100 font-mono text-xs p-3 outline-none"
        value={code}
        onChange={(e) => setCode(e.target.value)}
        spellCheck={false}
      />
      {example.expected_output && <div className="px-3 py-2 text-xs text-slate-400 border-t border-slate-800">Expected: <pre className="whitespace-pre-wrap inline">{example.expected_output}</pre></div>}
      {error && <div className="px-3 py-2 text-xs text-red-300 border-t border-slate-800">{error}</div>}
      {result && (
        <div className="grid md:grid-cols-2 gap-0 border-t border-slate-800 text-xs">
          <pre className="p-3 whitespace-pre-wrap overflow-auto"><b>stdout</b>\n{result.stdout || '(empty)'}</pre>
          <pre className="p-3 whitespace-pre-wrap overflow-auto border-l border-slate-800"><b>stderr</b>\n{result.stderr || '(empty)'}\nexit={result.exit_code} · {result.duration_ms}ms</pre>
        </div>
      )}
    </div>
  )
}
