export interface EchoResponse { task_id: string }

export interface WikiCollectRequest {
  topic: string
  sources?: string[]
  max_per_source?: number
}

export interface WikiCollectResponse {
  task_id: string
}

export interface WikiDraft {
  id: string
  status: string
  title: string
  content_md: string
  url: string
  source: string
  category: string
  revision: number
  updated_at: string
  task_id?: string
  summary?: string
  quality_score?: number
  language?: string
  created_at?: string
  published_at?: string
  wiki_doc_id?: string
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

export async function listDrafts(status = 'draft'): Promise<WikiDraft[]> {
  const r = await json<{ drafts: WikiDraft[] }>(`/api/wiki/drafts?status=${encodeURIComponent(status)}`)
  return r.drafts || []
}

export async function createDraft(input: Partial<WikiDraft>): Promise<WikiDraft> {
  return json('/api/wiki/drafts', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function updateDraft(id: string, input: Partial<WikiDraft>): Promise<WikiDraft> {
  return json(`/api/wiki/drafts/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function approveDraft(id: string): Promise<{ slug: string }> {
  return json(`/api/wiki/drafts/${encodeURIComponent(id)}/approve`, { method: 'POST' })
}

export async function rejectDraft(id: string): Promise<{ ok: boolean }> {
  return json(`/api/wiki/drafts/${encodeURIComponent(id)}/reject`, { method: 'POST' })
}

// --- OJ (Online Judge) ---

export type OJStatus = 'pending' | 'judging' | 'accepted' | 'wrong_answer' | 'tle' | 'mle' | 're' | 'ce'

export interface OJProblem {
  id: string
  title: string
  description_md: string
  tags: string[]
  difficulty: 'easy' | 'medium' | 'hard'
  time_limit_ms: number
  memory_limit_mb: number
  visible: boolean
  created_at: string
}

export interface OJTestCase {
  id: string
  problem_id: string
  input: string
  expected_output: string
  is_sample: boolean
  is_hidden: boolean
  score: number
  ordering: number
}

export interface OJSubmission {
  id: string
  problem_id: string
  code: string
  lang: string
  status: OJStatus
  score: number
  duration_ms: number
  memory_kb: number
  error_msg?: string
  created_at: string
}

export interface OJSubmissionResult {
  id: string
  submission_id: string
  test_case_id: string
  status: OJStatus
  actual_output?: string
  duration_ms: number
  memory_kb: number
  score: number
}

export async function listOJProblems(params: {
  limit?: number; offset?: number; difficulty?: string; tag?: string
} = {}): Promise<{ problems: OJProblem[]; total: number }> {
  const qs = new URLSearchParams()
  if (params.limit) qs.set('limit', String(params.limit))
  if (params.offset) qs.set('offset', String(params.offset))
  if (params.difficulty) qs.set('difficulty', params.difficulty)
  if (params.tag) qs.set('tag', params.tag)
  return json(`/api/oj/problems?${qs}`)
}

export async function getOJProblem(id: string): Promise<{ problem: OJProblem; sample_test_cases: OJTestCase[] }> {
  return json(`/api/oj/problems/${encodeURIComponent(id)}`)
}

export async function createOJSubmission(input: {
  problem_id: string; code: string; lang: string
}): Promise<{ submission_id: string }> {
  return json('/api/oj/submissions', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(input),
  })
}

export async function getOJSubmission(id: string): Promise<{
  submission: OJSubmission
  results: OJSubmissionResult[]
}> {
  return json(`/api/oj/submissions/${encodeURIComponent(id)}`)
}

export async function listOJSubmissions(params: {
  problem_id?: string; user_id?: string; limit?: number; offset?: number
} = {}): Promise<{ submissions: OJSubmission[]; total: number }> {
  const qs = new URLSearchParams()
  if (params.problem_id) qs.set('problem_id', params.problem_id)
  if (params.user_id) qs.set('user_id', params.user_id)
  if (params.limit) qs.set('limit', String(params.limit))
  if (params.offset) qs.set('offset', String(params.offset))
  return json(`/api/oj/submissions?${qs}`)
}
