import { describe, expect, it } from 'vitest'
import { filterLines, parsePage, type LogLine } from '@/logs/model'

const lines: LogLine[] = [
  { id: 1, time: '2026-11-15T11:14:01.000Z', source: 'manager', level: 'info', text: 'manager listening' },
  { id: 2, time: '2026-11-15T11:14:02.000Z', source: 'kubo', level: 'debug', text: 'dht trace' },
  { id: 3, time: '2026-11-15T11:14:03.000Z', source: 'kubo', level: 'error', text: 'gateway failed' },
]

describe('log lines', () => {
  it('filters by source, level, and text', () => {
    expect(filterLines(lines, { source: 'kubo', level: 'info', q: '' }).map((line) => line.id)).toEqual([3])
    expect(filterLines(lines, { source: 'all', level: 'debug', q: 'gateway' }).map((line) => line.id)).toEqual([3])
    expect(filterLines(lines, { source: 'all', level: 'info', q: 'missing' })).toEqual([])
  })

  it('parses a page and drops a bad line', () => {
    const page = parsePage({
      lines: [...lines, { id: 4, source: 'nope', level: 'info', text: 'x', time: 't' }],
      next: 3,
      node: 'down',
      capture: false,
    })
    expect(page?.lines).toHaveLength(3)
    expect(page?.node).toBe('down')
    expect(page?.capture).toBe(false)
    expect(parsePage({ lines: null })).toBeNull()
  })
})
