import { Outlet } from '@tanstack/react-router'
import { NavBar } from '@/components/NavBar'

export function App() {
  return (
    <div className="min-h-screen flex flex-col">
      <NavBar />
      <main className="flex-1 px-6 py-5">
        <Outlet />
      </main>
    </div>
  )
}
