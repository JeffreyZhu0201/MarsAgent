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

const routeTree = rootRoute.addChildren([indexRoute, wikiRoute, builderRoute, readerRoute])
export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
