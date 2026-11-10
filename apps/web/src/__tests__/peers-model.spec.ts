import { describe, expect, it } from 'vitest'
import { parseAddress } from '@/peers/address'
import { filterPeers, parseList, type PeerRow } from '@/peers/model'

const qm = 'QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN'
const ed = '12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d'

describe('parseAddress', () => {
  it('accepts the addresses the page is allowed to dial', () => {
    expect(parseAddress(`  /ip4/203.0.113.10/tcp/4001/p2p/${qm} \n`)).toEqual({
      address: `/ip4/203.0.113.10/tcp/4001/p2p/${qm}`,
      peerId: qm,
    })
    expect(parseAddress(`/ip4/203.0.113.10/udp/4001/quic-v1/p2p/${ed}`)?.peerId).toBe(ed)
    expect(parseAddress(`/dnsaddr/bootstrap.libp2p.io/p2p/${qm}`)?.address).toBe(`/dnsaddr/bootstrap.libp2p.io/p2p/${qm}`)
    expect(parseAddress(`/ip4/203.0.113.10/tcp/4001/p2p/${qm}/p2p-circuit/p2p/${ed}`)?.peerId).toBe(ed)
    expect(parseAddress(`/ip6/2001:db8::1/tcp/4001/ws/p2p/${qm}`)?.peerId).toBe(qm)
    expect(parseAddress('/ip4/203.0.113.10/tcp/443/wss/p2p/F0172000101')).toEqual({
      address: '/ip4/203.0.113.10/tcp/443/wss/p2p/f0172000101',
      peerId: 'f0172000101',
    })
  })

  it('rejects a URL, a bare id, and a missing peer id', () => {
    expect(parseAddress('')).toBeNull()
    expect(parseAddress('http://127.0.0.1:5001')).toBeNull()
    expect(parseAddress(qm)).toBeNull()
    expect(parseAddress('/ip4/203.0.113.10/tcp/4001')).toBeNull()
    expect(parseAddress(`/ip4/203.0.113.10/tcp/0/p2p/${qm}`)).toBeNull()
    expect(parseAddress('/dnsaddr/bootstrap.libp2p.io')).toBeNull()
    expect(parseAddress(`/ip4/203.0.113.10/tcp/4001/p2p/${qm}?x=1`)).toBeNull()
  })
})

describe('peers list helpers', () => {
  const rows: PeerRow[] = [
    { peerId: qm, direction: 'in', latencyMs: 12, agent: 'kubo/0.43.0' },
    { peerId: ed, direction: 'out', latencyMs: null, agent: 'go-libp2p' },
  ]

  it('parses a list and drops nothing the page did not ask for', () => {
    const parsed = parseList({
      total: 2,
      peers: [
        { peerId: qm, direction: 'in', latencyMs: 0, agent: 'kubo/0.43.0' },
        { peerId: ed, direction: 'out' },
      ],
    })
    expect(parsed?.peers[0]?.latencyMs).toBe(0)
    expect(parsed?.peers[1]?.agent).toBe('')
    expect(parseList({ total: 1, peers: [{ peerId: '', direction: 'in' }] })).toBeNull()
  })

  it('filters by peer id, including a /p2p/ prefix, and by software', () => {
    expect(filterPeers(rows, '')).toHaveLength(2)
    expect(filterPeers(rows, `/p2p/${qm.slice(0, 6)}`).map((peer) => peer.peerId)).toEqual([qm])
    expect(filterPeers(rows, 'LIBP2P').map((peer) => peer.peerId)).toEqual([ed])
    expect(filterPeers(rows, 'nobody')).toEqual([])
  })
})
