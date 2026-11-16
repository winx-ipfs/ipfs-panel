import { defineStore } from 'pinia'
import { ref } from 'vue'
import { parseDocument, parseResult, type ConfigDocument, type ConfigResult } from '@/config/model'
import { csrfHeaders } from '@/lib/csrf'
import { notifyUnauthorized } from '@/lib/session'

export type ConfigFailure = { ok: false; code: string; detail: string }

async function readBody(res: Response): Promise<Record<string, unknown>> {
  try {
    const body = (await res.json()) as unknown
    return body && typeof body === 'object' ? (body as Record<string, unknown>) : {}
  } catch {
    return {}
  }
}

function failure(body: Record<string, unknown>, fallback: string): ConfigFailure {
  return {
    ok: false,
    code: typeof body.error === 'string' ? body.error : fallback,
    detail: typeof body.detail === 'string' ? body.detail : '',
  }
}

export const useConfigStore = defineStore('config', () => {
  const saving = ref(false)

  async function load(): Promise<{ ok: true; doc: ConfigDocument } | ConfigFailure> {
    try {
      const res = await fetch('/api/config', { credentials: 'same-origin' })
      if (res.status === 401) {
        notifyUnauthorized()
        return { ok: false, code: 'unauthorized', detail: '' }
      }
      const body = await readBody(res)
      if (!res.ok) return failure(body, 'unavailable')
      const doc = parseDocument(body)
      if (!doc) return { ok: false, code: 'unavailable', detail: '' }
      return { ok: true, doc }
    } catch {
      return { ok: false, code: 'network', detail: '' }
    }
  }

  async function save(text: string): Promise<{ ok: true; doc: ConfigResult } | ConfigFailure> {
    return write('/api/config', { text })
  }

  async function restore(): Promise<{ ok: true; doc: ConfigResult } | ConfigFailure> {
    return write('/api/config/restore', {})
  }

  async function write(path: string, payload: Record<string, unknown>): Promise<{ ok: true; doc: ConfigResult } | ConfigFailure> {
    saving.value = true
    try {
      const res = await fetch(path, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', ...csrfHeaders() },
        body: JSON.stringify(payload),
      })
      if (res.status === 401) {
        notifyUnauthorized()
        return { ok: false, code: 'unauthorized', detail: '' }
      }
      const body = await readBody(res)
      if (!res.ok) return failure(body, 'unavailable')
      const doc = parseResult(body)
      if (!doc) return { ok: false, code: 'unavailable', detail: '' }
      return { ok: true, doc }
    } catch {
      return { ok: false, code: 'network', detail: '' }
    } finally {
      saving.value = false
    }
  }

  return { saving, load, save, restore }
})
