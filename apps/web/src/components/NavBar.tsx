import { Link, useRouterState } from '@tanstack/react-router'
import clsx from 'clsx'

const items = [
  { to: '/wiki', label: 'Wiki 浏览器' },
  { to: '/builder', label: '建课工作台' },
  { to: '/reader', label: '课程阅读器' },
] as const

export function NavBar() {
  const { location } = useRouterState()
  return (
    <header className="border-b bg-white">
      <nav className="max-w-6xl mx-auto px-6 h-14 flex items-center gap-6">
        <div className="font-semibold text-lg">MarsAgent</div>
        <div className="flex gap-4 text-sm">
          {items.map((i) => (
            <Link
              key={i.to}
              to={i.to}
              className={clsx(
                'px-2 py-1 rounded hover:bg-slate-100',
                location.pathname.startsWith(i.to) && 'bg-slate-100 font-medium',
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
