import { describe, expect, it } from 'vitest'
import { diffLines } from '@/config/diff'
import { jsonErrorLine, parseDocument, sceneDiverged } from '@/config/model'

describe('config document', () => {
  it('drops a private key before the page can show it', () => {
    const doc = parseDocument({
      text: '{"Identity":{"PeerID":"abc","PrivKey":"secret-key"},"Addresses":{"API":"/ip4/127.0.0.1/tcp/5001"}}\n',
      scene: null,
      sceneKeys: ['Routing.Type'],
      backup: false,
    })
    expect(doc?.text).not.toContain('secret-key')
    expect(doc?.text).not.toContain('PrivKey')
    expect(doc?.text).toContain('abc')
  })

  it('finds the broken line and a scene-key change', () => {
    expect(jsonErrorLine('{\n  "A": 1,\n}\n')).toBeGreaterThan(1)
    expect(jsonErrorLine('{"A":1}\n')).toBeNull()
    const before = '{"Routing":{"Type":"autoclient"},"Discovery":{"MDNS":{"Enabled":true}}}'
    const after = '{"Routing":{"Type":"auto"},"Discovery":{"MDNS":{"Enabled":true}}}'
    const other = '{"Routing":{"Type":"autoclient"},"Discovery":{"MDNS":{"Enabled":false}}}'
    expect(sceneDiverged(before, after, 'quiet', ['Routing.Type'])).toBe(true)
    expect(sceneDiverged(before, other, 'quiet', ['Routing.Type'])).toBe(false)
    expect(sceneDiverged(before, after, null, ['Routing.Type'])).toBe(false)
  })

  it('shows only the changed lines plus a little context', () => {
    const before = ['a', 'b', 'c', 'd', 'e'].join('\n')
    const after = ['a', 'b', 'c', 'D', 'e'].join('\n')
    const rows = diffLines(before, after)
    expect(rows).not.toBe('too_large')
    if (rows === 'too_large') return
    expect(rows.some((row) => row.kind === 'add' && row.text === 'D')).toBe(true)
    expect(rows.some((row) => row.kind === 'del' && row.text === 'd')).toBe(true)
    expect(rows.some((row) => row.kind === 'skip')).toBe(true)
  })
})
