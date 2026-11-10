import { defineStore } from 'pinia'
import { ref } from 'vue'
import { parseView, type ExploreView } from '@/explore/model'
import { notifyUnauthorized } from '@/lib/session'

export type ExploreStatus = 'idle' | 'loading' | 'ready' | 'blocked' | 'unreachable'

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

export const useExploreStore = defineStore('explore', () => {
  const view = ref<ExploreView | null>(null)
  const status = ref<ExploreStatus>('idle')
  const nodeState = ref('')
  const error = ref('')
  const preview = ref(false)
  let generation = 0

  function showPreview(next: ExploreView | null) {
    preview.value = true
    view.value = next
    error.value = ''
    nodeState.value = ''
    status.value = next ? 'ready' : 'idle'
  }

  function showBlocked(state: string) {
    preview.value = true
    view.value = null
    error.value = ''
    status.value = 'blocked'
    nodeState.value = state || 'down'
  }

  function idle() {
    if (preview.value) return
    view.value = null
    error.value = ''
    nodeState.value = ''
    status.value = 'idle'
  }

  async function lookup(target: string): Promise<void> {
    if (preview.value) return
    const mine = ++generation
    error.value = ''
    status.value = 'loading'
    view.value = null
    try {
      const res = await fetch(`/api/explore?target=${encodeURIComponent(target)}`, { credentials: 'same-origin' })
      if (mine !== generation) return
      if (res.status === 401) {
        notifyUnauthorized()
        return
      }
      const body = await readJSON(res)
      if (mine !== generation) return
      const problem = readError(body)
      if (res.status === 503 && problem.error === 'node_not_running') {
        status.value = 'blocked'
        nodeState.value = problem.state || 'down'
        return
      }
      if (!res.ok) {
        status.value = res.status === 0 ? 'unreachable' : 'idle'
        if (res.status >= 500 && problem.error !== 'timeout' && problem.error !== 'kubo_error' && problem.error !== 'too_large') {
          status.value = 'unreachable'
        }
        error.value = problem.error || 'kubo_error'
        if (status.value === 'unreachable') error.value = ''
        return
      }
      const parsed = parseView(body)
      if (!parsed) {
        status.value = 'unreachable'
        return
      }
      view.value = parsed
      status.value = 'ready'
      nodeState.value = ''
    } catch {
      status.value = 'unreachable'
      error.value = ''
    }
  }

  return { view, status, nodeState, error, preview, showPreview, showBlocked, idle, lookup }
})
