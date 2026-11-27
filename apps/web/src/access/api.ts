import { apiJSON } from '@/lib/api'
import { parseAccess, parseSessions, type AccessSession, type AccessSnapshot } from '@/access/model'

export interface AccessResult {
  snap: AccessSnapshot | null
  error: string
  status: number
}

export interface SessionResult {
  sessions: AccessSession[] | null
  error: string
  status: number
}

export async function fetchAccess(): Promise<AccessResult> {
  const result = await apiJSON('/api/access')
  return {
    snap: result.status === 200 ? parseAccess(result.body) : null,
    error: result.error,
    status: result.status,
  }
}

export async function fetchSessions(): Promise<SessionResult> {
  const result = await apiJSON('/api/access/sessions')
  return {
    sessions: result.status === 200 ? parseSessions(result.body) : null,
    error: result.error,
    status: result.status,
  }
}

export async function revokeSession(id: string): Promise<{ status: number; error: string }> {
  const result = await apiJSON('/api/access/sessions/revoke', { method: 'POST', body: { id } })
  return { status: result.status, error: result.error }
}

export async function revokeOtherSessions(): Promise<{ status: number; error: string }> {
  const result = await apiJSON('/api/access/sessions/revoke-others', { method: 'POST', body: {} })
  return { status: result.status, error: result.error }
}
