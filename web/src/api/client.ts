const API_ROOT = '/api/v1'

export class ApiError extends Error {
  status: number
  payload: unknown

  constructor(message: string, status: number, payload?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
  }
}

function cookieValue(name: string): string {
  const prefix = `${name}=`
  const entry = document.cookie
    .split('; ')
    .find((value) => value.startsWith(prefix))

  return entry ? decodeURIComponent(entry.slice(prefix.length)) : ''
}

interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown
}

async function request<T = unknown>(path: string, options: RequestOptions = {}): Promise<T> {
  const method = (options.method || 'GET').toUpperCase()
  const headers = new Headers(options.headers || {})

  headers.set('Accept', 'application/json')

  if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json')
  }

  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    const csrf = cookieValue('fv_csrf')
    if (csrf) {
      headers.set('X-CSRF-Token', csrf)
    }
  }

  const response = await fetch(`${API_ROOT}${path}`, {
    ...options,
    method,
    headers,
    credentials: 'include',
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  })

  if (response.status === 204) {
    return null as T
  }

  const contentType = response.headers.get('content-type') || ''
  let payload: unknown = null

  if (contentType.includes('application/json')) {
    payload = await response.json().catch(() => null)
  } else {
    payload = await response.text().catch(() => '')
  }

  if (!response.ok) {
    let message = `HTTP ${response.status}`

    if (payload && typeof payload === 'object') {
      const obj = payload as Record<string, unknown>
      message = (obj.error as string) || (obj.message as string) || message
    } else if (typeof payload === 'string' && payload) {
      message = payload
    }

    throw new ApiError(message, response.status, payload)
  }

  return payload as T
}

export const api = {
  get: <T = unknown>(path: string, options?: RequestOptions) =>
    request<T>(path, options),

  post: <T = unknown>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body }),

  patch: <T = unknown>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PATCH', body }),

  delete: <T = unknown>(path: string) =>
    request<T>(path, { method: 'DELETE' }),
}