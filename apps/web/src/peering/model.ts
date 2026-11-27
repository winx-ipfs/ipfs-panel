export type PeeringSync = 'ok' | 'apply' | 'restart'
export type PeeringState = 'connected' | 'reconnecting' | 'pending'

export interface PermanentPeer {
  peerId: string
  addrs: string[]
  state: PeeringState
}

export interface PermanentList {
  sync: PeeringSync
  peers: PermanentPeer[]
}

const states = new Set<PeeringState>(['connected', 'reconnecting', 'pending'])
const syncs = new Set<PeeringSync>(['ok', 'apply', 'restart'])

export function parsePermanent(value: unknown): PermanentList | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (typeof body.sync !== 'string' || !syncs.has(body.sync as PeeringSync)) return null
  if (!Array.isArray(body.peers)) return null
  const peers: PermanentPeer[] = []
  for (const item of body.peers) {
    if (!item || typeof item !== 'object') return null
    const row = item as Record<string, unknown>
    if (typeof row.peerId !== 'string' || !row.peerId) return null
    if (typeof row.state !== 'string' || !states.has(row.state as PeeringState)) return null
    if (!Array.isArray(row.addrs)) return null
    const addrs: string[] = []
    for (const addr of row.addrs) {
      if (typeof addr !== 'string') return null
      addrs.push(addr)
    }
    peers.push({ peerId: row.peerId, addrs, state: row.state as PeeringState })
  }
  return { sync: body.sync as PeeringSync, peers }
}

export function emptyPermanent(): PermanentList {
  return { sync: 'ok', peers: [] }
}
