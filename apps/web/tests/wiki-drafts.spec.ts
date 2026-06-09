import { expect, test } from '@playwright/test'

test('wiki draft editor shows draft title and approve button', async ({ page }) => {
  await page.route('**/api/wiki/tree', (route) =>
    route.fulfill({ json: { docs: [] } }),
  )

  await page.route('**/api/wiki/drafts?status=draft', (route) =>
    route.fulfill({
      json: {
        drafts: [
          {
            id: 'd1',
            status: 'draft',
            title: 'Draft A',
            content_md: '# Draft A\n\nBody text',
            url: '',
            source: 'test',
            category: 'general',
            revision: 1,
            updated_at: '',
          },
        ],
      },
    }),
  )

  await page.route('**/api/wiki/drafts/d1', (route) =>
    route.fulfill({ json: { ok: true } }),
  )

  await page.route('**/api/wiki/drafts/d1/approve', (route) =>
    route.fulfill({ json: { slug: 'draft-a' } }),
  )

  await page.goto('/wiki')

  // Draft title should appear in the review panel
  await expect(page.getByText('Draft A')).toBeVisible()

  // Click the draft to open the editor
  await page.getByRole('button', { name: 'Draft A' }).click()

  // Approve button should be visible in the editor
  await expect(page.getByRole('button', { name: '确认发布' })).toBeVisible()
})
