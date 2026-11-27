import { defineStore } from 'pinia'
import { ref } from 'vue'
import { addPermanent, applyPermanent, fetchPermanent, removePermanent } from '@/peering/api'
import { type PermanentList, type PermanentPeer, type PeeringSync } from '@/peering/model'

export type PeeringStatus = 'loading' | 'ready' | 'blocked' | 'failed'

export const usePeeringStore = defineStore('peering', () => {
  const peers = ref<PermanentPeer[]>([])
  const sync = ref<PeeringSync>('ok')
  const status = ref<PeeringStatus>('loading')
  const nodeState = ref('')
  const error = ref('')
  const preview = ref(false)
  let generation = 0

  function show(list: PermanentList) {
    preview.value = true
    peers.value = list.peers
    sync.value = list.sync
    error.value = ''
    nodeState.value = ''
    status.value = 'ready'
  }

  function clear() {
    preview.value = true
    peers.value = []
    sync.value = 'ok'
    error.value = ''
    status.value = 'blocked'
    nodeState.value = ''
  }

  function applyList(list: PermanentList) {
    peers.value = list.peers
    sync.value = list.sync
    error.value = ''
    nodeState.value = ''
    status.value = 'ready'
  }

  async function load(quiet = false): Promise<void> {
    if (preview.value) return
    const mine = ++generation
    if (!quiet) {
      error.value = ''
      status.value = 'loading'
    }
    try {
      const result = await fetchPermanent()
      if (mine !== generation) return
      if (result.status === 401) return
      if (result.status === 503 && result.error === 'node_not_running') {
        peers.value = []
        sync.value = 'ok'
        error.value = ''
        status.value = 'blocked'
        nodeState.value = result.state || 'down'
        return
      }
      if (result.status !== 200 || !result.list) {
        if (quiet && peers.value.length > 0) {
          error.value = result.error || 'network'
          return
        }
        error.value = result.error || 'network'
        status.value = 'failed'
        return
      }
      applyList(result.list)
    } catch {
      if (mine !== generation) return
      if (quiet && peers.value.length > 0) {
        error.value = 'network'
        return
      }
      status.value = 'failed'
      error.value = 'network'
    }
  }

  async function mutate(run: () => Promise<{ list: PermanentList | null; error: string; state: string; status: number }>): Promise<string> {
    if (preview.value) return ''
    const result = await run()
    if (result.status === 401) return 'unauthorized'
    if (result.status === 503 && result.error === 'node_not_running') {
      peers.value = []
      sync.value = 'ok'
      status.value = 'blocked'
      nodeState.value = result.state || 'down'
      return 'node_not_running'
    }
    if (result.status !== 200 || !result.list) return result.error || 'kubo_error'
    applyList(result.list)
    if (result.list.sync === 'restart') return ''
    return ''
  }

  function addAddress(address: string) {
    return mutate(() => addPermanent({ address }))
  }

  function addPeer(peerId: string) {
    return mutate(() => addPermanent({ peerId }))
  }

  function remove(peerId: string) {
    return mutate(() => removePermanent(peerId))
  }

  function apply() {
    return mutate(() => applyPermanent())
  }

  return {
    peers,
    sync,
    status,
    nodeState,
    error,
    preview,
    show,
    clear,
    load,
    addAddress,
    addPeer,
    remove,
    apply,
  }
})
