import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { NodeOverview, NodeState } from '@/node/types'

const states: readonly NodeState[] = ['down', 'starting', 'up', 'stopping', 'failed']

function isState(value: unknown): value is NodeState {
  return typeof value === 'string' && states.includes(value as NodeState)
}

export function parseOverview(value: unknown): NodeOverview | null {
  if (!value || typeof value !== 'object') return null
  const body = value as Record<string, unknown>
  if (!isState(body.state)) return null
  let error: NodeOverview['error'] = null
  if (body.error != null) {
    if (typeof body.error !== 'object') return null
    const raw = body.error as Record<string, unknown>
    if (typeof raw.code !== 'string') return null
    error = { code: raw.code, detail: typeof raw.detail === 'string' ? raw.detail : '' }
  }
  return {
    state: body.state,
    peerId: typeof body.peerId === 'string' ? body.peerId : null,
    version: typeof body.version === 'string' ? body.version : null,
    repoPath: typeof body.repoPath === 'string' ? body.repoPath : '',
    error,
  }
}

export const useNodeStore = defineStore('node', () => {
  const overview = ref<NodeOverview | null>(null)
  const unreachable = ref(false)
  const acting = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null
  let session = 0

  function delay(): number {
    if (unreachable.value) return 2000
    const state = overview.value?.state
    if (state === 'starting' || state === 'stopping') return 1000
    return 5000
  }

  function clearTimer() {
    if (timer) clearTimeout(timer)
    timer = null
  }

  function arm() {
    clearTimer()
    const mine = session
    timer = setTimeout(() => {
      if (mine === session) void refresh()
    }, delay())
  }

  async function refresh(): Promise<void> {
    const mine = session
    clearTimer()
    try {
      const res = await fetch('/api/node')
      if (!res.ok) throw new Error(String(res.status))
      const parsed = parseOverview(await res.json())
      if (!parsed) throw new Error('bad overview')
      if (mine !== session) return
      overview.value = parsed
      unreachable.value = false
    } catch {
      if (mine !== session) return
      unreachable.value = true
    } finally {
      if (mine === session) arm()
    }
  }

  async function send(path: string): Promise<void> {
    const mine = session
    acting.value = true
    clearTimer()
    try {
      const res = await fetch(path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      })
      if (!res.ok) throw new Error(String(res.status))
      const parsed = parseOverview(await res.json())
      if (!parsed) throw new Error('bad overview')
      if (mine !== session) return
      overview.value = parsed
      unreachable.value = false
    } catch {
      if (mine !== session) return
      unreachable.value = true
    } finally {
      acting.value = false
      if (mine === session) arm()
    }
  }

  function startPolling(): Promise<void> {
    session += 1
    clearTimer()
    return refresh()
  }

  function stopPolling() {
    session += 1
    clearTimer()
    acting.value = false
  }

  function start() {
    return send('/api/node/start')
  }

  function stop() {
    return send('/api/node/stop')
  }

  function restart() {
    return send('/api/node/restart')
  }

  function retry() {
    return refresh()
  }

  return {
    overview,
    unreachable,
    acting,
    startPolling,
    stopPolling,
    refresh,
    start,
    stop,
    restart,
    retry,
  }
})
