import { useState } from 'react'
import type { QuizItem, QuizOption } from '@/lib/api'

export function QuizPanel({ items }: { items: QuizItem[] }) {
  const [open, setOpen] = useState<Record<number, boolean>>({})
  if (!items?.length) return <p className="text-sm text-slate-400">本章暂无习题。</p>

  return (
    <div className="space-y-3">
      {items.map((q, idx) => (
        <div key={idx} className="rounded-3xl border border-white/70 bg-white/65 p-4 shadow-sm backdrop-blur-xl">
          <div className="mb-1 text-xs uppercase tracking-wide text-slate-500">{q.type || 'quiz'} #{idx + 1}</div>
          <div className="font-medium text-slate-900">{renderValue(q.question)}</div>
          {q.options?.length ? (
            <ul className="mt-3 space-y-2 text-sm text-slate-700">
              {q.options.map((o, i) => (
                <li key={i} className="rounded-2xl bg-white/60 px-3 py-2 ring-1 ring-white/70">
                  {formatOption(o)}
                </li>
              ))}
            </ul>
          ) : null}
          <button
            className="mt-3 min-h-11 rounded-full px-3 text-xs font-medium text-blue-700 underline underline-offset-4 hover:bg-blue-50 focus:outline-none focus:ring-4 focus:ring-blue-200/50"
            onClick={() => setOpen(prev => ({ ...prev, [idx]: !prev[idx] }))}
          >
            {open[idx] ? '隐藏答案' : '查看答案'}
          </button>
          {open[idx] && (
            <div className="mt-2 rounded-2xl bg-slate-50/80 p-3 text-sm text-slate-700 ring-1 ring-white/70">
              <b>答案：</b>{renderValue(q.answer || '未提供')}
              {q.explanation && <p className="mt-1 text-slate-600">{renderValue(q.explanation)}</p>}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}

function formatOption(option: string | QuizOption): string {
  if (typeof option === 'string') return option
  const label = option.label ? `${option.label}. ` : ''
  return `${label}${option.text || ''}`.trim() || JSON.stringify(option)
}

function renderValue(value: unknown): string {
  if (typeof value === 'string') return value
  if (value == null) return ''
  return JSON.stringify(value)
}
