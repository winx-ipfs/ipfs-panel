export type LogSource = 'manager' | 'kubo'
export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface LogLine {
  id: number
  time: string
  source: LogSource
  level: LogLevel
  text: string
}

export interface LogPage {
  lines: LogLine[]
  next: number
  node: string
  capture: boolean
}

const rank: Record<LogLevel, number> = { debug: 0, info: 1, warn: 2, error: 3 }

const sources = new Set<LogSource>(['manager', 'kubo'])
const levels = new Set<LogLevel>(['debug', 'info', 'warn', 'error'])

export function filterLines(
  lines: LogLine[],
  opts: { source: string; level: string; q: string },
): LogLine[] {
  const q = opts.q.trim().toLowerCase()
  const min = rank[opts.level as LogLevel] ?? rank.info
  return lines.filter((line) => {
    if (opts.source !== 'all' && line.source !== opts.source) return false
    if ((rank[line.level] ?? 0) < min) return false
    if (q && !line.text.toLowerCase().includes(q)) return false
    return true
  })
}

export function parsePage(body: unknown): LogPage | null {
  if (!body || typeof body !== 'object') return null
  const rec = body as Record<string, unknown>
  if (!Array.isArray(rec.lines)) return null
  const lines: LogLine[] = []
  for (const item of rec.lines) {
    const line = parseLine(item)
    if (line) lines.push(line)
  }
  return {
    lines,
    next: typeof rec.next === 'number' && Number.isFinite(rec.next) ? rec.next : 0,
    node: typeof rec.node === 'string' && rec.node ? rec.node : 'down',
    capture: rec.capture === true,
  }
}

export function parseLine(body: unknown): LogLine | null {
  if (!body || typeof body !== 'object') return null
  const rec = body as Record<string, unknown>
  if (typeof rec.id !== 'number' || !Number.isFinite(rec.id)) return null
  if (typeof rec.time !== 'string' || typeof rec.text !== 'string') return null
  if (typeof rec.source !== 'string' || !sources.has(rec.source as LogSource)) return null
  if (typeof rec.level !== 'string' || !levels.has(rec.level as LogLevel)) return null
  return {
    id: rec.id,
    time: rec.time,
    source: rec.source as LogSource,
    level: rec.level as LogLevel,
    text: rec.text,
  }
}

export function clock(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  const pad = (value: number, width = 2) => String(value).padStart(width, '0')
  return `${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}.${pad(date.getUTCMilliseconds(), 3)}`
}
