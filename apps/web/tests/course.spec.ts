import { expect, test } from '@playwright/test'

const THINKING_EVENTS = [
  { type: 'agent.start', task_id: 'task-1', agent: 'builder', message: '开始建课: Python 异步编程', ts: 1 },
  { type: 'agent.thinking', task_id: 'task-1', agent: 'planner', message: 'Planner 正在思考课程结构...', ts: 2 },
  { type: 'agent.thinking', task_id: 'task-1', agent: 'planner', message: 'Planner 推理过程:\n让我分析课程目标受众是已有基础的 Python 开发者...\n我将设计一个包含异步基础、异步IO、asyncio实质的三章结构。', ts: 3 },
  { type: 'agent.thinking', task_id: 'task-1', agent: 'author', message: 'Author 正在撰写章节「异步基础」...', ts: 4 },
  { type: 'agent.thinking', task_id: 'task-1', agent: 'author', message: 'Author 推理过程:\n需要从基本概念入手，解释协程与生成器的区别...', ts: 5 },
  { type: 'agent.progress', task_id: 'task-1', agent: 'builder', message: 'DAG 完成，共 3 章', ts: 6 },
  { type: 'task.done', task_id: 'task-1', message: '课程构建完成，共 3 章', ts: 7 },
]

function sseStream(events: object[]): string {
  return events.map((ev) => `data: ${JSON.stringify(ev)}\n\n`).join('')
}

test('course builder creates a course and shows progress', async ({ page }) => {
  await page.route('**/api/courses', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ json: { id: 'course-1', task_id: 'task-1' } })
      return
    }

    await route.fulfill({
      json: {
        courses: [
          {
            id: 'course-1',
            topic: 'Python',
            audience: '',
            depth: 'intermediate',
            status: 'ready',
            outline_json: '[]',
            storage_prefix: 'courses/course-1/',
            created_at: '',
            updated_at: '',
          },
        ],
      },
    })
  })

  await page.route('**/api/stream/task-1', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      body: 'data: {"type":"task.done","task_id":"task-1","message":"done","ts":1}\n\n',
    })
  })

  await page.goto('/builder')
  await page.getByRole('button', { name: '开始建课' }).click()

  await expect(page.getByText('task.done')).toBeVisible()
})

test('course builder shows thinking panel with agent reasoning events', async ({ page }) => {
  await page.route('**/api/courses', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ json: { id: 'course-think', task_id: 'task-think' } })
      return
    }
    await route.fulfill({ json: { courses: [] } })
  })

  let sent = false
  await page.route('**/api/stream/task-think', async (route) => {
    if (sent) return
    sent = true
    await route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      body: sseStream(THINKING_EVENTS),
    })
  })

  await page.goto('/builder')
  await page.getByRole('button', { name: '开始建课' }).click()

  // ThinkingPanel: LLM 推理过程 label should be visible
  await expect(page.getByText('LLM 推理过程')).toBeVisible()

  // ThinkingPanel: Planner reasoning should appear
  await expect(page.getByText('Planner 推理中').first()).toBeVisible()

  // ProgressFeed: agent.thinking badge should appear
  await expect(page.locator('.font-mono.text-xs')).toContainText('agent.thinking')

  // ProgressFeed: thinking events show truncated message (not full content in feed)
  await expect(page.locator('.font-mono.text-xs')).toContainText('planner')

  // ThinkingPanel: can expand a thinking card to see full reasoning
  await page.getByText('Planner 推理中').first().click()
  // Content should appear in the expanded thinking card (pre element)
  await expect(page.locator('pre').filter({ hasText: /需要从基本概念入手/ })).toBeVisible()
})

test('course reader renders quiz options returned as objects', async ({ page }) => {
  const outline = [
    {
      ch_id: 'ch_01',
      title: '条件语句',
      content_md: '# 条件语句',
      code_examples: [],
      quiz: [
        {
          type: 'MCQ',
          question: '哪个是 True？',
          options: [
            { label: 'A', text: '3 < 5' },
            { label: 'B', text: '3 > 5' },
          ],
          answer: 'A',
          explanation: '3 小于 5。',
        },
      ],
    },
  ]

  await page.route('**/api/courses/course-objects', async (route) => {
    await route.fulfill({
      json: {
        id: 'course-objects',
        topic: 'Python 条件语句',
        audience: 'beginner',
        depth: 'beginner',
        status: 'ready',
        outline_json: JSON.stringify(outline),
        storage_prefix: 'courses/course-objects/',
        created_at: '',
        updated_at: '',
      },
    })
  })
  await page.route('**/api/courses/course-objects/chapter/ch_01', async (route) => {
    await route.fulfill({ json: { content: '# 条件语句\n\n正文' } })
  })
  await page.route('**/api/courses', async (route) => {
    await route.fulfill({ json: { courses: [] } })
  })

  await page.goto('/reader?courseId=course-objects')

  await expect(page.getByText('A. 3 < 5')).toBeVisible()
  await expect(page.getByText('B. 3 > 5')).toBeVisible()
})
