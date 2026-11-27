import { describe, expect, it } from 'vitest'
import { parsePermanent } from '@/peering/model'

const peer = 'QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN'

describe('parsePermanent', () => {
  it('accepts a saved list and rejects a bad shape', () => {
    expect(parsePermanent({
      sync: 'apply',
      peers: [{ peerId: peer, addrs: ['/ip4/203.0.113.10/tcp/4001'], state: 'pending' }],
    })).toEqual({
      sync: 'apply',
      peers: [{ peerId: peer, addrs: ['/ip4/203.0.113.10/tcp/4001'], state: 'pending' }],
    })
    expect(parsePermanent({ sync: 'ok', peers: [] })).toEqual({ sync: 'ok', peers: [] })
    expect(parsePermanent({ sync: 'later', peers: [] })).toBeNull()
    expect(parsePermanent({ sync: 'ok', peers: [{ peerId: peer, addrs: [1], state: 'connected' }] })).toBeNull()
    expect(parsePermanent(null)).toBeNull()
  })
})