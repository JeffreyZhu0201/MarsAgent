import { useEffect, useRef, useState } from 'react'

export interface ProgressEvent {
  type: string
  task_id: string
  agent?: string
  pct?: number
  message?: string
  extra?: Record<string, unknown>
  ts: number
}

export interface UseSseState {
  events: ProgressEvent[]
  connected: boolean
  closed: boolean
  error?: string
}

export function useSse(taskId: string | null): UseSseState {
  const [events, setEvents] = useState<ProgressEvent[]>([])
  const [connected, setConnected] = useState(false)
  const [closed, setClosed] = useState(false)
  const [error, setError] = useState<string | undefined>()
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    if (!taskId) return
    setEvents([])
    setConnected(false)
    setClosed(false)
    setError(undefined)

    const streamUrl = `/api/stream/${encodeURIComponent(taskId)}`
    console.debug('[MarsAgent:SSE] connecting', { taskId, streamUrl })
    const es = new EventSource(streamUrl)
    esRef.current = es

    es.onopen = () => {
      console.debug('[MarsAgent:SSE] connected', { taskId })
      setConnected(true)
    }
    es.onmessage = (e) => {
      try {
        const ev = JSON.parse(e.data) as ProgressEvent
        console.groupCollapsed(
          `[MarsAgent:progress] ${ev.type} task=${ev.task_id} agent=${ev.agent ?? '-'}`,
        )
        console.debug('event', ev)
        console.debug('message', ev.message ?? '')
        if (ev.extra) console.debug('extra', ev.extra)
        console.groupEnd()
        setEvents((prev) => [...prev, ev])
        if (ev.type === 'task.done' || ev.type === 'task.failed') {
          console.debug('[MarsAgent:SSE] terminal event, closing', ev)
          es.close()
          setClosed(true)
        }
      } catch (err) {
        console.warn('[MarsAgent:SSE] malformed event skipped', { raw: e.data, err })
      }
    }
    es.onerror = () => {
      console.error('[MarsAgent:SSE] connection error', { taskId })
      setError('connection error')
      es.close()
      setClosed(true)
    }
    return () => {
      console.debug('[MarsAgent:SSE] cleanup', { taskId })
      es.close()
      esRef.current = null
    }
  }, [taskId])

  return { events, connected, closed, error }
}
