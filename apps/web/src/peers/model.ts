export type PeerDirection = '' | 'in' | 'out' | 'both'

export interface PeerRow {
  peerId: string
  direction: PeerDirection
  latencyMs: number | null
  agent: string
}

export interface PeerList {
  total: number
  truncated: boolean
  peers: PeerRow[]
}

const directions = new Set<PeerDirection>(['', 'in', 'out', 'both'])

export function parseList(value: unknown): PeerList | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (typeof body.total !== 'number' || !Number.isFinite(body.total) || body.total < 0) return null
  if (!Array.isArray(body.peers)) return null
  const peers: PeerRow[] = []
  for (const item of body.peers) {
    if (!item || typeof item !== 'object') return null
    const row = item as Record<string, unknown>
    if (typeof row.peerId !== 'string' || !row.peerId) return null
    const direction = typeof row.direction === 'string' ? row.direction : ''
    if (!directions.has(direction as PeerDirection)) return null
    let latencyMs: number | null = null
    if (row.latencyMs !== undefined) {
      if (typeof row.latencyMs !== 'number' || !Number.isFinite(row.latencyMs) || row.latencyMs < 0) return null
      latencyMs = row.latencyMs
    }
    const agent = typeof row.agent === 'string' ? row.agent : ''
    peers.push({ peerId: row.peerId, direction: direction as PeerDirection, latencyMs, agent })
  }
  return {
    total: body.total,
    truncated: body.truncated === true,
    peers,
  }
}

export function filterPeers(peers: PeerRow[], query: string): PeerRow[] {
  const text = query.trim()
  if (!text) return peers
  let idQuery = text
  if (idQuery.startsWith('/p2p/')) idQuery = idQuery.slice('/p2p/'.length)
  else if (idQuery.startsWith('/ipfs/')) idQuery = idQuery.slice('/ipfs/'.length)
  const agentQuery = text.toLowerCase()
  return peers.filter((peer) => peer.peerId.includes(idQuery) || peer.agent.toLowerCase().includes(agentQuery))
}
