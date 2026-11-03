// @vitest-environment happy-dom
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setCSRFToken } from '@/lib/csrf'
import { onUnauthorized } from '@/lib/session'
import { useFilesStore } from '@/stores/files'

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('files store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    setCSRFToken('token')
    vi.unstubAllGlobals()
  })

  it('loads a folder and reports a stopped node', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(json({
        path: '/',
        localGateway: 'http://127.0.0.1:8081',
        maxBytes: 1024,
        entries: [{ name: 'notes.txt', type: 'file', size: 5, cid: 'bafy', modified: null }],
      }))
      .mockResolvedValueOnce(json({ error: 'node_not_running', state: 'starting' }, 503))
    vi.stubGlobal('fetch', fetchMock)
    const store = useFilesStore()
    expect(await store.load('/')).toBe(true)
    expect(store.entries.map((entry) => entry.name)).toEqual(['notes.txt'])
    expect(store.localGateway).toBe('http://127.0.0.1:8081')
    expect(await store.load('/')).toBe(false)
    expect(store.status).toBe('blocked')
    expect(store.nodeState).toBe('starting')
  })

  it('sends mkdir with the session header and handles 401', async () => {
    const unauthorized = vi.fn()
    onUnauthorized(unauthorized)
    const fetchMock = vi.fn().mockResolvedValue(json({ error: 'unauthorized' }, 401))
    vi.stubGlobal('fetch', fetchMock)
    const store = useFilesStore()
    store.path = '/photos'
    const result = await store.mkdir('trip')
    expect(result.ok).toBe(false)
    expect(unauthorized).toHaveBeenCalled()
    expect(fetchMock).toHaveBeenCalledWith('/api/files/mkdir', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ path: '/photos/trip' }),
    }))
    const headers = fetchMock.mock.calls[0]?.[1].headers as Record<string, string>
    expect(headers['X-CSRF-Token']).toBe('token')
  })

  it('reports upload progress and a name conflict', async () => {
    class FakeXHR {
      status = 409
      responseText = JSON.stringify({ error: 'already_exists' })
      withCredentials = false
      upload = { onprogress: null as ((event: ProgressEvent) => void) | null }
      onload: (() => void) | null = null
      onerror: (() => void) | null = null
      open = vi.fn()
      setRequestHeader = vi.fn()
      send = vi.fn(() => {
        this.upload.onprogress?.({ lengthComputable: true, loaded: 1, total: 2 } as ProgressEvent)
        this.status = 409
        this.onload?.()
      })
    }
    vi.stubGlobal('XMLHttpRequest', FakeXHR)
    const store = useFilesStore()
    const file = new File(['hi'], 'notes.txt', { type: 'text/plain' })
    const result = await store.uploadOne(file, false)
    expect(result.status).toBe(409)
    expect(store.uploads[0]?.percent).toBe(50)
    expect(store.uploads[0]?.error).toBe('already_exists')
  })
})
