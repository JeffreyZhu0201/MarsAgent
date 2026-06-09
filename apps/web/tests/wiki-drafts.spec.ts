import { expect, test } from '@playwright/test'

test('wiki draft editor shows draft title and can approve', async ({ page }) => {
  let approveCalled = false
  const draft = {
    id: 'd1',
    status: 'draft',
    title: 'Draft A',
    content_md: '# Draft A\n\nBody text',
    url: 'https://example.com/draft-a',
    source: 'test',
    category: 'general',
    revision: 1,
    updated_at: '',
  }

  await page.route('**/api/wiki/tree', (route) =>
    route.fulfill({ json: { docs: [] } }),
  )

  await page.route('**/api/wiki/drafts?status=draft', (route) =>
    route.fulfill({ json: { drafts: approveCalled ? [] : [draft] } }),
  )

  await page.route('**/api/wiki/drafts/d1', async (route) => {
    if (route.request().method() === 'PUT') {
      await route.fulfill({ json: { ...draft, title: 'Draft A', revision: 2 } })
      return
    }
    await route.fulfill({ json: draft })
  })

  await page.route('**/api/wiki/drafts/d1/approve', async (route) => {
    approveCalled = true
    await route.fulfill({ json: { slug: 'draft-a' } })
  })

  await page.goto('/wiki')

  await expect(page.getByText('Draft A')).toBeVisible()
  await page.getByRole('button', { name: 'Draft A' }).click()
  await expect(page.getByRole('button', { name: '确认发布' })).toBeVisible()

  await page.getByRole('button', { name: '确认发布' }).click()

  await expect.poll(() => approveCalled).toBe(true)
})
