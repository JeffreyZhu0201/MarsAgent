import { Link, useRouterState } from '@tanstack/react-router'
import clsx from 'clsx'

const items = [
  { to: '/wiki', label: 'Wiki 浏览器' },
  { to: '/problems', label: 'OJ 题库' },
  { to: '/submissions', label: '提交记录' },
  { to: '/builder', label: '建课工作台' },
  { to: '/reader', label: '课程阅读器' },
] as const

export function NavBar() {
  const { location } = useRouterState()
  return (
    <header className="sticky top-0 z-50 border-b border-white/50 bg-white/45 backdrop-blur-2xl">
      <nav className="max-w-7xl mx-auto px-6 h-16 flex items-center gap-6">
        <div className="font-semibold text-lg tracking-tight">MarsAgent</div>
        <div className="flex gap-4 text-sm">
          {items.map((i) => (
            <Link
              key={i.to}
              to={i.to}
              className={clsx(
                'rounded-full px-3 py-1.5 transition hover:bg-white/70',
                location.pathname.startsWith(i.to) && 'bg-white/80 font-medium shadow-sm',
              )}
            >
              {i.label}
            </Link>
          ))}
        </div>
      </nav>
    </header>
  )
}
