import { defineStore } from 'pinia'
import { ref } from 'vue'
import { csrfHeaders } from '@/lib/csrf'
import { notifyUnauthorized } from '@/lib/session'
import { filterLines, parseLine, parsePage, type LogLine } from '@/logs/model'

export type LogsStatus = 'loading' | 'ready' | 'failed'

export const useLogsStore = defineStore('logs', () => {
  const lines = ref<LogLine[]>([])
  const base = ref<LogLine[]>([])
  const next = ref(0)
  const node = ref('down')
  const capture = ref(false)
  const status = ref<LogsStatus>('loading')
  const error = ref('')
  const detail = ref('')
  const source = ref('all')
  const level = ref('info')
  const q = ref('')
  const follow = ref(true)
  const preview = ref(false)
  let generation = 0
  let stream: EventSource | null = null
  let gapTimer = 0

  function closeStream() {
    window.clearTimeout(gapTimer)
    gapTimer = 0
    stream?.close()
    stream = null
  }

  function showPreview(shot: {
    status: LogsStatus
    error: string
    node: string
    capture: boolean
    follow: boolean
    source: string
    level: string
    q: string
    lines: LogLine[]
  }) {
    closeStream()
    preview.value = true
    generation += 1
    status.value = shot.status
    error.value = shot.error
    detail.value = ''
    node.value = shot.node
    capture.value = shot.capture
    follow.value = shot.follow
    source.value = shot.source
    level.value = shot.level
    q.value = shot.q
    base.value = shot.lines
    lines.value = shot.lines
    next.value = shot.lines.at(-1)?.id ?? 0
  }

  function applyLocal() {
    lines.value = filterLines(base.value, { source: source.value, level: level.value, q: q.value })
  }

  async function load() {
    if (preview.value) return
    const mine = ++generation
    error.value = ''
    detail.value = ''
    status.value = 'loading'
    closeStream()
    try {
      const params = new URLSearchParams({
        source: source.value,
        level: level.value,
        q: q.value,
        limit: '500',
      })
      const res = await fetch(`/api/logs?${params}`, { credentials: 'same-origin' })
      if (mine !== generation) return
      if (res.status === 401) {
        notifyUnauthorized()
        return
      }
      const body = await res.json().catch(() => ({}))
      if (mine !== generation) return
      if (!res.ok) {
        const problem = readProblem(body)
        error.value = problem.error || 'unavailable'
        detail.value = problem.detail
        status.value = 'failed'
        return
      }
      const page = parsePage(body)
      if (!page) {
        error.value = 'unavailable'
        status.value = 'failed'
        return
      }
      lines.value = page.lines
      base.value = page.lines
      next.value = page.next
      node.value = page.node
      capture.value = page.capture
      status.value = 'ready'
      openStream()
    } catch {
      if (mine !== generation) return
      error.value = 'network'
      status.value = 'failed'
    }
  }

  function openStream() {
    closeStream()
    if (preview.value || status.value !== 'ready') return
    const params = new URLSearchParams({
      after: String(next.value),
      source: source.value,
      level: level.value,
      q: q.value,
    })
    const sourceStream = new EventSource(`/api/logs/stream?${params}`)
    stream = sourceStream
    sourceStream.addEventListener('line', (event) => {
      const line = parseLine(JSON.parse((event as MessageEvent).data as string))
      if (!line || line.id <= next.value) return
      if (lines.value.some((item) => item.id === line.id)) return
      lines.value = trim([...lines.value, line])
      next.value = line.id
    })
    sourceStream.addEventListener('status', (event) => {
      const body = JSON.parse((event as MessageEvent).data as string) as { node?: string; capture?: boolean }
      if (typeof body.node === 'string') node.value = body.node
      capture.value = body.capture === true
    })
    sourceStream.addEventListener('gap', () => {
      window.clearTimeout(gapTimer)
      gapTimer = window.setTimeout(() => {
        void catchUp()
      }, 150)
    })
    sourceStream.onerror = () => {
      if (sourceStream.readyState === EventSource.CLOSED && status.value === 'ready' && lines.value.length === 0) {
        error.value = 'network'
        status.value = 'failed'
      }
    }
  }

  async function catchUp() {
    if (preview.value) return
    const params = new URLSearchParams({
      after: String(next.value),
      source: source.value,
      level: level.value,
      q: q.value,
      limit: '500',
    })
    const res = await fetch(`/api/logs?${params}`, { credentials: 'same-origin' })
    if (!res.ok) return
    const page = parsePage(await res.json().catch(() => null))
    if (!page) return
    const seen = new Set(lines.value.map((line) => line.id))
    const extra = page.lines.filter((line) => !seen.has(line.id))
    if (extra.length) lines.value = trim([...lines.value, ...extra])
    if (page.next > next.value) next.value = page.next
    node.value = page.node
    capture.value = page.capture
  }

  function setFollow(value: boolean) {
    follow.value = value
  }

  function clearView() {
    lines.value = []
    if (preview.value) base.value = []
  }

  async function setSource(value: string) {
    source.value = value
    if (preview.value) {
      applyLocal()
      return
    }
    await load()
  }

  async function setLevel(value: string) {
    level.value = value
    if (preview.value) {
      applyLocal()
      return
    }
    await load()
  }

  async function setQuery(value: string) {
    q.value = value
    if (preview.value) {
      applyLocal()
      return
    }
    await load()
  }

  function resetFilters() {
    source.value = 'all'
    level.value = 'info'
    q.value = ''
    if (preview.value) {
      applyLocal()
      return
    }
    void load()
  }

  async function applyLevel(subsystem: string, writeLevel: string): Promise<{ error: string; detail: string }> {
    if (preview.value) return { error: '', detail: '' }
    const res = await fetch('/api/logs/level', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', ...csrfHeaders() },
      body: JSON.stringify({ subsystem, level: writeLevel }),
    })
    if (res.status === 401) {
      notifyUnauthorized()
      return { error: 'unauthorized', detail: '' }
    }
    const body = await res.json().catch(() => ({}))
    if (!res.ok) return readProblem(body)
    return { error: '', detail: '' }
  }

  async function download() {
    const params = new URLSearchParams({ source: source.value === 'all' ? 'all' : source.value })
    const res = await fetch(`/api/logs/download?${params}`, { credentials: 'same-origin' })
    if (res.status === 401) {
      notifyUnauthorized()
      return
    }
    if (!res.ok) {
      error.value = 'unavailable'
      return
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download =
      source.value === 'manager' ? 'ipfs-panel-manager.txt' : source.value === 'kubo' ? 'ipfs-panel-kubo.txt' : 'ipfs-panel-logs.txt'
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  }

  function stop() {
    generation += 1
    closeStream()
  }

  return {
    lines,
    next,
    node,
    capture,
    status,
    error,
    detail,
    source,
    level,
    q,
    follow,
    preview,
    showPreview,
    load,
    setFollow,
    clearView,
    setSource,
    setLevel,
    setQuery,
    resetFilters,
    applyLevel,
    download,
    stop,
  }
})

function trim(lines: LogLine[]): LogLine[] {
  if (lines.length <= 2000) return lines
  return lines.slice(lines.length - 2000)
}

function readProblem(body: unknown): { error: string; detail: string } {
  if (!body || typeof body !== 'object') return { error: 'unavailable', detail: '' }
  const rec = body as Record<string, unknown>
  return {
    error: typeof rec.error === 'string' ? rec.error : 'unavailable',
    detail: typeof rec.detail === 'string' ? rec.detail : '',
  }
}
