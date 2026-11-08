export type FileKind = 'file' | 'directory'

export interface FileEntry {
  name: string
  type: FileKind
  size: number
  cid: string
  modified: string | null
}

export interface FileList {
  path: string
  localGateway: string
  maxBytes: number
  entries: FileEntry[]
}

export type SortKey = 'name' | 'size' | 'modified'
export type SortDir = 'asc' | 'desc'
export type UploadStatus = 'queued' | 'uploading' | 'done' | 'error'

export interface UploadJob {
  id: number
  name: string
  percent: number
  status: UploadStatus
  error: string
}

const maxPathBytes = 4096
const maxNameBytes = 255

function byteLength(value: string): number {
  return new TextEncoder().encode(value).length
}

export function cleanPath(value: string): string | null {
  if (!value || byteLength(value) > maxPathBytes || !value.startsWith('/')) return null
  if (value.includes('\\') || value.includes('"') || value.includes('\u0000')) return null
  if (value === '/') return '/'
  const raw = value.replace(/\/+$/, '')
  if (raw === '') return '/'
  const parts = raw.split('/')
  const clean: string[] = []
  for (let i = 0; i < parts.length; i += 1) {
    const part = parts[i] ?? ''
    if (i === 0) {
      if (part !== '') return null
      continue
    }
    if (!validName(part)) return null
    clean.push(part)
  }
  if (clean.length === 0) return null
  return `/${clean.join('/')}`
}

export function validName(name: string): boolean {
  if (!name || name === '.' || name === '..' || byteLength(name) > maxNameBytes) return false
  if (name.trim() !== name) return false
  if (name.includes('/') || name.includes('\\') || name.includes('"') || name.includes('\u0000')) return false
  for (const char of name) {
    const code = char.codePointAt(0) ?? 0
    if (code < 0x20 || code === 0x7f) return false
  }
  return true
}

export function joinPath(dir: string, name: string): string | null {
  const parent = cleanPath(dir)
  if (!parent || !validName(name)) return null
  return parent === '/' ? `/${name}` : `${parent}/${name}`
}

export function parentPath(path: string): string {
  const clean = cleanPath(path)
  if (!clean || clean === '/') return '/'
  const index = clean.lastIndexOf('/')
  return index <= 0 ? '/' : clean.slice(0, index)
}

export function baseName(path: string): string {
  const clean = cleanPath(path)
  if (!clean || clean === '/') return ''
  return clean.slice(clean.lastIndexOf('/') + 1)
}

export interface Crumb {
  name: string
  path: string
}

export function crumbs(path: string): Crumb[] {
  const clean = cleanPath(path) ?? '/'
  const items: Crumb[] = [{ name: '', path: '/' }]
  if (clean === '/') return items
  const parts = clean.split('/').filter(Boolean)
  let current = ''
  for (const part of parts) {
    current += `/${part}`
    items.push({ name: part, path: current })
  }
  return items
}

export function copyName(name: string): string {
  const dot = name.lastIndexOf('.')
  if (dot > 0) return `${name.slice(0, dot)} copy${name.slice(dot)}`
  return `${name} copy`
}

export function formatBytes(size: number): string {
  if (!Number.isFinite(size) || size < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  if (index === 0) return `${Math.round(value)} B`
  const digits = value >= 10 ? 0 : 1
  return `${value.toFixed(digits)} ${units[index]}`
}

export function localShareURL(gateway: string, cid: string): string | null {
  if (!gateway || !cid) return null
  let url: URL
  try {
    url = new URL(gateway)
  } catch {
    return null
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') return null
  if (url.hostname !== '127.0.0.1' && url.hostname !== '::1') return null
  url.pathname = `/ipfs/${encodeURIComponent(cid)}`
  url.search = ''
  url.hash = ''
  return url.toString()
}

export function sortEntries(entries: FileEntry[], key: SortKey, dir: SortDir): FileEntry[] {
  const sign = dir === 'asc' ? 1 : -1
  return [...entries].sort((a, b) => {
    if (key === 'name') {
      const folder = Number(a.type !== 'directory') - Number(b.type !== 'directory')
      if (folder !== 0) return folder
      return a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }) * sign
    }
    if (key === 'size') {
      return (a.size - b.size) * sign || a.name.localeCompare(b.name)
    }
    const am = a.modified ? Date.parse(a.modified) : Number.NaN
    const bm = b.modified ? Date.parse(b.modified) : Number.NaN
    if (Number.isNaN(am) && Number.isNaN(bm)) return a.name.localeCompare(b.name)
    if (Number.isNaN(am)) return 1
    if (Number.isNaN(bm)) return -1
    return (am - bm) * sign || a.name.localeCompare(b.name)
  })
}

export function parseList(value: unknown): FileList | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  const path = typeof body.path === 'string' ? cleanPath(body.path) : null
  if (!path || !Array.isArray(body.entries)) return null
  const entries: FileEntry[] = []
  for (const item of body.entries) {
    const entry = parseEntry(item)
    if (entry) entries.push(entry)
  }
  const maxBytes = typeof body.maxBytes === 'number' && body.maxBytes > 0 ? body.maxBytes : 512 * 1024 * 1024
  const localGateway = typeof body.localGateway === 'string' ? body.localGateway : ''
  return { path, localGateway, maxBytes, entries }
}

export function parseEntry(value: unknown): FileEntry | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (typeof body.name !== 'string' || !validName(body.name)) return null
  if (body.type !== 'file' && body.type !== 'directory') return null
  const size = typeof body.size === 'number' && body.size >= 0 ? body.size : 0
  const cid = typeof body.cid === 'string' ? body.cid : ''
  const modified = typeof body.modified === 'string' && body.modified ? body.modified : null
  return { name: body.name, type: body.type, size, cid, modified }
}

export function entryPath(dir: string, entry: FileEntry): string | null {
  return joinPath(dir, entry.name)
}
