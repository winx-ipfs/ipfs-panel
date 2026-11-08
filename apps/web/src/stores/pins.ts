import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Pin, PinJob, PinList } from '@/pins/model'
import { parseJobs, parseList, parsePin } from '@/pins/model'
import { csrfHeaders, csrfToken } from '@/lib/csrf'
import { notifyUnauthorized } from '@/lib/session'

export type PinsStatus = 'loading' | 'ready' | 'blocked' | 'unreachable'

export interface PinResult {
  ok: boolean
  status: number
  error: string
  state: string
}

function readError(value: unknown): { error: string; state: string } {
  if (!value || typeof value !== 'object') return { error: '', state: '' }
  const body = value as Record<string, unknown>
  return {
    error: typeof body.error === 'string' ? body.error : '',
    state: typeof body.state === 'string' ? body.state : '',
  }
}

async function readJSON(res: Response): Promise<unknown> {
  try {
    return await res.json()
  } catch {
    return {}
  }
}

export const usePinsStore = defineStore('pins', () => {
  const pins = ref<Pin[]>([])
  const jobs = ref<PinJob[]>([])
  const namesFrom = ref<'kubo' | 'panel'>('panel')
  const localGateway = ref('')
  const maxBytes = ref(512 * 1024 * 1024)
  const status = ref<PinsStatus>('loading')
  const nodeState = ref('')
  const preview = ref(false)

  function showPreview(list: PinList, nextJobs: PinJob[], blocked: string) {
    preview.value = true
    pins.value = list.pins
    jobs.value = nextJobs
    namesFrom.value = list.namesFrom
    localGateway.value = list.localGateway
    maxBytes.value = list.maxBytes
    if (blocked) {
      status.value = 'blocked'
      nodeState.value = blocked
      return
    }
    status.value = 'ready'
    nodeState.value = ''
  }

  function applyBlocked(state: string) {
    status.value = 'blocked'
    nodeState.value = state || 'down'
    pins.value = []
  }

  async function load(): Promise<boolean> {
    preview.value = false
    const first = status.value === 'loading' || pins.value.length === 0
    if (first) status.value = 'loading'
    try {
      const res = await fetch('/api/pins', { credentials: 'same-origin' })
      if (res.status === 401) {
        notifyUnauthorized()
        return false
      }
      const body = await readJSON(res)
      const problem = readError(body)
      if (res.status === 503 && problem.error === 'node_not_running') {
        applyBlocked(problem.state)
        return false
      }
      if (!res.ok) {
        if (status.value === 'loading') status.value = 'unreachable'
        return false
      }
      const parsed = parseList(body)
      if (!parsed) {
        status.value = 'unreachable'
        return false
      }
      pins.value = parsed.pins
      namesFrom.value = parsed.namesFrom
      localGateway.value = parsed.localGateway
      maxBytes.value = parsed.maxBytes
      status.value = 'ready'
      nodeState.value = ''
      return true
    } catch {
      status.value = 'unreachable'
      return false
    }
  }

  async function refreshJobs(): Promise<void> {
    if (preview.value) return
    const running = new Set(jobs.value.filter((job) => job.state === 'running').map((job) => job.id))
    try {
      const res = await fetch('/api/pins/jobs', { credentials: 'same-origin' })
      if (res.status === 401) {
        notifyUnauthorized()
        return
      }
      if (!res.ok) return
      const next = parseJobs(await readJSON(res))
      jobs.value = next
      const finished = next.some((job) => running.has(job.id) && job.state === 'succeeded')
      if (finished) await load()
    } catch {
      // The list stays. The next poll tries again.
    }
  }

  async function post(url: string, body: unknown): Promise<PinResult> {
    try {
      const res = await fetch(url, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', ...csrfHeaders() },
        body: JSON.stringify(body),
      })
      if (res.status === 401) {
        notifyUnauthorized()
        return { ok: false, status: 401, error: 'unauthorized', state: '' }
      }
      const payload = await readJSON(res)
      const problem = readError(payload)
      if (res.status === 503 && problem.error === 'node_not_running') applyBlocked(problem.state)
      if (res.ok) await refreshJobs()
      return { ok: res.ok, status: res.status, error: problem.error, state: problem.state }
    } catch {
      return { ok: false, status: 0, error: 'network', state: '' }
    }
  }

  function addPin(target: string, name: string, recursive: boolean) {
    return post('/api/pins', { target, name, recursive })
  }

  function pinPath(path: string, name: string) {
    return post('/api/pins/from-files', name ? { path, name } : { path })
  }

  function unpin(cid: string, type: Pin['type']) {
    return post('/api/pins/remove', { cid, type }).then(async (result) => {
      if (result.ok) await load()
      return result
    })
  }

  function cancel(id: string) {
    return post(`/api/pins/jobs/${id}/cancel`, {})
  }

  function upload(file: File, name: string, onProgress: (percent: number) => void): Promise<PinResult> {
    const query = new URLSearchParams({ filename: file.name })
    if (name) query.set('name', name)
    return new Promise((resolve) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `/api/pins/upload?${query.toString()}`)
      xhr.withCredentials = true
      xhr.setRequestHeader('Content-Type', 'application/octet-stream')
      xhr.setRequestHeader('X-CSRF-Token', csrfToken())
      xhr.upload.onprogress = (event) => {
        if (!event.lengthComputable || event.total === 0) return
        onProgress(Math.min(100, Math.round((event.loaded / event.total) * 100)))
      }
      xhr.onload = () => {
        let payload: unknown
        try {
          payload = JSON.parse(xhr.responseText) as unknown
        } catch {
          payload = {}
        }
        if (xhr.status === 401) {
          notifyUnauthorized()
          resolve({ ok: false, status: 401, error: 'unauthorized', state: '' })
          return
        }
        const problem = readError(payload)
        if (xhr.status === 503 && problem.error === 'node_not_running') applyBlocked(problem.state)
        const pin = parsePin(payload)
        if (xhr.status >= 200 && xhr.status < 300 && pin) {
          void load()
          resolve({ ok: true, status: xhr.status, error: '', state: '' })
          return
        }
        resolve({ ok: false, status: xhr.status, error: problem.error || 'network', state: problem.state })
      }
      xhr.onerror = () => resolve({ ok: false, status: 0, error: 'network', state: '' })
      xhr.send(file)
    })
  }

  return {
    pins,
    jobs,
    namesFrom,
    localGateway,
    maxBytes,
    status,
    nodeState,
    preview,
    showPreview,
    load,
    refreshJobs,
    addPin,
    pinPath,
    unpin,
    cancel,
    upload,
  }
})
