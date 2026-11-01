export type AuthPhase = 'setup' | 'anonymous' | 'authenticated'

export interface GuardTarget {
  name: string
  fullPath: string
  redirect: unknown
}

export type GuardDecision =
  | { type: 'allow' }
  | { type: 'route'; name: 'login' | 'setup'; query?: Record<string, string> }
  | { type: 'path'; path: string }

/** A same-app path. Rejects protocol-relative and off-site values. */
export function safeNext(value: unknown): string | null {
  if (typeof value !== 'string' || value.length === 0 || value.length > 2048) return null
  if (!value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return null
  if (value.includes('://') || value.includes('\\')) return null
  if (value === '/login' || value.startsWith('/login?') || value.startsWith('/login#')) return null
  if (value === '/setup' || value.startsWith('/setup?') || value.startsWith('/setup#')) return null
  return value
}

export function decide(phase: AuthPhase, to: GuardTarget): GuardDecision {
  if (to.name === 'setup') {
    if (phase === 'setup') return { type: 'allow' }
    if (phase === 'authenticated') return { type: 'path', path: safeNext(to.redirect) ?? '/' }
    return { type: 'route', name: 'login' }
  }
  if (to.name === 'login') {
    if (phase === 'anonymous') return { type: 'allow' }
    if (phase === 'setup') return { type: 'route', name: 'setup' }
    return { type: 'path', path: safeNext(to.redirect) ?? '/' }
  }
  if (phase === 'setup') return { type: 'route', name: 'setup' }
  if (phase !== 'authenticated') {
    const query: Record<string, string> = {}
    if (to.fullPath && to.fullPath !== '/') query.redirect = to.fullPath
    return { type: 'route', name: 'login', query }
  }
  return { type: 'allow' }
}
