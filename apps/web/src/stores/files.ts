import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { FileEntry, FileList, UploadJob } from '@/files/model'
import { cleanPath, joinPath, parseEntry, parseList } from '@/files/model'
import { csrfHeaders, csrfToken } from '@/lib/csrf'
import { notifyUnauthorized } from '@/lib/session'

export type FilesStatus = 'loading' | 'ready' | 'blocked' | 'unreachable'

interface ApiResult {
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

export const useFilesStore = defineStore('files', () => {
  const path = ref('/')
  const entries = ref<FileEntry[]>([])
  const localGateway = ref('')
  const maxBytes = ref(512 * 1024 * 1024)
  const status = ref<FilesStatus>('loading')
  const nodeState = ref('')
  const uploads = ref<UploadJob[]>([])
  let nextId = 1

  function showPreview(list: FileList, jobs: UploadJob[], blocked: string) {
    path.value = list.path
    entries.value = list.entries
    localGateway.value = list.localGateway
    maxBytes.value = list.maxBytes
    uploads.value = jobs
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
    entries.value = []
  }

  async function load(next: string): Promise<boolean> {
    const clean = cleanPath(next)
    if (!clean) {
      status.value = 'ready'
      return false
    }
    const first = status.value === 'loading' || entries.value.length === 0
    if (first) status.value = 'loading'
    try {
      const res = await fetch(`/api/files?path=${encodeURIComponent(clean)}`, { credentials: 'same-origin' })
      if (res.status === 401) {
        notifyUnauthorized()
        return false
      }
      const body = await readJSON(res)
      const problem = readError(body)
      if (res.status === 503 && problem.error === 'node_not_running') {
        path.value = clean
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
      path.value = parsed.path
      entries.value = parsed.entries
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

  async function mutate(url: string, body: unknown): Promise<ApiResult> {
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
      if (res.status === 503 && problem.error === 'node_not_running') {
        applyBlocked(problem.state)
      }
      if (res.ok) await load(path.value)
      return { ok: res.ok, status: res.status, error: problem.error, state: problem.state }
    } catch {
      return { ok: false, status: 0, error: 'network', state: '' }
    }
  }

  function mkdir(name: string) {
    const next = joinPath(path.value, name)
    if (!next) return Promise.resolve({ ok: false, status: 400, error: 'invalid_path', state: '' })
    return mutate('/api/files/mkdir', { path: next })
  }

  function rename(entry: FileEntry, name: string) {
    const from = joinPath(path.value, entry.name)
    if (!from || !name) return Promise.resolve({ ok: false, status: 400, error: 'invalid_path', state: '' })
    return mutate('/api/files/rename', { path: from, name })
  }

  function move(entry: FileEntry, folder: string) {
    const from = joinPath(path.value, entry.name)
    const to = joinPath(folder, entry.name)
    if (!from || !to || from === to) {
      return Promise.resolve({ ok: false, status: 400, error: 'invalid_path', state: '' })
    }
    return mutate('/api/files/move', { from, to })
  }

  function copy(entry: FileEntry, name: string) {
    const from = joinPath(path.value, entry.name)
    const to = joinPath(path.value, name)
    if (!from || !to || from === to) {
      return Promise.resolve({ ok: false, status: 400, error: 'invalid_path', state: '' })
    }
    return mutate('/api/files/copy', { from, to })
  }

  function remove(entry: FileEntry) {
    const target = joinPath(path.value, entry.name)
    if (!target) return Promise.resolve({ ok: false, status: 400, error: 'invalid_path', state: '' })
    return mutate('/api/files/delete', { path: target })
  }

  function updateJob(id: number, patch: Partial<UploadJob>) {
    uploads.value = uploads.value.map((job) => (job.id === id ? { ...job, ...patch } : job))
  }

  function beginUploads() {
    uploads.value = uploads.value.filter((job) => job.status === 'uploading' || job.status === 'queued')
  }

  function uploadOne(file: File, overwrite: boolean): Promise<ApiResult> {
    const target = joinPath(path.value, file.name)
    if (!target) return Promise.resolve({ ok: false, status: 400, error: 'invalid_path', state: '' })
    const id = nextId
    nextId += 1
    uploads.value = [...uploads.value, { id, name: file.name, percent: 0, status: 'uploading', error: '' }]
    return new Promise((resolve) => {
      const xhr = new XMLHttpRequest()
      const query = new URLSearchParams({ path: target })
      if (overwrite) query.set('overwrite', '1')
      xhr.open('POST', `/api/files/upload?${query.toString()}`)
      xhr.withCredentials = true
      xhr.setRequestHeader('Content-Type', 'application/octet-stream')
      xhr.setRequestHeader('X-CSRF-Token', csrfToken())
      xhr.upload.onprogress = (event) => {
        if (!event.lengthComputable || event.total === 0) return
        updateJob(id, { percent: Math.min(100, Math.round((event.loaded / event.total) * 100)) })
      }
      xhr.onload = () => {
        let payload: unknown
        try {
          payload = JSON.parse(xhr.responseText) as unknown
        } catch {
          payload = {}
        }
        const problem = readError(payload)
        if (xhr.status === 401) {
          notifyUnauthorized()
          updateJob(id, { status: 'error', error: 'unauthorized' })
          resolve({ ok: false, status: 401, error: 'unauthorized', state: '' })
          return
        }
        if (xhr.status === 409) {
          updateJob(id, { status: 'error', error: 'already_exists' })
          resolve({ ok: false, status: 409, error: 'already_exists', state: '' })
          return
        }
        if (xhr.status === 503 && problem.error === 'node_not_running') {
          applyBlocked(problem.state)
        }
        if (xhr.status < 200 || xhr.status >= 300) {
          const code = problem.error || 'unavailable'
          updateJob(id, { status: 'error', error: code })
          resolve({ ok: false, status: xhr.status, error: code, state: problem.state })
          return
        }
        const entry = parseEntry(payload)
        updateJob(id, { status: 'done', percent: 100, error: '' })
        if (entry) {
          const rest = entries.value.filter((item) => item.name !== entry.name)
          entries.value = [...rest, entry]
        }
        resolve({ ok: true, status: xhr.status, error: '', state: '' })
      }
      xhr.onerror = () => {
        updateJob(id, { status: 'error', error: 'network' })
        resolve({ ok: false, status: 0, error: 'network', state: '' })
      }
      xhr.send(file)
    })
  }

  return {
    path,
    entries,
    localGateway,
    maxBytes,
    status,
    nodeState,
    uploads,
    showPreview,
    load,
    mkdir,
    rename,
    move,
    copy,
    remove,
    beginUploads,
    uploadOne,
  }
})
