import { defineStore } from 'pinia'
import { ref } from 'vue'
import { csrfHeaders } from '@/lib/csrf'
import { notifyUnauthorized } from '@/lib/session'
import { parseList, type PeerRow } from '@/peers/model'

export type PeersStatus = 'loading' | 'ready' | 'blocked' | 'unreachable' | 'failed'

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

export const usePeersStore = defineStore('peers', () => {
  const peers = ref<PeerRow[]>([])
  const total = ref(0)
  const truncated = ref(false)
  const status = ref<PeersStatus>('loading')
  const nodeState = ref('')
  const error = ref('')
  const preview = ref(false)
  let generation = 0

  function showList(list: PeerRow[], nextTotal = list.length, nextTruncated = false) {
    preview.value = true
    peers.value = list
    total.value = nextTotal
    truncated.value = nextTruncated
    error.value = ''
    nodeState.value = ''
    status.value = 'ready'
  }

  function showBlocked(state: string) {
    preview.value = true
    peers.value = []
    total.value = 0
    truncated.value = false
    error.value = ''
    status.value = 'blocked'
    nodeState.value = state || 'down'
  }

  function applyBlocked(state: string) {
    peers.value = []
    total.value = 0
    truncated.value = false
    error.value = ''
    status.value = 'blocked'
    nodeState.value = state || 'down'
  }

  async function load(quiet = false): Promise<void> {
    if (preview.value) return
    const mine = ++generation
    if (!quiet) {
      error.value = ''
      status.value = 'loading'
    }
    try {
      const res = await fetch('/api/peers', { credentials: 'same-origin' })
      if (mine !== generation) return
      if (res.status === 401) {
        notifyUnauthorized()
        return
      }
      const body = await readJSON(res)
      if (mine !== generation) return
      const problem = readError(body)
      if (res.status === 503 && problem.error === 'node_not_running') {
        applyBlocked(problem.state)
        return
      }
      if (!res.ok) {
        if (quiet && peers.value.length > 0) {
          error.value = problem.error || 'kubo_error'
          return
        }
        error.value = problem.error || ''
        status.value = problem.error === 'kubo_error' || problem.error === 'timeout' || problem.error === 'too_large' ? 'failed' : 'unreachable'
        return
      }
      const parsed = parseList(body)
      if (!parsed) {
        status.value = quiet && peers.value.length > 0 ? status.value : 'unreachable'
        return
      }
      peers.value = parsed.peers
      total.value = parsed.total
      truncated.value = parsed.truncated
      error.value = ''
      nodeState.value = ''
      status.value = 'ready'
    } catch {
      if (mine !== generation) return
      if (quiet && peers.value.length > 0) {
        error.value = 'network'
        return
      }
      status.value = 'unreachable'
      error.value = ''
    }
  }

  async function connect(address: string): Promise<string> {
    if (preview.value) return ''
    try {
      const res = await fetch('/api/peers/connect', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', ...csrfHeaders() },
        body: JSON.stringify({ address }),
      })
      if (res.status === 401) {
        notifyUnauthorized()
        return 'unauthorized'
      }
      const body = await readJSON(res)
      const problem = readError(body)
      if (res.status === 503 && problem.error === 'node_not_running') {
        applyBlocked(problem.state)
        return 'node_not_running'
      }
      if (!res.ok) return problem.error || 'connect_failed'
      await load(true)
      return ''
    } catch {
      return 'network'
    }
  }

  return {
    peers,
    total,
    truncated,
    status,
    nodeState,
    error,
    preview,
    showList,
    showBlocked,
    load,
    connect,
  }
})
