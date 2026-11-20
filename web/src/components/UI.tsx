import { ReactNode } from 'react'

export function ErrorBanner({ error, onClose }: { error?: string | null; onClose?: () => void }) {
  if (!error) return null

  return (
    <div className="error-banner" role="alert">
      <span>{error}</span>
      {onClose && (
        <button className="icon-button" onClick={onClose} aria-label="Закрыть">
          ×
        </button>
      )}
    </div>
  )
}

export function Loading({ label = 'Загружаем данные…' }: { label?: string }) {
  return (
    <div className="loading">
      <span className="spinner" />
      {label}
    </div>
  )
}

export function Empty({ title, description }: { title: string; description: string }) {
  return (
    <div className="empty-state">
      <div className="empty-icon">◇</div>
      <h3>{title}</h3>
      <p>{description}</p>
    </div>
  )
}

export function Badge({
  children,
  tone = 'neutral',
}: {
  children: ReactNode
  tone?: 'neutral' | 'amber' | 'blue' | 'green' | 'red' | string
}) {
  return <span className={`badge badge-${tone}`}>{children}</span>
}

export function Topbar({
  title,
  description,
  right,
}: {
  title: string
  description?: string
  right?: ReactNode
}) {
  return (
    <header className="topbar">
      <div>
        <div className="breadcrumbs">
          FUTUREVOICE <span>/</span> ADMIN
        </div>
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      <div className="topbar-right">{right}</div>
    </header>
  )
}

export function MetricCard({
  label,
  value,
  note,
  icon,
  tone = 'neutral',
}: {
  label: string
  value?: string | number | null
  note?: string
  icon?: string
  tone?: string
}) {
  return (
    <div className="metric-card">
      <div className="metric-card-top">
        <span>{label}</span>
        <span className={`metric-icon metric-${tone}`}>{icon}</span>
      </div>
      <div className="metric-value">{value ?? '—'}</div>
      {note && <div className="metric-note">{note}</div>}
    </div>
  )
}