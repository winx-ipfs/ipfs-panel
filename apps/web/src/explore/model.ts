export type ExploreKind = 'folder' | 'file' | 'bytes' | 'data' | 'symlink' | 'block'

export interface ExploreLink {
  name: string
  cid: string
  size: number | null
}

export interface ExploreView {
  target: string
  cid: string
  version: number
  codec: string
  hash: string
  kind: ExploreKind
  size: number | null
  focus: string
  text: string
  data: unknown
  dataOmitted: boolean
  links: ExploreLink[]
  linksTruncated: boolean
  localGateway: string
}

export interface Crumb {
  label: string
  target: string
}

const kinds = new Set<ExploreKind>(['folder', 'file', 'bytes', 'data', 'symlink', 'block'])

export function parseView(value: unknown): ExploreView | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (typeof body.target !== 'string' || typeof body.cid !== 'string' || typeof body.codec !== 'string') return null
  if (typeof body.version !== 'number' || !Number.isFinite(body.version)) return null
  if (typeof body.kind !== 'string' || !kinds.has(body.kind as ExploreKind)) return null
  if (!Array.isArray(body.links)) return null
  const links: ExploreLink[] = []
  for (const item of body.links) {
    if (!item || typeof item !== 'object') return null
    const row = item as Record<string, unknown>
    if (typeof row.name !== 'string' || typeof row.cid !== 'string') return null
    const size = typeof row.size === 'number' && Number.isFinite(row.size) ? row.size : null
    links.push({ name: row.name, cid: row.cid, size })
  }
  return {
    target: body.target,
    cid: body.cid,
    version: body.version,
    codec: body.codec,
    hash: typeof body.hash === 'string' ? body.hash : '',
    kind: body.kind as ExploreKind,
    size: typeof body.size === 'number' && Number.isFinite(body.size) ? body.size : null,
    focus: typeof body.focus === 'string' ? body.focus : '',
    text: typeof body.text === 'string' ? body.text : '',
    data: 'data' in body ? body.data : undefined,
    dataOmitted: body.dataOmitted === true,
    links,
    linksTruncated: body.linksTruncated === true,
    localGateway: typeof body.localGateway === 'string' ? body.localGateway : '',
  }
}

export function crumbs(target: string): Crumb[] {
  const path = target.startsWith('/ipfs/') ? target : `/ipfs/${target}`
  const parts = path.split('/').filter(Boolean)
  if (parts[0] !== 'ipfs' || parts.length < 2) return []
  const out: Crumb[] = []
  for (let i = 1; i < parts.length; i += 1) {
    const label = parts[i]
    if (!label) continue
    out.push({
      label,
      target: `/ipfs/${parts.slice(1, i + 1).join('/')}`,
    })
  }
  return out
}

export function nextTarget(current: string, link: Pick<ExploreLink, 'name' | 'cid'>): string {
  if (!safeSegment(link.name)) return link.cid
  const base = current.startsWith('/ipfs/') ? current : `/ipfs/${current}`
  return `${base}/${link.name}`
}

export function exploreURL(gateway: string, target: string, cid: string): string | null {
  if (!gateway || !cid) return null
  let url: URL
  try {
    url = new URL(gateway)
  } catch {
    return null
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') return null
  if (url.hostname !== '127.0.0.1' && url.hostname !== '::1') return null
  const path = target.startsWith('/ipfs/') ? target : `/ipfs/${cid}`
  const segments = path.split('/').filter(Boolean)
  if (segments[0] !== 'ipfs' || segments.length < 2) return null
  url.pathname = `/${segments.map((part) => encodeURIComponent(part)).join('/')}`
  url.search = ''
  url.hash = ''
  return url.toString()
}

function safeSegment(name: string): boolean {
  if (!name || name === '.' || name === '..' || name.includes('/') || name.includes('\\')) return false
  for (const char of name) {
    const code = char.codePointAt(0) ?? 0
    if (code < 0x20 || code === 0x7f) return false
  }
  return true
}
