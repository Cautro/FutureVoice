import { useEffect, useState } from 'react'
import { api, ApiError } from './api/client'
import Sidebar from './components/Sidebar'
import LoginPage from './pages/Login'
import DashboardPage from './pages/Dashboard'
import RequestsPage from './pages/Requests'
import UsersPage from './pages/Users'
import SettingsPage from './pages/Settings'
import './App.css'

interface User {
  id: number
  first_name?: string
  last_name?: string
  username?: string
  role: string
}

export default function App() {
  const [authState, setAuthState] = useState<'loading' | 'authenticated' | 'anonymous' | 'error'>(
    'loading'
  )
  const [user, setUser] = useState<User | null>(null)
  const [activePage, setActivePage] = useState('overview')
  const [error, setError] = useState('')

  useEffect(() => {
    let alive = true

    api
      .get('/auth/me')
      .then((response: any) => {
        if (!alive) return
        setUser(response.user)
        setActivePage(response.user.role === 'admin' ? 'overview' : 'requests')
        setAuthState('authenticated')
      })
      .catch((err: any) => {
        if (!alive) return
        if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
          setAuthState('anonymous')
        } else {
          setError(err.message || 'Не удалось подключиться к backend')
          setAuthState('error')
        }
      })

    return () => {
      alive = false
    }
  }, [])

  async function logout() {
    try {
      await api.post('/auth/logout')
    } catch {
      // сессия уже могла сдохнуть — похуй
    }
    setUser(null)
    setAuthState('anonymous')
  }

  if (authState === 'loading') {
    return (
      <div className="app-loading">
        <span className="spinner" />
        Проверяем сессию…
      </div>
    )
  }

  if (authState === 'anonymous') return <LoginPage />

  if (authState === 'error') {
    return (
      <main className="error-screen">
        <h1>Не удалось подключиться</h1>
        <p>{error}</p>
        <button className="primary-button" onClick={() => window.location.reload()}>
          Повторить
        </button>
      </main>
    )
  }

  if (!user) return null

  const page =
    activePage === 'overview' && user.role === 'admin' ? (
      <DashboardPage />
    ) : activePage === 'requests' ? (
      <RequestsPage user={user} />
    ) : activePage === 'users' && user.role === 'admin' ? (
      <UsersPage user={user} />
    ) : (
      <SettingsPage user={user} />
    )

  return (
    <div className="app-shell">
      <Sidebar active={activePage} onChange={setActivePage} user={user} onLogout={logout} />
      <main className="main-area">
        <div className="mobile-brand">
          <span className="brand-mark">F</span>
          FutureVoice
        </div>
        {page}
      </main>
    </div>
  )
}