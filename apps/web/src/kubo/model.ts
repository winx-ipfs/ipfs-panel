import type { MessageSchema } from '@/locales/en'

export interface KuboJob {
  id: string
  action: string
  state: string
  phase: string
  version: string
  received: number
  total: number
  error: string
}

export interface KuboStatus {
  source: string
  version: string
  pinned: string
  supported: string
  latest: string
  upstream: string
  upgrade: boolean
  check: string
  job: KuboJob | null
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

export function parseJob(value: unknown): KuboJob | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (typeof body.id !== 'string' || body.id === '') return null
  if (typeof body.state !== 'string') return null
  return {
    id: body.id,
    action: text(body.action),
    state: body.state,
    phase: text(body.phase),
    version: text(body.version),
    received: typeof body.received === 'number' ? body.received : 0,
    total: typeof body.total === 'number' ? body.total : 0,
    error: text(body.error),
  }
}

export function parseKubo(value: unknown): KuboStatus | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (typeof body.source !== 'string' || body.source === '') return null
  let job: KuboJob | null = null
  if (body.job != null) {
    job = parseJob(body.job)
    if (!job) return null
  }
  return {
    source: body.source,
    version: text(body.version),
    pinned: text(body.pinned),
    supported: text(body.supported),
    latest: text(body.latest),
    upstream: text(body.upstream),
    upgrade: body.upgrade === true,
    check: text(body.check),
    job,
  }
}

const phases: Record<string, keyof MessageSchema> = {
  downloading: 'nodeKuboDownloading',
  verifying: 'nodeKuboVerifying',
  stopping: 'nodeKuboStopping',
  swapping: 'nodeKuboSwapping',
  migrating: 'nodeKuboMigrating',
  starting: 'nodeKuboStarting',
  rolling_back: 'nodeKuboRolling',
}

const failures: Record<string, keyof MessageSchema> = {
  verify_failed: 'nodeKuboVerifyFailed',
  offline: 'nodeKuboOffline',
  migrate_failed: 'nodeKuboMigrateFailed',
  start_failed: 'nodeKuboStartFailed',
  rollback_failed: 'nodeKuboRollbackFailed',
  stop_failed: 'nodeKuboStopFailed',
}

// jobMessageKey is the sentence for a running or finished job.
// A succeeded job has nothing to add: the version row is the result.
export function jobMessageKey(job: KuboJob | null): keyof MessageSchema | null {
  if (!job) return null
  if (job.state === 'rolled_back') return 'nodeKuboRolledBack'
  if (job.state === 'failed') return failures[job.error] ?? 'nodeKuboStartFailed'
  if (job.state !== 'running') return null
  return phases[job.phase] ?? 'nodeKuboDownloading'
}
