import { useSearch } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { CodeEditor } from '@/components/CodeEditor'
import { QuizPanel } from '@/components/QuizPanel'
import { getChapterMarkdown, getCourse, listCourses, parseOutline, type Chapter, type Course } from '@/lib/api'

export function CourseReader() {
  const search = useSearch({ from: '/reader' }) as { courseId?: string }
  const [courses, setCourses] = useState<Course[]>([])
  const [course, setCourse] = useState<Course | null>(null)
  const [active, setActive] = useState<string>('')
  const [md, setMd] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    listCourses().then(setCourses).catch(console.error)
  }, [])

  useEffect(() => {
    if (!search.courseId) return
    getCourse(search.courseId).then((c) => {
      setCourse(c)
      const first = parseOutline(c)[0]?.ch_id || ''
      setActive(first)
    }).catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [search.courseId])

  const outline = useMemo(() => course ? parseOutline(course) : [], [course])
  const activeChapter = outline.find((ch) => ch.ch_id === active) as Chapter | undefined

  useEffect(() => {
    if (!course || !active) return
    getChapterMarkdown(course.id, active)
      .then(setMd)
      .catch(() => setMd(activeChapter?.content_md || ''))
  }, [course, active, activeChapter])

  if (!search.courseId) {
    return (
      <div className="max-w-4xl mx-auto space-y-3">
        <h1 className="text-2xl font-semibold">课程阅读器</h1>
        <p className="text-slate-500">请选择一门课程。</p>
        <ul className="divide-y border rounded bg-white">
          {courses.map(c => <li key={c.id} className="p-3"><a className="text-blue-700 underline" href={`/reader?courseId=${encodeURIComponent(c.id)}`}>{c.topic}</a> <span className="text-sm text-slate-500">{c.status}</span></li>)}
        </ul>
      </div>
    )
  }

  return (
    <div className="grid grid-cols-[18rem_1fr] gap-6 max-w-7xl mx-auto">
      <aside className="border rounded bg-white p-3 h-[calc(100vh-7rem)] overflow-auto">
        <h1 className="font-semibold mb-1">{course?.topic || '课程'}</h1>
        <p className="text-xs text-slate-500 mb-3">{course?.status} · {course?.depth}</p>
        {error && <p className="text-xs text-red-600 mb-2">{error}</p>}
        <ul className="space-y-1">
          {outline.map((ch) => (
            <li key={ch.ch_id}>
              <button onClick={() => setActive(ch.ch_id)} className={`w-full text-left text-sm rounded px-2 py-1.5 ${active === ch.ch_id ? 'bg-slate-900 text-white' : 'hover:bg-slate-100'}`}>
                {ch.ch_id} · {ch.title}
              </button>
            </li>
          ))}
        </ul>
      </aside>

      <main className="min-w-0 space-y-6">
        {activeChapter ? (
          <>
            <article className="prose prose-slate max-w-none bg-white border rounded p-6">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{md || activeChapter.content_md || ''}</ReactMarkdown>
            </article>

            <section className="bg-white border rounded p-4">
              <h2 className="text-lg font-semibold mb-3">代码示例</h2>
              {activeChapter.code_examples?.length
                ? activeChapter.code_examples.map((ex, idx) => <CodeEditor key={idx} example={ex} />)
                : <p className="text-sm text-slate-400">本章暂无代码示例。</p>}
            </section>

            <section className="bg-white border rounded p-4">
              <h2 className="text-lg font-semibold mb-3">Quiz</h2>
              <QuizPanel items={activeChapter.quiz || []} />
            </section>
          </>
        ) : (
          <p className="text-slate-400 text-center mt-20">等待课程大纲生成完成。</p>
        )}
      </main>
    </div>
  )
}
