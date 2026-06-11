import { expect, test } from '@playwright/test'

const MOCK_PROBLEMS = [
  {
    id: 'prob-1',
    title: 'A + B',
    description_md: '## 题目描述\n\n读入两个整数，输出它们的和。',
    tags: ['math', 'implementation'],
    difficulty: 'easy',
    time_limit_ms: 1000,
    memory_limit_mb: 256,
    visible: true,
    created_at: '2026-06-01T00:00:00Z',
  },
  {
    id: 'prob-2',
    title: 'Fibonacci',
    description_md: '## Fibonacci\n\n输出第 n 项斐波那契数。',
    tags: ['dp'],
    difficulty: 'medium',
    time_limit_ms: 2000,
    memory_limit_mb: 256,
    visible: true,
    created_at: '2026-06-02T00:00:00Z',
  },
]

const MOCK_SAMPLES = [
  {
    id: 'tc-1',
    problem_id: 'prob-1',
    input: '1 2',
    expected_output: '3',
    is_sample: true,
    is_hidden: false,
    score: 100,
    ordering: 0,
  },
]

const MOCK_SUBMISSIONS = [
  {
    id: 'sub-accepted-001',
    problem_id: 'prob-1',
    code: 'print(3)',
    lang: 'python',
    status: 'accepted',
    score: 100,
    duration_ms: 42,
    memory_kb: 0,
    created_at: '2026-06-10T12:00:00Z',
  },
  {
    id: 'sub-wa-002',
    problem_id: 'prob-2',
    code: 'print(0)',
    lang: 'python',
    status: 'wrong_answer',
    score: 0,
    duration_ms: 38,
    memory_kb: 0,
    created_at: '2026-06-11T08:30:00Z',
  },
]

function mockProblemList(page: import('@playwright/test').Page, problems = MOCK_PROBLEMS) {
  return page.route('**/api/oj/problems**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue()
      return
    }
    const url = new URL(route.request().url())
    const difficulty = url.searchParams.get('difficulty') ?? ''
    const filtered = difficulty
      ? problems.filter((p) => p.difficulty === difficulty)
      : problems
    await route.fulfill({ json: { problems: filtered, total: filtered.length } })
  })
}

test('problem list shows problems and filters by difficulty', async ({ page }) => {
  await mockProblemList(page)

  await page.goto('/problems')

  await expect(page.getByRole('heading', { name: 'Problems' })).toBeVisible()
  await expect(page.getByText('2 total')).toBeVisible()
  await expect(page.getByRole('button', { name: /A \+ B/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /Fibonacci/ })).toBeVisible()
  await expect(page.getByText('math')).toBeVisible()

  await page.getByRole('combobox').selectOption('easy')
  await expect(page.getByRole('button', { name: /A \+ B/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /Fibonacci/ })).not.toBeVisible()
  await expect(page.getByText('1 total')).toBeVisible()
})

test('problem list empty state', async ({ page }) => {
  await mockProblemList(page, [])

  await page.goto('/problems')

  await expect(page.getByText('No problems found.')).toBeVisible()
})

test('problem detail shows description, samples, and accepts submission', async ({ page }) => {
  await page.route('**/api/oj/problems/prob-1', async (route) => {
    await route.fulfill({
      json: { problem: MOCK_PROBLEMS[0], sample_test_cases: MOCK_SAMPLES },
    })
  })

  let pollCount = 0
  await page.route('**/api/oj/submissions', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ status: 202, json: { submission_id: 'sub-new' } })
      return
    }
    await route.continue()
  })

  await page.route('**/api/oj/submissions/sub-new', async (route) => {
    pollCount += 1
    const status = pollCount === 1 ? 'judging' : 'accepted'
    await route.fulfill({
      json: {
        submission: {
          id: 'sub-new',
          problem_id: 'prob-1',
          code: 'print(3)',
          lang: 'python',
          status,
          score: status === 'accepted' ? 100 : 0,
          duration_ms: status === 'accepted' ? 55 : 0,
          memory_kb: 0,
          created_at: '2026-06-12T00:00:00Z',
        },
        results: [],
      },
    })
  })

  await page.goto('/problems/prob-1')

  await expect(page.getByRole('heading', { name: 'A + B' })).toBeVisible()
  await expect(page.getByText('读入两个整数')).toBeVisible()
  await expect(page.getByText('Sample Input/Output')).toBeVisible()
  await expect(page.getByText('3', { exact: true })).toBeVisible()

  await page.getByPlaceholder('Enter your code...').fill('print(3)')
  await page.getByRole('button', { name: 'Submit' }).click()

  await expect(page.getByText('✅ Accepted')).toBeVisible({ timeout: 5_000 })
  await expect(page.getByText('Score: 100')).toBeVisible()
})

test('problem detail shows wrong answer result', async ({ page }) => {
  await page.route('**/api/oj/problems/prob-1', async (route) => {
    await route.fulfill({
      json: { problem: MOCK_PROBLEMS[0], sample_test_cases: MOCK_SAMPLES },
    })
  })

  await page.route('**/api/oj/submissions', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ status: 202, json: { submission_id: 'sub-wa' } })
      return
    }
    await route.continue()
  })

  await page.route('**/api/oj/submissions/sub-wa', async (route) => {
    await route.fulfill({
      json: {
        submission: {
          id: 'sub-wa',
          problem_id: 'prob-1',
          code: 'print(0)',
          lang: 'python',
          status: 'wrong_answer',
          score: 0,
          duration_ms: 30,
          memory_kb: 0,
          created_at: '2026-06-12T00:00:00Z',
        },
        results: [],
      },
    })
  })

  await page.goto('/problems/prob-1')
  await page.getByPlaceholder('Enter your code...').fill('print(0)')
  await page.getByRole('button', { name: 'Submit' }).click()

  await expect(page.getByText('❌ WRONG_ANSWER')).toBeVisible()
})

test('navigate from problem list to detail and back', async ({ page }) => {
  await mockProblemList(page)
  await page.route('**/api/oj/problems/prob-1', async (route) => {
    await route.fulfill({
      json: { problem: MOCK_PROBLEMS[0], sample_test_cases: MOCK_SAMPLES },
    })
  })

  await page.goto('/problems')
  await page.getByRole('button', { name: /A \+ B/ }).click()

  await expect(page).toHaveURL(/\/problems\/prob-1/)
  await expect(page.getByRole('heading', { name: 'A + B' })).toBeVisible()

  await page.getByRole('button', { name: 'Back to Problems' }).click()
  await expect(page).toHaveURL(/\/problems$/)
})

test('submission history lists submissions with status labels', async ({ page }) => {
  await page.route('**/api/oj/submissions**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.continue()
      return
    }
    await route.fulfill({ json: { submissions: MOCK_SUBMISSIONS, total: 2 } })
  })

  await page.goto('/submissions')

  await expect(page.getByRole('heading', { name: 'Submissions' })).toBeVisible()
  await expect(page.getByText('Accepted')).toBeVisible()
  await expect(page.getByText('Wrong Answer')).toBeVisible()
  await expect(page.getByText('Score: 100')).toBeVisible()
  await expect(page.getByText('python').first()).toBeVisible()
})

test('submission history empty state links to problems', async ({ page }) => {
  await page.route('**/api/oj/submissions**', async (route) => {
    await route.fulfill({ json: { submissions: [], total: 0 } })
  })
  await mockProblemList(page)

  await page.goto('/submissions')

  await expect(page.getByText('No submissions yet.')).toBeVisible()
  await page.getByRole('button', { name: 'Browse problems' }).click()
  await expect(page).toHaveURL(/\/problems$/)
})

test('navbar navigates to OJ pages', async ({ page }) => {
  await mockProblemList(page)
  await page.route('**/api/oj/submissions**', async (route) => {
    await route.fulfill({ json: { submissions: [], total: 0 } })
  })

  await page.goto('/builder')
  await page.getByRole('link', { name: 'OJ 题库' }).click()
  await expect(page).toHaveURL(/\/problems$/)

  await page.getByRole('link', { name: '提交记录' }).click()
  await expect(page).toHaveURL(/\/submissions$/)
})
