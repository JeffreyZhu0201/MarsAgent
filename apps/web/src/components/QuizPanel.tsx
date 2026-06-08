import { useState } from 'react'
import type { QuizItem } from '@/lib/api'

export function QuizPanel({ items }: { items: QuizItem[] }) {
  const [open, setOpen] = useState<Record<number, boolean>>({})
  if (!items?.length) return <p className="text-sm text-slate-400">本章暂无习题。</p>

  return (
    <div className="space-y-3">
      {items.map((q, idx) => (
        <div key={idx} className="border rounded p-3 bg-white">
          <div className="text-xs uppercase tracking-wide text-slate-500 mb-1">{q.type || 'quiz'} #{idx + 1}</div>
          <div className="font-medium">{q.question}</div>
          {q.options?.length ? (
            <ul className="mt-2 list-disc ml-5 text-sm text-slate-700">
              {q.options.map((o, i) => <li key={i}>{o}</li>)}
            </ul>
          ) : null}
          <button className="text-xs text-blue-700 underline mt-2" onClick={() => setOpen(prev => ({ ...prev, [idx]: !prev[idx] }))}>
            {open[idx] ? '隐藏答案' : '查看答案'}
          </button>
          {open[idx] && (
            <div className="mt-2 text-sm bg-slate-50 rounded p-2">
              <b>答案：</b>{q.answer || '未提供'}
              {q.explanation && <p className="mt-1 text-slate-600">{q.explanation}</p>}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}
