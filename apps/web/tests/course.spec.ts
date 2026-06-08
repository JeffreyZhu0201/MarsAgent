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
