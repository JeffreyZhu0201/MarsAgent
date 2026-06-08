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
