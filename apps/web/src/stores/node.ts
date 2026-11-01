import { defineStore } from 'pinia'
import { ref } from 'vue'
import { csrfHeaders } from '@/lib/csrf'
import { notifyUnauthorized } from '@/lib/session'
import type { NodeOverview, NodeState } from '@/node/types'

const devMockEnabled = import.meta.env.DEV

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

  async function mocked(): Promise<boolean> {
    if (!devMockEnabled || typeof window === 'undefined') return false
    if (!new URLSearchParams(window.location.search).has('mock')) return false
    const { devMockFromLocation } = await import('@/dev/mockNode')
    const next = devMockFromLocation(window.location.search)
    if (!next) return false
    overview.value = next.overview
    unreachable.value = next.unreachable
    return true
  }

  async function refresh(): Promise<void> {
    const mine = session
    clearTimer()
    if (await mocked()) {
      if (mine === session) arm()
      return
    }
    let unauthorized = false
    try {
      const res = await fetch('/api/node', { credentials: 'same-origin' })
      if (res.status === 401) {
        unauthorized = true
        notifyUnauthorized()
        return
      }
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
      if (mine === session && !unauthorized) arm()
    }
  }

  async function send(path: string): Promise<void> {
    const mine = session
    acting.value = true
    clearTimer()
    if (await mocked()) {
      acting.value = false
      if (mine === session) arm()
      return
    }
    let unauthorized = false
    try {
      const res = await fetch(path, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', ...csrfHeaders() },
        body: '{}',
      })
      if (res.status === 401) {
        unauthorized = true
        notifyUnauthorized()
        return
      }
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
      if (mine === session && !unauthorized) arm()
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
