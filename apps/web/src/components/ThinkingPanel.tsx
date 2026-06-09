import { useState } from 'react'
import clsx from 'clsx'
import type { ProgressEvent } from '@/lib/useSse'

interface ThinkingPanelProps {
  events: ProgressEvent[]
}

const AGENT_LABELS: Record<string, string> = {
  planner: 'Planner',
  author: 'Author',
  codeeng: 'CodeEng',
  quiz: 'Quiz',
  validator: 'Validator',
}

const AGENT_COLORS: Record<string, string> = {
  planner: 'from-blue-500 to-cyan-400',
  author: 'from-violet-500 to-fuchsia-400',
  codeeng: 'from-emerald-500 to-teal-400',
  quiz: 'from-amber-500 to-orange-400',
  validator: 'from-rose-500 to-pink-400',
}

export function ThinkingPanel({ events }: ThinkingPanelProps) {
  const thinkingEvents = events.filter((e) => e.type === 'agent.thinking')

  if (thinkingEvents.length === 0) {
    return null
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <div className="flex h-5 w-5 items-center justify-center rounded-full bg-gradient-to-br from-violet-500 to-fuchsia-400">
          <svg className="h-3 w-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
          </svg>
        </div>
        <span className="text-xs font-medium uppercase tracking-[0.15em] text-slate-500">
          LLM 推理过程
        </span>
        <span className="rounded-full bg-violet-100 px-2 py-0.5 text-[10px] font-medium text-violet-700">
          {thinkingEvents.length} 步
        </span>
      </div>

      <div className="space-y-2">
        {thinkingEvents.map((ev, idx) => {
          const label = AGENT_LABELS[ev.agent ?? ''] ?? ev.agent ?? ''
          const color = AGENT_COLORS[ev.agent ?? ''] ?? 'from-slate-400 to-slate-500'
          const isLast = idx === thinkingEvents.length - 1

          // Strip the "Agent 推理过程:\n" prefix if present for cleaner display
          const rawMsg = ev.message ?? ''
          const displayMsg = rawMsg.replace(/^(Planner|Author|CodeEng|Quiz|Validator)\s*推理过程:\s*/i, '')

          return (
            <ThinkingCard
              key={idx}
              agent={label}
              agentKey={ev.agent ?? ''}
              accent={color}
              message={displayMsg}
              isLast={isLast}
            />
          )
        })}
      </div>
    </div>
  )
}

function ThinkingCard({
  agent,
  accent,
  message,
  isLast,
}: {
  agent: string
  agentKey?: string
  accent: string
  message: string
  isLast: boolean
}) {
  const [expanded, setExpanded] = useState(isLast)

  // Split message into lines for nicer display
  const lines = message.split('\n').filter(Boolean)
  const preview = lines.slice(0, 3).join('\n')
  const hasMore = lines.length > 3

  return (
    <div
      className={clsx(
        'rounded-2xl border border-violet-200/60 bg-gradient-to-br from-violet-50/80 to-fuchsia-50/60',
        'overflow-hidden transition-all duration-200',
      )}
    >
      {/* Header */}
      <button
        className="flex w-full items-center gap-2 px-3 py-2 text-left"
        onClick={() => setExpanded((p) => !p)}
      >
        <div className={clsx('h-5 w-5 rounded-full bg-gradient-to-br p-0.5', accent)}>
          <div className="flex h-full w-full items-center justify-center rounded-full bg-white/90">
            <span className="text-[8px] font-bold text-slate-700">{agent[0]}</span>
          </div>
        </div>
        <span className="flex-1 text-xs font-medium text-slate-700">{agent} 推理中</span>
        <svg
          className={clsx(
            'h-3.5 w-3.5 text-slate-400 transition-transform duration-200',
            expanded && 'rotate-180',
          )}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {/* Content */}
      {expanded ? (
        <div className="border-t border-violet-200/50 px-3 py-2">
          <pre className="whitespace-pre-wrap break-words font-mono text-[11px] leading-relaxed text-slate-700">
            {message}
          </pre>
        </div>
      ) : (
        <div className="border-t border-violet-200/50 px-3 py-2">
          <p className="font-mono text-[11px] text-slate-500 line-clamp-2">
            {preview}
            {hasMore && ' ...'}
          </p>
          <button
            className="mt-1 text-[10px] text-violet-600 underline"
            onClick={(e) => { e.stopPropagation(); setExpanded(true) }}
          >
            展开全部
          </button>
        </div>
      )}
    </div>
  )
}
