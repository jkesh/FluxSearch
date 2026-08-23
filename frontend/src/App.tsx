import { Link, Route, Routes, useLocation } from 'react-router-dom'
import ChatPage from './pages/ChatPage'
import DocumentsPage from './pages/DocumentsPage'
import ImportPage from './pages/ImportPage'
import SearchPage from './pages/SearchPage'
import SettingsPage from './pages/SettingsPage'
import { useTheme } from './hooks/useTheme'

function ChatIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-[18px] w-[18px] shrink-0">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
    </svg>
  )
}

function SearchIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-[18px] w-[18px] shrink-0">
      <circle cx="11" cy="11" r="7" />
      <path d="m21 21-4.35-4.35" />
    </svg>
  )
}

function ImportIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-[18px] w-[18px] shrink-0">
      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
      <polyline points="17 8 12 3 7 8" />
      <line x1="12" y1="3" x2="12" y2="15" />
    </svg>
  )
}

function LibraryIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-[18px] w-[18px] shrink-0">
      <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
      <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
    </svg>
  )
}

function SettingsIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-[18px] w-[18px] shrink-0">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  )
}

function SunIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-[18px] w-[18px] shrink-0">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
    </svg>
  )
}

function MoonIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-[18px] w-[18px] shrink-0">
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79Z" />
    </svg>
  )
}

const NAV_SECTIONS = [
  {
    label: '体验',
    items: [
      { path: '/', label: '文本对话', Icon: ChatIcon, match: (p: string) => p === '/' || p.startsWith('/c/') },
      { path: '/search', label: '向量检索', Icon: SearchIcon, match: (p: string) => p === '/search' },
    ],
  },
  {
    label: '知识库',
    items: [
      { path: '/import', label: '文档导入', Icon: ImportIcon, match: (p: string) => p === '/import' },
      { path: '/documents', label: '文档管理', Icon: LibraryIcon, match: (p: string) => p === '/documents' || p.startsWith('/documents/') },
    ],
  },
  {
    label: '系统',
    items: [
      { path: '/settings', label: '模型设置', Icon: SettingsIcon, match: (p: string) => p === '/settings' },
    ],
  },
] as const

export default function App() {
  const location = useLocation()
  const { mode, toggle } = useTheme()

  return (
    <div className="flex min-h-screen bg-base-200 text-base-content">
      <aside className="sticky top-0 flex h-screen w-[220px] shrink-0 flex-col border-r border-base-300/80 bg-base-100">
        <Link to="/" className="flex h-14 shrink-0 items-center gap-2.5 px-5">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary font-mono text-sm font-bold text-primary-content">
            F
          </span>
          <span className="min-w-0">
            <span className="block text-[15px] font-semibold leading-tight tracking-tight">FluxSearch</span>
            <span className="block text-[11px] leading-tight text-base-content/40">知识库 RAG</span>
          </span>
        </Link>

        <nav className="flex-1 overflow-y-auto px-3 py-2">
          {NAV_SECTIONS.map((section) => (
            <div key={section.label} className="mb-4">
              <div className="mb-1.5 px-2 text-[11px] font-medium uppercase tracking-wider text-base-content/35">
                {section.label}
              </div>
              <div className="space-y-0.5">
                {section.items.map(({ path, label, Icon, match }) => {
                  const active = match(location.pathname)
                  return (
                    <Link
                      key={path}
                      to={path}
                      className={`flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] transition-colors ${
                        active ? 'nav-item-active' : 'text-base-content/60 hover:bg-base-200/80 hover:text-base-content'
                      }`}
                    >
                      <Icon />
                      {label}
                    </Link>
                  )
                })}
              </div>
            </div>
          ))}
        </nav>

        <div className="border-t border-base-300/60 p-3">
          <button
            type="button"
            onClick={toggle}
            className="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] text-base-content/50 transition-colors hover:bg-base-200 hover:text-base-content"
          >
            {mode === 'dark' ? <SunIcon /> : <MoonIcon />}
            {mode === 'dark' ? '浅色模式' : '深色模式'}
          </button>
        </div>
      </aside>

      <main className="min-w-0 flex-1">
        <Routes>
          <Route path="/" element={<ChatPage />} />
          <Route path="/c/:id" element={<ChatPage />} />
          <Route path="/search" element={<SearchPage />} />
          <Route path="/import" element={<ImportPage />} />
          <Route path="/documents" element={<DocumentsPage />} />
          <Route path="/documents/:id" element={<DocumentsPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </main>
    </div>
  )
}
