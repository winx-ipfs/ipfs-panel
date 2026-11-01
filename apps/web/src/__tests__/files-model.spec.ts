import { describe, expect, it } from 'vitest'
import {
  cleanPath,
  copyName,
  crumbs,
  formatBytes,
  joinPath,
  localShareURL,
  parseList,
  sortEntries,
  validName,
  type FileEntry,
} from '@/files/model'

function entry(patch: Partial<FileEntry> & Pick<FileEntry, 'name'>): FileEntry {
  return {
    type: 'file',
    size: 0,
    cid: '',
    modified: null,
    ...patch,
  }
}

describe('file paths', () => {
  it('keeps absolute paths and rejects traversal', () => {
    expect(cleanPath('/')).toBe('/')
    expect(cleanPath('/photos/a.txt')).toBe('/photos/a.txt')
    expect(cleanPath('/笔记.txt')).toBe('/笔记.txt')
    expect(cleanPath('/../etc')).toBeNull()
    expect(cleanPath('notes.txt')).toBeNull()
    expect(cleanPath('/a//b')).toBeNull()
    expect(validName('my notes.txt')).toBe(true)
    expect(validName('..')).toBe(false)
    expect(joinPath('/photos', 'a.txt')).toBe('/photos/a.txt')
    expect(joinPath('/', '..')).toBeNull()
    expect(crumbs('/photos/a')).toEqual([
      { name: '', path: '/' },
      { name: 'photos', path: '/photos' },
      { name: 'a', path: '/photos/a' },
    ])
    expect(copyName('notes.txt')).toBe('notes copy.txt')
    expect(copyName('照片')).toBe('照片 copy')
  })
})

describe('file list helpers', () => {
  it('sorts folders first by name, and puts missing times last', () => {
    const entries = [
      entry({ name: 'b.txt', size: 2, modified: '2026-01-02T00:00:00Z' }),
      entry({ name: 'photos', type: 'directory', size: 9 }),
      entry({ name: 'a.txt', size: 5, modified: null }),
    ]
    expect(sortEntries(entries, 'name', 'asc').map((item) => item.name)).toEqual(['photos', 'a.txt', 'b.txt'])
    expect(sortEntries(entries, 'size', 'desc').map((item) => item.name)).toEqual(['photos', 'a.txt', 'b.txt'])
    const byTime = sortEntries(entries, 'modified', 'desc').map((item) => item.name)
    expect(byTime[0]).toBe('b.txt')
    expect(byTime.slice(1).sort()).toEqual(['a.txt', 'photos'])
  })

  it('builds a loopback share link and parses a list', () => {
    expect(formatBytes(2480)).toBe('2.4 KB')
    expect(localShareURL('http://127.0.0.1:8081', 'bafyabc')).toBe('http://127.0.0.1:8081/ipfs/bafyabc')
    expect(localShareURL('http://192.0.2.1:8081', 'bafyabc')).toBeNull()
    expect(localShareURL('http://127.0.0.1:5001', 'bafyabc')).toBe('http://127.0.0.1:5001/ipfs/bafyabc')
    const list = parseList({
      path: '/',
      localGateway: 'http://127.0.0.1:8081',
      maxBytes: 1024,
      entries: [{ name: 'notes.txt', type: 'file', size: 5, cid: 'bafy', modified: '2026-11-01T00:00:00Z' }],
    })
    expect(list?.entries[0]?.cid).toBe('bafy')
    expect(parseList({ path: '/../x', entries: [] })).toBeNull()
  })
})
