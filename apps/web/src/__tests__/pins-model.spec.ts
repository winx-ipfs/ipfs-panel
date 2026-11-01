import { describe, expect, it } from 'vitest'
import { parseTarget, validPinName } from '@/pins/cid'
import { filterPins, parseList, sortPins, type Pin } from '@/pins/model'

const v0 = 'QmPVGjYFugq4XUyBfoTHG6c3qxfBS26jEdaFM1gdAVuMZ2'
const v1 = 'bafkreibceirceirceirceirceirceirceirceirceirceirceirceircei'

describe('pin targets', () => {
  it('accepts the same CID shapes the manager accepts', () => {
    expect(parseTarget(`  ${v1}\n`)).toBe(v1)
    expect(parseTarget(`B${v1.slice(1).toUpperCase()}`)).toBe(v1)
    expect(parseTarget(v0)).toBe(v0)
    expect(parseTarget(`/ipfs/${v0}/docs/a.txt`)).toBe(`/ipfs/${v0}/docs/a.txt`)
  })

  it('rejects anything that is not a CID or /ipfs/ path', () => {
    for (const bad of ['', 'hello', 'Qm', '/ipns/' + v0, 'http://127.0.0.1/ipfs/' + v0, `/ipfs/${v0}/../x`]) {
      expect(parseTarget(bad)).toBeNull()
    }
  })

  it('checks names the way the manager does', () => {
    expect(validPinName('')).toBe(true)
    expect(validPinName('会议记录')).toBe(true)
    expect(validPinName('my notes')).toBe(true)
    expect(validPinName(' notes')).toBe(false)
    expect(validPinName('a/b')).toBe(false)
    expect(validPinName('line\nbreak')).toBe(false)
  })
})

describe('pin list helpers', () => {
  const pins: Pin[] = [
    { cid: v1, name: 'notes', type: 'recursive', size: 10 },
    { cid: v0, name: '', type: 'direct', size: null },
    { cid: 'bafkreiother', name: 'Photos', type: 'recursive', size: 3 },
  ]

  it('filters by text and type, and sorts unnamed last', () => {
    expect(filterPins(pins, 'photo', 'all').map((pin) => pin.name)).toEqual(['Photos'])
    expect(filterPins(pins, v0.slice(0, 6), 'direct')).toHaveLength(1)
    expect(sortPins(pins, 'en').map((pin) => pin.name)).toEqual(['notes', 'Photos', ''])
  })

  it('parses a list and omits an unknown size', () => {
    const list = parseList({
      namesFrom: 'panel',
      maxBytes: 1024,
      pins: [{ cid: v1, type: 'recursive', name: 'notes' }, { cid: v0, type: 'direct', size: 4 }],
    })
    expect(list?.namesFrom).toBe('panel')
    expect(list?.maxBytes).toBe(1024)
    expect(list?.pins[0]?.size).toBeNull()
    expect(list?.pins[1]?.size).toBe(4)
  })
})
