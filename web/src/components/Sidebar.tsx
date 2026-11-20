import { GRAFANA_URL, ROLE } from '../utils/constants'

interface User {
  id: number
  first_name?: string
  last_name?: string
  username?: string
  role: string
}

interface SidebarProps {
  active: string
  onChange: (id: string) => void
  user: User
  onLogout: () => void
}

const navigation = [
  { id: 'overview', title: 'Обзор', icon: '◫', admin: true },
  { id: 'requests', title: 'Заявки', icon: '▤' },
  { id: 'users', title: 'Пользователи', icon: '♙', admin: true },
  { id: 'settings', title: 'Инструменты', icon: '⚙' },
]

export default function Sidebar({ active, onChange, user, onLogout }: SidebarProps) {
  const visible = navigation.filter((item) => !item.admin || user.role === 'admin')

  return (
    <aside className="sidebar">
      <div className="brand">
        <span className="brand-mark">F</span>
        <span>FutureVoice</span>
      </div>

      <div className="workspace-label">РАБОЧЕЕ ПРОСТРАНСТВО</div>

      <nav className="side-nav">
        {visible.map((item) => (
          <button
            key={item.id}
            className={`nav-item ${active === item.id ? 'active' : ''}`}
            onClick={() => onChange(item.id)}
          >
            <span className="nav-icon">{item.icon}</span>
            <span>{item.title}</span>
            {active === item.id && <span className="nav-indicator" />}
          </button>
        ))}
      </nav>

      <div className="sidebar-bottom">
        {user.role === 'admin' && (
          <a
            className="nav-item external-link"
            href={GRAFANA_URL}
            target="_blank"
            rel="noreferrer"
          >
            <span className="nav-icon">⌁</span>
            <span>Grafana</span>
            <span className="external-arrow">↗</span>
          </a>
        )}

        <div className="sidebar-divider" />

        <div className="profile-block">
          <div className="avatar">
            {(user.first_name || user.username || 'F').slice(0, 1).toUpperCase()}
          </div>
          <div className="profile-meta">
            <strong>
              {[user.first_name, user.last_name].filter(Boolean).join(' ') ||
                user.username ||
                'Пользователь'}
            </strong>
            <span>{ROLE[user.role] || user.role}</span>
          </div>
          <button className="icon-button logout-button" title="Выйти" onClick={onLogout}>
            ↪
          </button>
        </div>
      </div>
    </aside>
  )
}