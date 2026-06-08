import { expect, test } from '@playwright/test'

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
