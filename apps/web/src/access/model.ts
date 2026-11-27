export interface AccessSnapshot {
  loopback: boolean
  remote: boolean
  scheme: string
  tls: string
  insecure: boolean
  trustProxy: boolean
  fingerprint: string
  listen: string[]
  reachable: string[]
  hosts: string[]
}

export interface AccessSession {
  id: string
  current: boolean
  created: string
  seen: string
  ip: string
}

function strings(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string')
}

export function parseAccess(body: unknown): AccessSnapshot | null {
  if (!body || typeof body !== 'object') return null
  const record = body as Record<string, unknown>
  if (typeof record.remote !== 'boolean' || typeof record.tls !== 'string' || typeof record.scheme !== 'string') {
    return null
  }
  return {
    loopback: record.loopback === true,
    remote: record.remote,
    scheme: record.scheme,
    tls: record.tls,
    insecure: record.insecure === true,
    trustProxy: record.trustProxy === true,
    fingerprint: typeof record.fingerprint === 'string' ? record.fingerprint : '',
    listen: strings(record.listen),
    reachable: strings(record.reachable),
    hosts: strings(record.hosts),
  }
}

export function parseSessions(body: unknown): AccessSession[] | null {
  if (!body || typeof body !== 'object') return null
  const sessions = (body as Record<string, unknown>).sessions
  if (!Array.isArray(sessions)) return null
  const out: AccessSession[] = []
  for (const item of sessions) {
    if (!item || typeof item !== 'object') return null
    const row = item as Record<string, unknown>
    if (typeof row.id !== 'string' || typeof row.current !== 'boolean') return null
    out.push({
      id: row.id,
      current: row.current,
      created: typeof row.created === 'string' ? row.created : '',
      seen: typeof row.seen === 'string' ? row.seen : '',
      ip: typeof row.ip === 'string' ? row.ip : '',
    })
  }
  return out
}
