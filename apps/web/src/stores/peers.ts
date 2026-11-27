import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiJSON } from '@/lib/api'
import { parseList, type PeerRow } from '@/peers/model'

export type PeersStatus = 'loading' | 'ready' | 'blocked' | 'unreachable' | 'failed'

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
      const result = await apiJSON('/api/peers')
      if (mine !== generation) return
      if (result.status === 401) return
      if (result.status === 0) throw new Error('network')
      if (result.status === 503 && result.error === 'node_not_running') {
        applyBlocked(result.state)
        return
      }
      if (result.status !== 200) {
        if (quiet && peers.value.length > 0) {
          error.value = result.error || 'kubo_error'
          return
        }
        error.value = result.error || ''
        status.value = result.error === 'kubo_error' || result.error === 'timeout' || result.error === 'too_large' ? 'failed' : 'unreachable'
        return
      }
      const parsed = parseList(result.body)
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
      const result = await apiJSON('/api/peers/connect', { method: 'POST', body: { address } })
      if (result.status === 401) return 'unauthorized'
      if (result.status === 0) return 'network'
      if (result.status === 503 && result.error === 'node_not_running') {
        applyBlocked(result.state)
        return 'node_not_running'
      }
      if (result.status !== 200) return result.error || 'connect_failed'
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
