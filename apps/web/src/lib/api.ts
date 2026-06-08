export interface EchoResponse { task_id: string }

export interface WikiCollectRequest {
  topic: string
  sources?: string[]
  max_per_source?: number
}

export interface WikiCollectResponse {
  task_id: string
}

export interface RagHit {
  doc_id: string
  chunk_id: string
  text: string
  score: number
  url: string
  source: string
  title?: string
  payload?: Record<string, unknown>
}

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

export interface QuizOption {
  label?: string
  text?: string
}

export interface QuizItem {
  type: string
  question: string
  options?: Array<string | QuizOption>
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
  const method = init?.method ?? 'GET'
  const started = performance.now()
  console.debug(`[MarsAgent:api] ${method} ${url} -> request`, {
    body: init?.body ? safeJson(init.body) : undefined,
  })
  const r = await fetch(url, init)
  const durationMs = Math.round(performance.now() - started)
  if (!r.ok) {
    const text = await r.text()
    console.error(`[MarsAgent:api] ${method} ${url} -> error`, {
      status: r.status,
      durationMs,
      body: text,
    })
    throw new Error(`${method} ${url} failed: ${r.status} ${text}`)
  }
  const data = await r.json() as T
  console.debug(`[MarsAgent:api] ${method} ${url} -> response`, {
    status: r.status,
    durationMs,
    data,
  })
  return data
}

function safeJson(body: BodyInit): unknown {
  if (typeof body !== 'string') return '[non-string body]'
  try {
    return JSON.parse(body)
  } catch {
    return body
  }
}

export async function postEcho(msg: string): Promise<EchoResponse> {
  return json('/api/echo', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ msg }),
  })
}

export async function collectWiki(req: WikiCollectRequest): Promise<WikiCollectResponse> {
  return json('/api/wiki/collect', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(req),
  })
}

export async function searchWiki(q: string, k = 20): Promise<RagHit[]> {
  const r = await json<{ hits: RagHit[] }>('/api/wiki/search', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ q, k }),
  })
  return r.hits || []
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
