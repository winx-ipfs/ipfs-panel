import type { PeerRow } from '@/peers/model'
import { emptyPermanent, type PermanentList } from '@/peering/model'

export interface PeersPreview {
  blocked: '' | 'down' | 'starting' | 'stopping' | 'failed'
  peers: PeerRow[]
  permanent: PermanentList
  draft: string
  busy: string
}

const qm = 'QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN'
const ed = '12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d'
const notes = 'QmPVGjYFugq4XUyBfoTHG6c3qxfBS26jEdaFM1gdAVuMZ2'
const quiet = '12D3KooWPeo9gaDV6URwwwyWWjEJsCaMeZ7PBE5vpqvR1KFnPv3B'
const dial = `/ip4/203.0.113.10/tcp/4001/p2p/${qm}`

const sample: PeerRow[] = [
  { peerId: ed, direction: 'out', latencyMs: 18, agent: 'kubo/0.43.0' },
  { peerId: qm, direction: 'in', latencyMs: 0, agent: 'go-libp2p/0.50.0' },
  { peerId: notes, direction: 'both', latencyMs: 240, agent: 'kubo/0.39.0' },
  { peerId: quiet, direction: 'out', latencyMs: 1400, agent: '' },
]

function shot(partial: Partial<PeersPreview>): PeersPreview {
  return {
    blocked: '',
    peers: [],
    permanent: emptyPermanent(),
    draft: '',
    busy: '',
    ...partial,
  }
}

export function peersPreview(search: string): PeersPreview | null {
  const name = new URLSearchParams(search).get('peers')
  if (name === 'down' || name === 'starting' || name === 'stopping' || name === 'failed') {
    return shot({ blocked: name })
  }
  if (name === 'empty') return shot({})
  if (name === 'list') return shot({ peers: sample })
  if (name === 'kept') {
    return shot({
      peers: sample,
      permanent: {
        sync: 'ok',
        peers: [
          { peerId: ed, addrs: ['/ip4/203.0.113.10/tcp/4001'], state: 'connected' },
          { peerId: quiet, addrs: [], state: 'reconnecting' },
        ],
      },
    })
  }
  if (name === 'apply') {
    return shot({
      peers: sample,
      permanent: {
        sync: 'apply',
        peers: [{ peerId: ed, addrs: ['/ip4/203.0.113.10/tcp/4001'], state: 'pending' }],
      },
    })
  }
  if (name === 'restart') {
    return shot({
      permanent: {
        sync: 'restart',
        peers: [{ peerId: ed, addrs: ['/ip4/203.0.113.10/tcp/4001'], state: 'pending' }],
      },
    })
  }
  if (name === 'adding') return shot({ draft: dial, busy: 'keep' })
  if (name === 'error') return shot({ draft: 'http://127.0.0.1:5001' })
  return null
}
