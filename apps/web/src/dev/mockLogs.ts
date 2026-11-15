import type { LogLine } from '@/logs/model'

export interface LogsPreview {
  status: 'loading' | 'ready' | 'failed'
  error: string
  node: string
  capture: boolean
  follow: boolean
  source: string
  level: string
  q: string
  lines: LogLine[]
}

function line(id: number, source: LogLine['source'], level: LogLine['level'], text: string): LogLine {
  const second = String(id).padStart(2, '0')
  return {
    id,
    time: `2026-11-15T11:14:${second}.000Z`,
    source,
    level,
    text,
  }
}

const stream: LogLine[] = [
  line(1, 'manager', 'info', 'manager listening addr=127.0.0.1:8080'),
  line(2, 'manager', 'info', 'audit event=login result=success username=ada'),
  line(3, 'kubo', 'info', 'Initializing daemon'),
  line(4, 'kubo', 'info', 'Kubo version: 0.43.1'),
  line(5, 'kubo', 'info', 'Repo version: 17'),
  line(6, 'kubo', 'info', 'Swarm listening on /ip4/127.0.0.1/tcp/4001'),
  line(7, 'kubo', 'info', 'core daemon is ready'),
  line(8, 'manager', 'info', 'audit event=node result=success state=up'),
  line(9, 'kubo', 'info', 'gateway local request /ipfs/bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi'),
  line(10, 'kubo', 'warn', 'bitswap wantlist peer slow to respond'),
  line(11, 'manager', 'info', 'audit event=files_upload result=success'),
  line(12, 'kubo', 'info', 'core received a block for the local pin'),
  line(13, 'kubo', 'error', 'dht routing table refresh failed once; will retry'),
  line(14, 'manager', 'warn', 'files op=stat code=timeout'),
  line(15, 'kubo', 'info', 'bitswap session idle'),
  line(16, 'manager', 'info', 'audit event=config_replace result=success'),
  line(17, 'kubo', 'info', 'provider queued 4 local records'),
  line(18, 'kubo', 'debug', 'dht query peer candidate skipped'),
  line(19, 'manager', 'info', 'audit event=log_level result=success subsystem=all level=info'),
  line(20, 'kubo', 'info', 'swarm connection closed cleanly'),
  line(21, 'kubo', 'warn', 'reprovider batch took longer than the last interval'),
  line(22, 'manager', 'info', 'audit event=logout result=success username=ada'),
  line(23, 'kubo', 'info', 'gateway local request /ipfs/bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi/readme.txt'),
  line(24, 'kubo', 'info', 'core heartbeat'),
  line(25, 'kubo', 'info', 'gateway local request finished'),
  line(26, 'manager', 'info', 'audit event=files_download result=success path=/gateway-notes.txt'),
  line(27, 'kubo', 'info', 'gateway refused a write; the public gateway is read only'),
  line(28, 'kubo', 'warn', 'gateway took 1.2s to read a local block'),
]

const filtered = stream.filter((item) => item.text.toLowerCase().includes('gateway'))

export function logsPreview(search: string): LogsPreview | null {
  const shot = new URLSearchParams(search).get('logs')
  if (!shot) return null
  const base: LogsPreview = {
    status: 'ready',
    error: '',
    node: 'up',
    capture: true,
    follow: true,
    source: 'all',
    level: 'info',
    q: '',
    lines: stream.filter((item) => item.level !== 'debug'),
  }
  if (shot === 'loading') return { ...base, status: 'loading', lines: [] }
  if (shot === 'streaming') return base
  if (shot === 'paused') return { ...base, follow: false }
  if (shot === 'filtered') return { ...base, q: 'gateway', lines: filtered }
  if (shot === 'empty') return { ...base, lines: [] }
  if (shot === 'stopped') return { ...base, node: 'down', capture: false, lines: [] }
  if (shot === 'error') return { ...base, status: 'failed', error: 'unavailable', lines: [] }
  return null
}
