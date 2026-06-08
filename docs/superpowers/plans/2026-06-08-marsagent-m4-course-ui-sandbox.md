# MarsAgent M4 — Course UI + Sandbox Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the M3 backend course builder into a usable interactive course demo: users submit a topic, watch 5-agent SSE progress, open the generated course, read Markdown chapters, run code examples in Docker sandbox, and inspect quiz answers.

**Architecture:** M4 is a frontend-heavy integration milestone plus small gateway API additions. Go exposes course list/chapter endpoints that read Postgres + MinIO, while React replaces the M1 echo demo with real course creation and a CourseReader that renders outline, chapter Markdown, code examples, and quizzes. The existing sandbox scheduler remains the execution backend, with UI controls and safer output handling.

**Tech Stack:** Go/Gin/Postgres/MinIO, Python LangGraph worker outputs from M3, React 18 + TanStack Router + ReactMarkdown + Vite, Docker sandbox.

---

## File Structure

```
MarsAgent/
├── apps/gateway/
│   └── internal/
│       ├── api/
│       │   ├── course.go       # modify: list courses + chapter content endpoint + stable JSON response
│       │   └── router.go       # modify: register GET /api/courses and /api/courses/:id/chapter/:ch_id
│       └── store/
│           └── course.go       # modify: ListCourses + ChapterContent via MinIO client
│
└── apps/web/src/
    ├── lib/
    │   └── api.ts              # modify: courses/sandbox API helpers + types
    ├── components/
    │   ├── CodeEditor.tsx      # new: textarea editor + Run button + stdout/stderr panel
    │   └── QuizPanel.tsx       # new: quiz render + answer toggle
    └── views/
        ├── CourseBuilder.tsx   # modify: real POST /api/courses + SSE + link to reader
        └── CourseReader.tsx    # modify: full reader implementation
```

---

## Task M4-0: Gateway course read API

**Files:**
- Modify: `apps/gateway/internal/store/course.go`
- Modify: `apps/gateway/internal/api/course.go`
- Modify: `apps/gateway/internal/api/router.go`

- [ ] **Step 1: Add JSON tags and list support to Course**

Modify `apps/gateway/internal/store/course.go` so `Course` has stable JSON field names and add `ListCourses`:

```go
// Course 代表一门课程。
type Course struct {
	ID            string `json:"id"`
	Topic         string `json:"topic"`
	Audience      string `json:"audience"`
	Depth         string `json:"depth"`
	Status        string `json:"status"`
	OutlineJSON   string `json:"outline_json"`
	StoragePrefix string `json:"storage_prefix"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// ListCourses returns recent courses for the single-tenant MVP.
func (s *CourseStore) ListCourses(ctx context.Context, limit int) ([]Course, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,topic,coalesce(audience,''),coalesce(depth,''),status,
		        coalesce(outline_json,'null'),coalesce(storage_prefix,''),
		        created_at,updated_at
		 FROM courses ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	courses := make([]Course, 0)
	for rows.Next() {
		var c Course
		var outlineJSON sql.NullString
		if err := rows.Scan(&c.ID, &c.Topic, &c.Audience, &c.Depth, &c.Status,
			&outlineJSON, &c.StoragePrefix, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if outlineJSON.Valid {
			c.OutlineJSON = outlineJSON.String
		}
		courses = append(courses, c)
	}
	return courses, rows.Err()
}
```

- [ ] **Step 2: Add chapter content helper**

In `apps/gateway/internal/store/course.go`, add a simple MinIO reader. Use env vars matching Python M2 storage defaults.

```go
import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)
```

Add:

```go
// GetChapterMarkdown reads courses/{id}/{chID}.md from MinIO.
func (s *CourseStore) GetChapterMarkdown(ctx context.Context, courseID, chID string) (string, error) {
	course, err := s.GetCourse(ctx, courseID)
	if err != nil {
		return "", err
	}
	prefix := course.StoragePrefix
	if prefix == "" {
		prefix = fmt.Sprintf("courses/%s/", courseID)
	}
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}
	accessKey := os.Getenv("MINIO_ROOT_USER")
	if accessKey == "" {
		accessKey = "minio"
	}
	secretKey := os.Getenv("MINIO_ROOT_PASSWORD")
	if secretKey == "" {
		secretKey = "minio_dev_pw"
	}
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return "", err
	}
	obj, err := mc.GetObject(ctx, "marsagent", prefix+chID+".md", minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()
	b, err := io.ReadAll(obj)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
```

- [ ] **Step 3: Add handlers**

Modify `apps/gateway/internal/api/course.go`:

```go
// GET /api/courses — list recent courses.
func listCoursesHandler(cs *store.CourseStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		courses, err := cs.ListCourses(c.Request.Context(), 20)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"courses": courses})
	}
}

// GET /api/courses/:id/chapter/:ch_id — fetch chapter markdown.
func getCourseChapterHandler(cs *store.CourseStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		md, err := cs.GetChapterMarkdown(c.Request.Context(), c.Param("id"), c.Param("ch_id"))
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": md})
	}
}
```

- [ ] **Step 4: Register routes**

Modify `apps/gateway/internal/api/router.go` inside the `if d.CourseStore != nil && d.Producer != nil` block or split read routes so reads work without producer:

```go
if d.CourseStore != nil {
	api.GET("/courses", listCoursesHandler(d.CourseStore))
	api.GET("/courses/:id", getCourseHandler(d.CourseStore))
	api.GET("/courses/:id/chapter/:ch_id", getCourseChapterHandler(d.CourseStore))
}
if d.CourseStore != nil && d.Producer != nil {
	api.POST("/courses", createCourseHandler(d.CourseStore, d.Producer))
}
```

- [ ] **Step 5: Verify and commit**

Run:

```bash
cd apps/gateway
go get github.com/minio/minio-go/v7@v7.0.82
go build ./...
```

Commit:

```bash
git add apps/gateway/go.mod apps/gateway/go.sum apps/gateway/internal/store/course.go apps/gateway/internal/api/course.go apps/gateway/internal/api/router.go
git commit -m "feat(gateway): add course list and chapter read APIs"
```

---

## Task M4-1: Frontend API client for courses and sandbox

**Files:**
- Modify: `apps/web/src/lib/api.ts`

- [ ] **Step 1: Add types and helpers**

Replace `apps/web/src/lib/api.ts` with:

```ts
export interface EchoResponse { task_id: string }

export interface CourseCreateRequest {
  topic: string
  audience?: string
  depth?: string
}

export interface CourseCreateResponse {
  id: string
  task_id: string
}

export interface Chapter {
  ch_id: string
  title: string
  objectives?: string[]
  prereqs?: string[]
  est_min?: number
  bloom_level?: string
  key_concepts?: string[]
  content_md?: string
  code_examples?: CodeExample[]
  quiz?: QuizItem[]
  status?: string
}

export interface Course {
  id: string
  topic: string
  audience: string
  depth: string
  status: string
  outline_json: string
  storage_prefix: string
  created_at: string
  updated_at: string
}

export interface CodeExample {
  lang: string
  title: string
  code: string
  expected_output?: string
}

export interface QuizItem {
  type: string
  question: string
  options?: string[]
  answer?: string
  explanation?: string
}

export interface SandboxResult {
  stdout: string
  stderr: string
  exit_code: number
  duration_ms: number
  truncated: boolean
}

async function json<T>(url: string, init?: RequestInit): Promise<T> {
  const r = await fetch(url, init)
  if (!r.ok) {
    const text = await r.text()
    throw new Error(`${init?.method ?? 'GET'} ${url} failed: ${r.status} ${text}`)
  }
  return r.json()
}

export async function postEcho(msg: string): Promise<EchoResponse> {
  return json('/api/echo', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ msg }),
  })
}

export async function createCourse(req: CourseCreateRequest): Promise<CourseCreateResponse> {
  return json('/api/courses', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(req),
  })
}

export async function listCourses(): Promise<Course[]> {
  const r = await json<{ courses: Course[] }>('/api/courses')
  return r.courses || []
}

export async function getCourse(id: string): Promise<Course> {
  return json(`/api/courses/${encodeURIComponent(id)}`)
}

export async function getChapterMarkdown(courseId: string, chId: string): Promise<string> {
  const r = await json<{ content: string }>(`/api/courses/${encodeURIComponent(courseId)}/chapter/${encodeURIComponent(chId)}`)
  return r.content || ''
}

export async function runSandbox(input: { lang: string; code: string; stdin?: string; timeout?: number }): Promise<SandboxResult> {
  return json('/api/sandbox/run', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export function parseOutline(course: Course): Chapter[] {
  try {
    const parsed = JSON.parse(course.outline_json || '[]')
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}
```

- [ ] **Step 2: Verify and commit**

Run:

```bash
cd apps/web
npm run build
```

Commit:

```bash
git add apps/web/src/lib/api.ts
git commit -m "feat(web): add course and sandbox API client"
```

---

## Task M4-2: CourseBuilder real course creation UI

**Files:**
- Modify: `apps/web/src/views/CourseBuilder.tsx`

- [ ] **Step 1: Replace echo demo with course form**

Replace `apps/web/src/views/CourseBuilder.tsx` with:

```tsx
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
    listCourses().then(setCourses).catch(console.error)
  }, [closed])

  async function onBuild() {
    setSubmitting(true)
    try {
      const r = await createCourse({ topic, audience, depth })
      setTaskId(r.task_id)
      setCourseId(r.id)
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
```

- [ ] **Step 2: Adjust router search typing if build requires it**

If TypeScript complains about `search={{ courseId }}`, update `apps/web/src/routes.tsx` reader route with a search validator:

```tsx
const readerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/reader',
  validateSearch: (search: Record<string, unknown>) => ({
    courseId: typeof search.courseId === 'string' ? search.courseId : undefined,
  }),
  component: CourseReader,
})
```

- [ ] **Step 3: Verify and commit**

Run:

```bash
cd apps/web
npm run build
```

Commit:

```bash
git add apps/web/src/views/CourseBuilder.tsx apps/web/src/routes.tsx
git commit -m "feat(web): build courses from CourseBuilder"
```

---

## Task M4-3: CodeEditor component with sandbox execution

**Files:**
- Create: `apps/web/src/components/CodeEditor.tsx`

- [ ] **Step 1: Create CodeEditor**

Create `apps/web/src/components/CodeEditor.tsx`:

```tsx
import { useState } from 'react'
import { runSandbox, type CodeExample, type SandboxResult } from '@/lib/api'

function normalizeLang(lang: string) {
  const l = lang.toLowerCase()
  if (l.includes('javascript') || l.includes('node')) return 'node'
  if (l.includes('go')) return 'go'
  return 'python'
}

export function CodeEditor({ example }: { example: CodeExample }) {
  const [code, setCode] = useState(example.code || '')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<SandboxResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function onRun() {
    setRunning(true)
    setError(null)
    try {
      const r = await runSandbox({ lang: normalizeLang(example.lang), code, timeout: 15 })
      setResult(r)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setRunning(false)
    }
  }

  return (
    <div className="border rounded bg-slate-950 text-slate-100 overflow-hidden my-4">
      <div className="flex items-center justify-between px-3 py-2 bg-slate-900 text-xs">
        <span>{example.title || 'Code'} · {example.lang || 'python'}</span>
        <button onClick={onRun} disabled={running} className="bg-emerald-600 text-white px-2 py-1 rounded disabled:opacity-50">
          {running ? 'Running…' : 'Run'}
        </button>
      </div>
      <textarea
        className="w-full min-h-48 bg-slate-950 text-slate-100 font-mono text-xs p-3 outline-none"
        value={code}
        onChange={(e) => setCode(e.target.value)}
        spellCheck={false}
      />
      {example.expected_output && <div className="px-3 py-2 text-xs text-slate-400 border-t border-slate-800">Expected: <pre className="whitespace-pre-wrap inline">{example.expected_output}</pre></div>}
      {error && <div className="px-3 py-2 text-xs text-red-300 border-t border-slate-800">{error}</div>}
      {result && (
        <div className="grid md:grid-cols-2 gap-0 border-t border-slate-800 text-xs">
          <pre className="p-3 whitespace-pre-wrap overflow-auto"><b>stdout</b>\n{result.stdout || '(empty)'}</pre>
          <pre className="p-3 whitespace-pre-wrap overflow-auto border-l border-slate-800"><b>stderr</b>\n{result.stderr || '(empty)'}\nexit={result.exit_code} · {result.duration_ms}ms</pre>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Verify and commit**

Run:

```bash
cd apps/web
npm run build
```

Commit:

```bash
git add apps/web/src/components/CodeEditor.tsx
git commit -m "feat(web): add sandbox-backed code editor"
```

---

## Task M4-4: QuizPanel component

**Files:**
- Create: `apps/web/src/components/QuizPanel.tsx`

- [ ] **Step 1: Create QuizPanel**

Create `apps/web/src/components/QuizPanel.tsx`:

```tsx
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
```

- [ ] **Step 2: Verify and commit**

Run:

```bash
cd apps/web
npm run build
```

Commit:

```bash
git add apps/web/src/components/QuizPanel.tsx
git commit -m "feat(web): add quiz panel"
```

---

## Task M4-5: CourseReader full implementation

**Files:**
- Modify: `apps/web/src/views/CourseReader.tsx`

- [ ] **Step 1: Implement reader**

Replace `apps/web/src/views/CourseReader.tsx` with:

```tsx
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
```

- [ ] **Step 2: Verify and commit**

Run:

```bash
cd apps/web
npm run build
```

Commit:

```bash
git add apps/web/src/views/CourseReader.tsx
git commit -m "feat(web): implement interactive course reader"
```

---

## Task M4-6: E2E smoke test and final review

**Files:**
- Create/Modify: `apps/web/tests/course.spec.ts`
- Modify if needed: `apps/web/playwright.config.ts`

- [ ] **Step 1: Add Playwright smoke test with mocked API**

Create `apps/web/tests/course.spec.ts`:

```ts
import { test, expect } from '@playwright/test'

test('course builder creates a course and shows progress', async ({ page }) => {
  await page.route('/api/courses', async route => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ json: { id: 'course-1', task_id: 'task-1' } })
      return
    }
    await route.fulfill({ json: { courses: [{ id: 'course-1', topic: 'Python', status: 'ready', outline_json: '[]', storage_prefix: 'courses/course-1/', created_at: '', updated_at: '' }] } })
  })
  await page.route('/api/stream/task-1', async route => {
    await route.fulfill({ status: 200, contentType: 'text/event-stream', body: 'data: {"type":"task.done","task_id":"task-1","message":"done","ts":1}\n\n' })
  })

  await page.goto('/builder')
  await page.getByRole('button', { name: '开始建课' }).click()
  await expect(page.getByText('task.done')).toBeVisible()
})
```

- [ ] **Step 2: Run checks**

Run:

```bash
cd apps/gateway
go test ./...
cd ../web
npm run build
npm run test:e2e -- --project=chromium tests/course.spec.ts
```

Expected:
- Go tests pass
- web build passes
- Playwright smoke passes

- [ ] **Step 3: Final code review**

Dispatch a fresh reviewer subagent to review M4 only. It must inspect:
- `apps/gateway/internal/store/course.go`
- `apps/gateway/internal/api/course.go`
- `apps/web/src/lib/api.ts`
- `apps/web/src/views/CourseBuilder.tsx`
- `apps/web/src/views/CourseReader.tsx`
- `apps/web/src/components/CodeEditor.tsx`
- `apps/web/src/components/QuizPanel.tsx`

Fix any high-confidence issues, rerun checks, then commit review fixes.

- [ ] **Step 4: Commit test**

```bash
git add apps/web/tests/course.spec.ts apps/web/playwright.config.ts
git commit -m "test(web): add course builder smoke test"
```

---

## M4 Acceptance Checklist

- [ ] `POST /api/courses` from CourseBuilder starts a course build and streams progress.
- [ ] CourseBuilder lists recent courses and links to reader.
- [ ] `GET /api/courses` returns recent courses.
- [ ] `GET /api/courses/:id/chapter/:ch_id` returns chapter Markdown from MinIO.
- [ ] CourseReader renders outline + Markdown content.
- [ ] CourseReader renders code examples and `Run` calls `/api/sandbox/run`.
- [ ] CourseReader renders quizzes and answer toggles.
- [ ] `go build ./...`, `go test ./...`, `npm run build`, and course Playwright smoke pass.

---

## Self-Review

| spec section | coverage |
|---|---|
| §4.2 Course artifacts + reader | M4-0, M4-5 |
| §4.3 Code running | M4-3, existing M3 sandbox |
| §5.1 Course REST APIs | M4-0 |
| §7 M4 interactive course demo | M4-2 through M4-6 |

**Placeholder scan:** No TBD/TODO placeholders. Each task includes exact files, code, commands, and commit messages.

**Known scope boundary:** Monaco is deferred in this plan in favor of a textarea-backed editor for a reliable M4 demo without adding large UI dependencies. The component boundary (`CodeEditor`) allows swapping in Monaco later without changing CourseReader.
