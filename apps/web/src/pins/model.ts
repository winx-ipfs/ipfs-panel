export type PinType = 'recursive' | 'direct'
export type PinFilter = 'all' | PinType
export type JobState = 'running' | 'succeeded' | 'failed' | 'cancelled'

export interface Pin {
  cid: string
  name: string
  type: PinType
  size: number | null
}

export interface PinJob {
  id: string
  cid: string
  name: string
  type: PinType
  state: JobState
  progress: number
  error: string
  created: string
}

export interface PinList {
  namesFrom: 'kubo' | 'panel'
  localGateway: string
  maxBytes: number
  pins: Pin[]
}

export function filterPins(pins: Pin[], query: string, kind: PinFilter): Pin[] {
  const needle = query.trim().toLowerCase()
  return pins.filter((pin) => {
    if (kind !== 'all' && pin.type !== kind) return false
    if (!needle) return true
    return pin.name.toLowerCase().includes(needle) || pin.cid.toLowerCase().includes(needle)
  })
}

export function sortPins(pins: Pin[], locale: string): Pin[] {
  return [...pins].sort((a, b) => {
    if (!a.name && b.name) return 1
    if (a.name && !b.name) return -1
    const byName = a.name.localeCompare(b.name, locale, { sensitivity: 'base', numeric: true })
    if (byName !== 0) return byName
    if (a.cid !== b.cid) return a.cid < b.cid ? -1 : 1
    return a.type < b.type ? -1 : a.type > b.type ? 1 : 0
  })
}

export function parseList(value: unknown): PinList | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (!Array.isArray(body.pins)) return null
  const pins: Pin[] = []
  for (const item of body.pins) {
    const pin = parsePin(item)
    if (pin) pins.push(pin)
  }
  const namesFrom = body.namesFrom === 'kubo' ? 'kubo' : 'panel'
  const localGateway = typeof body.localGateway === 'string' ? body.localGateway : ''
  const maxBytes = typeof body.maxBytes === 'number' && body.maxBytes > 0 ? body.maxBytes : 512 * 1024 * 1024
  return { namesFrom, localGateway, maxBytes, pins }
}

export function parsePin(value: unknown): Pin | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (typeof body.cid !== 'string' || !body.cid) return null
  if (body.type !== 'recursive' && body.type !== 'direct') return null
  const name = typeof body.name === 'string' ? body.name : ''
  const size = typeof body.size === 'number' && body.size >= 0 ? body.size : null
  return { cid: body.cid, name, type: body.type, size }
}

export function parseJob(value: unknown): PinJob | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (typeof body.id !== 'string' || !body.id) return null
  if (body.type !== 'recursive' && body.type !== 'direct') return null
  if (body.state !== 'running' && body.state !== 'succeeded' && body.state !== 'failed' && body.state !== 'cancelled') {
    return null
  }
  return {
    id: body.id,
    cid: typeof body.cid === 'string' ? body.cid : '',
    name: typeof body.name === 'string' ? body.name : '',
    type: body.type,
    state: body.state,
    progress: typeof body.progress === 'number' && body.progress > 0 ? body.progress : 0,
    error: typeof body.error === 'string' ? body.error : '',
    created: typeof body.created === 'string' ? body.created : '',
  }
}

export function parseJobs(value: unknown): PinJob[] {
  if (!value || typeof value !== 'object') return []
  const body = value as Record<string, unknown>
  if (!Array.isArray(body.jobs)) return []
  return body.jobs.flatMap((item) => {
    const job = parseJob(item)
    return job ? [job] : []
  })
}
