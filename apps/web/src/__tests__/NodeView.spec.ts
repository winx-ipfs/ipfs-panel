// @vitest-environment happy-dom
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createI18n } from 'vue-i18n'
import { en } from '@/locales/en'
import { zh } from '@/locales/zh'
import type { KuboStatus } from '@/kubo/model'
import type { NodeOverview } from '@/node/types'
import NodeView from '@/views/NodeView.vue'

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

const pathKubo: KuboStatus = {
  source: 'path',
  version: '0.43.1',
  pinned: '0.43.1',
  supported: '>=0.43.1 <0.45.0',
  latest: '0.43.1',
  upstream: '0.43.1',
  upgrade: false,
  check: 'ok',
  job: null,
}

function overview(patch: Partial<NodeOverview> = {}): NodeOverview {
  return {
    state: 'down',
    peerId: null,
    version: null,
    repoPath: '/srv/ipfs-panel/kubo',
    error: null,
    ...patch,
  }
}

function plugins(locale: 'en' | 'zh' = 'en') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'node', component: NodeView },
      { path: '/peers', name: 'peers', component: { template: '<div />' } },
      { path: '/pins', name: 'pins', component: { template: '<div />' } },
    ],
  })
  return [
    createPinia(),
    router,
    createI18n({ legacy: false, locale, messages: { en, zh } }),
  ]
}

function mountCard(
  body: NodeOverview | null,
  locale: 'en' | 'zh' = 'en',
  fail = false,
  kubo: KuboStatus | null = pathKubo,
) {
  const fetchMock = vi.fn().mockImplementation((url: string) => {
    if (fail) return Promise.reject(new Error('offline'))
    if (String(url).startsWith('/api/kubo')) {
      return kubo ? jsonResponse(kubo) : Promise.reject(new Error('offline'))
    }
    if (!body) return Promise.reject(new Error('offline'))
    return jsonResponse(body)
  })
  vi.stubGlobal('fetch', fetchMock)
  const wrapper = mount(NodeView, {
    global: { plugins: plugins(locale) },
  })
  return { wrapper, fetchMock }
}

describe('NodeView', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('shows the stopped sentence and only Start', async () => {
    const { wrapper } = mountCard(overview())
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('The node is stopped.')
    expect(wrapper.findAll('[data-node-action]').map((button) => button.text())).toEqual(['Start'])
    expect(wrapper.text()).toContain('Node folder')
    expect(wrapper.text()).toContain('/srv/ipfs-panel/kubo')
    expect(wrapper.text()).not.toContain('5001')
    wrapper.unmount()
  })

  it('shows version, peer id, Stop, and a quieter Restart while running', async () => {
    const { wrapper } = mountCard(
      overview({
        state: 'up',
        peerId: '12D3KooWPeer',
        version: 'kubo/0.32.1/',
        activity: { peers: 4, rateIn: 2048, rateOut: 0 },
      }),
    )
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('The node is running.')
    expect(wrapper.text()).toContain('Peers')
    expect(wrapper.text()).toContain('4')
    expect(wrapper.text()).toContain('In 2.0 KB/s')
    expect(wrapper.text()).toContain('Version')
    expect(wrapper.text()).toContain('kubo/0.32.1/')
    expect(wrapper.get('.peer').text()).toBe('12D3KooWPeer')
    const buttons = wrapper.findAll('[data-node-action]')
    expect(buttons.map((button) => button.text())).toEqual(['Stop', 'Restart'])
    expect(buttons[1]?.classes()).toContain('quiet')
    expect(wrapper.find('[data-copy="peer"]').exists()).toBe(true)
    expect(wrapper.find('[data-copy="version"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('hides buttons while starting and for a busy folder', async () => {
    const starting = mountCard(overview({ state: 'starting' }))
    await flushPromises()
    expect(starting.wrapper.get('[role="status"]').text()).toBe('The node is starting.')
    expect(starting.wrapper.findAll('[data-node-action]')).toHaveLength(0)
    starting.wrapper.unmount()

    const busy = mountCard(
      overview({ state: 'failed', error: { code: 'repo_busy', detail: 'held' } }),
    )
    await flushPromises()
    expect(busy.wrapper.get('[role="status"]').text()).toContain('Another Kubo')
    expect(busy.wrapper.findAll('[data-node-action]')).toHaveLength(0)
    expect(busy.wrapper.get('pre').text()).toBe('held')
    busy.wrapper.unmount()
  })

  it('does not replace a running node with the stopped sentence when the request fails', async () => {
    let nodeCalls = 0
    const running = overview({ state: 'up', peerId: '12D3KooWPeer', version: 'kubo/1/' })
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (String(url).startsWith('/api/kubo')) return jsonResponse(pathKubo)
      nodeCalls += 1
      if (nodeCalls === 1) return jsonResponse(running)
      return Promise.reject(new Error('offline'))
    })
    vi.stubGlobal('fetch', fetchMock)
    vi.useFakeTimers()
    const wrapper = mount(NodeView, {
      global: { plugins: plugins() },
    })
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('The node is running.')
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe(
      'The panel cannot reach the manager on this machine.',
    )
    expect(wrapper.text()).not.toContain('The node is stopped.')
    expect(wrapper.text()).toContain('12D3KooWPeer')
    expect(wrapper.findAll('[data-node-action]').map((button) => button.text())).toEqual(['Retry'])
    wrapper.unmount()
    vi.useRealTimers()
  })

  it('offers to install Kubo when the machine has none', async () => {
    const { wrapper, fetchMock } = mountCard(overview({ state: 'down' }), 'en', false, {
      ...pathKubo,
      source: 'missing',
      version: '',
      latest: '',
      upstream: '',
    })
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('The node is stopped.')
    expect(wrapper.text()).toContain('This panel can download Kubo 0.43.1 and start the node.')
    expect(wrapper.findAll('[data-node-action]').map((button) => button.text())).toEqual(['Install Kubo'])
    const link = wrapper.get('a[href="https://docs.ipfs.tech/install/command-line/"]')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.text()).toBe('Install Kubo yourself')
    await wrapper.get('[data-node-action="install"]').trigger('click')
    await flushPromises()
    const install = fetchMock.mock.calls.find((call) => call[0] === '/api/kubo/install')
    expect(install?.[1]).toMatchObject({ method: 'POST', body: '{}' })
    wrapper.unmount()
  })

  it('shows download progress and a checksum failure', async () => {
    const { wrapper } = mountCard(overview({ state: 'down' }), 'en', false, {
      ...pathKubo,
      source: 'missing',
      version: '',
      job: {
        id: 'job',
        action: 'install',
        state: 'running',
        phase: 'downloading',
        version: '0.43.1',
        received: 1024,
        total: 4096,
        error: '',
      },
    })
    await flushPromises()
    const bar = wrapper.get('progress')
    expect(bar.attributes('value')).toBe('1024')
    expect(bar.attributes('max')).toBe('4096')
    expect(wrapper.text()).toContain('Downloading Kubo 0.43.1.')
    expect(wrapper.get('[data-node-action="install"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()

    const failed = mountCard(overview({ state: 'down' }), 'en', false, {
      ...pathKubo,
      source: 'missing',
      version: '',
      job: {
        id: 'job',
        action: 'install',
        state: 'failed',
        phase: 'verifying',
        version: '0.43.1',
        received: 4096,
        total: 4096,
        error: 'verify_failed',
      },
    })
    await flushPromises()
    expect(failed.wrapper.text()).toContain('The download did not match the published checksum. Nothing was installed.')
    expect(failed.wrapper.find('progress').exists()).toBe(false)
    failed.wrapper.unmount()
  })

  it('shows an upgrade while the node is stopped, and a rollback', async () => {
    const stopped = mountCard(overview({ state: 'down' }), 'en', false, {
      ...pathKubo,
      source: 'managed',
      version: '0.43.1',
      latest: '0.44.0',
      upstream: '0.44.0',
      upgrade: true,
    })
    await flushPromises()
    expect(stopped.wrapper.get('[role="status"]').text()).toBe('The node is stopped.')
    expect(stopped.wrapper.get('[data-kubo-version]').text()).toContain('Kubo 0.43.1.')
    expect(stopped.wrapper.get('[data-kubo-version]').text()).toContain('A newer Kubo, 0.44.0, is available.')
    expect(stopped.wrapper.get('[data-node-action="upgrade"]').text()).toBe('Upgrade')
    expect(stopped.wrapper.get('[data-node-action="start"]').text()).toBe('Start')
    stopped.wrapper.unmount()

    const rolling = mountCard(overview({ state: 'down' }), 'en', false, {
      ...pathKubo,
      source: 'managed',
      version: '0.43.1',
      upgrade: false,
      job: {
        id: 'job',
        action: 'upgrade',
        state: 'rolled_back',
        phase: 'rolling_back',
        version: '0.44.0',
        received: 10,
        total: 10,
        error: '',
      },
    })
    await flushPromises()
    expect(rolling.wrapper.text()).toContain('The new Kubo did not start. The previous version is in place.')
    rolling.wrapper.unmount()
  })

  it('names a chosen binary and does not offer to replace it', async () => {
    const { wrapper } = mountCard(
      overview({
        state: 'failed',
        error: { code: 'binary_not_found', detail: 'missing' },
      }),
      'en',
      false,
      { ...pathKubo, source: 'flag', version: '' },
    )
    await flushPromises()
    expect(wrapper.get('code').text()).toBe('ipfs')
    expect(wrapper.text()).toContain('Using the Kubo you chose. This panel will not replace it.')
    expect(wrapper.find('[data-node-action="install"]').exists()).toBe(false)
    expect(wrapper.find('[data-node-action="upgrade"]').exists()).toBe(false)
    expect(wrapper.get('[data-node-action="retry"]').text()).toBe('Retry')
    wrapper.unmount()
  })

  it('shows the Chinese sentence', async () => {
    const { wrapper } = mountCard(overview(), 'zh')
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toBe('节点已停止。')
    expect(wrapper.get('[data-node-action="start"]').text()).toBe('启动')
    wrapper.unmount()
  })

  it('posts start and disables the button while the request is in flight', async () => {
    let release: (value: Response) => void = () => {}
    const pending = new Promise<Response>((resolve) => {
      release = resolve
    })
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (String(url).startsWith('/api/kubo')) return jsonResponse(pathKubo)
      if (url === '/api/node/start') return pending
      return jsonResponse(overview())
    })
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mount(NodeView, {
      global: { plugins: plugins() },
    })
    await flushPromises()
    const pendingClick = wrapper.get('[data-node-action="start"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-node-action="start"]').attributes('disabled')).toBeDefined()
    // Assert on the start request itself, not on it being the last call.
    // Once the node reports up, watch(connected) loads companions
    // (/api/peers, /api/pins) and those can land after this POST.
    const startCall = fetchMock.mock.calls.find((call) => call[0] === '/api/node/start')
    expect(startCall, 'the Start button must POST /api/node/start').toBeDefined()
    expect(startCall?.[1]).toEqual({
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': '' },
      body: '{}',
    })
    expect(
      fetchMock.mock.calls.filter((call) => call[0] === '/api/node/start'),
      'start must be posted exactly once',
    ).toHaveLength(1)
    release(
      new Response(JSON.stringify(overview({ state: 'starting' })), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    await pendingClick
    await flushPromises()
    expect(wrapper.findAll('[data-node-action]')).toHaveLength(0)
    wrapper.unmount()
  })
})

describe('NodeView start assertion survives companion loads', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('still detects the start POST when a companion request lands after it', async () => {
    const calls: string[] = []
    let startResolve: (value: Response) => void = () => {}
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      const target = String(url)
      calls.push(target)
      if (target.startsWith('/api/kubo')) return jsonResponse(pathKubo)
      if (target === '/api/node') {
        return Promise.resolve(
          new Response(JSON.stringify(overview()), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          }),
        )
      }
      if (target === '/api/node/start') {
        return new Promise<Response>((resolve) => {
          startResolve = resolve
        })
      }
      // /api/peers and /api/pins: the companions that fired the CI flake
      if (target === '/api/peers' || target === '/api/pins') {
        return Promise.resolve(
          new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } }),
        )
      }
      return Promise.resolve(new Response('{}', { status: 200 }))
    })
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mount(NodeView, { global: { plugins: plugins() } })
    await flushPromises()

    void wrapper.get('[data-node-action="start"]').trigger('click')
    await flushPromises()

    // The daemon comes up, so watch(connected) loads companions afterwards.
    startResolve(
      new Response(JSON.stringify(overview({ state: 'up', peerId: 'p', version: 'kubo/1/' })), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    await flushPromises()
    await flushPromises()

    // Deliberately assert from the flaky position: the start POST is no longer last.
    expect(calls[calls.length - 1]).not.toBe('/api/node/start')
    const startCall = fetchMock.mock.calls.find((call) => call[0] === '/api/node/start')
    expect(startCall?.[1]).toEqual({
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': '' },
      body: '{}',
    })
    wrapper.unmount()
  })
})
