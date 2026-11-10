import { describe, expect, it } from 'vitest'
import { explorePreview } from '@/dev/mockExplore'
import { crumbs, exploreURL, nextTarget, parseView } from '@/explore/model'

const cid = 'bafkqac3imvwgy3zao5xxe3de'

describe('explore model', () => {
  it('parses a block and keeps an empty link list', () => {
    const view = parseView({
      target: `/ipfs/${cid}/notes.txt`,
      cid,
      version: 1,
      codec: 'dag-pb',
      hash: 'sha2-256',
      kind: 'file',
      size: 5,
      text: 'hello',
      links: [],
      localGateway: 'http://127.0.0.1:8081',
    })
    expect(view?.kind).toBe('file')
    expect(view?.links).toEqual([])
    expect(view?.text).toBe('hello')
    expect(parseView({ kind: 'nope' })).toBeNull()
  })

  it('builds a path and the next link', () => {
    expect(crumbs(`/ipfs/${cid}/photos/cat.jpg`).map((crumb) => crumb.label)).toEqual([cid, 'photos', 'cat.jpg'])
    expect(crumbs(cid)[0]?.target).toBe(`/ipfs/${cid}`)
    expect(nextTarget(`/ipfs/${cid}`, { name: 'photos', cid: 'bafyabc' })).toBe(`/ipfs/${cid}/photos`)
    expect(nextTarget(`/ipfs/${cid}`, { name: '', cid: 'bafyabc' })).toBe('bafyabc')
    expect(nextTarget(`/ipfs/${cid}`, { name: 'a/b', cid: 'bafyabc' })).toBe('bafyabc')
  })

  it('draws the screenshot fixtures', () => {
    expect(explorePreview('?explore=start')?.view).toBeNull()
    expect(explorePreview('?explore=result')?.view?.kind).toBe('folder')
    expect(explorePreview('?explore=result')?.view?.links[1]?.name).toBe('会议记录.txt')
    expect(explorePreview('')).toBeNull()
  })

  it('copies a loopback link and refuses anything else', () => {
    expect(exploreURL('http://127.0.0.1:8081', `/ipfs/${cid}/notes.txt`, cid)).toBe(
      `http://127.0.0.1:8081/ipfs/${cid}/notes.txt`,
    )
    expect(exploreURL('http://192.0.2.1:8081', `/ipfs/${cid}`, cid)).toBeNull()
    expect(exploreURL('http://127.0.0.1:5001', cid, cid)).not.toContain('5001/api')
  })
})
