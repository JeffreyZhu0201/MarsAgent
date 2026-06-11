import {
  createRootRoute,
  createRoute,
  createRouter,
  Navigate,
} from '@tanstack/react-router'
import { App } from './App'
import { WikiBrowser } from './views/WikiBrowser'
import { CourseBuilder } from './views/CourseBuilder'
import { CourseReader } from './views/CourseReader'
import { ProblemList } from './views/ProblemList'
import { ProblemDetail } from './views/ProblemDetail'
import { SubmissionHistory } from './views/SubmissionHistory'

const rootRoute = createRootRoute({ component: App })

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: () => <Navigate to="/builder" />,
})

const wikiRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/wiki',
  component: WikiBrowser,
})

const builderRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/builder',
  component: CourseBuilder,
})

const readerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/reader',
  validateSearch: (search: Record<string, unknown>) => ({
    courseId: typeof search.courseId === 'string' ? search.courseId : undefined,
  }),
  component: CourseReader,
})

const problemsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/problems',
  component: ProblemList,
})

const problemDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/problems/$problemId',
  component: ProblemDetail,
})

const submissionsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/submissions',
  component: SubmissionHistory,
})

const routeTree = rootRoute.addChildren([
  indexRoute, wikiRoute, builderRoute, readerRoute,
  problemsRoute, problemDetailRoute, submissionsRoute,
])
export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
