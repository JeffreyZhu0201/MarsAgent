import type { ProgressEvent } from '@/lib/useSse'
import clsx from 'clsx'

const badge: Record<string, string> = {
  'agent.start': 'bg-blue-100 text-blue-700',
  'agent.progress': 'bg-amber-100 text-amber-700',
  'agent.error': 'bg-red-100 text-red-700',
  'agent.retry': 'bg-purple-100 text-purple-700',
  'agent.done': 'bg-emerald-100 text-emerald-700',
  'task.done': 'bg-emerald-200 text-emerald-800 font-medium',
  'task.failed': 'bg-red-200 text-red-800 font-medium',
}

export function ProgressFeed({ events }: { events: ProgressEvent[] }) {
  if (events.length === 0) {
    return <p className="text-slate-400 text-sm">无事件。</p>
  }
  return (
    <ul className="space-y-1 font-mono text-xs">
      {events.map((e, idx) => (
        <li key={idx} className="flex items-start gap-2">
          <span className={clsx('px-1.5 py-0.5 rounded', badge[e.type] ?? 'bg-slate-100')}>
            {e.type}
          </span>
          {typeof e.pct === 'number' && (
            <span className="text-slate-500">{e.pct}%</span>
          )}
          {e.agent && <span className="text-slate-500">[{e.agent}]</span>}
          {e.message && <span className="flex-1">{e.message}</span>}
        </li>
      ))}
    </ul>
  )
}
