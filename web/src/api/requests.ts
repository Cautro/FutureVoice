import { api } from './client'

export interface ListFilters {
  status?: string
  type?: string
  q?: string
  page?: number
  limit?: number
}

export interface UsersFilters {
  role?: string
  q?: string
  page?: number
  limit?: number
}

export const requestsApi = {
  list: (filters: ListFilters = {}) => {
    const params = new URLSearchParams()

    for (const [key, value] of Object.entries(filters)) {
      if (value !== undefined && value !== null && value !== '') {
        params.set(key, String(value))
      }
    }

    const query = params.toString()
    return api.get(`/admin/requests${query ? `?${query}` : ''}`)
  },

  get: (id: number | string) => {
    return api.get(`/admin/requests/${id}`)
  },

  updateStatus: (id: number | string, status: string, reason = '') => {
    return api.patch(`/admin/requests/${id}/status`, { status, reason })
  },

  assign: (id: number | string, assignedTo: number) => {
    return api.patch(`/admin/requests/${id}/assignee`, {
      assigned_to: assignedTo,
    })
  },

  addComment: (id: number | string, body: string) => {
    return api.post(`/admin/requests/${id}/comments`, { body })
  },

  addMessage: (id: number | string, body: string, visibility: 'student' | 'internal') => {
    return api.post(`/admin/requests/${id}/messages`, { body, visibility })
  },

  remove: (id: number | string) => {
    return api.delete(`/admin/requests/${id}`)
  },

  users: (filters: UsersFilters = {}) => {
    const params = new URLSearchParams()

    for (const [key, value] of Object.entries(filters)) {
      if (value !== undefined && value !== null && value !== '') {
        params.set(key, String(value))
      }
    }

    const query = params.toString()
    return api.get(`/admin/users${query ? `?${query}` : ''}`)
  },

  updateRole: (id: number | string, role: string) => {
    return api.patch(`/admin/users/${id}/role`, { role })
  },

  updateActive: (id: number | string, is_active: boolean) => {
    return api.patch(`/admin/users/${id}/active`, { is_active })
  },

  analytics: (from: string, to: string) => {
    const params = new URLSearchParams({ from, to })
    return api.get(`/admin/analytics/overview?${params.toString()}`)
  },
}