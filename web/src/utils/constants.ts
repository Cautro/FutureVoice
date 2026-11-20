export const STATUS = {
  open: { label: 'Новая', tone: 'amber' },
  in_progress: { label: 'В работе', tone: 'blue' },
  waiting_for_user: { label: 'Ожидает ответа', tone: 'amber' },
  closed: { label: 'Завершена', tone: 'green' },
  rejected: { label: 'Отклонена', tone: 'red' },
} as const

export type StatusKey = keyof typeof STATUS

export const TYPE = {
  complaint: 'Жалоба',
  offer: 'Предложение',
  '3dPrintOffer': '3D-печать',
} as const

export type TypeKey = keyof typeof TYPE

export const ROLE = {
  student: 'Ученик',
  moderator: 'Модератор',
  admin: 'Администратор',
} as const

export type RoleKey = keyof typeof ROLE

export const GRAFANA_URL =
  import.meta.env.VITE_GRAFANA_URL || 'http://localhost:3000'