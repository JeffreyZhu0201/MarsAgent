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

    const es = new EventSource(`/api/stream/${encodeURIComponent(taskId)}`)
    esRef.current = es

    es.onopen = () => setConnected(true)
    es.onmessage = (e) => {
      try {
        const ev = JSON.parse(e.data) as ProgressEvent
        setEvents((prev) => [...prev, ev])
        if (ev.type === 'task.done' || ev.type === 'task.failed') {
          es.close()
          setClosed(true)
        }
      } catch {
        // 跳过格式异常事件
      }
    }
    es.onerror = () => {
      setError('connection error')
      es.close()
      setClosed(true)
    }
    return () => {
      es.close()
      esRef.current = null
    }
  }, [taskId])

  return { events, connected, closed, error }
}
