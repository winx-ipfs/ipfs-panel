import { csrfHeaders } from '@/lib/csrf'
import { notifyUnauthorized } from '@/lib/session'

export interface ApiResult {
  status: number
  body: unknown
  error: string
  state: string
}

// apiJSON is the browser's call to the manager. A new page uses this instead
// of calling fetch itself: the session cookie, the CSRF header, and a 401
// all live here. The manager is the only process that talks to Kubo.
export async function apiJSON(path: string, init?: { method?: string; body?: unknown }): Promise<ApiResult> {
  const method = init?.method ?? 'GET'
  const headers: Record<string, string> = {}
  let payload: string | undefined
  if (init?.body !== undefined || (method !== 'GET' && method !== 'HEAD')) {
    headers['Content-Type'] = 'application/json'
    Object.assign(headers, csrfHeaders())
    if (init?.body !== undefined) payload = JSON.stringify(init.body)
  }
  let response: Response
  try {
    response = await fetch(path, {
      method,
      credentials: 'same-origin',
      headers,
      body: payload,
    })
  } catch {
    return { status: 0, body: {}, error: 'network', state: '' }
  }
  if (response.status === 401) notifyUnauthorized()
  let body: unknown
  try {
    body = await response.json()
  } catch {
    body = {}
  }
  let error = ''
  let state = ''
  if (body && typeof body === 'object') {
    const record = body as Record<string, unknown>
    if (typeof record.error === 'string') error = record.error
    if (typeof record.state === 'string') state = record.state
  }
  return { status: response.status, body, error, state }
}
