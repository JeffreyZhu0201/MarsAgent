import { Link } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import clsx from 'clsx'
import { createCourse, listCourses, type Course } from '@/lib/api'
import { useSse, type ProgressEvent } from '@/lib/useSse'
import { ProgressFeed } from '@/components/ProgressFeed'
import { ThinkingPanel } from '@/components/ThinkingPanel'

const AGENTS = [
  { key: 'planner', label: 'Planner', desc: '规划课程大纲', accent: 'from-blue-500 to-cyan-400' },
  { key: 'author', label: 'Author', desc: '撰写章节讲义', accent: 'from-violet-500 to-fuchsia-400' },
  { key: 'codeeng', label: 'CodeEng', desc: '生成代码示例', accent: 'from-emerald-500 to-teal-400' },
  { key: 'quiz', label: 'Quiz', desc: '设计练习测验', accent: 'from-amber-500 to-orange-400' },
  { key: 'validator', label: 'Validator', desc: '检查质量一致性', accent: 'from-rose-500 to-pink-400' },
] as const

export function CourseBuilder() {
  const [topic, setTopic] = useState('Python 异步编程入门')
  const [audience, setAudience] = useState('有基础的 Python 开发者')
  const [depth, setDepth] = useState('intermediate')
  const [taskId, setTaskId] = useState<string | null>(null)
  const [courseId, setCourseId] = useState<string | null>(null)
  const [courses, setCourses] = useState<Course[]>([])
  const [submitting, setSubmitting] = useState(false)
  const { events, connected, closed, error } = useSse(taskId)
  const activeAgent = useActiveAgent(events)
  const terminal = [...events].reverse().find((e: ProgressEvent) => e.type === 'task.done' || e.type === 'task.failed')

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
    <div className="max-w-7xl mx-auto space-y-5">
      <section className="glass-card relative overflow-hidden p-6 md:p-8">
        <div className="absolute inset-0 pointer-events-none bg-[radial-gradient(circle_at_20%_20%,rgba(59,130,246,0.18),transparent_28rem),radial-gradient(circle_at_85%_0%,rgba(168,85,247,0.18),transparent_24rem)]" />
        <div className="relative grid gap-6 lg:grid-cols-[1.05fr_0.95fr] items-center">
          <div className="space-y-4">
            <div className="inline-flex rounded-full border border-white/70 bg-white/55 px-3 py-1 text-xs font-medium text-blue-700 shadow-sm backdrop-blur-xl">
              AI Course Studio · 5-Agent Workflow
            </div>
            <div>
              <h1 className="text-3xl md:text-5xl font-semibold tracking-tight text-slate-950">
                建课工作台
              </h1>
              <p className="mt-3 max-w-2xl text-base leading-7 text-slate-600">
                输入主题后，MarsAgent 会调度 Planner、Author、CodeEng、Quiz、Validator 协作生成完整课程，并通过 SSE 实时展示进度。
              </p>
            </div>
            <div className="grid gap-3 sm:grid-cols-3 text-sm">
              <MetricCard label="流程" value="5 Agents" />
              <MetricCard label="输出" value="讲义 + 代码 + Quiz" />
              <MetricCard label="状态" value={terminal?.type === 'task.done' ? 'Ready' : taskId ? 'Running' : 'Idle'} />
            </div>
          </div>

          <AgentOrbit activeAgent={activeAgent} terminalType={terminal?.type} />
        </div>
      </section>

      <div className="grid gap-5 lg:grid-cols-[0.9fr_1.1fr]">
        <section className="glass-card p-5 md:p-6 space-y-4">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-slate-400">Create</div>
            <h2 className="text-xl font-semibold">课程参数</h2>
          </div>

          <div className="space-y-3">
            <label className="block text-sm font-medium" htmlFor="course-topic">课程主题</label>
            <input
              id="course-topic"
              className="glass-input w-full min-h-11"
              value={topic}
              onChange={(e) => setTopic(e.target.value)}
            />
          </div>

          <div className="grid md:grid-cols-2 gap-3">
            <div className="space-y-2">
              <label className="block text-sm font-medium" htmlFor="course-audience">受众</label>
              <input
                id="course-audience"
                className="glass-input w-full min-h-11"
                value={audience}
                onChange={(e) => setAudience(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <label className="block text-sm font-medium" htmlFor="course-depth">深度</label>
              <select
                id="course-depth"
                className="glass-input w-full min-h-11"
                value={depth}
                onChange={(e) => setDepth(e.target.value)}
              >
                <option value="beginner">beginner</option>
                <option value="intermediate">intermediate</option>
                <option value="advanced">advanced</option>
              </select>
            </div>
          </div>

          <button
            onClick={onBuild}
            disabled={submitting || !topic.trim()}
            className="glass-button min-h-11 w-full cursor-pointer"
          >
            {submitting ? '创建中…' : '开始建课'}
          </button>
        </section>

        <section className="glass-card p-5 md:p-6 space-y-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-xs uppercase tracking-[0.2em] text-emerald-500">Live Progress</div>
              <h2 className="text-xl font-semibold">Agent 生成过程</h2>
            </div>
            {courseId && closed && (
              <Link to="/reader" search={{ courseId }} className="glass-button whitespace-nowrap text-center">
                打开课程阅读器
              </Link>
            )}
          </div>

          {taskId ? (
            <>
              <div className="rounded-2xl border border-white/60 bg-white/45 p-3 text-xs text-slate-500 shadow-inner">
                course_id: <code>{courseId}</code> · task_id: <code>{taskId}</code> · {connected ? '已连接' : '连接中…'}
                {closed && ' · 已关闭'}
                {error && <span className="text-red-600"> · {error}</span>}
              </div>
              <AgentTimeline events={events} activeAgent={activeAgent} />
              <ThinkingPanel events={events} />
              <div className="max-h-64 overflow-auto rounded-2xl bg-white/45 p-3 ring-1 ring-white/60">
                <ProgressFeed events={events} />
              </div>
            </>
          ) : (
            <div className="rounded-3xl border border-dashed border-white/70 bg-white/35 p-8 text-center text-sm text-slate-500">
              点击“开始建课”后，这里会显示 Agent 动画、实时进度和最终状态。
            </div>
          )}
        </section>
      </div>

      <section className="glass-card p-5 md:p-6">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div>
            <div className="text-xs uppercase tracking-[0.2em] text-slate-400">Recent</div>
            <h2 className="text-xl font-semibold">最近课程</h2>
          </div>
        </div>
        <ul className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {courses.map((c) => (
            <li key={c.id} className="rounded-3xl bg-white/50 p-4 shadow-sm ring-1 ring-white/70">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="font-medium text-slate-900">{c.topic}</div>
                  <div className="mt-1 text-xs text-slate-500">{c.audience || '通用'} · {c.depth || 'intermediate'}</div>
                </div>
                <StatusPill status={c.status} />
              </div>
              <Link to="/reader" search={{ courseId: c.id }} className="mt-3 inline-flex text-sm font-medium text-blue-700 underline">
                阅读课程
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}

function useActiveAgent(events: ProgressEvent[]): string {
  const last = [...events].reverse().find((e) => e.agent && e.type !== 'task.done' && e.type !== 'task.failed')
  if (!last?.agent) return 'planner'
  if (last.agent === 'builder') {
    const msg = last.message?.toLowerCase() || ''
    if (msg.includes('dag') || msg.includes('章节')) return 'validator'
    return 'planner'
  }
  return last.agent
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-3xl bg-white/45 p-4 shadow-sm ring-1 ring-white/70 backdrop-blur-xl">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-1 font-semibold text-slate-900">{value}</div>
    </div>
  )
}

function AgentOrbit({ activeAgent, terminalType }: { activeAgent: string; terminalType?: string }) {
  return (
    <div className="relative mx-auto h-80 w-full max-w-md">
      <div className="absolute inset-8 rounded-full border border-white/60 bg-white/25 shadow-inner backdrop-blur-2xl" />
      <div className="absolute left-1/2 top-1/2 grid h-28 w-28 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-[2rem] bg-slate-950 text-center text-white shadow-2xl shadow-slate-900/20">
        <div>
          <div className="text-xs text-white/60">MarsAgent</div>
          <div className="text-lg font-semibold">Course</div>
        </div>
      </div>
      {AGENTS.map((agent, idx) => {
        const angle = (idx / AGENTS.length) * Math.PI * 2 - Math.PI / 2
        const x = Math.cos(angle) * 132
        const y = Math.sin(angle) * 112
        const active = activeAgent.includes(agent.key)
        return (
          <div
            key={agent.key}
            className={clsx(
              'absolute left-1/2 top-1/2 h-24 w-28 -translate-x-1/2 -translate-y-1/2 rounded-3xl border border-white/70 bg-white/60 p-3 text-center shadow-lg backdrop-blur-xl transition duration-300',
              active && terminalType !== 'task.failed' && 'scale-105 ring-4 ring-blue-200/60',
              terminalType === 'task.done' && 'ring-2 ring-emerald-200/70',
            )}
            style={{ transform: `translate(calc(-50% + ${x}px), calc(-50% + ${y}px))` }}
          >
            <div className={clsx('mx-auto mb-2 h-3 w-12 rounded-full bg-gradient-to-r', agent.accent, active && 'agent-pulse')} />
            <div className="text-sm font-semibold text-slate-900">{agent.label}</div>
            <div className="mt-1 text-[11px] leading-4 text-slate-500">{agent.desc}</div>
          </div>
        )
      })}
    </div>
  )
}

function AgentTimeline({ events, activeAgent }: { events: ProgressEvent[]; activeAgent: string }) {
  return (
    <div className="grid gap-2 sm:grid-cols-5">
      {AGENTS.map((agent) => {
        const touched = events.some((e) => (e.agent || '').includes(agent.key)) || activeAgent.includes(agent.key)
        return (
          <div key={agent.key} className={clsx('rounded-2xl bg-white/45 p-3 ring-1 ring-white/60', touched && 'bg-white/70')}>
            <div className={clsx('mb-2 h-1.5 rounded-full bg-gradient-to-r', agent.accent, activeAgent.includes(agent.key) && 'agent-pulse')} />
            <div className="text-xs font-semibold text-slate-800">{agent.label}</div>
            <div className="mt-1 text-[11px] text-slate-500">{touched ? '已激活' : '等待中'}</div>
          </div>
        )
      })}
    </div>
  )
}

function StatusPill({ status }: { status: string }) {
  const cls = status === 'ready'
    ? 'bg-emerald-100 text-emerald-700'
    : status === 'failed'
      ? 'bg-red-100 text-red-700'
      : 'bg-amber-100 text-amber-700'
  return <span className={clsx('rounded-full px-2.5 py-1 text-xs font-medium', cls)}>{status}</span>
}
