import { Outlet, NavLink } from 'react-router-dom'
import { useState } from 'react'
import AIChatPanel from './AIChatPanel'

const navItems = [
  { path: '/learn', label: '知识图谱', icon: '📚' },
  { path: '/practice', label: '练习题库', icon: '💻' },
  { path: '/dashboard', label: '学习仪表盘', icon: '📊' },
  { path: '/settings', label: '设置', icon: '⚙️' },
]

export default function Layout() {
  const [aiPanelOpen, setAIPanelOpen] = useState(false)

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200 sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-14">
            <div className="flex items-center gap-6">
              <h1 className="text-lg font-semibold text-gray-900">Learn Helper</h1>
              <nav className="flex gap-1">
                {navItems.map((item) => (
                  <NavLink
                    key={item.path}
                    to={item.path}
                    className={({ isActive }) =>
                      `px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                        isActive
                          ? 'bg-blue-50 text-blue-700'
                          : 'text-gray-600 hover:text-gray-900 hover:bg-gray-100'
                      }`
                    }
                  >
                    <span className="mr-1">{item.icon}</span>
                    {item.label}
                  </NavLink>
                ))}
              </nav>
            </div>
            <button
              onClick={() => setAIPanelOpen(!aiPanelOpen)}
              className="px-3 py-1.5 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 transition-colors"
            >
              {aiPanelOpen ? '关闭 AI' : '打开 AI'}
            </button>
          </div>
        </div>
      </header>

      <div className="flex">
        <main className={`flex-1 transition-all ${aiPanelOpen ? 'mr-96' : ''}`}>
          <Outlet />
        </main>
        {aiPanelOpen && (
          <AIChatPanel onClose={() => setAIPanelOpen(false)} />
        )}
      </div>
    </div>
  )
}