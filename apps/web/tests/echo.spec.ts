import { test, expect } from '@playwright/test'

test('echo round trip via SSE', async ({ page }) => {
  await page.goto('/builder')
  await page.getByRole('button', { name: /Send Echo/ }).click()

  // 等到 task.done badge 出现，最多 10s
  await expect(page.locator('text=task.done')).toBeVisible({ timeout: 10_000 })

  // 至少 1 条 agent.progress
  const progress = page.locator('text=agent.progress')
  await expect(progress.first()).toBeVisible()
})
