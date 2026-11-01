import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { inlineParts, shortenMiddle } from '@/lib/inline'
import { onUnauthorized } from '@/lib/session'
import { setCSRFToken } from '@/lib/csrf'
import { sentenceKey, visibleActions } from '@/node/present'
import type { NodeOverview } from '@/node/types'
import { useNodeStore } from '@/stores/node'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'

function overview(patch: Partial<NodeOverview> = {}): NodeOverview {
  return {
    state: 'down',
    peerId: null,
    version: null,
    repoPath: '/home/me/.local/share/ipfs-panel/kubo',
    error: null,
    ...patch,
  }
}

describe('inline code markers', () => {
  it('turns backtick pairs into code parts and leaves detail-like text alone', () => {
    expect(inlineParts('find the `ipfs` command')).toEqual([
      { code: false, text: 'find the ' },
      { code: true, text: 'ipfs' },
      { code: false, text: ' command' },
    ])
    expect(inlineParts('no markers')).toEqual([{ code: false, text: 'no markers' }])
    expect(shortenMiddle('12D3KooWRjS6k8iG8m3mS5kQ8o8n8Yw8vQd7pQh2oE4mN1aB2cD')).toBe(
      '12D3KooWRj…mN1aB2cD',
    )
    expect(shortenMiddle('12D3')).toBe('12D3')
  })
})

describe('node sentences and buttons', () => {
  it('covers every situation in the design', () => {
    expect(sentenceKey(overview(), false)).toBe('nodeStopped')
    expect(visibleActions(overview(), false)).toEqual(['start'])

    const forced = overview({ error: { code: 'stop_timeout', detail: 'killed' } })
    expect(sentenceKey(forced, false)).toBe('nodeStopForced')
    expect(visibleActions(forced, false)).toEqual(['start'])

    const starting = overview({ state: 'starting' })
    expect(sentenceKey(starting, false)).toBe('nodeStarting')
    expect(visibleActions(starting, false)).toEqual([])

    const up = overview({ state: 'up', peerId: '12D3', version: 'kubo/0.0.0/' })
    expect(sentenceKey(up, false)).toBe('nodeRunning')
    expect(visibleActions(up, false)).toEqual(['stop', 'restart'])

    const stopping = overview({ state: 'stopping' })
    expect(sentenceKey(stopping, false)).toBe('nodeStopping')
    expect(visibleActions(stopping, false)).toEqual([])

    const codes: Array<[string, string]> = [
      ['binary_not_found', 'nodeErrorBinary'],
      ['repo_not_writable', 'nodeErrorRepoWrite'],
      ['init_failed', 'nodeErrorInit'],
      ['api_not_loopback', 'nodeErrorApiPublic'],
      ['gateway_not_loopback', 'nodeErrorGatewayPublic'],
      ['panel_port_conflict', 'nodeErrorPanelPort'],
      ['port_in_use', 'nodeErrorPortInUse'],
      ['repo_busy', 'nodeErrorRepoBusy'],
      ['daemon_exited', 'nodeErrorDaemonExited'],
      ['health_timeout', 'nodeErrorHealthTimeout'],
      ['unknown', 'nodeErrorUnknown'],
      ['something_else', 'nodeErrorUnknown'],
    ]
    for (const [code, key] of codes) {
      const failed = overview({ state: 'failed', error: { code, detail: 'x' } })
      expect(sentenceKey(failed, false)).toBe(key)
      const actions =
        code === 'repo_busy' ? [] : code === 'binary_not_found' ? ['retry'] : ['start']
      expect(visibleActions(failed, false)).toEqual(actions)
    }

    expect(sentenceKey(overview({ state: 'down' }), true)).toBe('nodeUnreachable')
    expect(visibleActions(overview({ state: 'up' }), true)).toEqual(['retry'])
    expect(sentenceKey(null, true)).toBe('nodeUnreachable')
    expect(sentenceKey(null, false)).toBeNull()
    expect(visibleActions(null, false)).toEqual([])
  })

  it('keeps the designed English and Chinese sentences', () => {
    expect(en.navNode).toBe('Node')
    expect(zh.navNode).toBe('节点')
    expect(en.nodeStopped).toBe('The node is stopped.')
    expect(zh.nodeStopped).toBe('节点已停止。')
    expect(en.nodeStopForced).toBe(
      'The node is stopped. It did not exit on its own, so the panel forced it to stop.',
    )
    expect(zh.nodeStopForced).toBe('节点已停止。它没有自己退出，面板强制把它停掉了。')
    expect(en.nodeRunning).toBe('The node is running.')
    expect(zh.nodeRunning).toBe('节点正在运行。')
    expect(en.nodeUnreachable).toBe('The panel cannot reach the manager on this machine.')
    expect(zh.nodeUnreachable).toBe('面板连不上这台机器上的管理服务。')
    expect(en.nodeErrorRepoBusy).toBe(
      'Another Kubo is already using this folder. The panel will not stop that one.',
    )
    expect(zh.nodeErrorRepoBusy).toBe('已经有另一个 Kubo 在用这个文件夹。面板不会去停它。')
  })
})

describe('node store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
  })

  it('polls faster while starting and keeps the last snapshot when the manager is unreachable', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(json(overview({ state: 'starting' })))
      .mockResolvedValueOnce(json(overview({ state: 'up', peerId: 'peer', version: 'kubo/x/' })))
      .mockRejectedValueOnce(new Error('offline'))
    vi.stubGlobal('fetch', fetchMock)
    const store = useNodeStore()

    await store.startPolling()
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/node')
    expect(store.unreachable).toBe(false)
    expect(store.overview?.state).toBe('starting')

    await vi.advanceTimersByTimeAsync(999)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(store.overview?.state).toBe('up')

    await vi.advanceTimersByTimeAsync(4999)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(1)
    expect(store.unreachable).toBe(true)
    expect(store.overview?.state).toBe('up')
    expect(store.overview?.peerId).toBe('peer')

    store.stopPolling()
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('posts JSON to the relative action routes', async () => {
    const fetchMock = vi.fn().mockResolvedValue(json(overview({ state: 'starting' })))
    vi.stubGlobal('fetch', fetchMock)
    const store = useNodeStore()
    setCSRFToken('csrf-test')
    await store.start()
    expect(fetchMock).toHaveBeenCalledWith('/api/node/start', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': 'csrf-test' },
      body: '{}',
    })
    store.stopPolling()
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('treats 401 as signed out and stops polling', async () => {
    const seen = vi.fn()
    const stop = onUnauthorized(seen)
    const fetchMock = vi.fn().mockResolvedValue(new Response('{"error":"unauthorized"}', { status: 401 }))
    vi.stubGlobal('fetch', fetchMock)
    const store = useNodeStore()
    await store.startPolling()
    expect(store.unreachable).toBe(false)
    expect(store.overview).toBeNull()
    expect(seen).toHaveBeenCalledOnce()
    await vi.advanceTimersByTimeAsync(10000)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    stop()
    store.stopPolling()
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })
})

function json(body: NodeOverview): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}
