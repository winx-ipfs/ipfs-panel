import type { PeerRow } from '@/peers/model'

export interface PeersPreview {
  blocked: '' | 'down' | 'starting' | 'stopping' | 'failed'
  peers: PeerRow[]
}

const qm = 'QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN'
const ed = '12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d'
const notes = 'QmPVGjYFugq4XUyBfoTHG6c3qxfBS26jEdaFM1gdAVuMZ2'
const quiet = '12D3KooWPeo9gaDV6URwwwyWWjEJsCaMeZ7PBE5vpqvR1KFnPv3B'

const sample: PeerRow[] = [
  { peerId: ed, direction: 'out', latencyMs: 18, agent: 'kubo/0.43.0' },
  { peerId: qm, direction: 'in', latencyMs: 0, agent: 'go-libp2p/0.50.0' },
  { peerId: notes, direction: 'both', latencyMs: 240, agent: 'kubo/0.39.0' },
  { peerId: quiet, direction: 'out', latencyMs: 1400, agent: '' },
]

export function peersPreview(search: string): PeersPreview | null {
  const shot = new URLSearchParams(search).get('peers')
  if (shot === 'down' || shot === 'starting' || shot === 'stopping' || shot === 'failed') {
    return { blocked: shot, peers: [] }
  }
  if (shot === 'empty') return { blocked: '', peers: [] }
  if (shot === 'list') return { blocked: '', peers: sample }
  return null
}
