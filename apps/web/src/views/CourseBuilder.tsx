import { Link } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { createCourse, listCourses, type Course } from '@/lib/api'
import { useSse } from '@/lib/useSse'
import { ProgressFeed } from '@/components/ProgressFeed'

export function CourseBuilder() {
  const [topic, setTopic] = useState('Python 异步编程入门')
  const [audience, setAudience] = useState('有基础的 Python 开发者')
  const [depth, setDepth] = useState('intermediate')
  const [taskId, setTaskId] = useState<string | null>(null)
  const [courseId, setCourseId] = useState<string | null>(null)
  const [courses, setCourses] = useState<Course[]>([])
  const [submitting, setSubmitting] = useState(false)
  const { events, connected, closed, error } = useSse(taskId)

  useEffect(() => {
    console.debug('[MarsAgent:builder] refresh course list', { closed })
    listCourses()
      .then((items) => {
        console.debug('[MarsAgent:builder] course list loaded', { count: items.length, items })
        setCourses(items)
      })
      .catch((err) => console.error('[MarsAgent:builder] course list failed', err))
  }, [closed])

  async function onBuild() {
    const payload = { topic, audience, depth }
    console.debug('[MarsAgent:builder] create course requested', payload)
    setSubmitting(true)
    try {
      const r = await createCourse(payload)
      console.debug('[MarsAgent:builder] course accepted', r)
      setTaskId(r.task_id)
      setCourseId(r.id)
    } catch (err) {
      console.error('[MarsAgent:builder] create course failed', err)
      throw err
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="max-w-6xl mx-auto space-y-5">
      <div>
        <h1 className="text-2xl font-semibold">建课工作台</h1>
        <p className="text-sm text-slate-500 mt-1">输入主题，触发 Planner / Author / CodeEng / Quiz / Validator 生成课程。</p>
      </div>

      <section className="border rounded p-4 bg-white space-y-3">
        <label className="block text-sm font-medium">课程主题</label>
        <input className="border rounded px-3 py-2 text-sm w-full" value={topic} onChange={(e) => setTopic(e.target.value)} />
        <div className="grid md:grid-cols-2 gap-3">
          <div>
            <label className="block text-sm font-medium mb-1">受众</label>
            <input className="border rounded px-3 py-2 text-sm w-full" value={audience} onChange={(e) => setAudience(e.target.value)} />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">深度</label>
            <select className="border rounded px-3 py-2 text-sm w-full" value={depth} onChange={(e) => setDepth(e.target.value)}>
              <option value="beginner">beginner</option>
              <option value="intermediate">intermediate</option>
              <option value="advanced">advanced</option>
            </select>
          </div>
        </div>
        <button onClick={onBuild} disabled={submitting || !topic.trim()} className="bg-slate-900 text-white text-sm px-4 py-2 rounded disabled:opacity-50">
          {submitting ? '创建中…' : '开始建课'}
        </button>
      </section>

      {taskId && (
        <section className="border rounded p-4 bg-white space-y-3">
          <div className="text-xs text-slate-500">
            course_id: <code>{courseId}</code> · task_id: <code>{taskId}</code> · {connected ? '已连接' : '连接中…'}
            {closed && ' · 已关闭'}
            {error && <span className="text-red-600"> · {error}</span>}
          </div>
          {courseId && closed && <Link to="/reader" search={{ courseId }} className="inline-block text-sm text-blue-700 underline">打开课程阅读器</Link>}
          <ProgressFeed events={events} />
        </section>
      )}

      <section className="border rounded p-4 bg-white">
        <h2 className="font-medium mb-2">最近课程</h2>
        <ul className="divide-y">
          {courses.map((c) => (
            <li key={c.id} className="py-2 flex justify-between gap-3 text-sm">
              <span><b>{c.topic}</b> <span className="text-slate-500">({c.status})</span></span>
              <Link to="/reader" search={{ courseId: c.id }} className="text-blue-700 underline">阅读</Link>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}