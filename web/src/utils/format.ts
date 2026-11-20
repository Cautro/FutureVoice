import { STATUS, StatusKey } from './constants'

export function formatDate(value?: string | number | Date | null, withTime = true): string {
  if (!value) return '—'

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'

  return new Intl.DateTimeFormat(
    'ru-RU',
    withTime
      ? { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' }
      : { day: '2-digit', month: 'short', year: 'numeric' }
  ).format(date)
}

interface NameSource {
  author_first_name?: string
  author_last_name?: string
  author_username?: string
  first_name?: string
  last_name?: string
  username?: string
  user_id?: number | string
  id?: number | string
}

export function fullName(user?: NameSource | null): string {
  if (!user) return 'Пользователь'

  const name = [user.author_first_name || user.first_name, user.author_last_name || user.last_name]
    .filter(Boolean)
    .join(' ')

  const username = user.author_username || user.username

  if (name) return name
  if (username) return `@${username}`

  return `Пользователь #${user.user_id || user.id || '?'}`
}

export function statusLabel(value?: string | null): string {
  if (!value) return 'Неизвестно'
  return STATUS[value as StatusKey]?.label || value
}