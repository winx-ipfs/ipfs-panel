import { defineStore } from 'pinia'
import { ref } from 'vue'
import { csrfHeaders } from '@/lib/csrf'
import { notifyUnauthorized } from '@/lib/session'
import { parseCatalog, type SceneCatalog } from '@/scenes/model'

export const useScenesStore = defineStore('scenes', () => {
  const catalog = ref<SceneCatalog | null>(null)
  const loading = ref(false)
  const saving = ref(false)

  async function load(): Promise<boolean> {
    loading.value = catalog.value === null
    try {
      const res = await fetch('/api/scenes', { credentials: 'same-origin' })
      if (res.status === 401) {
        notifyUnauthorized()
        return false
      }
      if (!res.ok) return false
      const parsed = parseCatalog(await res.json())
      if (!parsed) return false
      catalog.value = parsed
      return true
    } catch {
      return false
    } finally {
      loading.value = false
    }
  }

  async function apply(id: string): Promise<{ ok: true; restart: string } | { ok: false; code: string }> {
    saving.value = true
    try {
      const res = await fetch('/api/scenes/apply', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', ...csrfHeaders() },
        body: JSON.stringify({ id }),
      })
      if (res.status === 401) {
        notifyUnauthorized()
        return { ok: false, code: 'unauthorized' }
      }
      const body = (await res.json().catch(() => ({}))) as { error?: string; restart?: string }
      if (!res.ok) return { ok: false, code: body.error || 'unknown' }
      await load()
      return { ok: true, restart: body.restart || 'later' }
    } catch {
      return { ok: false, code: 'network' }
    } finally {
      saving.value = false
    }
  }

  async function revert(): Promise<{ ok: true; restart: string } | { ok: false; code: string }> {
    saving.value = true
    try {
      const res = await fetch('/api/scenes/revert', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', ...csrfHeaders() },
        body: '{}',
      })
      if (res.status === 401) {
        notifyUnauthorized()
        return { ok: false, code: 'unauthorized' }
      }
      const body = (await res.json().catch(() => ({}))) as { error?: string; restart?: string }
      if (!res.ok) return { ok: false, code: body.error || 'unknown' }
      await load()
      return { ok: true, restart: body.restart || 'later' }
    } catch {
      return { ok: false, code: 'network' }
    } finally {
      saving.value = false
    }
  }

  return { catalog, loading, saving, load, apply, revert }
})
