import { useState } from 'react'
import { postEcho } from '@/lib/api'
import { useSse } from '@/lib/useSse'
import { ProgressFeed } from '@/components/ProgressFeed'

export function CourseBuilder() {
  const [msg, setMsg] = useState('hello from web')
  const [taskId, setTaskId] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const { events, connected, closed, error } = useSse(taskId)

  async function onSend() {
    setSubmitting(true)
    try {
      const r = await postEcho(msg)
      setTaskId(r.task_id)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="max-w-6xl mx-auto space-y-4">
      <h1 className="text-2xl font-semibold">建课工作台 (M1 demo)</h1>

      <div className="flex gap-2 items-center">
        <input
          aria-label="echo message"
          className="border rounded px-3 py-1.5 text-sm flex-1 max-w-md"
          value={msg}
          onChange={(e) => setMsg(e.target.value)}
        />
        <button
          onClick={onSend}
          disabled={submitting}
          className="bg-slate-900 text-white text-sm px-3 py-1.5 rounded disabled:opacity-50"
        >
          {submitting ? '发送中…' : 'Send Echo'}
        </button>
      </div>

      {taskId && (
        <div className="text-xs text-slate-500">
          task_id: <code>{taskId}</code>{' '}
          · {connected ? '已连接' : '连接中…'}
          {closed && ' · 已关闭'}
          {error && <span className="text-red-600"> · {error}</span>}
        </div>
      )}

      <section className="border rounded p-3 bg-white">
        <h2 className="text-sm font-medium mb-2">进度事件</h2>
        <ProgressFeed events={events} />
      </section>
    </div>
  )
}
